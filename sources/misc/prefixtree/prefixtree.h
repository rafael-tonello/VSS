#ifndef __PREFIX_TREE_H__
#define __PREFIX_TREE_H__

#include <map>
#include <unistd.h>
#include <functional>
#include <string>
#include <vector>
#include <mutex>
#include <iostream>

using namespace std;

#define ADDR_TYPE size_t
#define StrT string
#define StrSize .length()

class IBlockStorage{
public:

    /**
     * @brief Get the First Block Seq Address object.
     * 
     * @return the address of the first block
     */
    virtual ADDR_TYPE getFirstBlockSeqAddress() = 0;

    /**
     * @brief Creates a new block sequence.
     * 
     * @return ADDR_TYPE 
     */
    virtual ADDR_TYPE newBlockSeq() = 0;
    //virtual void releaseSeqSpace(ADDR_TYPE seqAddress, size_t seqTotalSize) = 0;
    virtual void write(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize, bool allowSpaceRelease) = 0;
    virtual uint read(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize) = 0;
    virtual void deleteBlockSq(ADDR_TYPE seqAddress) = 0;
    virtual size_t getChainSize(ADDR_TYPE seqAddress) = 0;

    virtual uint read(ADDR_TYPE seqAddress, char* buffer, size_t bufferSize){ return read(seqAddress, 0, buffer, bufferSize); }
    virtual void write(ADDR_TYPE seqAddress, char* buffer, size_t bufferSize, bool allowSpaceRelease){ write(seqAddress, 0, buffer, bufferSize, allowSpaceRelease); }
};

#define NoChildren 65535
const uint RETURN_ALL_CHILDS = 0;

struct Node{
    ADDR_TYPE address = 0;
    enum NodeType {RootNode, Normal, Invalid};
    StrT key = "";
    NodeType type = NodeType::Normal;
    StrT data = "";
    std::map<char, ADDR_TYPE> children = {};
};

template<class T>
class PrefixTree{
private:
    function<T(StrT)> sToT;
    function<StrT(T)> tToS;
    IBlockStorage* storage;
    Node rootNode;
    mutex locker;


    void recursiveGetChilds(Node &node, vector<StrT> &ret, uint maxResults = RETURN_ALL_CHILDS, function<bool(Node &node)> filter = nullptr, bool processChildsOfNodesExcludedByFilter = true)
    {   
        if (filter != nullptr && filter(node))
        {
            ret.push_back(node.key);
            if (!processChildsOfNodesExcludedByFilter)
                return;
        }

        if (maxResults != RETURN_ALL_CHILDS && ret.size() >= maxResults)
            return;

        for (auto &child: node.children)
        {
            auto childNode = readNodeFromStorage(child.second);
            if (childNode.type == Node::NodeType::Invalid)
                continue;

            recursiveGetChilds(childNode, ret, maxResults, filter, processChildsOfNodesExcludedByFilter);
            if (maxResults != RETURN_ALL_CHILDS && ret.size() >= maxResults)
                return;
        }
    }

    StrT getPrefix(StrT key1, StrT key2)
    {
        StrT prefix = "";
        size_t index = 0;
        while (index < key1 StrSize && index < key2 StrSize)
        {
            if (key1[index] == key2[index])
            {
                prefix += key1[index];
                index++;
            }
            else
                break;
        }
        return prefix;
    }

    /**
     * @brief Locate the child identified by 'key' in the 'currentNode'. If the child does not exists yet, 
     * create one and add it as child of 'currentNode'
     * 
     * @param currentNode The curentNode (that is, or will be, parent of child identified by 'key')
     * @param key the name of the child
     * @return Node return the child node
     */
    Node locateOrCreateChild(Node &currentNode, StrT key, bool readOnly)
    {
        //locate the next node in the childs. If not exists, create a new one.
        auto checkIndex = currentNode.key StrSize;
        if (key StrSize > checkIndex)
        {
            char toCheck = key[checkIndex];
            if (!currentNode.children.count(toCheck))
            {
                if (readOnly)
                    return Node{.type = Node::NodeType::Invalid};

                Node newNode { .key = key };
                writeNodeToStorage(newNode, true);
                
                currentNode.children[toCheck] = newNode.address;
                writeNodeToStorage(currentNode);
            }

            auto nodeReadFromStorage = readNodeFromStorage(currentNode.children[toCheck]);
            
            return nodeReadFromStorage;
        }
        else
        {
            return Node{.type = Node::NodeType::Invalid};
        }
    }

