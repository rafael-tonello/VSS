#include  "filestorage.h" 

#include <iostream>
 
FileStorage::FileStorage(uint blockSize, string fname)
{
    this->blockSize = blockSize;
    this->fname = fname;
} 
 
FileStorage::~FileStorage() 
{ 
    file.close();
} 

bool FileStorage::fileExists(string f)
{
    if (FILE *file = fopen(f.c_str(), "r")) {
        fclose(file);
        return true;
    } else {
        return false;
    }      
}

Error FileStorage::init()
{
    if (fileExists(fname))
    {
        //open a existent file (for read and write, do not trunc data)
        file.open(fname, std::fstream::in | std::fstream::out | std::fstream::binary);



        if (file.is_open() == false)
        {
            auto errorNumber = errno;
            //get errono message
            auto error = Errors::createError("Error opening file: " + string(strerror(errno)) + " (errno " + to_string(errorNumber) + ")");

            return error;
        }
    }
    else
    {   
        //system(string("cd $HOME").c_str());
        system(("touch " + fname).c_str());
        //create a new file for read and write
        file.open(fname, std::fstream::in | std::fstream::out | std::fstream::trunc | std::fstream::binary);

        if (file.is_open() == false)
        {
            auto errorNumber = errno;
            //get errono message
            auto error = Errors::createError("Error opening file: " + string(strerror(errno)) + " (errno " + to_string(errorNumber) + ")");

            return error;
        }

        FileHeader tmpheader
        {
            .firstblockAddr = sizeof(FileHeader),
            .deletedSeqAddr = 0
        };

        this->file.write((char*)&tmpheader, sizeof(tmpheader));
        //file.flush();

        //init the firstBlock
        this->initBlock(tmpheader.firstblockAddr);
    }

    file.seekg(0);
    
    this->file.read((char*)&this->header, sizeof(FileHeader));

    return "";

}
 
ADDR_TYPE FileStorage::getFirstBlockSeqAddress()
{
    //read header block;
    return this->header.firstblockAddr;
}

ADDR_TYPE FileStorage::newBlockSeq()
{
    ADDR_TYPE endOfFileAddr = getAddrForANewBlock();

    initBlock(endOfFileAddr);

    return endOfFileAddr;
}

void FileStorage::initBlock(ADDR_TYPE address)
{
    file.seekp(address);

    char* tmp = new char[blockSize];
    for (int c = 0; c < blockSize; c++) tmp[c] = 0;
    file.write((char*)tmp, blockSize);
    //file.flush();
    delete[] tmp;

    file.seekg(address);
}

size_t FileStorage::getFileSize(fstream &f){
    f.seekg(0, ios_base::end);
    return f.tellg();
}

ADDR_TYPE FileStorage::getAddrForANewBlock(){
    //check for a new blcok in the deleted blocks list
    ADDR_TYPE ret = getFileSize(file);
    if (header.deletedSeqAddr != 0)
    {
        std::cout<< "reusing adeleted block\n";

        ret = header.deletedSeqAddr;
        auto newDeletedBlocksHeading = readNextBlockAddress(ret);
        linkBlocks(header.deletedSeqAddr, 0);
        header.deletedSeqAddr = newDeletedBlocksHeading;
        file.seekp(0);
        this->file.write((char*)&header, sizeof(header));    
        //file.flush();
    }
    
    return ret;
}

void FileStorage::write(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize, bool allowSpaceRelease)
{
    auto [curentBlockAddress, writeBeginAddress] = seekChain(seqAddress, offsetFromSeqBegining);

    auto [finalBlockAddress, finalBlockLastWritePosition] = scrollChain(
        curentBlockAddress, 
        writeBeginAddress, 
        bufferSize, 
        0, 
        [&, buffer](uint amountToBeWorked, size_t workPosition){
            file.write((char*)(buffer + workPosition), (size_t)amountToBeWorked);
            //file.flush();
        }
    );

    //update second ADDR_TYPE of seqAddress block with the totalSize of data in the block
    ADDR_TYPE sizeAddr = seqAddress + sizeof(ADDR_TYPE);
    //size_t currentChainSize = 0;
    //file.seek(sizeAddr);
    //file.read((char*)&currentChainSize, sizeof(currentChainSize));
    size_t currentChainSize = getChainSize(seqAddress);

    if (currentChainSize < (bufferSize + offsetFromSeqBegining) || allowSpaceRelease)
    {

        currentChainSize = bufferSize + offsetFromSeqBegining;
        file.seekp(sizeAddr);
        file.write((char*)&currentChainSize, sizeof(currentChainSize));
        //file.flush();


        size_t tmp = 0;
        file.seekg(sizeAddr);
        file.read((char*)&tmp, sizeof(tmp));
    }
    
    //space release should be done here
    if (allowSpaceRelease)
    {
        auto nextBlockAddr = readNextBlockAddress(finalBlockAddress);
        if (nextBlockAddr != 0)
        {
            internalDeleteBlockSq(nextBlockAddr);
            linkBlocks(finalBlockAddress, 0);
        }
        
    }
}

uint FileStorage::read(ADDR_TYPE seqAddress, uint offsetFromSeqBegining, char* buffer, size_t bufferSize)
{
    uint result = 0;
    uint totalChainDataSize = getChainSize(seqAddress);
    if (offsetFromSeqBegining > totalChainDataSize)
        return 0;

    if (bufferSize + offsetFromSeqBegining > totalChainDataSize)
    {
        bufferSize = totalChainDataSize - offsetFromSeqBegining;

    }
    auto [curentBlockAddress, readBeginAddress] = seekChain(seqAddress, offsetFromSeqBegining);
    
    auto [finalBlockAddress, finalBlockLastWritePosition] = scrollChain(
        curentBlockAddress, 
        readBeginAddress, 
        bufferSize, 
        0, 
        [&, buffer](uint amountToBeWorked, size_t workPosition){
            file.read((char*)(buffer + workPosition), (size_t)amountToBeWorked);
            result += amountToBeWorked;
        },
        true
    );
    return result;
}

