package controller

import (
	"errors"
	"math/rand"
	"rtonello/vss/sources/misc"
	"rtonello/vss/sources/services/storage"
	"strconv"
	"strings"
	"time"
)

// ObservationID is a type alias for string, representing the unique identifier for an observation.
type ObservationID string

// IControllerVarHelper defines the interface for ControllerVarHelper, providing methods for managing variable observations, values, and flags.
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

	Lock(waitTimeoutMs uint) error
	Unlock() error
	IsLocked() bool
	IsClientObserving(clientID string, metadata string) bool

	ForeachObservation(f func(obsID ObservationID, clientID string, metadata string))
}

// VarHelper is a Go port of the C++ Controller_VarHelper logic.
// It operates on a storage.IStorage instance and stores observation/flags using
// the same key naming conventions: name + "._observers...", name + "._lock", etc.
type VarHelper struct {
	db               storage.IStorage
	name             string // full variable name, e.g. "vars.<varName>"
	allowRawDbAccess bool   // if true, allows access to vars starting with '_' (internal use only)
}

// NewControllerVarHelper creates a new helper bound to the provided storage.
// The actual controlled variable name must be set using SetControlledVarName.
func NewControllerVarHelper(db storage.IStorage, allowRawDbAccess bool) *VarHelper {
	return &VarHelper{db: db, allowRawDbAccess: allowRawDbAccess}
}

