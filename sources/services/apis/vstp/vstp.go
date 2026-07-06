package vstp

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"rtonello/vss/sources/controller"
	"rtonello/vss/sources/misc"
	tcpserver "rtonello/vss/sources/misc/evttcp/server"
	"rtonello/vss/sources/misc/logger"
	// storage removed: VSTP must use controller API only
)

const (
	SEND_SERVER_INFO_AND_CONFS             = "serverinfo"
	PING                                   = "ping"
	PONG                                   = "pong"
	SUGGEST_NEW_CLI_ID                     = "sugestednewid"
	CHANGE_OR_CONFIRM_CLI_ID               = "setid"
	TOTAL_VARIABLES_ALREADY_BEING_OBSERVED = "aoc"
	RESPONSE_BEGIN                         = "beginresponse"
	RESPONSE_END                           = "endresponse"

	SET_VAR          = "set"
	DELETE_VAR       = "delete"
	GET_VAR          = "get"
	GET_VAR_RESPONSE = "varvalue"

	SET_JSON_VARS = "setjson"

	TRANSACION_BEGIN          = "begintransaction"
	TRANSACTION_SET_VAR       = "transactionset"
	TRANSACTION_DELETE_VAR    = "transactiondelete"
	TRANSACTION_SET_JSON_VARS = "transactionsetjson"
	TRANSACTION_COMMIT        = "committransaction"
	TRANSACTION_ROLLBACK      = "rollbacktransaction"

	SUBSCRIBE_VAR                = "subscribe"
	UNSUBSCRIBE_VAR              = "unsubscribe"
	VAR_CHANGED                  = "varchanged"
	GET_CHILDS                   = "getchilds"
	GET_CHILDS_RESPONSE          = "childs"
	LOCK_VAR                     = "lock"
	UNLOCK_VAR                   = "unlock"
	UNLOCK_VAR_DONE              = "unlockdone"
	SERVER_BEGIN_HEADERS         = "beginserverheaders"
	SERVER_END_HEADERS           = "endserverheaders"
	HELP                         = "help"
	SET_TELNET_SESSION           = "telnet"
	CHECK_VAR_LOCK_STATUS        = "lockstatus"
	CHECK_VAR_LOCK_STATUS_RESULT = "lockstatusresult"
	ERROR                        = "error"
	SUCESS                       = "success"
)

// VSTP is a text protocol API implementation (a Go port of the C++ VSTP API).
// It implements controller.ApiInterface so it can be registered into TheController.
type VSTP struct {
	ctrl            controller.IController
	apiId           string
	clientsById     map[string]tcpserver.ITCPClient
	clientsByTcpCli map[tcpserver.ITCPClient]string
	clientsLock     sync.RWMutex

	incomingDataBuffer map[tcpserver.ITCPClient]string
	log                logger.INamedLogger
	cliIdCount         int64
}

// NewVSTP constructs and starts a VSTP server on the given port and registers
// itself on the provided controller (calls ApiStarted).
func NewVSTP(port int, ctrl controller.IController, logger logger.ILogger) (*VSTP, error) {
	v := &VSTP{
		ctrl:               ctrl,
		apiId:              "VSTPAPI",
		clientsById:        make(map[string]tcpserver.ITCPClient),
		clientsByTcpCli:    make(map[tcpserver.ITCPClient]string),
		incomingDataBuffer: make(map[tcpserver.ITCPClient]string),
		log:                logger.GetNamedLogger("Apis::VSTP"),
		cliIdCount:         0,
	}

	// create tcp server and set callbacks
	srv, err := tcpserver.New(tcpserver.WithPort(port))
	if err != nil {
		return nil, err
	}

	srv.SetOnCLientConnect(func(cli tcpserver.ITCPClient) {
		// assign unique id
		uid := misc.CreateFunName(1, 1) //strconv.FormatInt(time.Now().UnixNano(), 10)
		uid = strconv.FormatInt(v.cliIdCount, 10) + "-" + uid
		v.cliIdCount++
		// if underlying TCPClient has Tags, try set via type assertion
		if tc, ok := cli.(interface{ Tags() map[string]string }); ok {
			// ignore; Tags accessor not available - use send id message instead
			_ = tc
		}
		v.clientsLock.Lock()
		v.clientsById[uid] = cli
		v.clientsByTcpCli[cli] = uid
		v.incomingDataBuffer[cli] = ""
		v.clientsLock.Unlock()
		// send welcome info and id
		v.sentInitialHeaders(cli, uid)

		v.log.Info("New VSTP client connected, assigned id: " + uid)
	})

	srv.SetOnClientDisconnect(func(cli tcpserver.ITCPClient) {
		// remove from maps using secondary index
		v.clientsLock.Lock()
		if id, ok := v.clientsByTcpCli[cli]; ok {
			v.log.Info("VSTP client " + id + " disconnected")
			delete(v.clientsById, id)
			delete(v.clientsByTcpCli, cli)
		} else {
			// fallback: try to find by scanning (compat)
			for id, c := range v.clientsById {
				if c == cli {
					v.log.Info("VSTP client " + id + " disconnected")
					delete(v.clientsById, id)
					break
				}
			}
		}
		delete(v.incomingDataBuffer, cli)
		v.clientsLock.Unlock()
	})

	srv.SetOnClientData(func(cli tcpserver.ITCPClient, data []byte) {
		v.clientsLock.Lock()
		if _, ok := v.incomingDataBuffer[cli]; !ok {
			v.log.Warning("Received data from unknown client. The buffer will be initialized, but errors may occur because client was not properly initialized.")
			v.incomingDataBuffer[cli] = ""
			uid := "problematic " + misc.CreateFunName(1, 1)
			v.clientsLock.Lock()
			v.clientsById[uid] = cli
			v.clientsByTcpCli[cli] = uid
			v.clientsLock.Unlock()
		}

		buf := v.incomingDataBuffer[cli]
		v.clientsLock.Unlock()
		buf += string(data)
		// take lines
		for {
			if idx := strings.IndexByte(buf, '\n'); idx >= 0 {
				line := buf[:idx]
				buf = buf[idx+1:]
				line = strings.TrimRight(line, "\r")
				go v.processReceivedMessage(cli, line)
			} else {
				break
			}
		}
		v.clientsLock.Lock()
		v.incomingDataBuffer[cli] = buf
		v.clientsLock.Unlock()
	})

	// register to controller
	if ctrl != nil {
		ctrl.ApiStarted(v)
	}

	return v, nil
}

