package vsscli

import (
	"errors"
	"fmt"
	"rtonello/vss/sources/misc"
	evttcpclient "rtonello/vss/sources/misc/evttcp/client"
	VarBasedRSM "rtonello/vss/sources/misc/goVssClient/varbasedrsm"
	"strconv"
	"strings"
	"sync"
	"time"
)

type vssCmdAndPayload struct {
	Cmd     string
	Payload string
}

type ConnectionState string

type varChangeObservingInfo struct {
	varName  string
	customId string
	callback func(string, misc.DynamicVar)
}

type VssCli struct {
	tcpClient             *evttcpclient.TCPClient
	connectionIsActive    bool
	sbuffer               string
	serverHeaders         map[string]string
	serverHeadersLock     sync.Mutex
	onDataLocker          sync.Mutex
	clientId              string
	Prefix                string
	CustomObserverIdCount int
	VarChangeObservings   map[int]varChangeObservingInfo
	vssReuseID            bool

	OnCommandReceived        *misc.Stream[vssCmdAndPayload]
	OnConnectionStateChanged *misc.Stream[VarBasedRSM.ConnectionStateInfo]
}

func NewVssCli(initOptions ...func(*VssCli)) *VssCli {
	tmp := VssCli{}
	tmp.connectionIsActive = false
	tmp.OnCommandReceived = misc.NewStream[vssCmdAndPayload](false)
	tmp.serverHeaders = map[string]string{}
	tmp.OnConnectionStateChanged = misc.NewStream[VarBasedRSM.ConnectionStateInfo](false)
	tmp.clientId = ""
	tmp.CustomObserverIdCount = 0
	tmp.VarChangeObservings = map[int]varChangeObservingInfo{}
	tmp.vssReuseID = true

	//apply options
	for _, option := range initOptions {
		option(&tmp)
	}

	tmp.initVarChangeObserver()

	return &tmp
}

func WithPrefix(prefix string) func(*VssCli) {
	return func(instance *VssCli) {
		instance.Prefix = prefix
	}
}

// the default behaviour is to reuse the id. It allow a better integration with vss server, avoid the need to
// redo the observations. If you use a false value, you will need to redo all observations after a reconnection
func WithReuseID(reuseID bool) func(*VssCli) {
	return func(instance *VssCli) {
		instance.vssReuseID = reuseID
	}
}

// connects to a VSS server. If the connection results in sucess, the library
// will begin monitoring the connection and try to reconnect automatically.
//
// If connection is stablished with sucess, 'nil' will be returnted (instance
// function returns an error, or nil). Otherwise, the connection error will be
// returned. If the connection results in sucess (error == nil), the server will
// monitor the connection and will retry the connection when it is lost.
//
// Note that the VssCli will just monitor and retry the connection if the first
// attemp to do it results in sucess.
func (instance *VssCli) Connect(serverAndPort string) error {
	instance.OnConnectionStateChanged.Stream(VarBasedRSM.ConnectionStateInfo{State: VarBasedRSM.Connecting, Description: "Conneting to the server '" + serverAndPort + "'"})

	instance.tcpClient = evttcpclient.NewTCPClient()
	instance.tcpClient.OnData = instance.handleNewData

	err := instance.tcpClient.Connect(evttcpclient.ConnectWithString(serverAndPort))
	if err != nil {
		instance.OnConnectionStateChanged.Stream(VarBasedRSM.ConnectionStateInfo{State: VarBasedRSM.Disconnected, Description: "Disconnected due a error: '" + err.Error() + "'"})
		return fmt.Errorf("error connecting to the server: %w", err)
	}

	err = instance.readHeaders(withSecondsTimeout(5))
	if err != nil {
		instance.OnConnectionStateChanged.Stream(VarBasedRSM.ConnectionStateInfo{State: VarBasedRSM.Disconnected, Description: "Disconnected due a error: '" + err.Error() + "'"})
		return fmt.Errorf("error reading headers: %w", err)
	}

	err = instance.exchangeIds()
	if err != nil {
		instance.OnConnectionStateChanged.Stream(VarBasedRSM.ConnectionStateInfo{State: VarBasedRSM.Disconnected, Description: "Disconnected due a error: '" + err.Error() + "'"})
		return fmt.Errorf("error exchange Ids with the server: %w", err)
	}

	instance.OnConnectionStateChanged.Stream(VarBasedRSM.ConnectionStateInfo{State: VarBasedRSM.Connected, Description: "Connected to: '" + serverAndPort + "'"})
	go instance.monitorConnection(serverAndPort)

	return nil
}

