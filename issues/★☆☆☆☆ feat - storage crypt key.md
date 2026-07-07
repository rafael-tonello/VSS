# feat - storage crpt key
Currently, storage persistance is done without protection.

## proposal
create the option --storagecryptkey to receive a password to open storage files. This password should be passed to the storage and should be used to crypt the persisted data.
Also, vss should clear terminal after be launched...