func (v *VSTP) sentInitialHeaders(cli tcpserver.ITCPClient, uid string) {
	v.protocolWrite(cli, SERVER_BEGIN_HEADERS, "", "")

	// minimal info: protocol and system version if controller available
	v.protocolWrite(cli, SEND_SERVER_INFO_AND_CONFS, "", "PROTOCOL VERSION=2.0.0")
	if v.ctrl != nil {
		v.protocolWrite(cli, SEND_SERVER_INFO_AND_CONFS, "", "VSS VERSION="+v.ctrl.GetSystemVersion())
	}

	v.protocolWrite(cli, SUGGEST_NEW_CLI_ID, "", uid)

	v.protocolWrite(cli, SERVER_END_HEADERS, "", "")
}

// ApiInterface implementation -------------------------------------------------
func (v *VSTP) GetApiId() string {
	return v.apiId
}

func (v *VSTP) CheckAlive(clientId string) bool {
	v.clientsLock.RLock()
	_, ok := v.clientsById[clientId]
	v.clientsLock.RUnlock()
	return ok
}

func (v *VSTP) NotifyClient(clientId string, varsAndValues []misc.Tuple[string]) bool {

	v.clientsLock.RLock()
	cli, ok := v.clientsById[clientId]
	v.clientsLock.RUnlock()
	if !ok {
		return false
	}
	// send each tuple as VAR_CHANGED
	for _, t := range varsAndValues {

		if t.Len() < 3 {
			v.log.Warning("VSTP NotifyClient: invalid tuple length for client " + clientId)
			continue
		}

		meta := t.At(1)
		metaStr := ""
		if meta != "" {
			metaStr = "(" + meta + ")"
		}
		toSend := t.At(0) + metaStr + "=" + t.At(2)

		err := v.protocolWrite(cli, VAR_CHANGED, "", toSend)
		if err != nil {
			v.log.Error("Failed to send VAR_CHANGED to client:", err)
			return false
		}
	}
	return true
}

// Helpers ---------------------------------------------------------------------
func (v *VSTP) protocolWrite(cli tcpserver.ITCPClient, cmd, cmdId, data string) error {
	if cmdId != "" {
		cmd = cmd + "(" + cmdId + ")"
	}

	buf := cmd + ":" + data + "\n"
	//cliId := v.clientsByTcpCli[cli]
	//v.log.Debug("VSTP protocolWrite sending to " + cliId + ": " + buf)
	//fmt.Println("VSTP protocolWrite sending to", cliId, ":", buf)
	return cli.SendString(buf)
}