// SetControlledVarName sets the controlled variable name for the VarHelper instance.
func (c *VarHelper) SetControlledVarName(vname string) {
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
func (c *VarHelper) runLocked(f func()) {
	// follow C++ behavior: use name + "._observationLock" with a timeout
	_ = misc.NamedLockRun(c.name+"._observationLock", 10*time.Second, f)
}

// --- observation management --------------------------------------------

// RegisterObservation registers a new observation for the variable.
func (c *VarHelper) RegisterObservation(clientID string, metadata string) ObservationID {
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

// GetAllObservingClients retrieves all clients currently observing the variable, returning a map of ObservationID to a tuple containing client ID and metadata.
func (c *VarHelper) GetAllObservingClients() map[ObservationID]misc.Tuple[string] {
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

// RemoveObservationByID removes an observation by its ObservationID. It retrieves the client ID and metadata associated with the observation and calls RemoveObservationByClientAndMetadata to perform the removal.
func (c *VarHelper) RemoveObservationByID(obsID ObservationID) {
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

// RemoveObservationByClientAndMetadata removes an observation for a specific client and metadata. It locks the operation, finds the observation in the list, shifts subsequent entries down, and updates the count and byId mapping accordingly.
func (c *VarHelper) RemoveObservationByClientAndMetadata(clientID string, metadata string) {
	c.runLocked(func() {
		c.internalRemoveObserving(clientID, metadata)
	})
}

// RemoveAllClientObservations removes all observations for a specific client. It locks the operation, iterates through the list of observers, and calls internalRemoveObserving for each observation associated with the client.
func (c *VarHelper) RemoveAllClientObservations(clientID string) {
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

func (c *VarHelper) internalRemoveObserving(clientID string, metadata string) {
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
			// remove byId mapping for this specific client/metadata combination only
			c.db.DeleteValue(c.name+"._observers.byId."+clientID+".byMetadata."+metadata, false)
			break
		}
	}
}

// GetAllObservations retrieves all observations for the variable, returning a map of ObservationID to a tuple containing client ID and metadata. It internally calls GetAllObservingClients to obtain the data.
func (c *VarHelper) GetAllObservations() map[ObservationID]misc.Tuple[string] {
	return c.GetAllObservingClients()
}

// RemoveAllObservations removes all observations for the variable. It locks the operation, deletes the entire observers subtree from the database, and resets the observation count to zero.
func (c *VarHelper) RemoveAllObservations() {
	c.runLocked(func() {
		// delete the whole observers subtree
		c.db.DeleteValue(c.name+"._observers", true)
		c.db.Set(c.name+"._observers.list.count", misc.NewDynamicVar(0))
	})
}

// GetMetadatasOfAClient retrieves all metadata associated with a specific client observing the variable. It returns a slice of metadata strings.
func (c *VarHelper) GetMetadatasOfAClient(clientID string) []string {
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

// GetValue retrieves the current value of the variable from the database, returning it as a DynamicVar. If the variable does not exist, it returns an empty DynamicVar.
func (c *VarHelper) GetValue() misc.DynamicVar {
	return c.db.Get(c.name, misc.NewDynamicVar(""))
}

// SetValue sets the value of the variable in the database. If the variable name contains a wildcard "*", it does nothing, as this is not allowed. Otherwise, it updates the value in the database.
func (c *VarHelper) SetValue(v misc.DynamicVar) {
	if strings.Contains(c.name, "*") {
		// in C++ this returns an error. The Go interface has no error return, so we simply do nothing.
		return
	}
	c.db.Set(c.name, v)
}

// GetChildNames retrieves the names of child variables under the current variable. It filters out any empty names and, if raw database access is not allowed, it also filters out names starting with an underscore "_".
func (c *VarHelper) GetChildNames() []string {
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

// GetParentName retrieves the name of the parent variable by finding the last occurrence of a dot "." in the variable name. If a dot is found, it returns the substring before it; otherwise, it returns an empty string.
func (c *VarHelper) GetParentName() string {
	indx := strings.LastIndex(c.name, ".")
	if indx >= 0 {
		return c.name[:indx]
	}
	return ""
}

// SetFlag sets a flag for the variable in the database. It ensures that the flag name starts with an underscore "_" and stores the provided value under the variable's name concatenated with the flag name.
func (c *VarHelper) SetFlag(flagName string, value misc.DynamicVar) {
	if flagName == "" {
		return
	}
	if flagName[0] != '_' {
		flagName = "_" + flagName
	}
	c.db.Set(c.name+"."+flagName, value)
}

// GetFlag retrieves a flag for the variable from the database. It ensures that the flag name starts with an underscore "_" and returns the value stored under the variable's name concatenated with the flag name. If the flag does not exist, it returns the provided default value.
func (c *VarHelper) GetFlag(flagName string, defaultValue misc.DynamicVar) misc.DynamicVar {
	if flagName == "" {
		return defaultValue
	}
	if flagName[0] != '_' {
		flagName = "_" + flagName
	}
	return c.db.Get(c.name+"."+flagName, defaultValue)
}

// --- locking ----------------------------------------------------------

// IsLocked checks if the variable is currently locked by retrieving the "lock" flag from the database. It returns true if the lock value is not "0" and not empty, indicating that the variable is locked; otherwise, it returns false.
func (c *VarHelper) IsLocked() bool {
	t := c.GetFlag("lock", misc.NewDynamicVar("0"))
	lockVal := t.GetString()
	return lockVal != "0" && lockVal != ""
}

// Lock attempts to acquire a lock on the variable for a specified timeout period. It uses a global named lock "varLockerSystem" to ensure atomicity. If the lock is acquired within the timeout, it sets the "lock" flag to "1" and returns nil. If the timeout is reached without acquiring the lock, it returns an error indicating that the timeout was reached.
func (c *VarHelper) Lock(waitTimeoutMs uint) error {
	// Use a global locker "varLockerSystem" similar to C++ implementation.
	maxTimeout := time.Duration(waitTimeoutMs) * time.Millisecond
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

// Unlock releases the lock on the variable by setting the "lock" flag to "0" inside the "varLockerSystem" named lock. It ensures that the unlock operation is atomic and returns nil after successfully releasing the lock.
func (c *VarHelper) Unlock() error {
	// release by setting lock flag to 0 inside varLockerSystem
	_ = misc.NamedLockRun("varLockerSystem", 0, func() {
		c.SetFlag("lock", misc.NewDynamicVar("0"))
	})
	return nil
}

// IsClientObserving checks if a specific client is currently observing the variable with the given metadata. It returns true if the client is found in the observers list; otherwise, it returns false.
func (c *VarHelper) IsClientObserving(clientID string, metadata string) bool {
	return c.db.HasValue(c.name + "._observers.byId." + clientID + ".byMetadata." + metadata)
}

// ForeachObservation iterates over all observations for the variable, calling the provided function for each observation with its ObservationID, client ID, and metadata.
func (c *VarHelper) ForeachObservation(f func(obsID ObservationID, clientID string, metadata string)) {
	obs := c.GetAllObservingClients()
	for id, tup := range obs {
		client := tup.At(0)
		metadata := tup.At(1)
		f(id, client, metadata)
	}
}

// DeleteValue deletes the variable's value from the database. It calls the DeleteValue method on the underlying storage instance, passing the variable's name and a flag indicating whether to delete child values (set to false in this case).
func (c *VarHelper) DeleteValue() {
	c.db.DeleteValue(c.name, false)
}
