# feat - companion servers
Currently, VSS uses only its internal storage to find data, and also works only as individual instance. 
A companion-server is another vss server that current vss instance can connect to try find data that is not present in local storage. This way, multiple vss instances can share data between them, increasing the chances to find data.    

the companion server should be set using the option --companion-server, that is in the format ip[:port]

Companion-servers allow vss to act as a distributed system that allow:
 * vss instance be a cache to a more distant vss instance
 * act as a cluster of vss instances that share data

WHan a variable is setted locally, it will be sent also to the companion servers, allowing notification of clientes connect to them.