    /**
     * @brief Create a new node between 'currentNode' and 'parentNode'. The new node will be child of 'parentNode' and 'parent' of 'currentNode'
     * 
     * @param currentNode The current node, that should be, currently, child of 'parentNode'
     * @param prefixName The key of the intermediary node (the new node, the prefix node)
     * @param ParentNode The parent node of 'currentNode'
     * @return Node The createnode, that will be child of 'parentNode' and 'parent' of 'currentNode'
     */
    Node createPrefixNode(Node &currentNode, StrT prefixName, Node &parentNode)
    {
        //create a prefix node
        Node prefixNode {.address = storage->newBlockSeq(), .key = prefixName};

        //update parentNode (if no root node. If currentNOde is the rootNode, enter in a error state)
        if (currentNode.type == Node::NodeType::RootNode){
            //error, Bug
        }

        auto addrIndex = parentNode.key StrSize;
        if (addrIndex >= prefixName StrSize){
            //error/bug
        }
        auto toCheck = prefixName[addrIndex];
        parentNode.children[toCheck] = prefixNode.address;
        writeNodeToStorage(parentNode);
        
        //add currentNode as a child of new prefix
        addrIndex = prefixNode.key StrSize;
        
        if (addrIndex >= currentNode.key StrSize){//if (currentNode.key StrSize < addIndex)
            //error/bug
        }
        toCheck = currentNode.key[addrIndex];
        prefixNode.children[toCheck] = currentNode.address;
        writeNodeToStorage(prefixNode);
        

        return prefixNode;
    }

    /**
     * @brief This function witll write the node to the storage. If createNewAddress is passed as true, a new 
     * node will be created and the property 'address' of the 'node' will be set with this new address
     * 
     * @param node The node to be written
     * @param createNewAddress If true, a new address will be create. If false, the 'address' of 'node' will be used to write data to the storage
     * @return ADDR_TYPE the address of the node in the storage (the same final value of 'node.address')
     */
    ADDR_TYPE writeNodeToStorage(Node &node, bool createNewAddress = false)
    {
        //storage struct:
        //keysize(ADDR_TYPE)|key|childrensindexsize(uint)|childrensindex|datasize(4bytes, uint)|data

        if (createNewAddress)
            node.address = storage->newBlockSeq();
        
        uint keySize = (uint)(node.key StrSize);
        uint singleChildSize = sizeof(char) + sizeof(ADDR_TYPE); // (key + address)
        uint childrenStorageSize = singleChildSize * node.children.size();
        uint dataSize = (uint)(node.data StrSize);

        uint bufferSize = sizeof(uint) + keySize + sizeof(uint) + childrenStorageSize + sizeof(dataSize) + dataSize;
        char *buffer = new char[bufferSize];

        uint currIndex = 0;
        //put keysize in the 'buffer'
        for (uint c = 0; c < sizeof(uint); c++)
            buffer[currIndex++] = ((char*)&keySize)[c];

        //put key in the 'buffer'
        for (uint c = 0; c < keySize; c++)
            buffer[currIndex++] = node.key[c];

        //put childrenStorageSize in the 'buffer'
        for (uint c = 0; c < sizeof(uint); c++)
            buffer[currIndex++] = ((char*)&childrenStorageSize)[c];

        //put children in the 'buffer'
        for (auto &currChildren: node.children)
        {
            buffer[currIndex++] = currChildren.first;
            for (int c = 0; c < sizeof(currChildren.second); c++)
            {
                char dt = ((char*)(&currChildren.second))[c];
                buffer[currIndex++] = dt;
            }
        }

        //put dataSize in the 'buffer'
        for (uint c = 0; c < sizeof(uint); c++)
            buffer[currIndex++] = ((char*)&dataSize)[c];

        //put data in the 'buffer'
        for (uint c = 0; c < dataSize; c++)
            buffer[currIndex++] = node.data[c];

        //before write.. print to stdout the buffer (hex)
        //for (uint c = 0; c < bufferSize; c++){
        //    char tmp = buffer[c];
        //    cout << (int)tmp << " ";
        //    //printf("[%02x] ", (int)buffer[c]);
        //}
        //printf("\n");

        storage->write(node.address, buffer, bufferSize, true);

        delete[] buffer;

        return node.address;
    }

