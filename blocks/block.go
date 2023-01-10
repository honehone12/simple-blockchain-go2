package blocks

import (
	"encoding/json"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/txs"
	"time"
)

type BlockInfo struct {
	Height    uint64
	Hash      []byte
	Signature []byte
	Timestamp int64
}

type Block struct {
	Info              BlockInfo
	Bundle            txs.TxBundle
	PreviousBlockHash []byte
	StateHash         []byte
}

func NewBlock(
	transactions txs.TxBundle,
) *Block {
	block := Block{
		Info: BlockInfo{
			Height:    0,
			Hash:      nil,
			Signature: nil,
			Timestamp: time.Now().Unix(),
		},
		Bundle:            transactions,
		PreviousBlockHash: nil,
		StateHash:         nil,
	}
	return &block
}

func (b *Block) ToBytes() ([]byte, error) {
	return json.Marshal(b)
}

func (b *Block) GetKey() ([]byte, error) {
	return b.Info.Hash, nil
}

func (b *Block) GetIndex() ([]byte, error) {
	return common.ToHex(b.Info.Height)
}