func (instance *VssCli) Var(name string) VarBasedRSM.Var {
	name = instance.Prefix + name
	return VarBasedRSM.NewVar(name, instance)
}

func (instance *VssCli) readHeaders(timeout time.Duration) error {

	sucess := instance.subscribeOnCommandWithATimeout(func() {}, func(pack vssCmdAndPayload) bool {
		if pack.Cmd == ACT_SERVER_END_HEADERS {
			return true
		} else if pack.Cmd != ACT_SERVER_BEGIN_HEADERS {
			//key, value := misc.SeparateKeyAndValue(pack.Payload, "=:; ")
			instance.serverHeadersLock.Lock()
			instance.serverHeaders[pack.Cmd] = pack.Payload
			instance.serverHeadersLock.Unlock()
		}
		return false

	}, timeout)

	if !sucess {
		return errors.New("timeout reached when reading headers from server")
	}

	return nil
}

func (instance *VssCli) exchangeIds() error {
	if instance.clientId == "" || !instance.vssReuseID {
		instance.serverHeadersLock.Lock()
		suggestedId, found := instance.serverHeaders[ACT_SUGGEST_NEW_CLI_ID]
		instance.serverHeadersLock.Unlock()
		if !found {
			return errors.New("suggested id send by server was not found")
		}

		instance.clientId = suggestedId
	}

	instance.sendToServer(packFromCmdAndPayload(ACT_CHANGE_OR_CONFIRM_CLI_ID, instance.clientId))

	return nil
}

func (instance *VssCli) Disconnect() {
	instance.connectionIsActive = false
	_ = instance.tcpClient.Disconnect()
	instance.OnConnectionStateChanged.Stream(VarBasedRSM.ConnectionStateInfo{State: VarBasedRSM.Disconnected, Description: "Disconnected, normal"})
}

func (instance *VssCli) monitorConnection(serverAndPort string) {
	instance.connectionIsActive = true

	instance.tcpClient.OnDisconnect = func() {
		//for instance.connectionIsActive && !instance.tcpClient.IsConnected(){
		if instance.connectionIsActive {
			instance.OnConnectionStateChanged.Stream(VarBasedRSM.ConnectionStateInfo{State: VarBasedRSM.Disconnected, Description: "Connection lost"})
		}

		for instance.connectionIsActive {
			err := instance.Connect(serverAndPort)
			if err == nil {
				break
			}
			time.Sleep(1 * time.Second)

		}
	}
}

func (instance *VssCli) handleNewData(data []byte, size int) {
	instance.onDataLocker.Lock()
	instance.sbuffer += string(data)
	pos := strings.Index(instance.sbuffer, "\n")
	for pos > -1 {
		pack := string(instance.sbuffer[0:pos])
		instance.sbuffer = string(instance.sbuffer[pos+1 : len(instance.sbuffer)])
		pos = strings.Index(instance.sbuffer, "\n")

		instance.processReceivedPack(pack)

	}
	instance.onDataLocker.Unlock()
}

func (instance *VssCli) processReceivedPack(pack string) {
	actionStr, payload, found := strings.Cut(pack, ":")
	if !found {
		//report error

		return
	}

	instance.OnCommandReceived.Stream(vssCmdAndPayload{Cmd: actionStr, Payload: payload})
}

// will send all data that comes from OnCommandReceived stream to 'f' function.
// The 'f' function should return 'true' when the process is done.
// If the 'f' function return 'true' before the timeout be reached, instance func
// will return 'true', otherelse, will return 'false'.
// If you use 'withoutTimeout' function (or -1), instance func will only return
func (instance *VssCli) subscribeOnCommandWithATimeout(
	onReady func(),
	f func(vssCmdAndPayload) bool,
	timeout time.Duration,
) bool {

	done := make(chan struct{})
	var once sync.Once

	var subscribeId int
	subscribeId = instance.OnCommandReceived.Subscribe(func(data vssCmdAndPayload) {
		if f(data) {
			once.Do(func() {
				close(done)
			})
		}
	})

	onReady()

	var timeoutCh <-chan time.Time
	if timeout >= 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	var ok bool
	select {
	case <-done:
		ok = true
	case <-timeoutCh:
		ok = false
	}

	instance.OnCommandReceived.Unsubscribe(subscribeId)
	return ok
}