    void readNodeFromStorage(Node &nodeWithAddress)
    {
        uint currentIndex = 0;

        //read keysize
        uint keySize = 0;
        storage->read(nodeWithAddress.address, (char*)&keySize, sizeof(uint));
        currentIndex += sizeof(uint);

        //read the key
        char* keyBuffer = new char[keySize];
        storage->read(nodeWithAddress.address, currentIndex, keyBuffer, keySize);
        currentIndex += keySize;
        nodeWithAddress.key = bufferToString(keyBuffer, keySize);
        delete[] keyBuffer;

        //read children indexing size
        uint childrenIndexingSize = 0;
        storage->read(nodeWithAddress.address, currentIndex, (char*)&childrenIndexingSize, sizeof(uint));
        currentIndex += sizeof(uint);

        //read children indexing data
        char* childrenBuffer = new char[childrenIndexingSize];
        storage->read(nodeWithAddress.address, currentIndex, childrenBuffer, childrenIndexingSize);

        uint c = 0; 
        while (c < childrenIndexingSize)
        {
            auto key = childrenBuffer[c++];
            
            ADDR_TYPE address = 0;
            for (uint c2 = 0; c2 < sizeof(ADDR_TYPE); c2++)
                ((char*)&address)[c2] = childrenBuffer[c + c2]; 
            
            c += sizeof(ADDR_TYPE);

            nodeWithAddress.children[key] = address;
        }
        currentIndex += childrenIndexingSize;
        delete[] childrenBuffer;


        //read the data size
        uint dataSize = 0;
        storage->read(nodeWithAddress.address, currentIndex, (char*)&dataSize, sizeof(uint));
        currentIndex += sizeof(uint);

        //read the key
        char* dataBuffer = new char[dataSize];
        storage->read(nodeWithAddress.address, currentIndex, dataBuffer, dataSize);
        currentIndex += dataSize;
        nodeWithAddress.data = bufferToString(dataBuffer, dataSize);
        delete[] dataBuffer;

    }

    Node readNodeFromStorage(ADDR_TYPE address)
    {
        Node ret = Node{.address = address};
        readNodeFromStorage(ret);
        return ret;
    }

    StrT bufferToString(char* dataBuffer, uint dataSize)
    {
        StrT ret = "";
        //low performance, but compatible with string and String(arduinoIDE, WString)
        for (uint c = 0; c < dataSize; c++)
            ret += dataBuffer[c];

        return ret;
    }

    void initRootNode()
    {
        //check if the root node already exists or if is the first time here
        rootNode = Node{.address = storage->getFirstBlockSeqAddress()};
        
        //create the root node if it not exists in the block storage
        if (storage->getChainSize(storage->getFirstBlockSeqAddress()) == 0)
            writeNodeToStorage(rootNode);

        readNodeFromStorage(rootNode);
    }

    /**
     * @brief Locate a node in the tree. If not in readonly, the tree structure will be changed
     * 
     * @param key The key you are looking for
     * @param currentNode The current node is used because it is a recursive function. You should specify the rootNode for the first call and a children node in recursive calls
     * @param parentNode The parent  node of currentNode. On first call, you must use 'Node{.type = Node::NodeType::Invalid}' and the 'currentNode' in recursive calls
     * @param readOnly if trye, the tree will be changed to create the node if it does not exists. If false, 'Node{.type = Node::NodeType::Invalid}' will be returned if node is not present
     * @return Node The node or 'Node{.type = Node::NodeType::Invalid}'
     */
    Node find(StrT key, Node &currentNode, Node &lastValid, Node &parentNode, bool readOnly = false)
    {
        lastValid = currentNode;

        if (key == currentNode.key)
            return currentNode;

        auto prefixName = getPrefix(currentNode.key, key);
        auto comparisionPosition = currentNode.key StrSize;

        if (prefixName StrSize >= comparisionPosition)
        {
            auto childNode = locateOrCreateChild(currentNode, key, readOnly);

            if (childNode.type == Node::NodeType::Invalid)
                return childNode;

            //recursive call with the child as currentNode again
            return find(key, childNode, lastValid, currentNode, readOnly);
        }
        else if (prefixName StrSize < comparisionPosition)
        {
            if (readOnly)
                return Node{.type = Node::NodeType::Invalid};

            auto prefixNode = createPrefixNode(currentNode, prefixName, parentNode);
            
            //recall the find function with the prefix as currentNode
            return find(key, prefixNode, lastValid, parentNode, readOnly);
        }
        return Node{.type = Node::NodeType::Invalid};
    }
            
public:
    /**
     * @brief Construct a new Prefix Tree object
     * @param storage The storage to be used. The storage should be initialized before pass it to the PrefixTree. Inside the folder 'storages' 
     * there is some storages that could be used. you also can write your own storage class by implementing the IBlockStorage interface
     * @param StringToTFunc A function that should convert a string to a T object. This funciton is very important, because the PrefixTree uses string internally
     * @param TToStringFunc A function that should convert a T object to a string. Is the opposite of the StringToTFunc
     */
    PrefixTree(IBlockStorage* storage, function<T(StrT)> StringToTFunc, function<StrT(T)> TToStringFunc): storage(storage), tToS(TToStringFunc), sToT(StringToTFunc)
    {
        initRootNode();
    }

    //PrefixTree(string filename, function<T(StrT)> StringToTFunc, function<StrT(T)> TToStringFunc, uint blockSize = 64): tToS(TToStringFunc), sToT(StringToTFunc)
    //{
    //    FileStorage storage(blockSize, filename);
    //    storage.init();
//
    //    initRootNode();
    //}
    
