package accounts

import (
	"encoding/json"
	"simple-blockchain-go2/common"
)

type AccountState struct {
	Nonce   uint64
	Balance uint64
}

// this is not used but only in faucet
// all other accounts are pushed from memory
type Account struct {
	PublicKey []byte
	State     AccountState
	Timestamp int64 // time created or time last update ??
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
	ok := as.Nonce == nonce
	if ok {
		as.Nonce++
	}
	return ok
}

func (as *AccountState) ToBytes() ([]byte, error) {
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
