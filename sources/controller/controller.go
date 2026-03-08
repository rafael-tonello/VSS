package controller

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"rtonello/vss/sources/misc"
	"rtonello/vss/sources/misc/confs"
	"rtonello/vss/sources/misc/logger"
	"rtonello/vss/sources/services/apis"
	"rtonello/vss/sources/services/storage"
)

type IController interface {
	// SetVar creates or updates a variable. It returns an error if the variable
	// is internal (starts with '_') or locked.
	SetVar(name string, value misc.DynamicVar) chan error

	// GetVars returns a channel that will receive a GetVarsResult containing
	// the slice of tuples (name, value) and an error if one occurred. Work runs
	// in a background goroutine.
	GetVars(name string, defaultValue misc.DynamicVar) chan GetVarsResult

	DelVar(name string) chan error

	GetChildsOfVar(parentName string) chan []string

	// LockVar tries to acquire a lock for a variable with a timeout in milliseconds.
	LockVar(varName string, timeoutMs uint) chan error

	// UnlockVar releases the lock for a variable.
	UnlockVar(varName string) chan error

	// IsVarLocked checks if a variable is locked.
	IsVarLocked(varName string) chan bool

	// ObserveVar registers a client observation of a variable and sends the
	// current value to the client via the API. If the client is disconnected,
	// the controller will schedule a liveness check.
	ObserveVar(varName, clientId, customIdsAndMetainfo string, api apis.IApi)

	// StopObservingVar removes a client's observation for a variable.
	StopObservingVar(varName, clientId, customIdsAndMetainfo string, api apis.IApi)

	// ApiStarted registers a new API implementation with the controller.
	ApiStarted(api apis.IApi)

	// ClientConnected registers a client and updates it with currently observed vars.
	ClientConnected(clientId string, api apis.IApi) (string, int)

	// GetSystemVersion returns the configured system version string.
	GetSystemVersion() string
}

// GetVarsResult is the returned value from GetVars: Values is the list of
// (name,value) tuples, Err is non-nil when an error occurred.
type GetVarsResult struct {
	Values []misc.Tuple[misc.DynamicVar]
	Err    error
}

// TheController is a Go port of the C++ TheController class. It coordinates
// variable operations and client notifications using ControllerVarHelper and
// ControllerClientHelper.
type TheController struct {
	log                        logger.ILogger
	confs                      confs.IConfs
	db                         storage.IStorage
	apis                       map[string]apis.IApi
	maxTimeWaitingClientSecond int64
	systemVersion              string
	maxKeyLength               int
	maxKeyWordLength           int
	maxValueSize               int
	allowRawDbAccess           bool
}

// ControllerClientHelperApi is the minimal API used by controller client helper.
// It mirrors the small apis.IApi used in ControllerClientHelper.
// reuse apis.IApi defined in controller_clienthelper.go

// NewController constructs a controller bound to logger, confs and storage.
func NewController(log logger.ILogger, confs confs.IConfs, db storage.IStorage, systemVersion string) *TheController {
	c := &TheController{
		log:                        log,
		confs:                      confs,
		db:                         db,
		apis:                       make(map[string]apis.IApi),
		systemVersion:              systemVersion,
		maxTimeWaitingClientSecond: 12 * 60 * 60, // default like C++	}
	}

	confMaxTimeWaitingClient := confs.GetConfig("maxTimeWaitingClient_seconds").Value()
	if confMaxTimeWaitingClient.GetInt64() > 0 {
		c.maxTimeWaitingClientSecond = confMaxTimeWaitingClient.GetInt64()
	}

	confMaxKeyLength := confs.GetConfig("maxKeyLength").Value()
	c.maxKeyLength = int(confMaxKeyLength.GetInt64())

	confMaxKeyWordLength := confs.GetConfig("maxKeyWordLength").Value()
	c.maxKeyWordLength = int(confMaxKeyWordLength.GetInt64())

	confMaxValueSize := confs.GetConfig("maxValueSize").Value()
	c.maxValueSize = int(confMaxValueSize.GetInt64())

	confRAwDbAccess := confs.GetConfig("allowRawDbAccess").Value()
	c.allowRawDbAccess = confRAwDbAccess.GetBool()

	return c
}

