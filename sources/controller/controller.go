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

// IController defines the interface for TheController, providing methods for managing variables, locks, and client observations.
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
	ObserveVar(varName, clientID, customIdsAndMetainfo string, api apis.IApi)

	// StopObservingVar removes a client's observation for a variable.
	StopObservingVar(varName, clientID, customIdsAndMetainfo string, api apis.IApi)

	// APIStarted registers a new API implementation with the controller.
	APIStarted(api apis.IApi)

	// ClientConnected registers a client and updates it with currently observed vars.
	ClientConnected(clientID string, api apis.IApi) (string, int)

	// GetSystemVersion returns the configured system version string.
	GetSystemVersion() string
}

// GetVarsResult is the returned value from GetVars: Values is the list of
// (name,value) tuples, Err is non-nil when an error occurred.
type GetVarsResult struct {
	Values []misc.Tuple[misc.DynamicVar]
	Err    error
}

type GetSingleVarResult struct {
	Value misc.Tuple[misc.DynamicVar]
	Err   error
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

func (c *TheController) setVar(name string, value misc.DynamicVar) chan error {
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

// SetVar creates or updates a variable. It returns an error if the variable
// is internal (starts with '_') or locked.
func (c *TheController) SetVar(name string, value misc.DynamicVar) chan error {
	out := make(chan error, 1)
	vnames, err := c.resolveVarNames(name)
	if err != nil {
		out <- err
		return out
	}

	go func() {
		for _, vname := range vnames {
			resChan := c.setVar(vname, value)
			err := <-resChan
			if err != nil {
				out <- err
				return
			}
		}
		out <- nil
	}()

	return out
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

		//trim a possible '.' at end of parentName
		parentName = strings.TrimSuffix(parentName, ".")

		children := c.resolveWildCardsgetAllChildNames(parentName)
		for _, child := range children {
			result = append(result, c.resolveWildCards(child+remainQuery)...)

		}
	} else {

		result = append(result, query)
	}

	return result
}

func (c *TheController) resolveWildCardsgetAllChildNames(parentName string) []string {
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

		subChildren := c.resolveWildCardsgetAllChildNames(full)
		result = append(result, subChildren...)
	}
	return result
}

// #endregion }

func (c *TheController) getVar(name string, defaultValue misc.DynamicVar) chan GetSingleVarResult {
	out := make(chan GetSingleVarResult, 1)
	go func() {
		// readFromDb recursively collects values
		res := misc.Tuple[misc.DynamicVar]{}
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(name)

		dv := vh.GetValue()
		if (&dv).GetString() != "" {
			res = misc.NewTuple[misc.DynamicVar](misc.NewDynamicVar(name), dv)
		}

		out <- GetSingleVarResult{res, nil}
	}()
	return out
}

// GetVars runs the C++ getVar logic asynchronously and returns a channel
// that will receive a slice of tuples (name, value) encoded as two DynamicVar
// elements: [nameAsDynamicVar, value].
func (c *TheController) GetVars(name string, defaultValue misc.DynamicVar) chan GetVarsResult {
	out := make(chan GetVarsResult, 1)
	// validate
	if name == "" {
		out <- GetVarsResult{nil, errors.New("variable name cannot be empty")}
		return out
	}

	vnames, err := c.resolveVarNames(name)
	if err != nil {
		out <- GetVarsResult{nil, err}
		return out
	}

	go func() {
		allValues := []misc.Tuple[misc.DynamicVar]{}
		for _, vname := range vnames {
			resChan := c.getVar(vname, defaultValue)
			res := <-resChan

			if res.Value.Len() == 0 {
				continue
			}

			tmp := res.Value.Second()

			if tmp.GetString() != "" {
				allValues = append(allValues, res.Value)
			}
		}
		out <- GetVarsResult{allValues, nil}

		if (len(allValues) == 0) && defaultValue.GetString() != "" {
			// if no values found, return default value
			out <- GetVarsResult{[]misc.Tuple[misc.DynamicVar]{misc.NewTuple[misc.DynamicVar](misc.NewDynamicVar(name), defaultValue)}, nil}
		}
	}()

	return out
}

