package vsscli

const (
	INVALID_ACT string = ""

	// this package is sent by the server whenever this needs to inform server
	// states to clients. For example, when the TCP connection is stablished
	// with the server, it will send some useful information, like VSTP
	// protocol version, the server (VSS) version and the scape character that
	// should be used by clients.
	ACT_SEND_SERVER_INFO_AND_CONFS string = "serverinfo"

	//This package should be send to the server when you need to check if it is
	// working/active
	ACT_PING string = "ping"

	// this package is sent by the server in response to a 'PING'
	// COMMAND/ACTION
	ACT_PONG string = "pong"

	// imediately after the TCP connection be stablished, the server will send
	// this package to suggest a new client ID. If the client already have a
	// previous ID, it can ignore this new and ID and informe the server about
	//the old one by sending a 'CHANGE_OR_CONFIRM_CLI_ID' package
	ACT_SUGGEST_NEW_CLI_ID string = "sugestednewid"

	// When a client informs the server about a previous used ID, the server
	// will notify the client about all variables with observations assotiated
	// with this ID. This is very useful when a connection to the server is
	//being restored
	ACT_CHANGE_OR_CONFIRM_CLI_ID string = "setid"

	ACT_TOTAL_VARIABLES_ALREADY_BEING_OBSERVED string = "aoc"
	ACT_RESPONSE_BEGIN                         string = "beginresponse"
	ACT_RESPONSE_END                           string = "endresponse"
	ACT_SET_VAR                                string = "set"
	ACT_DELETE_VAR                             string = "delete"
	ACT_GET_VAR                                string = "get"
	ACT_GET_VAR_RESPONSE                       string = "varvalue"
	ACT_SUBSCRIBE_VAR                          string = "subscribe"
	ACT_UNSUBSCRIBE_VAR                        string = "unsubscribe"
	ACT_VAR_CHANGED                            string = "varchanged"
	ACT_GET_CHILDS                             string = "getchilds"
	ACT_GET_CHILDS_RESPONSE                    string = "childs"
	ACT_LOCK_VAR                               string = "lock"
	ACT_UNLOCK_VAR                             string = "unlock"
	ACT_LOCK_VAR_RESULT                        string = "lockresult"
	ACT_UNLOCK_VAR_DONE                        string = "unlockdone"
	ACT_SERVER_BEGIN_HEADERS                   string = "beginserverheaders"
	ACT_SERVER_END_HEADERS                     string = "endserverheaders"
	ACT_HELP                                   string = "help"
	ACT_SET_TELNET_SESSION                     string = "telnet"
	ACT_CHECK_VAR_LOCK_STATUS                  string = "lockstatus"
	ACT_CHECK_VAR_LOCK_STATUS_RESULT           string = "lockstatusresult"
	ACT_ERROR                                  string = "error"
)
