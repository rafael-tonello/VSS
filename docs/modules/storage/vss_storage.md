# About vss Storage
VSS Storage is the internal vss service responsible for save and make data available. Internally, VSS works with a concept of "storage drivers", which are implementations of the IStorage interface. This allow users to choose between different storage system for vss, according to their needs. Each driver has its own advantages and disadvantages, and can be used in different scenarios.

# Available Storage Drivers
## RamCachedDB
RamCachedDB is an in-memory storage driver that provides fast read and write operations. It works by keeping all data in memory and periodically dumping it to disk to ensure persistence. This makes it ideal for scenarios where performance is critical and data can be easily reconstructed in case of a crash. However, it may not be suitable for large datasets or scenarios where data durability is a concern.

All data is loaded in the memory at VSS statup, so if you have a large dataset, it may take some time for VSS to start. Also, when any data is changed in the memory, all data is dumped to the disk, so if you have a large dataset, it may cause performance issues.

## RamCachedDBPkv
This is a variation of the RamCachedDB driver that uses a prefix tree (also known as a trie). When data is changed, only the changed date are written to the prefix tree. As of RamCahceDB, this driver also keeps all data in memory, and can let VSS startup slower for large datasets, but differently from RamCachedDB, it is faster to write data to disk, since only the changed data are written. 