    /**
     * @brief Stores a key-value pair in the tree. If the key already exists, the value will be updated
     */
    void set(StrT key, T data)
    {
        locker.lock();
        Node lastValidNode;
        auto invalidNode = Node{.type = Node::NodeType::Invalid};
        Node theNode = this->find(key, this->rootNode, lastValidNode, invalidNode, false);
        if (theNode.type == Node::NodeType::Invalid)
        {
            //throw a runtime error
            locker.unlock();
            throw runtime_error("Node not found");
        }
        
        theNode.data = tToS(data);
        
        this->writeNodeToStorage(theNode);
        locker.unlock();
    }

    /**
     * @brief Get the value of a key. If the key does not exists, the defaultValue will be returned
     */
    T get(StrT key, T defaultValue = T())
    {
        locker.lock();
        Node lastValidNode;
        auto invalidNode = Node{.type = Node::NodeType::Invalid};
        auto result = this->find(key, this->rootNode, lastValidNode, invalidNode, true);
        locker.unlock();
        if (result.type == Node::NodeType::Invalid)
            return defaultValue;
        
        return sToT(result.data);
    }
    /**
     * @brief removes a key from the tree. Internally, a empty value will be write to the key value (the node itself will not be removed, because it could have childs)
     * the remotion of the node value should release space inside the IStorage in use (according to the implementation of the IStorage)
     */
    void remove(StrT key){
        locker.lock();
        Node lastValidNode;
        auto invalidNode = Node{.type = Node::NodeType::Invalid};
        Node theNode = this->find(key, this->rootNode, lastValidNode, invalidNode, false);
        if (theNode.type == Node::NodeType::Invalid)
        {
            //throw a runtime error
            locker.unlock();
            return;
        }
        
        theNode.data = "";
        
        this->writeNodeToStorage(theNode);
        locker.unlock();
    }

    /**
     * @brief Alias to remove
     */
    void del(StrT key){ remove(key); }
    

    /**
     * @brief Check if a key exists in the tree
     */
    bool exists(StrT key)
    {
        locker.lock();
        Node lastValidNode;
        auto invalidNode = Node{.type = Node::NodeType::Invalid};
        auto result = this->find(key, this->rootNode, lastValidNode, invalidNode, true);
        locker.unlock();
        return result.type != Node::NodeType::Invalid || result.data == "";
    }

    /**
     * @brief Alias to exists
     */
    bool contains(StrT key)
    {
        return exists(key);
    }

    /**
     * search for childs of a key. The search is recursive and include childs of childs and so on.
     * @param key The key to search for childs
     * @param maxResults The maximum number of results to be returned. If RETURN_ALL_CHILDS is passed, all childs will be returned
     * @param filter A function that will be called for each node. If the function returns true, the node will be included in the result. If false, the node will be excluded
     * @param processChildsOfNodesExcludedByFilter If true, the childs of nodes excluded by the 'filter' function will be processed. If false, the childs of excluded nodes 
     * will also be excluded
     */
    //onlyNodesWithValues allow a "on the fly" filter, returning only nodes with values
    vector<StrT> searchChilds(StrT key, uint maxResults = RETURN_ALL_CHILDS, function<bool(Node &node)> filter = nullptr, bool processChildsOfNodesExcludedByFilter = true)
    {
        locker.lock();
        Node lastValidNode;
        auto invalidNode = Node{.type = Node::NodeType::Invalid};
        auto result = this->find(key, this->rootNode, lastValidNode, invalidNode, true);

        //lastValidNode should be a child of 'key' (key should be at begining of the lastValidNode.key)
        if (lastValidNode.key.find(key) != 0)
        {
            //if the key is not found, return empty vector
            locker.unlock();
            return {};
        }

        //recursive scroll childs
        vector<StrT> ret;
        recursiveGetChilds(lastValidNode, ret, maxResults, filter, processChildsOfNodesExcludedByFilter);
        locker.unlock();
        return ret;
    }

    vector<StrT> suggest(StrT key, uint maxResults = RETURN_ALL_CHILDS, function<bool(Node &node)> filter = nullptr, bool processChildsOfNodesExcludedByFilter = true)
    {
        locker.lock();
        Node lastValidNode;
        auto invalidNode = Node{.type = Node::NodeType::Invalid};
        auto result = this->find(key, this->rootNode, lastValidNode, invalidNode, true);

        //recursive scroll childs
        vector<StrT> ret;
        recursiveGetChilds(lastValidNode, ret, maxResults, filter, processChildsOfNodesExcludedByFilter);
        locker.unlock();
        return ret;
    }

    /**
     * @brief Get the storage object
     */
    IBlockStorage* getStorage()
    {
        return storage;
    }
};

#endif
