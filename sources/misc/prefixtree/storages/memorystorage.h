#ifndef __MEMORYSTORAGE__H__ 
#define __MEMORYSTORAGE__H__ 

#include <stdexcept>
#include <iostream>
#include <vector>
#include "prefixtree.h"

//this class is used for tests, and the use of 'uint' instead pointer are proposital
class MemoryStorage: public IBlockStorage{
private:
    vector<char*> rootBlocks;
    uint blockSize;

    //ADDR_TYPE currentBlockAddress = 0;
    //uint currentBlockOffset = 0;

    uint COMMON_BLOCK_HEADER_SIZE = 0;

    //if parentBlock != 0, the parentBlock field 'nextBlockAddress' will be set to 0 (meaning that have no next block)
    void internalDelete(ADDR_TYPE address, ADDR_TYPE parentBlock = 0);
public:
    MemoryStorage(uint blockSize);
    ~MemoryStorage();
    uint calculateMemorySpace();
    uint calculateChainMemorySize(ADDR_TYPE address);
    ADDR_TYPE getFirstBlockSeqAddress();
    ADDR_TYPE newBlockSeq();
    void write(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize, bool allowSpaceRelease);
    tuple<ADDR_TYPE, size_t> seekChain(ADDR_TYPE initialBlockAddress, uint seekSize, bool readOnly = false);
    tuple<ADDR_TYPE, size_t> _seekChain(ADDR_TYPE currentBlockAddress, uint seekSize, uint currentSeekPosition, uint currentBlockOffset, bool readOnly);
    tuple<ADDR_TYPE, size_t> scrollChain(
        ADDR_TYPE blockAddress, 
        uint blockOffset, 
        size_t totalWorkSize, 
        size_t currentWorkPos, 
        function<void(
            char* blockP,
            size_t blockPWriteAddr, 
            size_t workPosition
        )> byteWorkFunc,
        bool readOnly = false
    );

    /**
     * @brief Get the Or Create Block. If readonly = true and end of chain is reached, the value 0 is returned
     * 
     * @param parentBlockAddress 
     * @param readOnly 
     * @return ADDR_TYPE 
     */
    ADDR_TYPE getOrCreateBlock(ADDR_TYPE parentBlockAddress, bool readOnly);
    ADDR_TYPE createNewBlock();
    uint read(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize);
    void deleteBlockSq(ADDR_TYPE seqAddress);
    size_t getChainSize(ADDR_TYPE seqAddress);
};
 
#endif 