func (c *TheController) delVar(varname string) chan error {
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

// DelVar removes variables matching the name (possibly wildcard) asynchronously.
func (c *TheController) DelVar(varname string) chan error {
	out := make(chan error, 1)
	vnames, err := c.resolveVarNames(varname)
	if err != nil {
		out <- err
		return out
	}

	go func() {
		for _, vname := range vnames {
			resChan := c.delVar(vname)
			err := <-resChan
			if err != nil {
				out <- err
				return
			}
		}
		out <- nil
	}()

	return out
}

func (c *TheController) getChildsOfVar(parentName string) chan []string {
	out := make(chan []string, 1)
	go func() {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(parentName)
		out <- vh.GetChildNames()
	}()
	return out
}

// GetChildsOfVar returns immediate child names for parentName asynchronously.
func (c *TheController) GetChildsOfVar(parentName string) chan []string {
	out := make(chan []string, 1)

	vnames, err := c.resolveVarNames(parentName)
	if err != nil {
		out <- []string{}
		return out
	}

	go func() {
		allChilds := []string{}
		for _, vname := range vnames {
			resChan := c.getChildsOfVar(vname)
			childs := <-resChan
			allChilds = append(allChilds, childs...)
		}
		out <- allChilds
	}()

	return out
}

// Runs in background and returns a channel with the resulting error (nil on success).
func (c *TheController) lockVar(varName string, timeoutMs uint) chan error {
	ch := make(chan error, 1)
	go func() {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(varName)
		ch <- vh.Lock(timeoutMs)
	}()
	return ch
}

// LockVar tries to acquire a lock for a variable with a timeout in milliseconds.
func (c *TheController) LockVar(varName string, timeoutMs uint) chan error {
	result := make(chan error, 1)
	vnames, err := c.resolveVarNames(varName)
	if err != nil {
		result <- err
		return result
	}

	go func() {
		for _, vname := range vnames {
			resChan := c.lockVar(vname, timeoutMs)
			err := <-resChan
			if err != nil {
				result <- err
				return
			}
		}
		result <- nil
	}()

	return result
}

func (c *TheController) unlockVar(varName string) chan error {
	ch := make(chan error, 1)
	go func() {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(varName)
		ch <- vh.Unlock()
	}()
	return ch
}

// UnlockVar releases the lock for a variable. Runs in background.
func (c *TheController) UnlockVar(varName string) chan error {
	result := make(chan error, 1)
	vnames, err := c.resolveVarNames(varName)
	if err != nil {
		result <- err
		return result
	}

	go func() {
		for _, vname := range vnames {
			resChan := c.unlockVar(vname)
			err := <-resChan
			if err != nil {
				result <- err
				return
			}
		}
		result <- nil
	}()

	return result
}

func (c *TheController) isVarLocked(varName string) chan bool {
	ch := make(chan bool, 1)
	go func() {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(varName)
		ch <- vh.IsLocked()
	}()
	return ch
}

// IsVarLocked checks if a variable is locked. Runs in background and returns a channel with the boolean result.
func (c *TheController) IsVarLocked(varName string) chan bool {
	result := make(chan bool, 1)
	vnames, err := c.resolveVarNames(varName)
	if err != nil {
		result <- false
		return result
	}

	go func() {
		for _, vname := range vnames {
			resChan := c.isVarLocked(vname)
			isLocked := <-resChan
			if isLocked {
				result <- true
				return
			}
		}
		result <- false
	}()

	return result
}

func (c *TheController) observeVar(varName, clientID, customIdsAndMetainfo string, api apis.IApi) {
	vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
	vh.SetControlledVarName(varName)
	if !vh.IsClientObserving(clientID, customIdsAndMetainfo) {
		client := NewControllerClientHelper(c.db, clientID, api)
		vh.RegisterObservation(clientID, customIdsAndMetainfo)
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

// ObserveVar registers a client observation of a variable and sends the
// current value to the client via the API. If the client is disconnected,
// the controller will schedule a liveness check.
func (c *TheController) ObserveVar(varName, clientID, customIdsAndMetainfo string, api apis.IApi) {
	vnames, err := c.resolveVarNames(varName)
	if err != nil {
		c.log.Error("TheController", "Error resolving variable names for observation: "+err.Error())
		return
	}

	for _, vname := range vnames {
		c.observeVar(vname, clientID, customIdsAndMetainfo, api)
	}
}

func (c *TheController) stopObservingVar(varName, clientID, customIdsAndMetainfo string, api apis.IApi) {
	vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
	vh.SetControlledVarName(varName)
	vh.RemoveObservationByClientAndMetadata(clientID, customIdsAndMetainfo)
	ch := NewControllerClientHelper(c.db, clientID, api)
	ch.UnregisterObservation(varName)
}

// StopObservingVar removes a client's observation for a variable.
func (c *TheController) StopObservingVar(varName, clientID, customIdsAndMetainfo string, api apis.IApi) {
	vnames, err := c.resolveVarNames(varName)
	if err != nil {
		c.log.Error("TheController", "Error resolving variable names for stopping observation: "+err.Error())
		return
	}

	for _, vname := range vnames {
		c.stopObservingVar(vname, clientID, customIdsAndMetainfo, api)
	}
}

// APIStarted registers a new API implementation with the controller.
func (c *TheController) APIStarted(api apis.IApi) {
	c.apis[api.GetAPIID()] = api
}

// ClientConnected registers a client and updates it with currently observed vars.
func (c *TheController) ClientConnected(clientID string, api apis.IApi) (string, int) {
	if clientID == "" {
		clientID = strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	client := NewControllerClientHelper(c.db, clientID, api)
	observing := client.GetObservingVarsCount()
	go c.updateClientAboutObservatingVars(*client)
	return clientID, observing
}

// updateClientAboutObservatingVars sends current values for all variables the client observes.
func (c *TheController) updateClientAboutObservatingVars(ch ClientHelper) {
	observingVars := ch.GetObservingVars()
	for _, v := range observingVars {
		vals := c.getVarInternal(v)
		payload := []misc.Tuple[string]{}
		for _, dv := range vals {
			vName := dv.At(0)
			vValue := dv.At(1)
			varHelper := NewControllerVarHelper(c.db, c.allowRawDbAccess)
			varHelper.SetControlledVarName(vName)

			metadatas := varHelper.GetMetadatasOfAClient(ch.GetClientID())
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
func (c *TheController) checkClientLiveTime(ch ClientHelper) {
	if !ch.IsConnected() {
		if ch.TimeSinceLastLiveTime() >= c.maxTimeWaitingClientSecond {
			c.deleteClient(ch)
		}
	}
}

// deleteClient removes a client from observation and removes its observing entries.
func (c *TheController) deleteClient(ch ClientHelper) {
	vars := ch.GetObservingVars()
	for _, currVar := range vars {
		vh := NewControllerVarHelper(c.db, c.allowRawDbAccess)
		vh.SetControlledVarName(currVar)
		vh.RemoveAllClientObservations(ch.GetClientID())
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
		clientID := tup.At(0)
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
		}(clientID, metadata)
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
