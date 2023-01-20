package executers

import (
	"encoding/json"
	"errors"
	"log"
	"simple-blockchain-go2/accounts"
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/storage"
	"time"
)

type Faucet struct {
	generator []byte

	storageHandle storage.StorageHandle
}

func NewFaucet(storageHandle storage.StorageHandle) *Faucet {
	return &Faucet{
		generator:     nil,
		storageHandle: storageHandle,
	}
}

func (f *Faucet) Init() error {
	resCh := f.storageHandle.Get(storage.Blocks, storage.Index, make([]byte, 8))
	res := <-resCh
	if res.Err != nil {
		return res.Err
	}
	if res.Value == nil {
		log.Println("storage is empty")
		return nil
	}

	blk := blocks.Block{}
	err := json.Unmarshal(res.Value, &blk)
	if err != nil {
		return err
	}

	resCh = f.storageHandle.Get(
		storage.Accounts, storage.Key, blk.Info.PublicKey,
	)
	res = <-resCh
	if res.Err != nil {
		return res.Err
	}
	if res.Value == nil {
		generatorAccount := accounts.Account{
			PublicKey: blk.Info.PublicKey,
			State: accounts.AccountState{
				Nonce:   0,
				Balance: common.GeneratorBalance,
			},
			Timestamp: time.Now().Unix(),
		}
		resCh = f.storageHandle.Put(storage.Accounts, &generatorAccount)
		res = <-resCh
		if res.Err != nil {
			return res.Err
		}
		log.Println("genetator balance has set")
	}

	f.generator = blk.Info.PublicKey
	return nil
}

func (f *Faucet) Generator() ([]byte, error) {
	if f.generator == nil {
		return nil, errors.New("generator is not initialized yet")
	}
	return f.generator, nil
}
