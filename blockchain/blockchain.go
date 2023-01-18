package blockchain

const (
	SlotsInEpoch uint32 = 30
)

type BlockchainInfo interface {
	NextHeight() uint64
	NextEpoch() uint64
	NextSlot() uint32
	PreviousBlockHash() []byte
}

type Blockchain struct {
	nextHeight        uint64
	nextEpoch         uint64
	nextSlot          uint32
	previousBlockHash []byte
}

func NewBlockchain() *Blockchain {
	return &Blockchain{
		nextHeight: 0,
	}
}

func (bc *Blockchain) Init(height uint64, prevHash []byte) {
	bc.nextHeight = height
	bc.nextEpoch = height / uint64(SlotsInEpoch)
	bc.nextSlot = uint32(height) % SlotsInEpoch
	bc.previousBlockHash = prevHash
}

func (bc *Blockchain) incrementHeight() {
	bc.nextHeight++
}

func (bc *Blockchain) incrementEpoch() {
	bc.nextEpoch++
}

func (bc *Blockchain) icrementSlot() {
	bc.nextSlot = (bc.nextSlot + 1) % SlotsInEpoch
	if bc.nextSlot == 0 {
		bc.incrementEpoch()
	}
}

func (bc *Blockchain) Increment(blockHash []byte) {
	bc.incrementHeight()
	bc.icrementSlot()
	bc.previousBlockHash = blockHash
}

func (bc *Blockchain) NextHeight() uint64 {
	return bc.nextHeight
}

func (bc *Blockchain) NextEpoch() uint64 {
	return bc.nextEpoch
}

func (bc *Blockchain) NextSlot() uint32 {
	return bc.nextSlot
}

func (bc *Blockchain) PreviousBlockHash() []byte {
	return bc.previousBlockHash
}
