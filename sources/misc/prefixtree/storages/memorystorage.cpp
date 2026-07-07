#include  "memorystorage.h" 

//if parentBlock != 0, the parentBlock field 'nextBlockAddress' will be set to 0 (meaning that have no next block)
void MemoryStorage::internalDelete(ADDR_TYPE address, ADDR_TYPE parentBlock)
{
    auto nextAddress = *((ADDR_TYPE*)address);
    char* block = (char*)address;
    delete []block;

    if (nextAddress != 0)
        internalDelete(nextAddress, 0);

    if (parentBlock != 0)
        *(ADDR_TYPE*)parentBlock = 0;
}

MemoryStorage::MemoryStorage(uint blockSize){
    auto minSize = sizeof(ADDR_TYPE) + sizeof(uint) + 1;
    if (blockSize < minSize)
    {
        string error = "Block size should have a size of at lear "+to_string(minSize)+" bytes ("+to_string(sizeof(ADDR_TYPE))+" for next block address, "+to_string(sizeof(uint))+" bytes for the total chain size, int bytes, in first block, and 1 for data)";
        throw runtime_error(error.c_str());
    }

    COMMON_BLOCK_HEADER_SIZE = sizeof(ADDR_TYPE);
    this->blockSize = blockSize;

    rootBlocks.push_back((char*)createNewBlock());
}

MemoryStorage::~MemoryStorage(){
    for(auto &c: rootBlocks)
        internalDelete((ADDR_TYPE)c);
    
    rootBlocks.clear();
}

uint MemoryStorage::calculateMemorySpace()
{
    uint result = 0;
    for (auto &c: rootBlocks)
    {
        result += calculateChainMemorySize((ADDR_TYPE)c);        
    }

    return result;
}

uint MemoryStorage::calculateChainMemorySize(ADDR_TYPE address)
{
    auto nextAddress = *((ADDR_TYPE*)address);
    if (nextAddress != 0)
        return blockSize + calculateChainMemorySize(nextAddress);
    else
        return blockSize;
}

ADDR_TYPE MemoryStorage::getFirstBlockSeqAddress()
{
    return reinterpret_cast<ADDR_TYPE>(rootBlocks[0]);
}

ADDR_TYPE MemoryStorage::newBlockSeq()
{
    ADDR_TYPE newBlockAddr = createNewBlock();
    rootBlocks.push_back((char*)newBlockAddr);
    return newBlockAddr;
}

void MemoryStorage::write(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize, bool allowSpaceRelease)
{
    auto [curentBlockAddress, writeBeginAddress] = seekChain(seqAddress, offsetFromSeqBegining);

    auto [finalBlockAddress, finalBlockLastWritePosition] = scrollChain(curentBlockAddress, writeBeginAddress, bufferSize, 0, [buffer](char* currBlock, size_t currBlockWritePos, size_t workPosition)
    {
        currBlock[currBlockWritePos] = buffer[workPosition];
    });

    //update second ADDR_TYPE of seqAddress block with the totalSize of data in the block
    size_t* sizeAddr = (size_t*)(seqAddress + COMMON_BLOCK_HEADER_SIZE);
    auto currentChainSize = *sizeAddr;
    
    if (currentChainSize < (bufferSize + offsetFromSeqBegining))
    {
        *sizeAddr = bufferSize + offsetFromSeqBegining;
        currentChainSize = *sizeAddr;
    }
    else
    {
        //space release should be done here
    }
}

tuple<ADDR_TYPE, size_t> MemoryStorage::seekChain(ADDR_TYPE initialBlockAddress, uint seekSize, bool readOnly)
{
    auto firstBlock_totalChainSize_headersize = sizeof(size_t);
    return _seekChain(initialBlockAddress, seekSize, 0, COMMON_BLOCK_HEADER_SIZE + firstBlock_totalChainSize_headersize, readOnly);
}

tuple<ADDR_TYPE, size_t> MemoryStorage::_seekChain(ADDR_TYPE currentBlockAddress, uint seekSize, uint currentSeekPosition, uint currentBlockOffset, bool readOnly)
{

    auto amountInTheCurrentBlock = (blockSize - currentBlockOffset);

    if (currentSeekPosition + amountInTheCurrentBlock >= seekSize)
    {
        amountInTheCurrentBlock = seekSize - currentSeekPosition;
        return { currentBlockAddress,  currentBlockOffset + amountInTheCurrentBlock};
    }
    else
    {
        char* block = (char*)currentBlockAddress;
        ADDR_TYPE *nextBlockAddress = (ADDR_TYPE*)block; //the nextBlolck addres is in the first typeof(ADDR_TYPE) bytes of the parentBlock
        if (*nextBlockAddress == 0)
        {
            if (readOnly)
                return { currentBlockAddress,  blockSize-1};

            char* newBlock = new char[blockSize];
            *((ADDR_TYPE*)newBlock) = 0; //set the first typeof(ADDR_TYPE) byters with the value 0 (indicanting that there is no next block)
            *nextBlockAddress = reinterpret_cast<ADDR_TYPE>(newBlock);
        }

        return _seekChain(*nextBlockAddress, seekSize, currentSeekPosition + amountInTheCurrentBlock, COMMON_BLOCK_HEADER_SIZE, readOnly);
    }
}