// Gets the value of a variable. Returns a list of variables (you can,
// get variables using '*' char.)
func (instance *VssCli) GetVar(name string) ([]VarBasedRSM.KeyValue, error) {
	name = instance.Prefix + name

	result := []VarBasedRSM.KeyValue{}
	resultError := error(nil)

	responseInTime := instance.subscribeOnCommandWithATimeout(func() {
		dt := packFromCmdAndPayload(ACT_GET_VAR, name)
		err := instance.sendToServer(dt)

		if err != nil {
			resultError = fmt.Errorf("error sending 'GetVar' command to the server: %w", err)
		}

	}, func(data vssCmdAndPayload) bool {
		if data.Cmd == ACT_RESPONSE_END && strings.Contains(data.Payload, ACT_GET_VAR) && strings.Contains(data.Payload, name) {
			return true
		} else if data.Cmd == ACT_GET_VAR_RESPONSE {
			//find * position name
			wilfPos := strings.Index(name, "*")

			//remove the * from the nanme
			if wilfPos > -1 {
				name = name[0:wilfPos]
			}

			if strings.Contains(data.Payload, name) {
				key, value := misc.SeparateKeyAndValue(data.Payload, "=:; ")
				result = append(result, VarBasedRSM.KeyValue{Key: key, Value: misc.NewDynamicVar(misc.WithString(value))})
			}
		}
		return false
	}, withSecondsTimeout(5))

	if !responseInTime {
		return result, errors.New("timeout reached")
	}

	return result, resultError

}

func (instance *VssCli) GetOneVar(name string) (VarBasedRSM.KeyValue, error) {
	//do not add the prefix to the name, because the GetVar function will do it
	result, err := instance.GetVar(name)
	if err != nil {
		return VarBasedRSM.KeyValue{}, err
	}

	if len(result) == 0 {
		return VarBasedRSM.KeyValue{}, errors.New("no variables returned by the server")
	}

	return result[0], nil
}

func (instance *VssCli) DeleteVar(name string) error {
	name = instance.Prefix + name
	dt := packFromCmdVarnameAndValue(ACT_DELETE_VAR, name, misc.NewEmptyDynamicVar())
	returnError := error(nil)

	responseInTime := instance.subscribeOnCommandWithATimeout(func() {
		err := instance.sendToServer(dt)
		if err != nil {
			returnError = fmt.Errorf("error sending 'DeleteVar' command to the server: %w", err)
		}
	}, func(toCheck vssCmdAndPayload) bool {
		return toCheck.Cmd == ACT_RESPONSE_END && strings.Contains(toCheck.Payload+"\n", dt)
	}, withSecondsTimeout(5))

	if !responseInTime {
		return errors.New("DeleteVar command timeout reached")
	}

	return returnError
}

// Changes the value of a variable
func (instance *VssCli) SetVar(varName string, value misc.DynamicVar) error {
	varName = instance.Prefix + varName
	dt := packFromCmdVarnameAndValue(ACT_SET_VAR, varName, value)
	returnError := error(nil)

	responseInTime := instance.subscribeOnCommandWithATimeout(func() {
		err := instance.sendToServer(dt)
		if err != nil {
			returnError = fmt.Errorf("error sending 'SetVar' command to the server: %w", err)
		}

	}, func(toCheck vssCmdAndPayload) bool {
		return toCheck.Cmd == ACT_RESPONSE_END && strings.Contains(toCheck.Payload+"\n", dt)
	}, withSecondsTimeout(2))

	if !responseInTime {
		return errors.New("SetVar command timeout reached")
	}

	return returnError
}

