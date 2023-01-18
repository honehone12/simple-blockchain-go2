package memory

import (
	"log"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/txs"

	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

type Mempool struct {
	pool map[string]txs.Transaction
}

func NewMempool() *Mempool {
	return &Mempool{
		pool: make(map[string]txs.Transaction),
	}
}

func (mp *Mempool) Len() int {
	return len(mp.pool)
}

func (mp *Mempool) Append(tx *txs.Transaction) {
	key := base58.Encode(tx.Hash[:])
	mp.pool[key] = *tx
	log.Printf("mempool current length: %d\n", len(mp.pool))
}

func (mp *Mempool) get(n int) []txs.Transaction {
	if n <= 0 {
		return nil
	}

	all := maps.Values(mp.pool)
	if n > len(all) {
		return nil
	}
	if n == 1 {
		return all
	}

	slices.SortFunc(all, func(a, b txs.Transaction) bool {
		return a.InnerData.Nonce < b.InnerData.Nonce
	})
	return all[:n]
}

func (mp *Mempool) GetTxsForBlock() []txs.Transaction {
	txLen := len(mp.pool)
	if txLen == 0 {
		return nil
	}
	if txLen == 1 {
		return mp.get(1)
	}

	len := common.LastPowerOf2(txLen)
	return mp.get(len)
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
