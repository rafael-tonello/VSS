#ifndef __FILESTORAGE__H__
#define __FILESTORAGE__H__

#include "../prefixtree.h"


#include <fstream>
#include <string.h>
#include "../common/errors.h"

using namespace std;

using Error=string;

struct FileHeader{
    ADDR_TYPE firstblockAddr;
    ADDR_TYPE deletedSeqAddr;
};

class FileStorage: public IBlockStorage { 
private:
    //File file;
    string fname;
    fstream file;
    FileHeader header;
    uint blockSize;

    ADDR_TYPE getAddrForANewBlock();
    tuple<ADDR_TYPE, size_t> seekChain(ADDR_TYPE initialBlockAddress, uint seekSize, bool readOnly = false);
    tuple<ADDR_TYPE, size_t> _seekChain(ADDR_TYPE currentBlockAddress, uint seekSize, uint currentSeekPosition, uint currentBlockOffset, bool readOnly);

    /** @brief this function scrools over the storage and automatically crate new blocks and seek to correct positions that just need to be readed or write.
     * The workFunc is called after filesystem is positioned and seeked to be write or read.
    */
    tuple<ADDR_TYPE, size_t> scrollChain(
        ADDR_TYPE blockAddress, 
        uint blockOffset, 
        size_t totalWorkSize, 
        size_t currentWorkPos, 
        function<void(
            uint amountToBeWorked, 
            size_t workPosition
        )> workFunc,
        bool readOnly = false
    );
    ADDR_TYPE readNextBlockAddress(ADDR_TYPE blockAddr);
    ADDR_TYPE getOrCreateNextBlock(ADDR_TYPE parentBlockAddress, bool readOnly);
    void initBlock(ADDR_TYPE address);
    void linkBlocks(ADDR_TYPE parentBlock, ADDR_TYPE childBlock);
    ADDR_TYPE getAddressOfTheLastBlockInTheSequence(ADDR_TYPE parentBlockAddr);
    void internalDeleteBlockSq(ADDR_TYPE seqAddress, ADDR_TYPE parentBlockToEraseSeqReference = 0);

    bool fileExists(string f);
    size_t getFileSize(fstream &f);
public:
    FileStorage(uint blockSize, string fname); 
    ~FileStorage(); 
    Error init();

    ADDR_TYPE getFirstBlockSeqAddress() override;
    ADDR_TYPE newBlockSeq() override;

    void write(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize, bool allowSpaceRelease) override;
    uint read(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize) override;
    void deleteBlockSq(ADDR_TYPE seqAddress) override;
    size_t getChainSize(ADDR_TYPE seqAddress) override;
}; 
 
#endif 