tuple<ADDR_TYPE, size_t> MemoryStorage::scrollChain(
    ADDR_TYPE blockAddress, 
    uint blockOffset, 
    size_t totalWorkSize, 
    size_t currentWorkPos, 
    function<void(
        char* blockP,
        size_t blockPWriteAddr, 
        size_t workPosition
    )> byteWorkFunc,
    bool readOnly
)
{
    char* block = (char*)blockAddress;
    //calculate the amount to be write in the current block
    auto writeSize = (blockSize - blockOffset);
    if (currentWorkPos + writeSize > totalWorkSize)
        writeSize = totalWorkSize - currentWorkPos;

    //write data to block
    for (int c = 0; c < writeSize; c++)
        byteWorkFunc(block, blockOffset+c, currentWorkPos+c);
    
    //write next block
    auto newBufferCurrentPos = currentWorkPos + writeSize;
    if (newBufferCurrentPos < totalWorkSize)
    {
        auto newBlockAddress = getOrCreateBlock(blockAddress, readOnly);
        if (readOnly && newBlockAddress == 0)
            return {blockAddress, blockSize-1};

        //offset 1 is because first ADDR_TYPE of block (typeof(ADDR_TYPE) bytes) is reserved for address of next chain block
        return scrollChain(newBlockAddress, COMMON_BLOCK_HEADER_SIZE, totalWorkSize, newBufferCurrentPos, byteWorkFunc);
    }
    else 
        return { blockAddress,  blockOffset + writeSize};
}

/**
 * @brief Get the Or Create Block. If readonly = true and end of chain is reached, the value 0 is returned
 * 
 * @param parentBlockAddress 
 * @param readOnly 
 * @return ADDR_TYPE 
 */
ADDR_TYPE MemoryStorage::getOrCreateBlock(ADDR_TYPE parentBlockAddress, bool readOnly)
{
    char* parentBlock = (char*)parentBlockAddress;
    ADDR_TYPE *currentNextBlockAddress = (ADDR_TYPE*)parentBlock; //the nextBlolck addres is in the first typeof(ADDR_TYPE) bytes of the parentBlock

    if (*currentNextBlockAddress == 0)
    {
        if (readOnly)
            return 0;

        
        *currentNextBlockAddress = createNewBlock();
    }


    ADDR_TYPE *tmp = (ADDR_TYPE*)parentBlock; //the nextBlolck addres is in the first typeof(ADDR_TYPE) bytes of the parentBlock

    return *currentNextBlockAddress;
}

ADDR_TYPE MemoryStorage::createNewBlock()
{
    char* newBlock = new char[blockSize];
    *((ADDR_TYPE*)newBlock) = 0; //set the first typeof(ADDR_TYPE) byters with the value 0 (indicanting that there is no next block)
    *((size_t*)(newBlock + COMMON_BLOCK_HEADER_SIZE)) = 0;
    
    return reinterpret_cast<ADDR_TYPE>(newBlock);
}

uint MemoryStorage::read(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize){ 
    uint totalCommited = 0;
    auto [curentBlockAddress, writeBeginAddress] = seekChain(seqAddress, offsetFromSeqBegining, true);
    auto [finalBlockAddress, finalBlockLastWritePosition] = scrollChain(curentBlockAddress, writeBeginAddress, bufferSize, 0, [&, buffer](char* currBlock, size_t currBlockReadPos, size_t workPosition)
    {
        buffer[workPosition] = currBlock[currBlockReadPos];
        totalCommited++;
    }, true);

    return totalCommited;
}

void MemoryStorage::deleteBlockSq(ADDR_TYPE seqAddress) {
    for (int c = rootBlocks.size()-1; c >= 0; c--)
    {
        ADDR_TYPE tmp = (ADDR_TYPE)(rootBlocks[c]);
        if (tmp == seqAddress)
            rootBlocks.erase(rootBlocks.begin() + c);
    }
    internalDelete(seqAddress);
}

size_t MemoryStorage::getChainSize(ADDR_TYPE seqAddress)
{
    size_t *dataLengthAddr = (size_t*)(seqAddress + COMMON_BLOCK_HEADER_SIZE); //the nextBlolck addres is in the first typeof(ADDR_TYPE) bytes of the parentBlock
    auto dataLength = *dataLengthAddr;
    return dataLength;
}
