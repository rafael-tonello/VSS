# About VSS
  Vss is a variable server, a state share system and a key-value database. It allow you to write, change and read variables from multiple sources (apps, systems, terminal, ...).
  
  Vss is a server that provides some APIs to manage nested variables/key-value pairs from multiple clients. All variables are shared between these clients, that means you can write a variable in any client and read it in any client.
  
  Vss treats variables as streams, allowing you to watch for changes and be notified when the value of the variables are changed.
  
  Vss uses a combination of memory cache and disk persistce for data storage. 

  The variables are written with object notation, allowing nesting. It allow a better organization of the data and helps VSS to organize variables and notify observer and allow you to multiple variables by use of a wildcard ('*' char).

# Features
- Key-value database with nested keys (using '.' as separator).
- Clients can subscribe to variables and be notified when they change.
- Supports both HTTP and a custom TCP-based protocol (VSTP).
- Configurable via file, environment variables, and command line arguments.

# What I can do with VSS?
- Share state between multiple applications and services.
- Build real-time dashboards and monitoring tools.
- Integrate with home automation systems.
- Use as a lightweight database for small to medium-sized applications.
- Build event-driven applications that react to variable changes.
- Use in IoT projects to manage device states and configurations.
- Allow communication between applications (microservices, web apps, etc.).

# Compiling and running

use ./pman.h build to comile the project. A binary file will  be created in the 'build' folder.

# Configuration source order

Vss can load configuration values from three sources: command line arguments, environment variables, and a configuration file. When the same setting is defined in more than one source, VSS uses the following priority order:
1. Command line arguments (highest priority)
2. Environment variables
3. Configuration file (lowest priority)

read mode details in [Configuration Importante](./docs/confs_sources_priority.md)

# Interacting with VSS from terminal
    You can interact with VSS using its HTTP API or VSTP protocol.

## setting and getting a variable
You can set and get variables on terminal by use of curl command. See the examples bellow:

```bash
#setting a variable
curl -X POST -d "the value of the variable" \
http://192.168.100.2:5023/n0/tests/testvariable

#in this case, the variable 'n0.tests.testvariable' will be set with the value 'the value of the variable'. If the variable not exists, it will be created.
```

```bash
#getting a variable value
curl http://192.168.100.2:5023/n0/tests/testvariable
#result: {"n0":{"tests":{"testvariable":{"_value":"The value of the variable"}}}}

#this command will get the value of 'n0/tests/testvariable' in a json (the default result format of the HTTP API).

#you can also request the data in the format of a plain text, adding the header 'accept' to the request:
curl -H "accept: text/plain" http://192.168.100.2:5023/n0/tests/testvariable
#result: n0.tests.testvariable=the value of the variable
```

# Further information

An overview on how VSS work with data when a 'var set' in requested
```
+---------+           +------------ server -----------+
| client  |           |  +-----+            +------+  |
+---------+           |  | RAM |            | disk |  |
  |                   |  +-----+            +------+  |
  |                   +-----|---------------------|---+
  |                    |    |                     |
  |                    |    |                     |
  |   var set          |    |                     |
  |-----'------------->|    |                     |
  |                    |--->|--+  paralel process |
  |                    |-------|--------'-------->|
  |                    |    |  |                  |--+
  |  var set notifi-   |    |<-+                  |  |
  |  cations           |<---|                     |  |
  |<---------'---------|    |                     |  |
  |                    |    |                     |  | Write to disk
  |                    |    |                     |  | process may
  |                    |    |                     |  | take longer
  |                    |    |                     |  | 
  |                    |    |                     |  |
  |                    |    |                     |  |
  |                    |    |                     |<-+
  |                    |    |                     |
```

# things to study and other information
When i wrote this app, I reused ancient codes, that was not very well structured. So some things should be refactored.

-> Dismember VSTP service/module in the following structure (just a suggestion):
    serviceController - Orchestrate the workflow
    VSTP read and writer - recieve and write data to web sockets, puttin headers, sizes and checksums
    pack processor and generator - interprete and create packs in the VSTP format (command + data)


# The VSTP Protocol
The vstp protocol is a api developed to be easy to implement and lightweight to run with microcontrollers. The vstp, besides very simple, allow the use of all VSS server (set vars, get vars, observe vars, ...).

The protocol is inspired by text files, and work by send commands in lines, where a line should contains the command, its arguments and be ended with a line break.

You can also comunicate with the VSS using telnet and VSTP protocol. When you enter in a telnet session with the VSTP port, you can use the '--help' command to be a list of available commands.

## Starting a VSTP the session
The VSTP session starts with a single TCP socket connection to the VSTP port. When you connect to this port, the server will send some initial information to you. Lets take a look:

    sbh:
    sci:PROTOCOL VERSION=1.2.0
    sci:VSS VERSION=0.21.0+Sedna
    sci:SCAPE CHARACTER=
    id:UID1698369563120248AFhr
    seh:

  line 1 (sbh:) -> this line are sent by the server to inform that the initiar server header will be send. 'sbh' is the abbreviation of 'sever begin headers'.

  line 2 (sci:PROTOCOL VERSION=1.2.0) -> here, server sen the first header. This header contains the version of the protocol (not of the VSS). The command used here are the 'sci', that means "server configurations and informations". The payload of 'sci' command is always a key=value pair. Int this case, the 'PROTOCOL VERSION' key was sent with the "1.2.0" as its value.

  line 3 (sci:VSS VERSION=0.21.0+Sedna) -> Here, the server sent its version (the version of the VSS).

  line 4 (sci:SCAPE CHARACTER=) -> The scape character are used to send speical chars, like a line break. In this case, the scape character is not being used.

  Line 5 (id:UID1698369563120248AFhr) -> Here, the server sent a suggestion of an id to you. These id is used internaly by the server to control variable observations. If youare restoring a connection and had received another one in a previous section, you can send it to the server, that will reply you with a resume of all variables you are observing before. We will take a look in how to do it later.

  Line 6 (seh:) -> server sent this to ifnorm that all header it have done sending the headers. 'seh' means "server end headers"


## Confirming or changing the client id

In the start of the section server sends a list of headers with useful information. One of these information that is very important is the 'ID'. Every client connected to the VSTP server must have an unique identification that allow server to manage the observation and some other information related to the client. and that's why the server sends this id at the beginning of the session, along with its headers.

But is very common we lost the connection due network problems and a "thousand" of other reasons. Furthmore, the VSS is designed to work with variable observation, allow you to be notified when a variable of your interest is changed, instead of consulting theis value constantly (a technique known as 'polling').

Now, supose you have to resend observations request allways the connection is lost and restored. You would need to do things like use a lot of code to identify this situation is each place you needs to observate a variable, or create internal vector to known which variable is already requested to the server and request agains eacho of the previous observations when connection is restored.

This situation would not be pleasant at all and fortunately the solution to this problem already exists. And to and to avoid having to do all of the above, you need just to discard the id sent by the server in the headers and send a previus used one.

Of corse, if you are starting the comunication, you will not have one previous id. In this case, you just need to use the one sent by the server in the headers

With all explained, lets see how to continue how VSTP session in the both situation (with a new ID and with a previous one):
  
Read more about VSTP in [VSTP Protocol](./docs/VSTP.md)



<!-- vss logo -->
![VSS Logo](./docs/vss_logo.svg)