// processReceivedMessage parses and executes the protocol command
func (v *VSTP) processReceivedMessage(cli tcpserver.ITCPClient, message string) {
	// message form: command(id):payload
	cmd, cmdId, payload := separateCommandIdAndPayload(message)
	cmd = strings.TrimSpace(cmd)
	payload = strings.TrimSpace(payload)

	if v.ctrl == nil {
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, cmd)
		v.protocolWrite(cli, ERROR, cmdId, "vss error: no controller available")
		v.protocolWrite(cli, RESPONSE_END, cmdId, cmd)
		return
	}

	switch cmd {
	case SET_VAR:
		// payload: var=val
		name, val := separateKeyAndValue(payload)
		if name != "" {
			// use controller to set the variable and wait for result
			err := <-v.ctrl.SetVar(name, misc.NewDynamicVar(val))
			v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, SET_VAR)
			if err != nil {
				v.protocolWrite(cli, ERROR, cmdId, "error: set error: "+err.Error())
			} else {
				v.protocolWrite(cli, SUCESS, cmdId, "")
			}
			v.protocolWrite(cli, RESPONSE_END, cmdId, SET_VAR)
		} else {
			v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, SET_VAR)
			v.protocolWrite(cli, ERROR, cmdId, "set error: invalid variable name")
			v.protocolWrite(cli, RESPONSE_END, cmdId, SET_VAR)
		}
	case DELETE_VAR:
		name := payload
		err := <-v.ctrl.DelVar(name)
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, DELETE_VAR)
		if err != nil {
			v.protocolWrite(cli, ERROR, cmdId, err.Error())
		} else {
			v.protocolWrite(cli, SUCESS, cmdId, "")
		}
		v.protocolWrite(cli, RESPONSE_END, cmdId, DELETE_VAR)
	case GET_VAR:
		name := payload
		// read value(s) and reply
		res := <-v.ctrl.GetVars(name, misc.NewDynamicVar(""))
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, GET_VAR)
		if res.Err != nil {
			v.protocolWrite(cli, ERROR, cmdId, "get error: "+res.Err.Error())
		} else {
			for _, tup := range res.Values {
				nameDV := tup.At(0)
				valDV := tup.At(1)
				nameStr := (&nameDV).GetString()
				valStr := (&valDV).GetString()
				v.protocolWrite(cli, GET_VAR_RESPONSE, cmdId, nameStr+"="+valStr)
			}
		}
		v.protocolWrite(cli, RESPONSE_END, cmdId, GET_VAR)
	case SUBSCRIBE_VAR:
		// payload may contain metadata like name(meta)
		name, meta := separateNameAndMetadata(payload)
		// register observation on controller
		// we need a client id: use the secondary index for O(1) lookup
		v.clientsLock.Lock()
		cid, ok := v.clientsByTcpCli[cli]
		if !ok || cid == "" {
			cid = strconv.FormatInt(time.Now().UnixNano(), 10)
			v.clientsById[cid] = cli
			v.clientsByTcpCli[cli] = cid
		}
		v.clientsLock.Unlock()
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, SUBSCRIBE_VAR)
		v.ctrl.ObserveVar(name, cid, meta, v)
		v.protocolWrite(cli, RESPONSE_END, cmdId, SUBSCRIBE_VAR)
		v.log.Info("Client subscribed to variable: " + name + " (metadata: " + meta + ")")
	case UNSUBSCRIBE_VAR:
		name, meta := separateNameAndMetadata(payload)
		// find client id via secondary index
		v.clientsLock.RLock()
		cid, ok := v.clientsByTcpCli[cli]
		v.clientsLock.RUnlock()
		if ok && cid != "" {
			v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, UNSUBSCRIBE_VAR)
			v.ctrl.StopObservingVar(name, cid, meta, v)
			v.protocolWrite(cli, SUCESS, cmdId, "")
			v.protocolWrite(cli, RESPONSE_END, cmdId, UNSUBSCRIBE_VAR)
		}
	case LOCK_VAR:
		// payload: var[=timeout]
		name, val := separateKeyAndValue(payload)
		timeout := uint(^uint(0))
		if val != "" {
			if t, err := strconv.Atoi(val); err == nil {
				timeout = uint(t)
			}
		}

		err := <-v.ctrl.LockVar(name, timeout)
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, LOCK_VAR)
		if err != nil {
			v.protocolWrite(cli, ERROR, cmdId, err.Error())
		} else {
			v.protocolWrite(cli, SUCESS, cmdId, "")
		}
		v.protocolWrite(cli, RESPONSE_END, cmdId, LOCK_VAR)
	case UNLOCK_VAR:
		name, _ := separateKeyAndValue(payload)
		if v.ctrl != nil {
			<-v.ctrl.UnlockVar(name)
		}
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, UNLOCK_VAR)
		v.protocolWrite(cli, UNLOCK_VAR_DONE, cmdId, name)
		v.protocolWrite(cli, RESPONSE_END, cmdId, UNLOCK_VAR)

	case SET_JSON_VARS:
		v.setJsonVars(cli, cmdId, payload)

	case PING:
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, PING)
		v.protocolWrite(cli, PONG, cmdId, "")
		v.protocolWrite(cli, RESPONSE_END, cmdId, PING)

	// client sends its requested id to resume a previous session
	case CHANGE_OR_CONFIRM_CLI_ID:
		// payload is the requested id
		newId := payload
		// update mapping: remove old id pointing to this cli and set newId->cli
		{
			// get oldId via secondary index and update both maps
			v.clientsLock.Lock()
			oldId := v.clientsByTcpCli[cli]
			if oldId != "" {
				delete(v.clientsById, oldId)
			}

			v.log.Info("Client requested id change: " + oldId + " -> " + newId)
			v.clientsById[newId] = cli
			v.clientsByTcpCli[cli] = newId
			v.clientsLock.Unlock()
		}
		// notify controller that client connected (so it can re-send observing vars)
		_, observing := v.ctrl.ClientConnected(newId, v)
		// send total observed count
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, CHANGE_OR_CONFIRM_CLI_ID)
		v.protocolWrite(cli, TOTAL_VARIABLES_ALREADY_BEING_OBSERVED, cmdId, strconv.Itoa(observing))
		v.protocolWrite(cli, RESPONSE_END, cmdId, CHANGE_OR_CONFIRM_CLI_ID)

	case GET_CHILDS:
		// payload is parent var
		parent := payload
		childs := <-v.ctrl.GetChildsOfVar(parent)
		// join by comma
		resp := strings.Join(childs, ",")
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, GET_CHILDS)
		v.protocolWrite(cli, GET_CHILDS_RESPONSE, cmdId, resp)
		v.protocolWrite(cli, RESPONSE_END, cmdId, GET_CHILDS)

	case CHECK_VAR_LOCK_STATUS:
		name := payload
		locked := <-v.ctrl.IsVarLocked(name)
		status := "unlocked"
		if locked {
			status = "locked"
		}
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, CHECK_VAR_LOCK_STATUS)
		v.protocolWrite(cli, CHECK_VAR_LOCK_STATUS_RESULT, cmdId, name+"="+status)
		v.protocolWrite(cli, RESPONSE_END, cmdId, CHECK_VAR_LOCK_STATUS)
	default:
		// ignore unknown
		v.log.Info("Unknown VSTP command received: " + cmd)
	}
}

