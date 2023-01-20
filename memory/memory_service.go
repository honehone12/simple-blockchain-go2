package memory

import (
	"errors"
	"simple-blockchain-go2/accounts"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/common/merkle"
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
	PublicKey []byte
	State     *accounts.AccountState
	ResultCh  chan<- *common.Result[*accounts.AccountState]
}

type MemoryHandle interface {
	LenTx() int
	AppendTx(tx *txs.Transaction)
	GetTxsForBlock() []txs.Transaction
	RemoveTxs(keys []string)
	ContainsTx(key []byte) bool

	PutAccountState(
		key []byte, state *accounts.AccountState,
	) <-chan *common.Result[*accounts.AccountState]
	GetAccountState(key []byte) <-chan *common.Result[*accounts.AccountState]
	GetStateHash() ([]byte, error)
}

type MemoryService struct {
	mempool  *Mempool
	stateMem *MemoryDb[*accounts.AccountState]

	txCh    chan MempoolRequest
	stateCh chan StateMemRequest
}

func NewMemoryService() *MemoryService {
	return &MemoryService{
		mempool: NewMempool(),
		stateMem: NewMemoryDb(func() *accounts.AccountState {
			return &accounts.AccountState{
				Nonce:   0,
				Balance: 0,
			}
		}),
		txCh:    make(chan MempoolRequest, 10),
		stateCh: make(chan StateMemRequest, 10),
	}
}

func (ms *MemoryService) Run() {
	go ms.run()
}

func (ms *MemoryService) LenTx() int {
	return ms.mempool.Len()
}

func (ms *MemoryService) ContainsTx(key []byte) bool {
	return ms.mempool.Contains(base58.Encode(key))
}

func (ms *MemoryService) AppendTx(tx *txs.Transaction) {
	req := MempoolRequest{
		RequestKind: Append,
		Tx:          tx,
		Keys:        nil,
	}
	ms.txCh <- req
}

func (ms *MemoryService) GetTxsForBlock() []txs.Transaction {
	return ms.mempool.GetTxsForBlock()
}

func (ms *MemoryService) RemoveTxs(keys []string) {
	req := MempoolRequest{
		RequestKind: Remove,
		Tx:          nil,
		Keys:        keys,
	}
	ms.txCh <- req
}

func (ms *MemoryService) GetStateHash() ([]byte, error) {
	nodes, err := ms.stateMem.GetMerkleNodes()
	if err != nil {
		return nil, err
	}
	merkleRoot := merkle.NewMerkleTreeFromNodes(nodes)
	return merkleRoot.RootNode.Data, nil
}

func (ms *MemoryService) GetAccountState(
	key []byte,
) <-chan *common.Result[*accounts.AccountState] {
	resCh := make(chan *common.Result[*accounts.AccountState])
	req := StateMemRequest{
		RequestKind: Get,
		PublicKey:   key,
		State:       nil,
		ResultCh:    resCh,
	}
	ms.stateCh <- req
	return resCh
}

func (ms *MemoryService) PutAccountState(
	key []byte, state *accounts.AccountState,
) <-chan *common.Result[*accounts.AccountState] {
	resCh := make(chan *common.Result[*accounts.AccountState])
	req := StateMemRequest{
		RequestKind: Put,
		PublicKey:   key,
		State:       state,
		ResultCh:    resCh,
	}
	ms.stateCh <- req
	return resCh
}

func (ms *MemoryService) run() {
	for {
		select {
		case txReq := <-ms.txCh:
			ms.handleTx(txReq)
		case stateReq := <-ms.stateCh:
			ms.handleState(stateReq)
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
		if req.PublicKey != nil && req.ResultCh != nil {
			state, ok := ms.stateMem.Get(base58.Encode(req.PublicKey))
			if ok {
				req.ResultCh <- &common.Result[*accounts.AccountState]{
					Value: state,
					Err:   nil,
				}
				return
			} else {
				req.ResultCh <- &common.Result[*accounts.AccountState]{
					Value: &accounts.AccountState{
						Nonce:   0,
						Balance: 0,
					},
					Err: errors.New("not found the key"),
				}
				return
			}
		}
	case Put:
		if req.PublicKey != nil && req.State != nil {
			ms.stateMem.Put(
				base58.Encode(req.PublicKey), req.State,
			)
		}
	default:
	}
	req.ResultCh <- nil
}