tuple<ADDR_TYPE, size_t> FileStorage::seekChain(ADDR_TYPE initialBlockAddress, uint seekSize, bool readOnly)
{
    auto firstBlock_totalChainSize_headersize = sizeof(size_t);
    return _seekChain(initialBlockAddress, seekSize, 0, sizeof(ADDR_TYPE) + firstBlock_totalChainSize_headersize, readOnly);
}

tuple<ADDR_TYPE, size_t> FileStorage::_seekChain(ADDR_TYPE currentBlockAddress, uint seekSize, uint currentSeekPosition, uint currentBlockOffset, bool readOnly)
{
    auto amountInTheCurrentBlock = (blockSize - currentBlockOffset);

    if (currentSeekPosition + amountInTheCurrentBlock >= seekSize)
    {
        amountInTheCurrentBlock = seekSize - currentSeekPosition;
        return { currentBlockAddress,  currentBlockOffset + amountInTheCurrentBlock};
    }
    else
    {
        ADDR_TYPE nextBlockAddress = readNextBlockAddress(currentBlockAddress);

        if (nextBlockAddress == 0)
        {
            if (readOnly)
                return { currentBlockAddress,  blockSize-1};

            nextBlockAddress = this->newBlockSeq();
        }

        return _seekChain(nextBlockAddress, seekSize, currentSeekPosition + amountInTheCurrentBlock, sizeof(ADDR_TYPE), readOnly);
    }
}


tuple<ADDR_TYPE, size_t> FileStorage::scrollChain(
    ADDR_TYPE blockAddress, 
    uint blockOffset, 
    size_t totalWorkSize, 
    size_t currentWorkPos, 
    function<void(
        uint amountToBeWorked, 
        size_t workPosition
    )> workFunc,
    bool readOnly
)
{
    //calculate the amount to be write in the current block
    uint writeSize = (blockSize - blockOffset);
    if (currentWorkPos + writeSize > totalWorkSize)
        writeSize = totalWorkSize - currentWorkPos;

    //write data to block
    file.seekg(blockAddress + blockOffset);
    file.seekp(blockAddress + blockOffset);
    workFunc(writeSize, currentWorkPos);
    
    //write next block
    auto newCurrentBufferPos = currentWorkPos + writeSize;
    if (newCurrentBufferPos < totalWorkSize)
    {
        auto newBlockAddress = getOrCreateNextBlock(blockAddress, readOnly);
        if (readOnly && newBlockAddress == 0)
            return {blockAddress, blockSize-1};

        //offset 1 is because first ADDR_TYPE of block (typeof(ADDR_TYPE) bytes) is reserved for address of next chain block
        return scrollChain(newBlockAddress, sizeof(ADDR_TYPE), totalWorkSize, newCurrentBufferPos, workFunc);
    }
    else
        return { blockAddress,  blockOffset + writeSize};
}


ADDR_TYPE FileStorage::readNextBlockAddress(ADDR_TYPE blockAddr)
{
    if (blockAddr == 0)
        return 0;

    file.seekg(blockAddr);
    //fread()
    ADDR_TYPE result = 0;

    file.read((char*)&result, sizeof(result));

    return result;
}

ADDR_TYPE FileStorage::getOrCreateNextBlock(ADDR_TYPE parentBlockAddress, bool readOnly)
{
    ADDR_TYPE currentNextBlockAddress = readNextBlockAddress(parentBlockAddress);

    if (currentNextBlockAddress == 0)
    {
        if (readOnly)
            return 0;
        
        currentNextBlockAddress = newBlockSeq();

        linkBlocks(parentBlockAddress, currentNextBlockAddress);
    }


    return currentNextBlockAddress;
}

void FileStorage::internalDeleteBlockSq(ADDR_TYPE seqAddress, ADDR_TYPE parentBlockToEraseSeqReference)
{
    if (seqAddress == 0)
        return;
        
    auto lastBlockAddr = getAddressOfTheLastBlockInTheSequence(header.deletedSeqAddr);

    if (lastBlockAddr != 0)
        linkBlocks(lastBlockAddr, seqAddress);
    else
    {
        header.deletedSeqAddr = seqAddress;
        file.seekp(0);
        this->file.write((char*)&header, sizeof(header));
        //file.flush();
    }


    if (parentBlockToEraseSeqReference != 0)
        linkBlocks(parentBlockToEraseSeqReference, 0);
}

void FileStorage::deleteBlockSq(ADDR_TYPE seqAddress)
{
    this->internalDeleteBlockSq(seqAddress);
}

ADDR_TYPE FileStorage::getAddressOfTheLastBlockInTheSequence(ADDR_TYPE parentBlockAddr)
{
    auto nextBlockAddr = readNextBlockAddress(parentBlockAddr);
    if (nextBlockAddr == 0)
        return parentBlockAddr;
    else
        return getAddressOfTheLastBlockInTheSequence(nextBlockAddr);
}

void FileStorage::linkBlocks(ADDR_TYPE parentBlock, ADDR_TYPE childBlock)
{
    file.seekp(parentBlock);
    file.write((char*)&childBlock, sizeof(childBlock));
    //file.flush();
}

size_t FileStorage::getChainSize(ADDR_TYPE seqAddress)
{
    ADDR_TYPE sizeAddr = seqAddress + sizeof(ADDR_TYPE);
    size_t currentChainSize = 0;
    file.seekg(sizeAddr);
    file.read((char*)&currentChainSize, sizeof(currentChainSize));

    return currentChainSize;
}