func (v *VSTP) setJsonVars(cli tcpserver.ITCPClient, cmdId string, payload string) {
	if v.ctrl == nil {
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, SET_JSON_VARS)
		v.protocolWrite(cli, ERROR, cmdId, "vss error error: no controller available")
		v.protocolWrite(cli, RESPONSE_END, cmdId, SET_JSON_VARS)
		return
	}

	parent, jsonStr := separateKeyAndValue(payload)
	if parent == "" || jsonStr == "" {
		v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, SET_JSON_VARS)
		v.protocolWrite(cli, ERROR, cmdId, "setjson error: invalid payload format, expected parent=json")
		v.protocolWrite(cli, RESPONSE_END, cmdId, SET_JSON_VARS)
		return
	}

	//parse the json using encoding/json
	var data map[string]interface{}
	json.Unmarshal([]byte(jsonStr), &data)

	var toSet map[string]misc.DynamicVar = make(map[string]misc.DynamicVar)

	//declare extractNames (used recursively to flatten the json into var names and values)
	var extractNames func(parent string, data map[string]interface{})

	extractNames = func(parent string, data map[string]interface{}) {
		for k, v := range data {
			name := parent + "." + k
			switch val := v.(type) {
			case map[string]interface{}:
				extractNames(name, val)
			default:
				toSet[name] = misc.NewDynamicVar(val)
			}
		}
	}

	extractNames(parent, data)

	for name, val := range toSet {
		err := <-v.ctrl.SetVar(name, val)
		if err != nil {
			v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, SET_JSON_VARS)
			v.protocolWrite(cli, ERROR, cmdId, "setjson error: "+err.Error())
			v.protocolWrite(cli, RESPONSE_END, cmdId, SET_JSON_VARS)
		}
	}

	v.protocolWrite(cli, RESPONSE_BEGIN, cmdId, SET_JSON_VARS)
	v.protocolWrite(cli, SUCESS, cmdId, "")
	v.protocolWrite(cli, RESPONSE_END, cmdId, SET_JSON_VARS)
}

// small helpers ---------------------------------------------------------------
func separateKeyAndValue(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if strings.ContainsAny(string(s[i]), "=;: ") {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

func separateCommandIdAndPayload(s string) (string, string, string) {
	commandWithId, payload := separateKeyAndValue(s)
	command, id := separateNameAndMetadata(commandWithId)

	return command, id, payload
}

func separateNameAndMetadata(original string) (string, string) {
	if p := strings.IndexByte(original, '('); p >= 0 {
		name := original[:p]
		meta := original[p+1:]
		meta = strings.TrimSuffix(meta, ")")
		return name, meta
	}
	return original, ""
}
