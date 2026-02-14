# VSTP Protocol Documentation

**Variables and Streams Transmission Protocol (VSTP)**

Version: 1.0.0

## Overview

VSTP (Variables and Streams Transmission Protocol) is a line-based text protocol designed for communication with the Variable And Streams Server (VSS). Similar to Redis' RESP protocol, VSTP provides a simple, human-readable interface for managing variables, subscriptions, and locks in a distributed system.

## Protocol Basics

### Connection & Transport

- **Transport**: TCP
- **Encoding**: UTF-8 text
- **Line Termination**: `\n` (newline character, optionally `\r\n`)
- **Message Format**: `command:payload\n`

### Message Structure

All messages follow this basic structure:

```
<COMMAND>:<PAYLOAD>\n
```

- **COMMAND**: Case-sensitive command identifier
- **PAYLOAD**: Command-specific data (may be empty)
- **Separator**: Single colon (`:`)
- **Terminator**: Newline (`\n`)

Example:
```
set:myvar=hello world
get:myvar
ping:
```

### Character Escaping

Since VSTP is a line-based protocol, certain characters must be escaped to prevent protocol corruption:

- **Backslash** (`\`) → `\\`
- **Newline** (`\n`) → `\\n`
- **Carriage Return** (`\r`) → `\\r`

**Escaping Rules:**
1. All payloads (variable names, values, metadata) must be escaped before transmission
2. Backslash must be escaped **first** when encoding
3. Backslash must be unescaped **last** when decoding
4. Commands are never escaped (only payloads)

**Example:**
```
Original value:  "Hello\nWorld"
Escaped:         "Hello\\nWorld"
Transmitted:     set:myvar=Hello\\nWorld
```

**Edge cases:**
```
Value: "test\\data"     → Escaped: "test\\\\data"
Value: "line1\nline2"   → Escaped: "line1\\nline2"
Value: "path\\file"     → Escaped: "path\\\\file"
```

## Connection Lifecycle

### 1. Initial Connection

When a client connects to the VSTP server, the server immediately sends initialization headers:

```
beginserverheaders:
serverinfo:PROTOCOL VERSION=1.0.0
serverinfo:VSS VERSION=<version>
sugestednewid:<generated-id>
endserverheaders:
```

**Explanation:**
- `beginserverheaders` - Marks the start of server initialization
- `serverinfo` - Provides server and protocol version information
- `sugestednewid` - Server suggests a unique client ID (format: `<counter>-<random>`)
- `endserverheaders` - Marks the end of initialization

### 2. Client ID Management

#### Setting/Confirming Client ID

**Command**: `setid:<client-id>`

The client should send this command to:
- Accept the suggested ID from the server, OR
- Request a specific ID (for session resumption)

**Server Response:**
```
beginresponse:setid:<client-id>
aoc:<count>
endresponse:setid:<client-id>
```

Where `aoc` (Already Observing Count) indicates how many variables this client was already subscribed to (useful for reconnection).

## Response Envelope

Most commands trigger a response envelope from the server:

```
beginresponse:<original-command>:<original-payload>
<response-data-lines>
endresponse:<original-command>:<original-payload>
```

This allows clients to match responses to requests and handle errors properly.

## Core Commands

### Variable Operations

#### SET - Set Variable Value

**Client Request:**
```
set:<variable-name>=<value>
```

**Server Response:**
```
beginresponse:set:<variable-name>=<value>
[error:<error-message>]
endresponse:set:<variable-name>=<value>
```

**Example:**
```
Client: set:temperature=23.5
Server: beginresponse:set:temperature=23.5
Server: endresponse:set:temperature=23.5
```

#### GET - Get Variable Value(s)

**Client Request:**
```
get:<variable-path>
```

Supports wildcards for pattern matching.

**Server Response:**
```
beginresponse:get:<variable-path>
[error:<error-message>]
[varvalue:<name>=<value>]+
endresponse:get:<variable-path>
```

**Example:**
```
Client: get:temperature
Server: beginresponse:get:temperature
Server: varvalue:temperature=23.5
Server: endresponse:get:temperature
```

Multiple variables can be returned:
```
Client: get:sensor.*
Server: beginresponse:get:sensor.*
Server: varvalue:sensor.temp=23.5
Server: varvalue:sensor.humidity=65
Server: endresponse:get:sensor.*
```

#### DELETE - Delete Variable

**Client Request:**
```
delete:<variable-name>
```

**Server Response:**
```
beginresponse:delete:<variable-name>
deleteresult:<variable-name>=success
endresponse:delete:<variable-name>
```

Or on error:
```
beginresponse:delete:<variable-name>
deleteresult:<variable-name>=failure:<error-message>
endresponse:delete:<variable-name>
```

### Subscription System

#### SUBSCRIBE - Subscribe to Variable Changes

**Client Request:**
```
subscribe:<variable-name>
subscribe:<variable-name>(<metadata>)
```

The metadata field allows clients to tag subscriptions for filtering.

**Server Response:**
```
beginresponse:subscribe:<variable-name>
endresponse:subscribe:<variable-name>
```

**Notifications:**

When a subscribed variable changes, the server sends:
```
varchanged:<variable-name>=<new-value>
varchanged:<variable-name>(<metadata>)=<new-value>
```

**Example:**
```
Client: subscribe:temperature
Server: beginresponse:subscribe:temperature
Server: endresponse:subscribe:temperature

[... later, when temperature changes ...]
Server: varchanged:temperature=24.0
```

#### UNSUBSCRIBE - Unsubscribe from Variable

**Client Request:**
```
unsubscribe:<variable-name>
unsubscribe:<variable-name>(<metadata>)
```

**Server Response:**
```
beginresponse:unsubscribe:<variable-name>
endresponse:unsubscribe:<variable-name>
```

### Lock Operations

Variables can be locked for exclusive access (distributed locking mechanism).

#### LOCK - Lock a Variable

**Client Request:**
```
lock:<variable-name>
lock:<variable-name>=<timeout-ms>
```

If timeout is omitted, it uses maximum timeout.

**Server Response:**
```
beginresponse:lock:<variable-name>
lockresult:<variable-name>=success
endresponse:lock:<variable-name>
```

Or on failure:
```
beginresponse:lock:<variable-name>
lockresult:<variable-name>=failure:<error-message>
endresponse:lock:<variable-name>
```

#### UNLOCK - Unlock a Variable

**Client Request:**
```
unlock:<variable-name>
```

**Server Response:**
```
beginresponse:unlock:<variable-name>
unlockdone:<variable-name>
endresponse:unlock:<variable-name>
```

#### LOCKSTATUS - Check Lock Status

**Client Request:**
```
lockstatus:<variable-name>
```

**Server Response:**
```
lockstatusresult:<variable-name>=locked
```
or
```
lockstatusresult:<variable-name>=unlocked
```

### Hierarchy Operations

#### GETCHILDS - Get Child Variables

**Client Request:**
```
getchilds:<parent-variable>
```

**Server Response:**
```
beginresponse:getchilds:<parent-variable>
childs:<child1>,<child2>,<child3>,...
endresponse:getchilds:<parent-variable>
```

**Example:**
```
Client: getchilds:sensor
Server: beginresponse:getchilds:sensor
Server: childs:sensor.temperature,sensor.humidity,sensor.pressure
Server: endresponse:getchilds:sensor
```

### Utility Commands

#### PING - Keep-Alive / Connectivity Test

**Client Request:**
```
ping:
```

**Server Response:**
```
beginresponse:ping:
pong:
endresponse:ping:
```

## Error Handling

Errors are sent within response envelopes:

```
beginresponse:<command>:<payload>
error:<error-description>
endresponse:<command>:<payload>
```

**Example:**
```
Client: set:=invalid
Server: beginresponse:set:=invalid
Server: error:set error: invalid variable name
Server: endresponse:set:=invalid
```

## Complete Protocol Flow Example

### Simple GET/SET Session

```
[Connection established]

Server: beginserverheaders:
Server: serverinfo:PROTOCOL VERSION=1.2.0
Server: serverinfo:VSS VERSION=1.0.0
Server: sugestednewid:0-abc123
Server: endserverheaders:

Client: setid:0-abc123

Server: beginresponse:setid:0-abc123
Server: aoc:0
Server: endresponse:setid:0-abc123

Client: set:myapp.status=running

Server: beginresponse:set:myapp.status=running
Server: endresponse:set:myapp.status=running

Client: get:myapp.status

Server: beginresponse:get:myapp.status
Server: varvalue:myapp.status=running
Server: endresponse:get:myapp.status

Client: delete:myapp.status

Server: beginresponse:delete:myapp.status
Server: deleteresult:myapp.status=success
Server: endresponse:delete:myapp.status
```

### Subscription Example

```
[After connection and ID setup...]

Client: subscribe:sensor.temperature

Server: beginresponse:subscribe:sensor.temperature
Server: endresponse:subscribe:sensor.temperature

Client: set:sensor.temperature=23.5

Server: beginresponse:set:sensor.temperature=23.5
Server: endresponse:set:sensor.temperature=23.5

Server: varchanged:sensor.temperature=23.5

[... later ...]

Client: set:sensor.temperature=24.0

Server: beginresponse:set:sensor.temperature=24.0
Server: endresponse:set:sensor.temperature=24.0

Server: varchanged:sensor.temperature=24.0

Client: unsubscribe:sensor.temperature

Server: beginresponse:unsubscribe:sensor.temperature
Server: endresponse:unsubscribe:sensor.temperature
```

### Session Resumption Example

```
[Connection established - reconnecting client]

Server: beginserverheaders:
Server: serverinfo:PROTOCOL VERSION=1.2.0
Server: serverinfo:VSS VERSION=1.0.0
Server: sugestednewid:5-xyz789
Server: endserverheaders:

Client: setid:0-abc123

Server: beginresponse:setid:0-abc123
Server: aoc:3
Server: endresponse:setid:0-abc123

[Client was observing 3 variables, subscriptions are restored]
```

## Command Reference Table

| Command | Direction | Purpose |
|---------|-----------|---------|
| `beginserverheaders` | Server → Client | Start of initial headers |
| `endserverheaders` | Server → Client | End of initial headers |
| `serverinfo` | Server → Client | Server/protocol version info |
| `sugestednewid` | Server → Client | Suggested client ID |
| `setid` | Client → Server | Set/confirm client ID |
| `aoc` | Server → Client | Already observing count |
| `beginresponse` | Server → Client | Start of response envelope |
| `endresponse` | Server → Client | End of response envelope |
| `set` | Client → Server | Set variable value |
| `get` | Client → Server | Get variable value(s) |
| `varvalue` | Server → Client | Variable value response |
| `delete` | Client → Server | Delete variable |
| `deleteresult` | Server → Client | Delete operation result |
| `subscribe` | Client → Server | Subscribe to variable changes |
| `unsubscribe` | Client → Server | Unsubscribe from variable |
| `varchanged` | Server → Client | Variable change notification |
| `lock` | Client → Server | Lock variable |
| `lockresult` | Server → Client | Lock operation result |
| `unlock` | Client → Server | Unlock variable |
| `unlockdone` | Server → Client | Unlock confirmation |
| `lockstatus` | Client → Server | Check lock status |
| `lockstatusresult` | Server → Client | Lock status response |
| `getchilds` | Client → Server | Get child variables |
| `childs` | Server → Client | Child variables list |
| `ping` | Client → Server | Connectivity test |
| `pong` | Server → Client | Ping response |
| `error` | Server → Client | Error message |

## Implementation Guidelines

### For Client Developers

1. **Connection Handling**
   - Implement buffered line reading (messages are `\n` terminated)
   - Parse messages by splitting on the first `:` character
   - Handle the initial server headers sequence
   - Store the suggested client ID or provide your own

2. **Request/Response Matching**
   - Use the response envelope (`beginresponse`/`endresponse`) to match responses
   - The envelope echoes the original command and payload
   - Multiple data lines may appear between begin/end markers

3. **Subscription Management**
   - Track active subscriptions client-side
   - Handle `varchanged` notifications asynchronously
   - On reconnection, use `setid` with your previous ID to restore subscriptions

4. **Error Handling**
   - Check for `error:` lines within response envelopes
   - Handle connection drops gracefully (reconnect with same ID)

5. **Best Practices**
   - Send one command per line
   - Wait for complete response envelopes before considering operation complete
   - Implement connection keep-alive with `ping`
   - Use metadata in subscriptions to filter notifications

### Metadata Usage

Metadata allows you to tag subscriptions for filtering purposes:

```
subscribe:variable.name(tag1)
```

When the variable changes, notifications include the metadata:
```
varchanged:variable.name(tag1)=newvalue
```

This enables multiple components to subscribe to the same variable with different tags and filter notifications accordingly.

### Variable Naming

Variables use dot notation for hierarchy:
```
app.module.component.property
sensor.room1.temperature
config.database.connection.timeout
```

Wildcards are supported in GET operations:
```
get:sensor.*
get:sensor.room1.*
```

## Security Considerations

- VSTP does not include built-in authentication or encryption
- Implement at transport layer (TLS/SSL) or network level (VPN, firewall rules)
- Client IDs are not cryptographically secure and should not be relied upon for authentication
- Lock mechanisms provide coordination, not security guarantees

## Version History

### Version 1.0.0 (Current)
- Character escaping for newlines, carriage returns, and backslashes
- Full support for all documented commands
- Client ID management with session resumption
- Subscription system with metadata support
- Distributed locking mechanism
- Hierarchical variable queries

---

*This documentation is based on the VSTP implementation in vstp.go. For questions or issues, refer to the source code or project maintainers.*