// Begin to observate a variable. Whenever the variable value is changed, the
// function 'f' will be called with the new value. The name of the variable
// also is informed and instance is very useful when you observate variables using
// the '*' char. You can, also, use the optionals functions 'withoutVarName'
// and 'WithVarname' to specify the 'f' argument
func (instance *VssCli) Subscribe(varName string, callback func(varName string, newValue misc.DynamicVar)) (int, error) {
	varName = instance.Prefix + varName
	cId := instance.CustomObserverIdCount
	cIdS := strconv.Itoa(cId)
	instance.CustomObserverIdCount++

	instance.VarChangeObservings[cId] = varChangeObservingInfo{varName: varName, customId: cIdS, callback: callback}

	err := instance.sendToServer(packFromCmdAndPayload(ACT_SUBSCRIBE_VAR, varName+"("+cIdS+")"))
	if err != nil {
		instance.Unsubscribe(cId)
		return -1, fmt.Errorf("error sending var subscription request to the server: %w", err)
	}

	return cId, nil
}

func (instance *VssCli) Unsubscribe(subscriptionID int) error {
	_, found := instance.VarChangeObservings[subscriptionID]
	if !found {
		return errors.New("subscription id not found")
	}

	err := instance.sendToServer(
		packFromCmdAndPayload(
			ACT_UNSUBSCRIBE_VAR,
			instance.VarChangeObservings[subscriptionID].varName+"("+instance.VarChangeObservings[subscriptionID].customId+")",
		),
	)
	delete(instance.VarChangeObservings, subscriptionID)

	if err != nil {
		return fmt.Errorf("error sending var unsubscription request to the server: %w", err)
	}

	return nil
}

func (instance *VssCli) initVarChangeObserver() {
	instance.OnCommandReceived.Subscribe(func(data vssCmdAndPayload) {
		if data.Cmd == ACT_VAR_CHANGED {
			varNameWithCustomId, newValue := misc.SeparateKeyAndValue(data.Payload, "=:;")
			name, customId := instance.separateNameAndMetadata(varNameWithCustomId)

			customIdInt, err := strconv.Atoi(customId)
			if err != nil {
				return
			}

			_, found := instance.VarChangeObservings[customIdInt]
			if !found {
				return
			}

			callback := instance.VarChangeObservings[customIdInt].callback
			//remove the prefix from name
			name = name[len(instance.Prefix):]
			callback(name, misc.NewDynamicVar(misc.WithString(newValue)))
		}
	})

}

// Locks a variable in the VSS server. When a variable is locked, no can
// changes its value until it is unlocked. The function 'LockVar' will wait to
// lock the variable by the amount of milisseconds specified in 'timeout'.
// If you want to not use a timeout (and wait by an "infinite" time), you can
// use 'withoutTimeout()' function (or just specify the value -1) for the
// argument 'timeout'
func (instance *VssCli) Lock(lockName string, timeoutMs int) error {

	lockName = instance.Prefix + lockName
	returnError := error(nil)

	responseInTime := instance.subscribeOnCommandWithATimeout(func() {
		dt := packFromCmdVarnameAndValue(ACT_LOCK_VAR, lockName, misc.NewDynamicVar(misc.WithInt(int64(timeoutMs))))
		err := instance.sendToServer(dt)
		if err != nil {
			returnError = fmt.Errorf("error sending 'LockVar' command to the server: %w", err)
		}

	}, func(toCheck vssCmdAndPayload) bool {
		if toCheck.Cmd == ACT_LOCK_VAR_RESULT {
			if strings.Contains(toCheck.Payload, lockName) {
				if strings.Contains(toCheck.Payload, "success") {
					return true
				} else {
					returnError = errors.New("error locking var: " + lockName)
					return true
				}
			}
		}
		return false
	}, withSecondsTimeout(5))

	if !responseInTime {
		return errors.New("LockVar command timeout reached")
	}

	return returnError
}

// UnLocks a variable in the VSS server.
func (instance *VssCli) Unlock(lockName string) error {
	lockName = instance.Prefix + lockName
	returnError := error(nil)

	responseInTime := instance.subscribeOnCommandWithATimeout(func() {
		dt := packFromCmdVarnameAndValue(ACT_UNLOCK_VAR, lockName, misc.NewEmptyDynamicVar())
		err := instance.sendToServer(dt)
		if err != nil {
			returnError = fmt.Errorf("error sending 'UnlockVar' command to the server: %w", err)
		}
	}, func(toCheck vssCmdAndPayload) bool {
		return toCheck.Cmd == ACT_UNLOCK_VAR_DONE && strings.Contains(toCheck.Payload+"\n", lockName)
	}, withSecondsTimeout(5))

	if !responseInTime {
		return errors.New("UnlockVar command timeout reached")
	}

	return returnError
}

