package controller

import (
	"errors"
	"fmt"
	"math/rand"
	"rtonello/vss/sources/misc"
	"rtonello/vss/sources/services/storage"
	"strconv"
	"strings"
	"time"
)

type ObservationID string

type IControllerVarHelper interface {

	// Is very important to ContollerVArHelper to know the controlled variable name
	SetControlledVarName(vname string)

	// Register a new observation for a client.
	// A cliente could observate the same variable multiple times with different metadata.
	RegisterObservation(clientID string, metadata string) ObservationID

	//map with all clients observing this variable. Each entry is the observation ID mapped to a tuple with client ID and metadata
	GetAllObservingClients() map[ObservationID]misc.Tuple[string]

	RemoveObservationById(obsID ObservationID)
	RemoveObservationByClientAndMetadata(clientID string, metadata string)
	RemoveAllClientObservations(clientID string)

	GetAllObservations() map[ObservationID]misc.Tuple[string]
	RemoveAllObservations()

	GetMetadatasOfAClient(clientID string) []string

	GetValue() misc.DynamicVar
	SetValue(v misc.DynamicVar)

	GetChildNames() []string
	GetParentName() string

	SetFlag(flagName string, value misc.DynamicVar)
	GetFlag(flagName string, defaultValue misc.DynamicVar) misc.DynamicVar

	Lock(waitTimeout_ms uint) error
	Unlock() error
	IsLocked() bool
	IsClientObserving(clientId string, metadata string) bool

	ForeachObservation(f func(obsID ObservationID, clientID string, metadata string))
}

// ControllerVarHelper is a Go port of the C++ Controller_VarHelper logic.
// It operates on a storage.IStorage instance and stores observation/flags using
// the same key naming conventions: name + "._observers...", name + "._lock", etc.
type ControllerVarHelper struct {
	db               storage.IStorage
	name             string // full variable name, e.g. "vars.<varName>"
	allowRawDbAccess bool   // if true, allows access to vars starting with '_' (internal use only)
}

// NewControllerVarHelper creates a new helper bound to the provided storage.
// The actual controlled variable name must be set using SetControlledVarName.
func NewControllerVarHelper(db storage.IStorage, allowRawDbAccess bool) *ControllerVarHelper {
	return &ControllerVarHelper{db: db, allowRawDbAccess: allowRawDbAccess}
}

func (c *ControllerVarHelper) SetControlledVarName(vname string) {
	if c.allowRawDbAccess {
		c.name = vname
		return
	}

	if strings.HasPrefix(vname, "vars.") {
		c.name = vname
	} else {
		c.name = "vars." + vname
	}
}

// --- helpers ------------------------------------------------------------
func (c *ControllerVarHelper) runLocked(f func()) {
	// follow C++ behavior: use name + "._observationLock" with a timeout
	_ = misc.NamedLockRun(c.name+"._observationLock", 10*time.Second, f)
}

// --- observation management --------------------------------------------
func (c *ControllerVarHelper) RegisterObservation(clientID string, metadata string) ObservationID {
	var obsID ObservationID
	c.runLocked(func() {
		tmp := c.db.Get(c.name+"._observers.list.count", misc.NewDynamicVar(0))
		cnt := int(tmp.GetInt64())

		// set list entries
		c.db.Set(c.name+"._observers.list."+strconv.Itoa(cnt)+".clientId", misc.NewDynamicVar(clientID))
		c.db.Set(c.name+"._observers.list."+strconv.Itoa(cnt)+".metadata", misc.NewDynamicVar(metadata))

		// set byId mapping
		c.db.Set(c.name+"._observers.byId."+clientID+".byMetadata."+metadata, misc.NewDynamicVar(cnt))
		c.db.Set(c.name+"._observers.list.count", misc.NewDynamicVar(cnt+1))

		obsID = ObservationID(strconv.Itoa(cnt))
	})
	return obsID
}

