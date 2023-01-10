package keys

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"simple-blockchain-go2/txs"

	"golang.org/x/crypto/sha3"
)

type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

func GenerateKey() (*KeyPair, error) {
	pub, pri, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		PrivateKey: pri,
		PublicKey:  pub,
	}, nil
}

func (kp *KeyPair) Sign(tx *txs.Transaction) error {
	sig := ed25519.Sign(kp.PrivateKey, tx.InnerData.Data)
	tx.InnerData.Signature = sig

	enc, err := json.Marshal(tx.InnerData)
	if err != nil {
		return err
	}
	hash := sha3.Sum256(enc)
	tx.Hash = hash
	return nil
}

func (kp *KeyPair) SignRaw(data []byte) []byte {
	return ed25519.Sign(kp.PrivateKey, data)
}
