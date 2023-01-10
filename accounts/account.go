package accounts

import (
	"encoding/json"
	"log"
	"simple-blockchain-go2/common"
)

type AccountState struct {
	Nonce   uint64
	Balance uint64
}

type Account struct {
	PublicKey []byte
	State     AccountState
	Timestamp int64
}

func (as *AccountState) Subtract(amount uint64) bool {
	if amount > as.Balance {
		return false
	}

	as.Balance -= amount
	return true
}

func (as *AccountState) Add(amount uint64) bool {
	max := common.GeneratorBalance - as.Balance
	if amount > max {
		return false
	}

	as.Balance += amount
	return true
}

func (as *AccountState) CheckNonce(nonce uint64) bool {
	log.Printf("checking nonce, received: %d expected: %d", nonce, as.Nonce)
	ok := as.Nonce == nonce
	if ok {
		as.Nonce++
	}
	return ok
}

func (as AccountState) ToBytes() ([]byte, error) {
	return json.Marshal(as)
}

func (a *Account) ToBytes() ([]byte, error) {
	return json.Marshal(a.State)
}

func (a *Account) GetKey() ([]byte, error) {
	return a.PublicKey, nil
}

func (a *Account) GetIndex() ([]byte, error) {
	return common.ToHex(a.Timestamp)
}