func (c *ControllerVarHelper) GetAllObservingClients() map[ObservationID]misc.Tuple[string] {
	res := make(map[ObservationID]misc.Tuple[string])
	c.runLocked(func() {
		tmp := c.db.Get(c.name+"._observers.list.count", misc.NewDynamicVar(0))
		cnt := int(tmp.GetInt64())
		for i := 0; i < cnt; i++ {
			t1 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(i)+".clientId", misc.NewDynamicVar(""))
			client := t1.GetString()
			t2 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(i)+".metadata", misc.NewDynamicVar(""))
			metadata := t2.GetString()
			if client != "" {
				res[ObservationID(strconv.Itoa(i))] = misc.NewTuple[string](client, metadata)
			}
		}
	})
	return res
}

func (c *ControllerVarHelper) RemoveObservationById(obsID ObservationID) {
	idx, err := strconv.Atoi(string(obsID))
	if err != nil {
		return
	}
	t1 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(idx)+".clientId", misc.NewDynamicVar(""))
	client := t1.GetString()
	t2 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(idx)+".metadata", misc.NewDynamicVar(""))
	metadata := t2.GetString()
	if client == "" && metadata == "" {
		return
	}
	c.RemoveObservationByClientAndMetadata(client, metadata)
}

func (c *ControllerVarHelper) RemoveObservationByClientAndMetadata(clientID string, metadata string) {
	c.runLocked(func() {
		c.internalRemoveObserving(clientID, metadata)
	})
}

func (c *ControllerVarHelper) RemoveAllClientObservations(clientID string) {
	c.runLocked(func() {
		tmp := c.db.Get(c.name+"._observers.list.count", misc.NewDynamicVar(0))
		cnt := int(tmp.GetInt64())
		for i := cnt - 1; i >= 0; i-- {
			t1 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(i)+".clientId", misc.NewDynamicVar(""))
			client := t1.GetString()
			t2 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(i)+".metadata", misc.NewDynamicVar(""))
			metadata := t2.GetString()
			if client == clientID {
				c.internalRemoveObserving(client, metadata)
			}
		}
	})
}

func (c *ControllerVarHelper) internalRemoveObserving(clientID string, metadata string) {
	tmp := c.db.Get(c.name+"._observers.list.count", misc.NewDynamicVar(0))
	actualCount := int(tmp.GetInt64())
	for i := actualCount - 1; i >= 0; i-- {
		t1 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(i)+".clientId", misc.NewDynamicVar(""))
		currClient := t1.GetString()
		t2 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(i)+".metadata", misc.NewDynamicVar(""))
		currMetadata := t2.GetString()
		if currClient == clientID && currMetadata == metadata {
			// shift left entries from i+1..end
			for j := i; j < actualCount-1; j++ {
				t3 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(j+1)+".clientId", misc.NewDynamicVar(""))
				nextClient := t3.GetString()
				t4 := c.db.Get(c.name+"._observers.list."+strconv.Itoa(j+1)+".metadata", misc.NewDynamicVar(""))
				nextMetadata := t4.GetString()
				c.db.Set(c.name+"._observers.list."+strconv.Itoa(j)+".clientId", misc.NewDynamicVar(nextClient))
				c.db.Set(c.name+"._observers.list."+strconv.Itoa(j)+".metadata", misc.NewDynamicVar(nextMetadata))
				c.db.Set(c.name+"._observers.byId."+nextClient+".byMetadata."+nextMetadata, misc.NewDynamicVar(j))
			}
			// remove last list node
			c.db.DeleteValue(c.name+"._observers.list."+strconv.Itoa(actualCount-1), true)
			actualCount--
			c.db.Set(c.name+"._observers.list.count", misc.NewDynamicVar(actualCount))
			// remove byId mapping for this client/metadata
			c.db.DeleteValue(c.name+"._observers.byId."+clientID, true)
			break
		}
	}
}

func (c *ControllerVarHelper) GetAllObservations() map[ObservationID]misc.Tuple[string] {
	return c.GetAllObservingClients()
}

func (c *ControllerVarHelper) RemoveAllObservations() {
	c.runLocked(func() {
		// delete the whole observers subtree
		c.db.DeleteValue(c.name+"._observers", true)
		c.db.Set(c.name+"._observers.list.count", misc.NewDynamicVar(0))
	})
}