// SetVar creates or updates a variable. It returns an error if the variable
// is internal (starts with '_') or locked.
func (c *TheController) SetVar(name string, value misc.DynamicVar) chan error {
	ch := make(chan error, 1)
	go func() {
		name = misc.GetOnly(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._/\\,.-*")
		if name == "" {
			ch <- errors.New("variable name cannot be empty")
			return
		}
		if !c.allowRawDbAccess && name[0] == '_' || containsUnderscoreDot(name) {
			ch <- errors.New("variables started with underscore are for internal use only")
			return
		}

		if len(name) > c.maxKeyLength {
			ch <- errors.New("variable name exceeds maximum length of " + strconv.Itoa(c.maxKeyLength))
			return
		}

		if len(value.GetString()) > c.maxValueSize {
			ch <- errors.New("variable value exceeds maximum size of " + strconv.Itoa(c.maxValueSize))
			return
		}

		if c.maxKeyWordLength > 0 {
			parts := []string{name}
			if strings.Contains(name, ".") {
				parts = strings.Split(name, ".")
			}

			for _, p := range parts {
				if len(p) > c.maxKeyWordLength {
					ch <- errors.New("a part of the variable name exceeds maximum length of " + strconv.Itoa(c.maxKeyWordLength))
					return
				}
			}
		}

		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(name)
		//accept only text valid chars (remove all special chars)

		if vh.IsLocked() {
			ch <- errors.New("variable is locked")
			return
		}

		strValue := value.GetString()
		value.SetString(strValue)

		vh.SetValue(value)
		// async notify
		go c.notifyVarModification(name, value)
		ch <- nil
	}()
	return ch
}

func containsUnderscoreDot(name string) bool {
	// returns true when name contains the internal pattern '._'
	for i := 0; i < len(name)-1; i++ {
		if name[i] == '.' && name[i+1] == '_' {
			return true
		}
	}
	return false
}

// #region varname resolving mechanism {
func (c *TheController) resolveVarNames(query string) ([]string, error) {
	//strig can have
	result := []string{}

	vNames, err := c.resolveIncrementInstructions(query)
	if err != nil {
		return nil, fmt.Errorf("error resolving incremental instructions: %w", err)
	}

	for _, vName := range vNames {
		wildcardResolved := c.resolveWildCards(vName)
		result = append(result, wildcardResolved...)
	}

	return result, nil
}

func (c *TheController) resolveIncrementInstructions(query string) ([]string, error) {
	//string can have (many) increment instructions.
	//increment isntruction are defined inside ${}, and have the folowing format ${i;initialvalue,maxValue[,step]}, where:
	//	i: stands for increment instruction
	//	initialvalue: the value of the first variable that will be generated by this instruction
	//	maxValue: the maximum value that will be generated by this instruction. when the generated value reaches maxValue, it will wrap around to initialValue
	//	step: optional, the value to increment at each step, default is 1

	if strings.Contains(query, "${i") {
		before := query[:strings.Index(query, "${")]
		after := query[strings.Index(query, "}")+1:]
		rule := query[strings.Index(query, "${")+4 : strings.Index(query, "}")]

		parts := strings.Split(rule, ",")
		if len(parts) < 2 {
			return nil, errors.New("invalid increment instruction: " + rule)
		}

		initialValue, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, errors.New("invalid initial value in increment instruction: " + parts[0])
		}

		maxValue, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, errors.New("invalid max value in increment instruction: " + parts[1])
		}

		step := 1
		if len(parts) >= 3 {
			step, err = strconv.Atoi(parts[2])
			if err != nil {
				return nil, errors.New("invalid step value in increment instruction: " + parts[2])
			}
		}

		result := []string{}
		for i := initialValue; i <= maxValue; i += step {
			recursivelyResolved, err := c.resolveIncrementInstructions(before + strconv.Itoa(i) + after)
			if err != nil {
				return nil, err
			}

			result = append(result, recursivelyResolved...)
		}

	} else {
		return []string{query}, nil
	}

	return []string{}, nil
}

