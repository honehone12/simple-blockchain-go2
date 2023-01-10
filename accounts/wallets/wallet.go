package wallets

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"simple-blockchain-go2/accounts/keys"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/txs"
)

const (
	keypairFile = "%s.key"
)

type Wallet struct {
	keyPair *keys.KeyPair
	nonce   uint64
	balance uint64
}

func NewWallet(name string) (*Wallet, error) {
	keyFile := fmt.Sprintf(keypairFile, name)
	if common.ExistFile(keyFile) {
		f, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, err
		}

		key := keys.KeyPair{}
		err = json.Unmarshal(f, &key)
		if err != nil {
			return nil, err
		}
		return fromKeyPair(&key), nil
	}

	key, err := keys.GenerateKey()
	if err != nil {
		return nil, err
	}
	enc, err := json.Marshal(key)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(keyFile, enc, 0644)
	if err != nil {
		return nil, err
	}
	log.Printf("new account '%s' is created", name)
	return fromKeyPair(key), nil
}

func (w *Wallet) Init(nonce uint64, balance uint64) {
	w.nonce = nonce
	w.balance = balance
}

func (w *Wallet) Sign(tx *txs.Transaction) error {
	tx.InnerData.PublicKey = w.PublicKey()
	tx.InnerData.Nonce = w.nonce
	return w.keyPair.Sign(tx)
}

func (w *Wallet) SignRaw(data []byte) []byte {
	return w.keyPair.SignRaw(data)
}

func fromKeyPair(kp *keys.KeyPair) *Wallet {
	return &Wallet{
		keyPair: kp,
		nonce:   0,
		balance: 0,
	}
}

func (w *Wallet) PublicKey() []byte {
	return w.keyPair.PublicKey
}
