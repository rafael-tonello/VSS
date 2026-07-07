// This file contains the C++ implementation that will be compiled with CGO
#include "prefixtree.h"
#include "storages/filestorage.h"
#include "common/errors.h"
#include <string>
#include <vector>
#include <exception>

using namespace std;

typedef int (*FilterFunction)(char* key, char* value);

// InstanceInfo structure to hold the tree and storage instances
struct InstanceInfo {
    PrefixTree<string>* tree = nullptr;
    FileStorage* storage = nullptr;
    char* getBuffer = nullptr;
    char** searchChildsBuffer = nullptr;
};

extern "C" {

void* pkv_new(char* filename, unsigned int tree_block_size) {
    auto ret = new InstanceInfo();
    
    ret->storage = new FileStorage(tree_block_size, filename);
    
    auto initResult = ret->storage->init();
    if (initResult != Errors::NoError) {
        delete ret->storage;
        delete ret;
        return nullptr;
    }
    
    ret->tree = new PrefixTree<string>(ret->storage, [](string v){return v;}, [](string v){return v;});
    return (void*)ret;
}

void pkv_free(void* p) {
    if (p == nullptr) return;
    
    auto instance = (InstanceInfo*)p;
    if (instance->tree) delete instance->tree;
    if (instance->storage) delete instance->storage;
    
    // Clean up buffers
    if (instance->getBuffer) {
        delete[] instance->getBuffer;
    }
    
    if (instance->searchChildsBuffer) {
        for (int c = 0; instance->searchChildsBuffer[c] != nullptr; c++) {
            delete[] instance->searchChildsBuffer[c];
        }
        delete[] instance->searchChildsBuffer;
    }
    
    delete instance;
}

void pkv_set(void* p, char* key, char* value) {
    if (p == nullptr || key == nullptr || value == nullptr) return;
    
    auto instanceInfo = (InstanceInfo*)p;
    try {
        instanceInfo->tree->set(string(key), string(value));
    } catch(exception& e) {
        // Error handling - could log here if needed
    }
}

char* pkv_get(void* p, char* key) {
    if (p == nullptr || key == nullptr) return nullptr;
    
    auto instanceInfo = (InstanceInfo*)p;
    try {
        auto ret = instanceInfo->tree->get(string(key));
        
        // Clean up old buffer
        if (instanceInfo->getBuffer) {
            delete[] instanceInfo->getBuffer;
        }
        
        instanceInfo->getBuffer = new char[ret.size()+1];
        memcpy(instanceInfo->getBuffer, ret.c_str(), ret.size());
        instanceInfo->getBuffer[ret.size()] = '\0';
        
        return instanceInfo->getBuffer;
    } catch(exception& e) {
        return nullptr;
    }
}

int exists(void* p, char* key) {
    if (p == nullptr || key == nullptr) return 0;
    
    auto instanceInfo = (InstanceInfo*)p;
    try {
        // Try to get the value - if it succeeds and returns non-empty string, it exists
        auto result = instanceInfo->tree->get(string(key));
        return result != "" ? 1 : 0;
    } catch(...) {
        return 0;
    }
}

void pkv_delete(void* p, char* key) {
    if (p == nullptr || key == nullptr) return;
    
    auto instanceInfo = (InstanceInfo*)p;
    try {
        instanceInfo->tree->remove(string(key));
    } catch(exception& e) {
        // Error handling
    }
}

void freeSearchChildsResult(void* p) {
    if (p == nullptr) return;
    
    auto instanceInfo = (InstanceInfo*)p;
    if (instanceInfo->searchChildsBuffer != nullptr) {
        for (int c = 0; instanceInfo->searchChildsBuffer[c] != nullptr; c++) {
            delete[] instanceInfo->searchChildsBuffer[c];
        }
        delete[] instanceInfo->searchChildsBuffer;
        instanceInfo->searchChildsBuffer = nullptr;
    }
}

char** searchChilds(void* p, char* key, unsigned int maxResults, FilterFunction filter, int processChildsOfNodesExcludedByFilter) {
    if (p == nullptr || key == nullptr) return nullptr;
    
    auto instanceInfo = (InstanceInfo*)p;
    
    // Clear previous results
    freeSearchChildsResult(p);
    
    vector<string> ret;
    
    try {
        if (filter == nullptr) {
            // No filter - return all children with non-empty data
            ret = instanceInfo->tree->searchChilds(string(key), maxResults, [](Node &node){
                return node.data != "";  // Only include nodes with data
            }, true);
        } else {
            // With filter - apply custom filter function
            ret = instanceInfo->tree->searchChilds(string(key), maxResults, [&](Node &node){
                return filter((char*)node.key.c_str(), (char*)node.data.c_str()) == 1;
            }, processChildsOfNodesExcludedByFilter != 0);
        }
    } catch(exception& e) {
        return nullptr;
    }
    
    // Allocate result buffer
    instanceInfo->searchChildsBuffer = new char*[ret.size()+1];
    for (size_t c = 0; c < ret.size(); c++) {
        instanceInfo->searchChildsBuffer[c] = new char[ret[c].size()+1];
        memcpy(instanceInfo->searchChildsBuffer[c], ret[c].c_str(), ret[c].size());
        instanceInfo->searchChildsBuffer[c][ret[c].size()] = '\0';
    }
    instanceInfo->searchChildsBuffer[ret.size()] = nullptr;
    
    return instanceInfo->searchChildsBuffer;
}

} // extern "C"
