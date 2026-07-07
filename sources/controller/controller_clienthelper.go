//clients holds an id and its api (the API where it is connected through)

// Controller_ClientHelper(StorageInterface *db, string clientId, apis.IApi* api);
// Controller_ClientHelper(StorageInterface *db, string clientId, map<string, apis.IApi*> apis, Controller_ClientHelperError &error);
// int64_t getLastLiveTime();
// int64_t timeSinceLastLiveTime();
// void updateLiveTime();
// bool isConnected();
// vector<string> getObservingVars();
// int getObservingVarsCount();
// API::ClientSendResult notify(vector<tuple<string, string, DynamicVar>> varsAndValues);
// void registerNewObservation(string varName);
// void unregisterObservation(string varName);
// string getClientId();
// void removeClientFromObservationSystem();

package controller

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"rtonello/vss/sources/misc"
	"rtonello/vss/sources/services/apis"
	"rtonello/vss/sources/services/storage"
)

// IControllerClientHelper defines the interface for ControllerClientHelper, providing methods for managing client connections and observations.
type IControllerClientHelper interface {

	// Is very important to ControllerClientHelper to know the controlled client id and its API
	SetControlledClientId(clientID string, APIID string)

	GetLastLiveTime() int64
	TimeSinceLastLiveTime() int64
	UpdateLiveTime()
	IsConnected() bool

	GetObservingVars() []string
	GetObservingVarsCount() int

	Notify(varsAndValues []misc.Tuple[string]) misc.DynamicVar

	RegisterNewObservation(varName string)
	UnregisterObservation(varName string)

	GetClientId() string
	GetAPIId() string
}

// ClientHelper is a Go port of the C++ Controller_ClientHelper.
type ClientHelper struct {
	db               storage.IStorage
	clientID         string
	api              apis.IApi
	allowRawDbAccess bool
}

// NewControllerClientHelper constructs a helper with a concrete apis.IApi and initializes persistent state.
func NewControllerClientHelper(db storage.IStorage, clientID string, api apis.IApi) *ClientHelper {
	h := &ClientHelper{db: db, clientID: clientID, api: api}
	h.initialize()
	return h
}

// NewControllerClientHelperWithApis looks up the apiId stored in DB and returns the helper bound to that API if found.
func NewControllerClientHelperWithApis(db storage.IStorage, clientID string, apis map[string]apis.IApi) (*ClientHelper, error) {
	t := db.Get("internal.clients.byId."+clientID+".apiId", misc.NewDynamicVar(""))
	APIID := t.GetString()
	if APIID == "" {
		return nil, ErrAPIIdNotFound
	}
	api, ok := apis[APIID]
	if !ok {
		return nil, ErrAPIIdNotFound
	}
	return NewControllerClientHelper(db, clientID, api), nil
}

var (
	// ErrAPIIdNotFound is returned when the API ID for a client is not found in the database.
	ErrAPIIdNotFound = errors.New("API_NOT_FOUND")
)

func (h *ClientHelper) initialize() {
	_ = misc.NamedLockRun("db.intenal.clients", 2*time.Second, func() {
		if !h.db.HasValue("internal.clients.byId." + h.clientID) {
			tcnt := h.db.Get("internal.clients.list.count", misc.NewDynamicVar(0))
			cnt := int(tcnt.GetInt64())
			h.db.Set("internal.clients.list.count", misc.NewDynamicVar(cnt+1))
			h.db.Set("internal.clients.list."+strconv.Itoa(cnt), misc.NewDynamicVar(h.clientID))
			h.db.Set("internal.clients.byId."+h.clientID, misc.NewDynamicVar(cnt))
		}
		if h.api != nil {
			h.db.Set("internal.clients.byId."+h.clientID+".apiId", misc.NewDynamicVar(h.api.GetAPIID()))
		}
		h.UpdateLiveTime()
	})
}

func currentTimeSeconds() int64 {
	return time.Now().Unix()
}

// SetControlledClientID sets the controlled client ID and updates the API ID in the database if available.
func (h *ClientHelper) SetControlledClientID(clientID string, APIID string) {
	h.clientID = clientID
	if h.api != nil {
		h.db.Set("internal.clients.byId."+h.clientID+".apiId", misc.NewDynamicVar(h.api.GetAPIID()))
	} else if APIID != "" {
		h.db.Set("internal.clients.byId."+h.clientID+".apiId", misc.NewDynamicVar(APIID))
	}
}

// GetLastLiveTime retrieves the last recorded live time for the client from the database.
func (h *ClientHelper) GetLastLiveTime() int64 {
	t := h.db.Get("internal.clients.byId."+h.clientID+".lastLiveTime", misc.NewDynamicVar(0))
	return t.GetInt64()
}

// TimeSinceLastLiveTime calculates the time elapsed since the last live time for the client.
func (h *ClientHelper) TimeSinceLastLiveTime() int64 {
	return currentTimeSeconds() - h.GetLastLiveTime()
}

// UpdateLiveTime updates the last live time for the client in the database to the current time.
func (h *ClientHelper) UpdateLiveTime() {
	h.db.Set("internal.clients.byId."+h.clientID+".lastLiveTime", misc.NewDynamicVar(currentTimeSeconds()))
}

// IsConnected checks if the client is connected by calling the CheckAlive method on the API. If connected, it updates the live time.
func (h *ClientHelper) IsConnected() bool {
	if h.api == nil {
		return false
	}
	ret := h.api.CheckAlive(h.clientID)
	if ret {
		h.UpdateLiveTime()
	}
	return ret
}

