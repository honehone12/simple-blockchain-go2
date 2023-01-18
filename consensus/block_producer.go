package consensus

import (
	"log"
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/executer"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/txs"
)

type BlockProducer struct {
	memoryHandle memory.MemoryHandle
	blockChain   blockchain.BlockchainInfo
	wallet       *wallets.Wallet
	executer     executer.ExecuteHandle
}

func NewBlockProducer(
	mem memory.MemoryHandle,
	bc blockchain.BlockchainInfo,
	w *wallets.Wallet,
	exe executer.ExecuteHandle,
) *BlockProducer {
	return &BlockProducer{
		memoryHandle: mem,
		blockChain:   bc,
		wallet:       w,
		executer:     exe,
	}
}

func (bp *BlockProducer) ExecuteTxs(
	transactions []txs.Transaction, errCh chan<- error,
) (chan<- bool, error) {
	return bp.executer.Execute(transactions, errCh)
}

func (bp *BlockProducer) NewBlock(
	errCh chan<- error,
) (*blocks.Block, chan<- bool, error) {
	log.Println("creating block...")

	txsInBlock := bp.memoryHandle.GetTxsForBlock()
	bundle := txs.TxBundle{
		Transactions: txsInBlock,
	}
	finCh, err := bp.executer.Execute(txsInBlock, errCh)
	if err != nil {
		return nil, nil, err
	}

	// new state
	// previous state is included as previoushash
	state, err := bp.memoryHandle.GetStateHash()
	if err != nil {
		return nil, nil, err
	}

	blk := blocks.NewBlock(
		bp.blockChain.NextHeight(),
		bp.wallet.PublicKey(),
		bundle,
		bp.blockChain.PreviousBlockHash(),
		state,
	)
	err = blk.HashBlock()
	if err != nil {
		return nil, nil, err
	}

	blk.Info.Signature = bp.wallet.SignRaw(blk.Info.Hash)
	return blk, finCh, nil
}