func (c *TheController) resolveWildCards(query string) []string {
	result := []string{}
	if strings.Contains(query, "*") {
		parentName := query[:strings.Index(query, "*")]
		remainQuery := query[strings.Index(query, "*")+1:]

		result = append(result, parentName)

		children := c.resolveWildCards_getAllChildNames(parentName)
		for _, child := range children {
			result = append(result, c.resolveWildCards(child+remainQuery)...)

		}
	} else {

		result = append(result, query)
	}

	return result
}

func (c *TheController) resolveWildCards_getAllChildNames(parentName string) []string {
	result := []string{}
	vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
	vh.SetControlledVarName(parentName)
	childs := vh.GetChildNames()
	for _, ch := range childs {
		full := parentName

		if full != "" {
			full += "." + ch
		} else {
			full = ch
		}
		result = append(result, full)

		subChildren := c.resolveWildCards_getAllChildNames(full)
		result = append(result, subChildren...)
	}
	return result
}

// #endregion }

// GetVars runs the C++ getVar logic asynchronously and returns a channel
// that will receive a slice of tuples (name, value) encoded as two DynamicVar
// elements: [nameAsDynamicVar, value].
func (c *TheController) GetVars(name string, defaultValue misc.DynamicVar) chan GetVarsResult {
	out := make(chan GetVarsResult, 1)
	go func() {
		// readFromDb recursively collects values
		var readFromDb func(string, bool) []misc.Tuple[misc.DynamicVar]
		readFromDb = func(nname string, childsToo bool) []misc.Tuple[misc.DynamicVar] {
			res := []misc.Tuple[misc.DynamicVar]{}
			vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
			vh.SetControlledVarName(nname)

			dv := vh.GetValue()
			if (&dv).GetString() != "" {
				res = append(res, misc.NewTuple[misc.DynamicVar](misc.NewDynamicVar(nname), dv))
			}

			if childsToo {
				prefix := nname
				if prefix != "" {
					prefix = prefix + "."
				}
				childs := vh.GetChildNames()
				for _, curr := range childs {
					tmp := readFromDb(prefix+curr, childsToo)
					// insert at beginning to match original order
					if len(tmp) > 0 {
						res = append(tmp, res...)
					}
				}
			}
			return res
		}

		// validate
		if name == "" {
			out <- GetVarsResult{nil, errors.New("variable name cannot be empty")}
			return
		}

		childsToo := false
		namep := name
		if strings.Contains(namep, "*") {
			if strings.HasSuffix(namep, "*") {
				childsToo = true
				// strip trailing '*' and possible trailing '.'
				if strings.HasSuffix(namep, ".*") {
					namep = namep[:len(namep)-2]
				} else {
					namep = namep[:len(namep)-1]
				}

				namep = strings.TrimSuffix(namep, ".")
			} else {
				// invalid wildcard position
				out <- GetVarsResult{nil, errors.New("wildcard can only be used at the end of the var name")}
				return
			}
		}

		values := readFromDb(namep, childsToo)
		if len(values) == 0 {
			values = append(values, misc.NewTuple[misc.DynamicVar](misc.NewDynamicVar(namep), defaultValue))
		}
		out <- GetVarsResult{values, nil}
	}()
	return out
}

// DelVar removes variables matching the name (possibly wildcard) asynchronously.
func (c *TheController) DelVar(varname string) chan error {
	out := make(chan error, 1)
	go func() {
		resChan := c.GetVars(varname, misc.NewDynamicVar(""))
		valRes := <-resChan
		if valRes.Err != nil {
			out <- valRes.Err
			return
		}
		vals := valRes.Values
		if len(vals) == 0 {
			out <- errors.New("error deleting variable: maybe it doesn't exist")
			return
		}

		if len(vals) > 1 {
			fmt.Println("Warning: Deleting multiple variables matching pattern '" + varname + "'")
		}
		// delete each found var
		for _, t := range vals {
			nameDV := t.At(0)
			dv := nameDV
			nameStr := (&dv).GetString()
			// delete from storage under "vars." prefix
			vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
			vh.SetControlledVarName(nameStr)
			vh.DeleteValue()

			go c.notifyVarModification(nameStr, misc.NewDynamicVar(""))
		}
		out <- nil
	}()
	return out
}

