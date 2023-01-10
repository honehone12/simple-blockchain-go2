package memory

import (
	"simple-blockchain-go2/txs"

	"github.com/btcsuite/btcutil/base58"
)

type Mempool struct {
	pool map[string]txs.Transaction
}

func NewMempool() *Mempool {
	return &Mempool{
		pool: make(map[string]txs.Transaction),
	}
}

func (mp *Mempool) Append(tx *txs.Transaction) {
	key := base58.Encode(tx.Hash[:])
	mp.pool[key] = *tx
}

func (mp *Mempool) Remove(keys []string) {
	for _, k := range keys {
		delete(mp.pool, k)
	}
}
