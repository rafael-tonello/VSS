package httpapi

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"rtonello/vss/sources/controller"
	"rtonello/vss/sources/misc"
)

// HTTPAPI is a lightweight Go port of the original C++ Http API. It implements
// apis.IApi so it can be registered on the controller. It exposes simple
// REST endpoints for GET/POST/DELETE and a WebSocket endpoint for subscriptions.
type HTTPAPI struct {
	port       int
	ctrl       controller.IController
	apiID      string
	upgrader   websocket.Upgrader
	clients    map[string]*websocket.Conn
	connToID   map[*websocket.Conn]string
	clientsMtx sync.Mutex
}

// New creates and starts the HTTP API on the given port and registers the API
// with the controller (calls ApiStarted).
func New(port int, ctrl controller.IController) (*HTTPAPI, error) {
	h := &HTTPAPI{
		port:     port,
		ctrl:     ctrl,
		apiID:    "HTTPAPI",
		upgrader: websocket.Upgrader{},
		clients:  make(map[string]*websocket.Conn),
		connToID: make(map[*websocket.Conn]string),
	}

	// register with controller
	if ctrl != nil {
		ctrl.ApiStarted(h)
	}

	// routes: handle all paths so GET/POST/DELETE map directly to /path/to/var
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.handlePath)

	go func() {
		_ = http.ListenAndServe(":"+strconv.Itoa(port), mux)
	}()

	return h, nil
}

// GetAPIID returns the API identifier.
func (h *HTTPAPI) GetAPIID() string {
	return h.apiID
}

// CheckAlive checks if a client with the given ID is currently connected.
func (h *HTTPAPI) CheckAlive(clientID string) bool {
	h.clientsMtx.Lock()
	defer h.clientsMtx.Unlock()
	_, ok := h.clients[clientID]
	return ok
}

// NotifyClient sends vars changes to the websocket client. Returns true when
// the client appears to be alive and the send succeeded.
func (h *HTTPAPI) NotifyClient(clientID string, varsAndValues []misc.Tuple[string]) bool {
	h.clientsMtx.Lock()
	conn, ok := h.clients[clientID]
	h.clientsMtx.Unlock()
	if !ok || conn == nil {
		return false
	}

	// build a JSON payload: [{"name":"...","meta":"...","value":"..."}, ...]
	out := []map[string]string{}
	for _, t := range varsAndValues {
		m := map[string]string{"name": t.At(0), "meta": t.At(1), "value": t.At(2)}
		out = append(out, m)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return false
	}

	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		// remove dead connection
		h.clientsMtx.Lock()
		if cid, ok := h.connToID[conn]; ok {
			delete(h.clients, cid)
			delete(h.connToID, conn)
		}
		h.clientsMtx.Unlock()
		return false
	}
	return true
}

