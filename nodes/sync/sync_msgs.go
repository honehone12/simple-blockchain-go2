package sync

import (
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/p2p"
	"simple-blockchain-go2/txs"
)

type SyncMsgKind byte

const (
	TxMsg SyncMsgKind = iota + 1
	BlockRequestMessage
	BlockResponseMessage
)

type TransactionMsg struct {
	Origins []p2p.NodeInfo
	Tx      txs.Transaction
}

func (tm *TransactionMsg) Verify() (bool, error) {
	for _, ori := range tm.Origins {
		if !ori.Verify() {
			return false, nil
		}
	}
	return tm.Tx.Verify()
}

type BlockRequestMsg struct {
	From   p2p.NodeInfo
	Height uint64
}

func (bqm *BlockRequestMsg) Verify() bool {
	return bqm.From.Verify()
}

type BlockResponseMsg struct {
	From  p2p.NodeInfo
	Block blocks.Block
}

func (bsm *BlockResponseMsg) Verify() bool {
	return bsm.From.Verify() && bsm.Block.Verify()
}
