package genesis

import (
	"crypto/rand"
	"errors"
	"log"
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/storage"
	"simple-blockchain-go2/txs"
	"time"

	"golang.org/x/crypto/sha3"
)

type Generator struct {
	wallet        *wallets.Wallet
	storageHandle storage.StorageHandle
}

func NewGenerator(name string) (*Generator, error) {
	if common.ExistFile(storage.DatabaseFileName(name)) {
		return nil, errors.New("file already exist")
	}

	s, err := storage.NewStorageService(name)
	if err != nil {
		return nil, err
	}
	w, err := wallets.NewWallet(name)
	if err != nil {
		return nil, err
	}

	s.Run()
	return &Generator{
		wallet:        w,
		storageHandle: s,
	}, nil
}

func (gen *Generator) Generate() error {
	prev := make([]byte, 32)
	_, err := rand.Read(prev)
	if err != nil {
		return err
	}
	state := make([]byte, 32)
	_, err = rand.Read(state)
	if err != nil {
		return err
	}

	b := blocks.Block{
		Info: blocks.BlockInfo{
			Height:    0,
			Hash:      nil,
			PublicKey: gen.wallet.PublicKey(),
			Signature: nil,
			Timestamp: time.Now().Unix(),
		},
		Bundle:            txs.TxBundle{Transactions: nil},
		PreviousBlockHash: prev,
		StateHash:         state,
	}

	add := make([]byte, 0, 64)
	add = append(add, prev...)
	add = append(add, state...)
	hash := sha3.Sum256(add)
	b.Info.Hash = hash[:]
	b.Info.Signature = gen.wallet.SignRaw(hash[:])

	resCh := gen.storageHandle.Put(storage.Blocks, &b)
	res := <-resCh
	if res.Err != nil {
		return res.Err
	}
	log.Printf("created genesis block hash:%x\n", hash)
	return nil
}