// handlePath maps HTTP methods and websocket upgrades on any path to variable
// operations. Example: GET /a/b/c -> get var "a.b.c"; POST /a/b/c -> set var
// "a.b.c"; DELETE /a/b/c -> delete var "a.b.c"; WebSocket upgrade to the
// same path will subscribe to that var.
func (h *HTTPAPI) handlePath(w http.ResponseWriter, r *http.Request) {
	// Derive var name from path
	// strip leading '/'
	path := strings.TrimPrefix(r.URL.Path, "/")
	varName := strings.ReplaceAll(path, "/", ".")
	// if root path or empty, reject
	if varName == "" {
		http.Error(w, "variable path required", http.StatusNotFound)
		return
	}

	// Check for websocket upgrade
	if strings.ToLower(r.Header.Get("Connection")) == "upgrade" && strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
		h.handleWSForVar(w, r, varName)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetByName(w, r, varName)
	case http.MethodPost:
		h.handlePostByName(w, r, varName)
	case http.MethodDelete:
		h.handleDeleteByName(w, r, varName)
	case "LOCK":
		h.handleLockByName(w, r, varName)
	case "UNLOCK":
		h.handleUnlockByName(w, r, varName)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *HTTPAPI) handleGetByName(w http.ResponseWriter, r *http.Request, name string) {
	q := r.URL.Query()
	exporter := strings.ToLower(q.Get("export"))
	if exporter == "" {
		exporter = "json"

		//check the accept header
		accept := r.Header.Get("Accept")
		if strings.Contains(accept, "text/plain") {
			exporter = "plain"
		}
	}

	pretty := strings.ToLower(q.Get("pretty")) == "true"
	full := strings.ToLower(q.Get("fullnames")) == "true"

	//check if is a lock of unlock operations
	if strings.ToLower(q.Get("action")) == "lock" {
		//convert locktimeout query param to a header
		if lockTimeout := q.Get("locktimeout"); lockTimeout != "" {
			r.Header.Set("Lock-Timeout", lockTimeout)
		}
		h.handleLockByName(w, r, name)
		return

	} else if strings.ToLower(q.Get("action")) == "unlock" {
		h.handleUnlockByName(w, r, name)
		return
	}

	res := <-h.ctrl.GetVars(name, misc.NewDynamicVar(""))
	if res.Err != nil {
		http.Error(w, res.Err.Error(), http.StatusInternalServerError)
		return
	}
	vals := res.Values

	if exporter == "plain" {
		lines := []string{}
		base := trimWildcardBase(name)
		for _, t := range vals {
			nameDV := t.At(0)
			valDV := t.At(1)
			nameStr := (&nameDV).GetString()
			if !full && base != "" && strings.HasPrefix(nameStr, base+".") {
				nameStr = strings.TrimPrefix(nameStr, base+".")
			}
			lines = append(lines, nameStr+"="+(&valDV).GetString())
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Join(lines, "\n")))
		return
	} else if exporter != "json" {
		http.Error(w, "invalid export format (use json or plain)", http.StatusBadRequest)
		return
	}

	// Build nested JSON structure from dot-separated variable names.
	root := make(map[string]any)
	for _, t := range vals {
		nameDV := t.At(0)
		valDV := t.At(1)
		fullName := (&nameDV).GetString()
		if !full {
			base := trimWildcardBase(name)
			if base == fullName {
				if len(vals) == 1 {
					//find last dot
					if idx := strings.LastIndex(fullName, "."); idx >= 0 {
						//serialize single json value (not object) for root-level var
						ttype := valDV.InferType()
						switch ttype {
						case reflect.Int:
							bytesOutput := []byte(strconv.FormatInt(valDV.GetInt64(), 10))
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(bytesOutput)
							return
						case reflect.Float64:
							bytesOutput := []byte(strconv.FormatFloat(valDV.GetFloat64(), 'f', -1, 64))
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(bytesOutput)
							return
						case reflect.Bool:
							bytesOutput := []byte("trye")
							if !valDV.GetBool() {
								bytesOutput = []byte("false")
							} else {
							}
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(bytesOutput)
							return
						default:
							//string
							bytesOutput := []byte("\"" + valDV.GetString() + "\"")
							w.Header().Set("Content-Type", "application/json")
							w.WriteHeader(http.StatusOK)
							_, _ = w.Write(bytesOutput)
							return
						}
					}
					//else, let fullName intact (get root values - with no parent, for example)
				} else {
					fullName = "_value"
				}
			} else if base != "" && strings.HasPrefix(fullName, base+".") {
				fullName = strings.TrimPrefix(fullName, base+".")
			}
		}
		parsed := parseDynamicVar(valDV)

		insertNested(root, strings.Split(fullName, "."), parsed)
	}
	// normalize numeric-keyed maps into arrays where appropriate
	final := normalizeArrays(root)

	var out []byte
	var err error
	if pretty {
		out, err = json.MarshalIndent(final, "", "  ")
	} else {
		out, err = json.Marshal(final)
	}
	if err != nil {
		http.Error(w, "encoding error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func (h *HTTPAPI) handlePostByName(w http.ResponseWriter, r *http.Request, name string) {
	// If Content-Type is application/json and body is an object, set multiple vars
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") {
		var obj map[string]interface{}
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&obj); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		h.setJson("", obj)

		w.WriteHeader(http.StatusNoContent)
		return
	}

	// otherwise treat body as raw value for the path var
	body, _ := ioutil.ReadAll(r.Body)
	val := string(body)
	if h.ctrl != nil {
		if err := <-h.ctrl.SetVar(name, misc.NewDynamicVar(val)); err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPAPI) setJson(parent string, obj map[string]interface{}) error {
	if parent != "" {
		parent = parent + "."
	}
	for key, value := range obj {
		//check if value is another map
		switch t := value.(type) {
		case map[string]interface{}:

			ret := h.setJson(parent+key, t)
			if ret != nil {
				//add context to error
				ret = fmt.Errorf("error setting var (with json content) %s: %w", parent+key, ret)
				return ret
			}
		default:
			//set value
			err := <-h.ctrl.SetVar(parent+key, misc.NewDynamicVar(value))
			if err != nil {
				return fmt.Errorf("error setting var %s: %w", parent+key, err)
			}
		}

	}
	return nil
}

