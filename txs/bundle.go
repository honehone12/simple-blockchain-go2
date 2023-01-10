package txs

import (
	"encoding/json"
	"errors"
	"simple-blockchain-go2/common/merkle"
)

type TxBundle struct {
	Transactions []Transaction
}

func (txb *TxBundle) HashTransactions() ([]byte, error) {
	if len(txb.Transactions) == 0 {
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

	mTree, err := merkle.NewMerkleTree(raws)
	if err != nil {
		return nil, err
	}
	return mTree.RootNode.Data, nil
}
