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

type IControllerClientHelper interface {

	// Is very important to ControllerClientHelper to know the controlled client id and its API
	SetControlledClientId(clientId string, apiId string)

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

// ControllerClientHelper is a Go port of the C++ Controller_ClientHelper.
type ControllerClientHelper struct {
	db               storage.IStorage
	clientId         string
	api              apis.IApi
	allowRawDbAccess bool
}

// NewControllerClientHelper constructs a helper with a concrete apis.IApi and initializes persistent state.
func NewControllerClientHelper(db storage.IStorage, clientId string, api apis.IApi) *ControllerClientHelper {
	h := &ControllerClientHelper{db: db, clientId: clientId, api: api}
	h.initialize()
	return h
}

// NewControllerClientHelperWithApis looks up the apiId stored in DB and returns the helper bound to that API if found.
func NewControllerClientHelperWithApis(db storage.IStorage, clientId string, apis map[string]apis.IApi) (*ControllerClientHelper, error) {
	t := db.Get("internal.clients.byId."+clientId+".apiId", misc.NewDynamicVar(""))
	apiId := t.GetString()
	if apiId == "" {
		return nil, ErrAPIIdNotFound
	}
	api, ok := apis[apiId]
	if !ok {
		return nil, ErrAPIIdNotFound
	}
	return NewControllerClientHelper(db, clientId, api), nil
}

var (
	ErrAPIIdNotFound = errors.New("API_NOT_FOUND")
)

func (h *ControllerClientHelper) initialize() {
	_ = misc.NamedLockRun("db.intenal.clients", 2*time.Second, func() {
		if !h.db.HasValue("internal.clients.byId." + h.clientId) {
			tcnt := h.db.Get("internal.clients.list.count", misc.NewDynamicVar(0))
			cnt := int(tcnt.GetInt64())
			h.db.Set("internal.clients.list.count", misc.NewDynamicVar(cnt+1))
			h.db.Set("internal.clients.list."+strconv.Itoa(cnt), misc.NewDynamicVar(h.clientId))
			h.db.Set("internal.clients.byId."+h.clientId, misc.NewDynamicVar(cnt))
		}
		if h.api != nil {
			h.db.Set("internal.clients.byId."+h.clientId+".apiId", misc.NewDynamicVar(h.api.GetApiId()))
		}
		h.UpdateLiveTime()
	})
}

func currentTimeSeconds() int64 {
	return time.Now().Unix()
}

func (h *ControllerClientHelper) SetControlledClientId(clientId string, apiId string) {
	h.clientId = clientId
	if h.api != nil {
		h.db.Set("internal.clients.byId."+h.clientId+".apiId", misc.NewDynamicVar(h.api.GetApiId()))
	} else if apiId != "" {
		h.db.Set("internal.clients.byId."+h.clientId+".apiId", misc.NewDynamicVar(apiId))
	}
}

func (h *ControllerClientHelper) GetLastLiveTime() int64 {
	t := h.db.Get("internal.clients.byId."+h.clientId+".lastLiveTime", misc.NewDynamicVar(0))
	return t.GetInt64()
}

func (h *ControllerClientHelper) TimeSinceLastLiveTime() int64 {
	return currentTimeSeconds() - h.GetLastLiveTime()
}

func (h *ControllerClientHelper) UpdateLiveTime() {
	h.db.Set("internal.clients.byId."+h.clientId+".lastLiveTime", misc.NewDynamicVar(currentTimeSeconds()))
}

func (h *ControllerClientHelper) IsConnected() bool {
	if h.api == nil {
		return false
	}
	ret := h.api.CheckAlive(h.clientId)
	if ret {
		h.UpdateLiveTime()
	}
	return ret
}

func (h *ControllerClientHelper) GetObservingVarsCount() int {
	t := h.db.Get("internal.clients.byId."+h.clientId+".observing.count", misc.NewDynamicVar(0))
	return int(t.GetInt64())
}

func (h *ControllerClientHelper) GetObservingVars() []string {
	res := []string{}
	childs := h.db.GetChildNames("internal.clients.byId." + h.clientId + ".observing")
	for _, c := range childs {
		if strings.Contains(c, "count") || strings.Contains(c, "size") {
			continue
		}
		tcurr := h.db.Get("internal.clients.byId."+h.clientId+".observing."+c, misc.NewDynamicVar(""))
		curr := tcurr.GetString()
		if len(curr) > 5 && strings.HasPrefix(curr, "vars.") {
			curr = curr[5:]
		}
		res = append(res, curr)
	}
	return res
}

func (h *ControllerClientHelper) Notify(varsAndValues []misc.Tuple[string]) misc.DynamicVar {
	if h.api == nil {
		return misc.NewDynamicVar("DISCONNECTED")
	}
	if h.api.NotifyClient(h.clientId, varsAndValues) {
		h.UpdateLiveTime()
		return misc.NewDynamicVar("LIVE")
	}
	return misc.NewDynamicVar("DISCONNECTED")
}

func (h *ControllerClientHelper) RegisterNewObservation(varName string) {
	varName = "vars." + varName
	_ = misc.NamedLockRun("db.intenal.clients", 2*time.Second, func() {
		h.UpdateLiveTime()
		tcurr := h.db.Get("internal.clients.byId."+h.clientId+".observing.count", misc.NewDynamicVar(0))
		curr := int(tcurr.GetInt64())
		h.db.Set("internal.clients.byId."+h.clientId+".observing.count", misc.NewDynamicVar(curr+1))
		h.db.Set("internal.clients.byId."+h.clientId+".observing."+strconv.Itoa(curr), misc.NewDynamicVar(varName))
	})
}

func (h *ControllerClientHelper) findVarIndexOnObservingVars(varName string) int {
	tcnt := h.db.Get("internal.clients.byId."+h.clientId+".observing.count", misc.NewDynamicVar(0))
	cnt := int(tcnt.GetInt64())
	for i := 0; i < cnt; i++ {
		tv := h.db.Get("internal.clients.byId."+h.clientId+".observing."+strconv.Itoa(i), misc.NewDynamicVar(""))
		v := tv.GetString()
		if v == varName {
			return i
		}
	}
	return -1
}

func (h *ControllerClientHelper) UnregisterObservation(varName string) {
	varName = "vars." + varName
	_ = misc.NamedLockRun("db.intenal.clients", 5*time.Second, func() {
		h.UpdateLiveTime()
		tcurr := h.db.Get("internal.clients.byId."+h.clientId+".observing.count", misc.NewDynamicVar(0))
		curr := int(tcurr.GetInt64())
		idx := h.findVarIndexOnObservingVars(varName)
		if idx > -1 {
			for c := idx; c < curr-1; c++ {
				next := h.db.Get("internal.clients.byId."+h.clientId+".observing."+strconv.Itoa(c+1), misc.NewDynamicVar(""))
				h.db.Set("internal.clients.byId."+h.clientId+".observing."+strconv.Itoa(c), next)
			}
			h.db.DeleteValue("internal.clients.byId."+h.clientId+".observing."+strconv.Itoa(curr-1), false)
			h.db.Set("internal.clients.byId."+h.clientId+".observing.count", misc.NewDynamicVar(curr-1))
		}
	})
}

func (h *ControllerClientHelper) GetClientId() string {
	return h.clientId
}

func (h *ControllerClientHelper) GetAPIId() string {
	ta := h.db.Get("internal.clients.byId."+h.clientId+".apiId", misc.NewDynamicVar(""))
	return ta.GetString()
}

func (h *ControllerClientHelper) RemoveClientFromObservationSystem() {
	_ = misc.NamedLockRun("db.intenal.clients", 5*time.Second, func() {
		tci := h.db.Get("internal.clients.byId."+h.clientId, misc.NewDynamicVar(-1))
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
		h.db.DeleteValue("internal.clients.byId."+h.clientId, true)
	})
}