func (h *HTTPAPI) handleDeleteByName(w http.ResponseWriter, _ *http.Request, name string) {
	if err := <-h.ctrl.DelVar(name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPAPI) handleLockByName(w http.ResponseWriter, r *http.Request, name string) {
	lockTimeout := r.Header.Get("Lock-Timeout")
	if lockTimeout == "" {
		lockTimeout = "30" // default to 30 seconds
	}
	timeout, err := strconv.Atoi(lockTimeout)
	if err != nil {
		http.Error(w, "invalid Lock-Timeout value", http.StatusBadRequest)
		return
	}

	if timeout < 1 {
		http.Error(w, "Lock-Timeout must be greater than 0", http.StatusBadRequest)
		return
	}

	err = <-h.ctrl.LockVar(name, uint(timeout*1000)) // convert to milliseconds
	if err != nil {
		if strings.Contains(err.Error(), "timeout") {
			http.Error(w, err.Error(), http.StatusRequestTimeout)
			return
		}

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPAPI) handleUnlockByName(w http.ResponseWriter, _ *http.Request, name string) {
	err := <-h.ctrl.UnlockVar(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Websocket handling --------------------------------------------------------
func (h *HTTPAPI) handleWSForVar(w http.ResponseWriter, r *http.Request, varName string) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	// assign client id
	cid := strconv.FormatInt(time.Now().UnixNano(), 10)
	h.clientsMtx.Lock()
	h.clients[cid] = conn
	h.connToID[conn] = cid
	h.clientsMtx.Unlock()

	// notify controller about connection
	if h.ctrl != nil {
		h.ctrl.ClientConnected(cid, h)
		// register observation immediately for this var
		h.ctrl.ObserveVar(varName, cid, "", h)
	}

	// read loop: allow clients to send unsubscribe/subscribe commands for other vars
	for {
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		if mt != websocket.TextMessage {
			continue
		}
		s := strings.TrimSpace(string(msg))
		if strings.HasPrefix(s, "subscribe:") {
			_, payload := separateKeyAndValue(s)
			name, meta := separateNameAndMetadata(payload)
			if h.ctrl != nil {
				h.ctrl.ObserveVar(name, cid, meta, h)
			}
		} else if strings.HasPrefix(s, "unsubscribe:") {
			_, payload := separateKeyAndValue(s)
			name, meta := separateNameAndMetadata(payload)
			if h.ctrl != nil {
				h.ctrl.StopObservingVar(name, cid, meta, h)
			}
		}
	}

	// cleanup on disconnect
	h.clientsMtx.Lock()
	delete(h.clients, cid)
	delete(h.connToID, conn)
	h.clientsMtx.Unlock()
	conn.Close()
}

// helpers -------------------------------------------------------------------
func trimWildcardBase(name string) string {
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, "*") {
		base := strings.TrimSuffix(name, "*")
		base = strings.TrimSuffix(base, ".")
		return base
	}
	return name
}

func separateKeyAndValue(s string) (string, string) {
	for i := 0; i < len(s); i++ {
		if strings.ContainsAny(string(s[i]), ":=") {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
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

// parseDynamicVar attempts to decode the internal DynamicVar string as JSON
// to preserve types (numbers, booleans, objects). If decoding fails it
// returns the raw string.
func parseDynamicVar(dv misc.DynamicVar) any {
	s := (&dv).GetString()
	if s == "" {
		return ""
	}
	var out any
	if err := json.Unmarshal([]byte(s), &out); err == nil {
		return out
	}
	// fallback: return original string
	return s
}

// insertNested inserts value into root under the dotted path parts. It
// converts primitive nodes into maps with a "_value" key when children are
// added later, ensuring that nodes with both value and children have their
// value under "_value".
func insertNested(root map[string]any, parts []string, value any) {
	node := root
	for i := 0; i < len(parts)-1; i++ {
		key := parts[i]
		if key == "" {
			continue
		}
		v, ok := node[key]
		if !ok {
			newmap := make(map[string]any)
			node[key] = newmap
			node = newmap
			continue
		}
		switch typed := v.(type) {
		case map[string]any:
			node = typed
		default:
			// primitive exists where we need a map: convert to map with _value
			newmap := make(map[string]any)
			newmap["_value"] = typed
			node[key] = newmap
			node = newmap
		}
	}

	last := parts[len(parts)-1]
	if last == "" {
		return
	}
	existing, ok := node[last]
	if !ok {
		node[last] = value
		return
	}
	if m, ok := existing.(map[string]any); ok {
		m["_value"] = value
		return
	}
	// overwrite primitive existing value
	node[last] = value
}

// normalizeArrays recursively walks the structure and converts maps whose
// keys are all non-negative integers into slices. Missing indices produce nil
// entries so array positions stay aligned with keys.
func normalizeArrays(v any) any {
	previusNum := -1
	switch t := v.(type) {
	case map[string]any:
		// first normalize children
		for k, child := range t {
			t[k] = normalizeArrays(child)
		}
		// check if all keys are numeric
		if len(t) == 0 {
			return t
		}
		indices := make([]int, 0, len(t))
		numericKeys := true
		maxIdx := -1
		for k := range t {
			idx, err := strconv.Atoi(k)
			if err != nil || idx < 0 || idx != previusNum+1 {
				numericKeys = false
				break
			}
			indices = append(indices, idx)
			if idx > maxIdx {
				maxIdx = idx
			}
			previusNum = idx
		}
		if !numericKeys {
			return t
		}
		// build slice with nil defaults
		arr := make([]any, maxIdx+1)
		for k, child := range t {
			idx, _ := strconv.Atoi(k)
			arr[idx] = child
		}
		return arr
	case []any:
		for i := range t {
			t[i] = normalizeArrays(t[i])
		}
		return t
	default:
		return v
	}
}