// GetChildsOfVar returns immediate child names for parentName asynchronously.
func (c *TheController) GetChildsOfVar(parentName string) chan []string {
	out := make(chan []string, 1)
	go func() {
		if strings.Contains(parentName, "*") {
			out <- []string{}
			return
		}
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(parentName)
		out <- vh.GetChildNames()
	}()
	return out
}

// LockVar tries to acquire a lock for a variable with a timeout in milliseconds.
// Runs in background and returns a channel with the resulting error (nil on success).
func (c *TheController) LockVar(varName string, timeoutMs uint) chan error {
	ch := make(chan error, 1)
	go func() {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(varName)
		ch <- vh.Lock(timeoutMs)
	}()
	return ch
}

// UnlockVar releases the lock for a variable. Runs in background.
func (c *TheController) UnlockVar(varName string) chan error {
	ch := make(chan error, 1)
	go func() {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(varName)
		ch <- vh.Unlock()
	}()
	return ch
}

// IsVarLocked checks if a variable is locked. Runs in background and returns a channel with the boolean result.
func (c *TheController) IsVarLocked(varName string) chan bool {
	ch := make(chan bool, 1)
	go func() {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(varName)
		ch <- vh.IsLocked()
	}()
	return ch
}

// ObserveVar registers a client observation of a variable and sends the
// current value to the client via the API. If the client is disconnected,
// the controller will schedule a liveness check.
func (c *TheController) ObserveVar(varName, clientId, customIdsAndMetainfo string, api apis.IApi) {
	vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
	vh.SetControlledVarName(varName)
	if !vh.IsClientObserving(clientId, customIdsAndMetainfo) {
		client := NewControllerClientHelper(c.db, clientId, api)
		vh.RegisterObservation(clientId, customIdsAndMetainfo)
		client.RegisterNewObservation(varName)

		// send current var
		vals := c.getVarInternal(varName)
		payload := []misc.Tuple[string]{}
		for _, dv := range vals {
			// dv: tuple(name, value)
			payload = append(payload, misc.NewTuple[string](dv.At(0), customIdsAndMetainfo, dv.At(1)))
		}
		dv := client.Notify(payload)
		if (&dv).GetString() != "LIVE" {
			c.checkClientLiveTime(*client)
		}
	}
}

// StopObservingVar removes a client's observation for a variable.
func (c *TheController) StopObservingVar(varName, clientId, customIdsAndMetainfo string, api apis.IApi) {
	vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
	vh.SetControlledVarName(varName)
	vh.RemoveObservationByClientAndMetadata(clientId, customIdsAndMetainfo)
	ch := NewControllerClientHelper(c.db, clientId, api)
	ch.UnregisterObservation(varName)
}

// ApiStarted registers a new API implementation with the controller.
func (c *TheController) ApiStarted(api apis.IApi) {
	c.apis[api.GetApiId()] = api
}