// GetObservingVarsCount retrieves the count of variables that the client is currently observing from the database.
func (h *ClientHelper) GetObservingVarsCount() int {
	t := h.db.Get("internal.clients.byId."+h.clientID+".observing.count", misc.NewDynamicVar(0))
	return int(t.GetInt64())
}

// GetObservingVars retrieves the list of variable names that the client is currently observing, filtering out any entries that contain "count" or "size".
func (h *ClientHelper) GetObservingVars() []string {
	res := []string{}
	childs := h.db.GetChildNames("internal.clients.byId." + h.clientID + ".observing")
	for _, c := range childs {
		if strings.Contains(c, "count") || strings.Contains(c, "size") {
			continue
		}
		tcurr := h.db.Get("internal.clients.byId."+h.clientID+".observing."+c, misc.NewDynamicVar(""))
		curr := tcurr.GetString()
		if len(curr) > 5 && strings.HasPrefix(curr, "vars.") {
			curr = curr[5:]
		}
		res = append(res, curr)
	}
	return res
}

// Notify sends a notification to the client with the provided variable names and values. It returns a DynamicVar indicating the connection status ("LIVE" or "DISCONNECTED").
func (h *ClientHelper) Notify(varsAndValues []misc.Tuple[string]) misc.DynamicVar {
	if h.api == nil {
		return misc.NewDynamicVar("DISCONNECTED")
	}
	if h.api.NotifyClient(h.clientID, varsAndValues) {
		h.UpdateLiveTime()
		return misc.NewDynamicVar("LIVE")
	}
	return misc.NewDynamicVar("DISCONNECTED")
}

// RegisterNewObservation registers a new variable observation for the client. It increments the observation count and stores the variable name in the database.
func (h *ClientHelper) RegisterNewObservation(varName string) {
	varName = "vars." + varName
	_ = misc.NamedLockRun("db.intenal.clients", 2*time.Second, func() {
		h.UpdateLiveTime()
		tcurr := h.db.Get("internal.clients.byId."+h.clientID+".observing.count", misc.NewDynamicVar(0))
		curr := int(tcurr.GetInt64())
		h.db.Set("internal.clients.byId."+h.clientID+".observing.count", misc.NewDynamicVar(curr+1))
		h.db.Set("internal.clients.byId."+h.clientID+".observing."+strconv.Itoa(curr), misc.NewDynamicVar(varName))
	})
}

func (h *ClientHelper) findVarIndexOnObservingVars(varName string) int {
	tcnt := h.db.Get("internal.clients.byId."+h.clientID+".observing.count", misc.NewDynamicVar(0))
	cnt := int(tcnt.GetInt64())
	for i := 0; i < cnt; i++ {
		tv := h.db.Get("internal.clients.byId."+h.clientID+".observing."+strconv.Itoa(i), misc.NewDynamicVar(""))
		v := tv.GetString()
		if v == varName {
			return i
		}
	}
	return -1
}

// UnregisterObservation removes a variable observation for the client. It finds the index of the variable in the observing list, shifts subsequent variables down, and decrements the observation count.
func (h *ClientHelper) UnregisterObservation(varName string) {
	varName = "vars." + varName
	_ = misc.NamedLockRun("db.intenal.clients", 5*time.Second, func() {
		h.UpdateLiveTime()
		tcurr := h.db.Get("internal.clients.byId."+h.clientID+".observing.count", misc.NewDynamicVar(0))
		curr := int(tcurr.GetInt64())
		idx := h.findVarIndexOnObservingVars(varName)
		if idx > -1 {
			for c := idx; c < curr-1; c++ {
				next := h.db.Get("internal.clients.byId."+h.clientID+".observing."+strconv.Itoa(c+1), misc.NewDynamicVar(""))
				h.db.Set("internal.clients.byId."+h.clientID+".observing."+strconv.Itoa(c), next)
			}
			h.db.DeleteValue("internal.clients.byId."+h.clientID+".observing."+strconv.Itoa(curr-1), false)
			h.db.Set("internal.clients.byId."+h.clientID+".observing.count", misc.NewDynamicVar(curr-1))
		}
	})
}

// GetClientID returns the client ID associated with this ClientHelper instance.
func (h *ClientHelper) GetClientID() string {
	return h.clientID
}

// GetAPIId retrieves the API ID associated with the client from the database. If not found, it returns an empty string.
func (h *ClientHelper) GetAPIId() string {
	ta := h.db.Get("internal.clients.byId."+h.clientID+".apiId", misc.NewDynamicVar(""))
	return ta.GetString()
}

// RemoveClientFromObservationSystem removes the client from the observation system. It shifts subsequent clients down in the list, decrements the client count, and deletes the client's entry from the database.
func (h *ClientHelper) RemoveClientFromObservationSystem() {
	_ = misc.NamedLockRun("db.intenal.clients", 5*time.Second, func() {
		tci := h.db.Get("internal.clients.byId."+h.clientID, misc.NewDynamicVar(-1))
		clientIndex := int(tci.GetInt64())
		if clientIndex > -1 {
			tcc := h.db.Get("internal.clients.list.count", misc.NewDynamicVar(0))
			currentCount := int(tcc.GetInt64())
			for c := clientIndex; c < currentCount-1; c++ {
				next := h.db.Get("internal.clients.list."+strconv.Itoa(c+1), misc.NewDynamicVar(""))
				h.db.Set("internal.clients.list."+strconv.Itoa(c), next)
			}
			currentCount--
			h.db.DeleteValue("internal.clients.list."+strconv.Itoa(currentCount), false)
			h.db.Set("internal.clients.list.count", misc.NewDynamicVar(currentCount))
		}
		h.db.DeleteValue("internal.clients.byId."+h.clientID, true)
	})
}
