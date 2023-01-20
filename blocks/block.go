package blocks

import (
	"crypto/ed25519"
	"encoding/json"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/txs"
	"time"

	"golang.org/x/crypto/sha3"
)

type BlockInfo struct {
	Height    uint64
	Hash      []byte
	Signature []byte
	PublicKey []byte
	Timestamp int64
}

type Block struct {
	Info              BlockInfo
	Bundle            txs.TxBundle
	PreviousBlockHash []byte
	StateHash         []byte
}

func NewBlock(
	height uint64,
	pubKey []byte,
	transactions txs.TxBundle,
	prevHash []byte,
	stateHash []byte,
) *Block {
	block := Block{
		Info: BlockInfo{
			Height:    height,
			Hash:      nil,
			Signature: nil,
			PublicKey: pubKey,
			Timestamp: time.Now().Unix(),
		},
		Bundle:            transactions,
		PreviousBlockHash: prevHash,
		StateHash:         stateHash,
	}
	return &block
}

func (b *Block) HashBlock() error {
	txHash, err := b.Bundle.HashTransactions()
	if err != nil {
		return err
	}
	add := make(
		[]byte, 0, len(txHash)+len(b.PreviousBlockHash)+len(b.StateHash),
	)
	add = append(add, txHash...)
	add = append(add, b.PreviousBlockHash...)
	add = append(add, b.StateHash...)
	hash := sha3.Sum256(add)
	b.Info.Hash = hash[:]
	return nil
}

func (bi *BlockInfo) Verify() bool {
	return bi.PublicKey != nil && len(bi.PublicKey) == common.PublicKeySize &&
		bi.Hash != nil && len(bi.Hash) == common.HashSize &&
		bi.Signature != nil && len(bi.Signature) > 0 &&
		bi.Timestamp > 0 &&
		ed25519.Verify(bi.PublicKey, bi.Hash, bi.Signature)
}

func (b *Block) Verify() bool {
	return b.Bundle.Verify() &&
		b.StateHash != nil && len(b.StateHash) == common.HashSize &&
		b.PreviousBlockHash != nil && len(b.PreviousBlockHash) == common.HashSize &&
		b.Info.Verify()
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
