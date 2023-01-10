package memory

import (
	"errors"
	"simple-blockchain-go2/accounts"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/txs"

	"github.com/btcsuite/btcutil/base58"
)

type RequestKind byte

const (
	Append RequestKind = iota + 1
	Remove
	Get
	Put
)

type MempoolRequest struct {
	RequestKind
	Tx   *txs.Transaction
	Keys []string
}

type StateMemRequest struct {
	RequestKind
	PublickKey []byte
	State      *accounts.AccountState
	ResultCh   chan<- common.Result[accounts.AccountState]
}

type MemoryHandle interface {
	AppendTx(tx *txs.Transaction)
	RemoveTxs(keys []string)

	PutAccountState(key []byte, state *accounts.AccountState)
	GetAccountState(key []byte) <-chan common.Result[accounts.AccountState]
}

type MemoryService struct {
	mempool  *Mempool
	stateMem *MemoryDb[accounts.AccountState]

	txCh    chan MempoolRequest
	stateCh chan StateMemRequest
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		mempool: NewMempool(),
		stateMem: NewMemoryDb(
			accounts.AccountState{
				Nonce:   0,
				Balance: 0,
			},
		),
		txCh:    make(chan MempoolRequest),
		stateCh: make(chan StateMemRequest),
	}
}

func (ms *MemoryService) Run() {
	go ms.run()
}

func (ms *MemoryService) AppendTx(tx *txs.Transaction) {
	req := MempoolRequest{
		RequestKind: Append,
		Tx:          tx,
		Keys:        nil,
	}
	ms.txCh <- req
}

func (ms *MemoryService) RemoveTxs(keys []string) {
	req := MempoolRequest{
		RequestKind: Remove,
		Tx:          nil,
		Keys:        keys,
	}
	ms.txCh <- req
}

func (ms *MemoryService) GetAccountState(
	key []byte,
) <-chan common.Result[accounts.AccountState] {
	resCh := make(chan common.Result[accounts.AccountState])
	req := StateMemRequest{
		RequestKind: Get,
		PublickKey:  key,
		State:       nil,
		ResultCh:    resCh,
	}
	ms.stateCh <- req
	return resCh
}

func (ms *MemoryService) PutAccountState(
	key []byte, state *accounts.AccountState,
) {
	req := StateMemRequest{
		RequestKind: Put,
		PublickKey:  key,
		State:       state,
		ResultCh:    nil,
	}
	ms.stateCh <- req
}

func (ms *MemoryService) run() {
	for {
		select {
		case req := <-ms.txCh:
			ms.handleTx(req)
		case req := <-ms.stateCh:
			ms.handleState(req)
		}
	}
}

func (ms *MemoryService) handleTx(req MempoolRequest) {
	switch req.RequestKind {
	case Append:
		if req.Tx != nil {
			ms.mempool.Append(req.Tx)
		}
	case Remove:
		if req.Keys != nil {
			ms.mempool.Remove(req.Keys)
		}
	default:
	}
}

func (ms *MemoryService) handleState(req StateMemRequest) {
	switch req.RequestKind {
	case Get:
		if req.PublickKey != nil && req.ResultCh != nil {
			state, ok := ms.stateMem.Get(base58.Encode(req.PublickKey))
			if ok {
				req.ResultCh <- common.Result[accounts.AccountState]{
					Value: *state,
					Err:   nil,
				}
			} else {
				req.ResultCh <- common.Result[accounts.AccountState]{
					Value: accounts.AccountState{},
					Err:   errors.New("not found the key"),
				}
			}
		}
	case Put:
		if req.PublickKey != nil && req.State != nil {
			ms.stateMem.Put(
				base58.Encode(req.PublickKey), *req.State,
			)
		}
	default:
	}
}
