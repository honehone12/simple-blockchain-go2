package memory

import (
	"log"
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
	log.Printf("mempool current length: %d\n", len(mp.pool))
}

func (mp *Mempool) Remove(keys []string) {
	i := 0
	for _, k := range keys {
		delete(mp.pool, k)
		i++
	}
	log.Printf("%d deleted from mempool\n", i)
}

func (mp *Mempool) Contains(key string) bool {
	_, ok := mp.pool[key]
	return ok
}
