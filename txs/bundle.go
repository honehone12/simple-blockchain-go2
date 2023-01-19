package txs

import (
	"encoding/json"
	"errors"
	"simple-blockchain-go2/common/merkle"

	"github.com/btcsuite/btcutil/base58"
)

type TxBundle struct {
	Transactions []Transaction
}

func (txb *TxBundle) HashTransactions() ([]byte, error) {
	if txb.Transactions == nil || len(txb.Transactions) == 0 {
		return nil, errors.New("emptry tx bundle")
	}

	var raws [][]byte
	for _, tx := range txb.Transactions {
		enc, err := json.Marshal(&tx)
		if err != nil {
			return nil, err
		}
		raws = append(raws, enc)
	}

	mTree, err := merkle.NewMerkleTreeFromRaw(raws)
	if err != nil {
		return nil, err
	}
	return mTree.RootNode.Data, nil
}

func (txb *TxBundle) ToKeys() []string {
	lenTx := len(txb.Transactions)
	keys := make([]string, lenTx)
	for i := 0; i < lenTx; i++ {
		keys[i] = base58.Encode(txb.Transactions[i].Hash[:])
	}
	return keys
}

func (txb *TxBundle) Verify() bool {
	if txb.Transactions == nil || len(txb.Transactions) == 0 {
		return false
	}
	for _, tx := range txb.Transactions {
		ok, err := tx.Verify()
		if err != nil || !ok {
			return false
		}
	}
	return true
}