func (instance *VssCli) RunLocked(lockName string, lockTimeoutMs int, failOnLockTimeout bool, callback func() error) error {
	lockName = instance.Prefix + lockName
	err := instance.Lock(lockName, lockTimeoutMs)
	if err != nil {
		if failOnLockTimeout {
			return fmt.Errorf("error locking the variable: %w", err)
		}
	}

	err = callback()
	if err != nil {
		return fmt.Errorf("error running the locked code: %w", err)
	}

	err = instance.Unlock(lockName)
	if err != nil {
		return fmt.Errorf("error unlocking the variable: %w", err)
	}

	return nil
}

func (instance *VssCli) GetChildNames(parentName string) ([]string, error) {
	parentName = instance.Prefix + parentName
	result := []string{}
	resultError := error(nil)

	responseInTime := instance.subscribeOnCommandWithATimeout(func() {
		dt := packFromCmdAndPayload(ACT_GET_CHILDS, parentName)
		err := instance.sendToServer(dt)

		if err != nil {
			resultError = fmt.Errorf("error sending 'GetChilds' command to the server: %w", err)
		}
	}, func(data vssCmdAndPayload) bool {
		if data.Cmd == ACT_RESPONSE_END && strings.Contains(data.Payload, ACT_GET_CHILDS) && strings.Contains(data.Payload, parentName) {
			return true
		} else if data.Cmd == ACT_GET_CHILDS_RESPONSE {
			//split by ','
			result = strings.Split(data.Payload, ",")
		}
		return false
	}, withSecondsTimeout(5))

	if !responseInTime {
		return result, errors.New("timeout reached")
	}

	return result, resultError
}

func (instance *VssCli) separateNameAndMetadata(nameWithMetadata string) (string, string) {
	before, after, found := strings.Cut(nameWithMetadata, "(")
	if found {
		if (after != "") && (after[len(after)-1] == ')') {
			after = after[0 : len(after)-1]
		}

		return before, after
	}

	return nameWithMetadata, ""
}

func (instance *VssCli) GetConnectionStateChangedEvent() *misc.Stream[VarBasedRSM.ConnectionStateInfo] {
	return instance.OnConnectionStateChanged
}

func (instance *VssCli) GetQueue(queueName string) (VarBasedRSM.VarBasedQueue, error) {
	//note.. Do not add prefix (it will be added when queue calls the VssCli functions)
	//return NewVssQueue(WithExistingVssCli(instance), queueName)
	return VarBasedRSM.NewQueue(instance, queueName)
}

func (instance *VssCli) sendToServer(data string) error {
	err := instance.tcpClient.SendString(data)
	if err != nil {
		return fmt.Errorf("error sending data to the server: %w", err)
	}

	return nil
}

func packFromCmdAndPayload(cmd string, payload string) string {
	return cmd + ":" + payload + "\n"
}

func packFromCmdVarnameAndValue(cmd string, varname string, value misc.DynamicVar) string {
	return cmd + ":" + varname + "=" + value.GetString() + "\n"
}

// instance function is used in the VssClie function that have timeout.
// When called, the value -1 is returned (-1 is the value used by VssCli for
// wait for infinity time)
//func withoutTimeout() int {
//	return -1
//}

func withMilisecondsTimeout(miliseconds int) time.Duration {
	return time.Duration(miliseconds) * time.Millisecond
}

func withSecondsTimeout(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

// Generates a function that can be used to listen to variables without receive
// the 'varName'
//func withoutVarName(f func(newValue misc.DynamicVar)) func(varName string, newValue misc.DynamicVar) {
//	return func(varName string, newValue misc.DynamicVar) {
//		f(newValue)
//	}
//}

// To be honest, use instance function is not needed. You can just inform you
// desired function. But you can use it to a better readable code
//func withVarName(f func(varName string, newValue misc.DynamicVar)) func(varName string, newValue misc.DynamicVar) {
//	return func(varName string, newValue misc.DynamicVar) {
//		f(varName, newValue)
//	}
//}