// ClientConnected registers a client and updates it with currently observed vars.
func (c *TheController) ClientConnected(clientId string, api apis.IApi) (string, int) {
	if clientId == "" {
		clientId = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	client := NewControllerClientHelper(c.db, clientId, api)
	observing := client.GetObservingVarsCount()
	go c.updateClientAboutObservatingVars(*client)
	return clientId, observing
}

// updateClientAboutObservatingVars sends current values for all variables the client observes.
func (c *TheController) updateClientAboutObservatingVars(ch ControllerClientHelper) {
	observingVars := ch.GetObservingVars()
	for _, v := range observingVars {
		vals := c.getVarInternal(v)
		payload := []misc.Tuple[string]{}
		for _, dv := range vals {
			vName := dv.At(0)
			vValue := dv.At(1)
			varHelper := NewControllerVarHelper(c.db, c.allowRawDbAccess)
			varHelper.SetControlledVarName(vName)

			metadatas := varHelper.GetMetadatasOfAClient(ch.GetClientId())
			if len(metadatas) == 0 {
				payload = append(payload, misc.NewTuple[string](vName, "", vValue))
			} else {
				for _, metadata := range metadatas {
					payload = append(payload, misc.NewTuple[string](vName, metadata, vValue))
				}
			}
		}
		dv := ch.Notify(payload)
		if (&dv).GetString() != "LIVE" {
			c.checkClientLiveTime(ch)
			return
		}
	}
}

// checkClientLiveTime checks and deletes a client when it has been disconnected
// for longer than the configured timeout.
func (c *TheController) checkClientLiveTime(ch ControllerClientHelper) {
	if !ch.IsConnected() {
		if ch.TimeSinceLastLiveTime() >= c.maxTimeWaitingClientSecond {
			c.deleteClient(ch)
		}
	}
}

// deleteClient removes a client from observation and removes its observing entries.
func (c *TheController) deleteClient(ch ControllerClientHelper) {
	vars := ch.GetObservingVars()
	for _, currVar := range vars {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(currVar)
		vh.RemoveAllClientObservations(ch.GetClientId())
	}
	ch.RemoveClientFromObservationSystem()
}

// notifyVarModification informs clients about a variable change.
func (c *TheController) notifyVarModification(varName string, value misc.DynamicVar) {
	go func() {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(varName)
		obs := vh.GetAllObservations()
		c.notifyClientsAboutVarChange(obs, varName, value)
		c.notifyParentGenericObservers(varName, varName, value)

		// also notify wildcard children
		childWildcard := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		childWildcard.SetControlledVarName(varName + ".*")
		c.notifyClientsAboutVarChange(childWildcard.GetAllObservations(), varName, value)
	}()
}

// notifyParentGenericObservers notifies parents that have generic observers (a.* patterns)
func (c *TheController) notifyParentGenericObservers(varName, changedVarName string, value misc.DynamicVar) {
	if idx := lastDotIndex(varName); idx != -1 {
		parent := varName[:idx]
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(parent + ".*")
		c.notifyClientsAboutVarChange(vh.GetAllObservations(), changedVarName, value)
		c.notifyParentGenericObservers(parent, changedVarName, value)
	}
}

func lastDotIndex(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

// notifyClientsAboutVarChange sends change notifications to observing clients.
func (c *TheController) notifyClientsAboutVarChange(observations map[ObservationID]misc.Tuple[string], changedVarName string, value misc.DynamicVar) {
	// idxs := []int{}
	// idxToKey := map[int]ObservationID{}
	// for k := range observations {
	// if i, err := strconv.Atoi(string(k)); err == nil {
	// idxs = append(idxs, i)
	// idxToKey[i] = k
	// }
	// }
	// sort.Sort(sort.Reverse(sort.IntSlice(idxs)))

	// for _, i := range idxs {
	for key, _ := range observations {
		tup := observations[key]

		//tup := observations[idxToKey[i]]
		clientId := tup.At(0)
		metadata := tup.At(1)

		go func(clientId, metadata string) {
			ch, err := NewControllerClientHelperWithApis(c.db, clientId, c.apis)
			if err != nil {
				if err == ErrAPIIdNotFound {
					c.log.Error("TheController", "Client notification failed: API not found for client "+clientId)
					ch2 := NewControllerClientHelper(c.db, clientId, nil)
					c.deleteClient(*ch2)
					return
				}
				c.log.Error("TheController", "Client notification failed: unknown error")
				return
			}

			payload := []misc.Tuple[string]{misc.NewTuple[string](changedVarName, metadata, (&value).GetString())}
			res := ch.Notify(payload)
			if (&res).GetString() != "LIVE" {
				c.checkClientLiveTime(*ch)
			}
		}(clientId, metadata)
	}
}

// getVarInternal reads a variable and its children and returns a slice of tuples (name, valueString)
func (c *TheController) getVarInternal(name string) []misc.Tuple[string] {
	res := []misc.Tuple[string]{}
	vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
	vh.SetControlledVarName(name)
	dv := vh.GetValue()
	if (&dv).GetString() != "" {
		res = append(res, misc.NewTuple[string](name, (&dv).GetString()))
	}
	childs := vh.GetChildNames()
	for _, ch := range childs {
		full := name
		if full != "" {
			full += "." + ch
		} else {
			full = ch
		}
		vvh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vvh.SetControlledVarName(full)
		dv2 := vvh.GetValue()
		res = append(res, misc.NewTuple[string](full, (&dv2).GetString()))
	}
	return res
}

// GetSystemVersion returns the configured system version string.
func (c *TheController) GetSystemVersion() string {
	return c.systemVersion
}