func (c *ControllerVarHelper) GetMetadatasOfAClient(clientID string) []string {
	result := make([]string, 0)

	//childs := c.db.GetChilds(R(c.name + "._observers.byId." + clientID + ".byMetadata"))
	childs := c.db.GetChildNames(c.name + "._observers.byId." + clientID + ".byMetadata")
	for _, curr := range childs {
		if strings.Contains(curr, ".") {
			metadata := curr[strings.Index(curr, ".")+1:]
			result = append(result, metadata)
		}
	}
	return result
}

// --- value & flag management ------------------------------------------
func (c *ControllerVarHelper) GetValue() misc.DynamicVar {
	return c.db.Get(c.name, misc.NewDynamicVar(""))
}

func (c *ControllerVarHelper) SetValue(v misc.DynamicVar) {
	if strings.Contains(c.name, "*") {
		// in C++ this returns an error. The Go interface has no error return, so we simply do nothing.
		return
	}
	c.db.Set(c.name, v)
}

func (c *ControllerVarHelper) GetChildNames() []string {
	childs := c.db.GetChildNames(c.name)
	res := make([]string, 0)
	for _, ch := range childs {
		if ch == "" {
			continue
		}
		if !c.allowRawDbAccess && ch[0] == '_' {
			continue
		}
		res = append(res, ch)
	}
	return res
}

func (c *ControllerVarHelper) GetParentName() string {
	indx := strings.LastIndex(c.name, ".")
	if indx >= 0 {
		return c.name[:indx]
	}
	return ""
}

func (c *ControllerVarHelper) SetFlag(flagName string, value misc.DynamicVar) {
	if flagName == "" {
		return
	}
	if flagName[0] != '_' {
		flagName = "_" + flagName
	}
	c.db.Set(c.name+"."+flagName, value)
}

func (c *ControllerVarHelper) GetFlag(flagName string, defaultValue misc.DynamicVar) misc.DynamicVar {
	if flagName == "" {
		return defaultValue
	}
	if flagName[0] != '_' {
		flagName = "_" + flagName
	}
	return c.db.Get(c.name+"."+flagName, defaultValue)
}

// --- locking ----------------------------------------------------------
func (c *ControllerVarHelper) IsLocked() bool {
	t := c.GetFlag("lock", misc.NewDynamicVar("0"))
	lockVal := t.GetString()
	return lockVal != "0" && lockVal != ""
}

func (c *ControllerVarHelper) Lock(waitTimeout_ms uint) error {
	// Use a global locker "varLockerSystem" similar to C++ implementation.
	maxTimeout := time.Duration(waitTimeout_ms) * time.Millisecond
	if maxTimeout == 0 {
		maxTimeout = 30 * time.Second
	}

	tryLock := func() bool {
		locked := false
		// critical section that checks/set _lock atomically using the named lock
		_ = misc.NamedLockRun("varLockerSystem", 0, func() {
			if !c.IsLocked() {
				c.SetFlag("lock", misc.NewDynamicVar("1"))
				locked = c.IsLocked()
			}
		})
		return locked
	}

	start := time.Now()
	for {
		if tryLock() {
			return nil
		}
		if time.Since(start) >= maxTimeout {
			return errors.New("timeout reached")
		}
		// small random backoff like the C++ code
		sleep := time.Duration(1+rand.Intn(9)) * time.Millisecond
		time.Sleep(sleep)
	}
}

func (c *ControllerVarHelper) Unlock() error {
	// release by setting lock flag to 0 inside varLockerSystem
	_ = misc.NamedLockRun("varLockerSystem", 0, func() {
		c.SetFlag("lock", misc.NewDynamicVar("0"))
	})
	return nil
}

func (c *ControllerVarHelper) IsClientObserving(clientId string, metadata string) bool {
	return c.db.HasValue(c.name + "._observers.byId." + clientId + ".byMetadata." + metadata)
}

func (c *ControllerVarHelper) ForeachObservation(f func(obsID ObservationID, clientID string, metadata string)) {
	obs := c.GetAllObservingClients()
	for id, tup := range obs {
		client := tup.At(0)
		metadata := tup.At(1)
		f(id, client, metadata)
	}
}

func (c *ControllerVarHelper) DeleteValue() {
	fmt.Println("Deleting var:", c.name)
	c.db.DeleteValue(c.name, false)
}
