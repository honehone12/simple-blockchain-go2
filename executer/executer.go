package executer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/rpc"
	"simple-blockchain-go2/storage"
	"simple-blockchain-go2/txs"

	"golang.org/x/exp/slices"
)

type Executer struct {
	storageHandle storage.StorageHandle
	memoryHandle  memory.MemoryHandle

	faucet *Faucet
}

type ExecuteHandle interface {
	Execute(
		transanctions []txs.Transaction, errCh chan<- error,
	) (chan<- bool, error)
}

type dirtyAccounts struct {
	accounts [][]byte
}

func newDirtyAccounts() *dirtyAccounts {
	return &dirtyAccounts{accounts: make([][]byte, 0)}
}

func (da *dirtyAccounts) add(pubKey []byte) {
	if !slices.ContainsFunc(da.accounts, func(pk []byte) bool {
		return bytes.Equal(pubKey, pk)
	}) {
		da.accounts = append(da.accounts, pubKey)
	}
}

func NewExecuter(
	storage storage.StorageHandle, memory memory.MemoryHandle,
) *Executer {
	return &Executer{
		storageHandle: storage,
		memoryHandle:  memory,
		faucet:        NewFaucet(storage),
	}
}

func (e *Executer) Init() error {
	return e.faucet.Init()
}

func (e *Executer) Execute(
	transanctions []txs.Transaction, errCh chan<- error,
) (chan<- bool, error) {
	log.Println("executing transactions...")

	dirty := newDirtyAccounts()
	for _, tx := range transanctions {
		call := rpc.Call{}
		err := json.Unmarshal(tx.InnerData.Data, &call)
		if err != nil {
			log.Println(
				"failed to unmarshal , should be removed before block creation: ",
				err.Error(),
			)
			continue
		}

		if !call.Verify() {
			log.Println("found unverified call, should be removed before block creation")
			continue
		}

		switch call.Call {
		case rpc.Airdrop:
			err = e.executeAirdrop(
				call.Params,
				tx.InnerData.PublicKey,
				tx.InnerData.Nonce,
				dirty,
			)
		case rpc.Transfer:
			err = e.executeTransfer(
				call.Params,
				tx.InnerData.PublicKey,
				tx.InnerData.Nonce,
				dirty,
			)
		default:
			log.Println("found undefined call, should be removed before block creation")
			continue
		}
		if err != nil {
			// !!
			// need to reset dirty account
			er := e.revert(dirty, err)
			if er != nil {
				return nil, er
			}
			log.Printf("reverted, original error: %s\n", err.Error())
		}
	}

	log.Println("all execution done")
	finCh := make(chan bool)
	go e.waitForFinality(finCh, errCh, dirty)
	return finCh, nil
}

func (e *Executer) waitForFinality(
	finCh <-chan bool,
	errCh chan<- error,
	dirty *dirtyAccounts,
) {
	ok := <-finCh
	if ok {
		// !!
		// dirty is mistake
		// need all account for change
		err := e.storageHandle.PushAccounts(e.memoryHandle, dirty.accounts)
		if err != nil {
			er := e.revert(dirty, err)
			if er != nil {
				errCh <- er
			}
		}
	} else {
		err := e.revert(dirty, errors.New("block was rejected"))
		if err != nil {
			errCh <- err
		}
	}
}

func (e *Executer) revert(dirty *dirtyAccounts, prevError error) error {
	log.Printf("error: %s reverting...\n", prevError.Error())
	err := e.storageHandle.FetchAccounts(e.memoryHandle, dirty.accounts)
	if err != nil {
		return fmt.Errorf(
			"original error: %s, another error accoured on revert: %s",
			prevError.Error(), err.Error(),
		)
	}
	return nil
}

func (e *Executer) executeAirdrop(
	rawParam []byte,
	caller []byte, nonce uint64,
	dirty *dirtyAccounts,
) error {
	log.Println("executing airdrop...")

	param := rpc.AirdropParam{}
	err := json.Unmarshal(rawParam, &param)
	if err != nil {
		return errors.New(
			"failed to umarshal airdrop param, should be checked before execute",
		)
	}

	gen, err := e.faucet.Generator()
	if err != nil {
		return err
	}
	err = e.airdropImpl(
		caller, nonce,
		gen,
		param.Amount,
	)
	if err != nil {
		dirty.add(caller)
		dirty.add(gen)
		return err
	}
	return nil
}

func (e *Executer) airdropImpl(
	to []byte, nonce uint64,
	from []byte,
	amount uint64,
) error {
	fromCh := e.memoryHandle.GetAccountState(from)
	toCh := e.memoryHandle.GetAccountState(to)

	fromRes := <-fromCh
	if fromRes.Err != nil {
		return fromRes.Err
	}
	fromState := fromRes.Value

	toRes := <-toCh
	if toRes.Err != nil {
		log.Printf("creating new account: %x\n", to)
	}
	toState := toRes.Value
	if !toState.CheckNonce(nonce) {
		return errors.New("nonce is Invalid")
	}

	if !fromState.Subtract(amount) {
		return errors.New("not enough balance")
	}
	if !toState.Add(amount) {
		return errors.New("overflow")
	}

	fromCh = e.memoryHandle.PutAccountState(from, &fromState)
	toCh = e.memoryHandle.PutAccountState(to, &toState)
	<-fromCh
	<-toCh
	return nil
}

func (e *Executer) executeTransfer(
	rawParam []byte,
	caller []byte, nonce uint64,
	dirty *dirtyAccounts,
) error {
	log.Println("executing transfer")

	param := rpc.TransferParam{}
	err := json.Unmarshal(rawParam, &param)
	if err != nil {
		return errors.New(
			"failed to umarshal transfer param, should be checked before execute",
		)
	}

	err = e.transferImpl(
		caller,
		nonce,
		param.To,
		param.Amount,
	)
	if err != nil {
		dirty.add(caller)
		dirty.add(param.To)
		return err
	}
	return nil
}

func (e *Executer) transferImpl(
	from []byte, nonce uint64,
	to []byte,
	amount uint64,
) error {
	fromCh := e.memoryHandle.GetAccountState(from)
	toCh := e.memoryHandle.GetAccountState(to)

	fromRes := <-fromCh
	if fromRes.Err != nil {
		return fromRes.Err
	}
	fromState := fromRes.Value
	if !fromState.CheckNonce(nonce) {
		return errors.New("nonce is Invalid")
	}

	toRes := <-toCh
	if toRes.Err != nil {
		return toRes.Err
	}
	toState := toRes.Value

	if !fromState.Subtract(amount) {
		return errors.New("not enough balance")
	}
	if !toState.Add(amount) {
		return errors.New("overflow")
	}

	fromCh = e.memoryHandle.PutAccountState(from, &fromState)
	toCh = e.memoryHandle.PutAccountState(to, &toState)
	<-fromCh
	<-toCh
	return nil
}
