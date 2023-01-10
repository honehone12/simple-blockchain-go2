package blockchain

type BlockchainInfo interface {
	LastHeight() uint64
}

type Blockchain struct {
	lastHeight uint64
}

func NewBlockchain() *Blockchain {
	return &Blockchain{
		lastHeight: 0,
	}
}

func (bc *Blockchain) Increment() {
	bc.lastHeight++
}

func (bc *Blockchain) LastHeight() uint64 {
	return bc.lastHeight
}
