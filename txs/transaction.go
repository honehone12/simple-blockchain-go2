package txs

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"log"
	"simple-blockchain-go2/common"
	"time"

	"golang.org/x/crypto/sha3"
)

type TransactionData struct {
	Data      []byte
	PublicKey []byte
	Nonce     uint64
	Signature []byte
	Timestamp int64
}

type Transaction struct {
	Hash      []byte
	InnerData TransactionData
	Status    bool
}

func NewTransaction(data []byte) Transaction {
	return Transaction{
		Hash: nil,
		InnerData: TransactionData{
			Data:      data,
			PublicKey: nil,
			Nonce:     0,
			Signature: nil,
			Timestamp: time.Now().Unix(),
		},
		Status: false,
	}
}

func (tx *Transaction) CheckContents() bool {
	return tx.InnerData.Timestamp > 0 &&
		tx.InnerData.Data != nil && len(tx.InnerData.Data) > 0 &&
		len(tx.InnerData.Data) <= common.MaxPayloadSize &&
		tx.InnerData.PublicKey != nil &&
		len(tx.InnerData.PublicKey) == common.PublicKeySize &&
		tx.InnerData.Signature != nil && len(tx.InnerData.Signature) > 0 &&
		tx.Hash != nil && len(tx.Hash) == common.HashSize
}

func (tx *Transaction) Verify() (bool, error) {
	if !tx.CheckContents() {
		return false, nil
	}

	enc, err := json.Marshal(&tx.InnerData)
	if err != nil {
		return false, err
	}
	hash := sha3.Sum256(enc)
	if !bytes.Equal(hash[:], tx.Hash) {
		log.Println("transaction hash is broken")
		return false, nil
	}

	return ed25519.Verify(
		tx.InnerData.PublicKey,
		tx.InnerData.Data,
		tx.InnerData.Signature,
	), nil
}
