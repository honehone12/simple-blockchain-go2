package consensus

import (
	"bytes"
	"log"
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/executer"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/txs"

	"golang.org/x/crypto/sha3"
)

type BlockProducer struct {
	memoryHandle   memory.MemoryHandle
	blockchainInfo blockchain.BlockchainInfo
	wallet         *wallets.Wallet
	executer       executer.ExecutionHandle
}

func NewBlockProducer(
	mem memory.MemoryHandle,
	bc blockchain.BlockchainInfo,
	w *wallets.Wallet,
	exe executer.ExecutionHandle,
) *BlockProducer {
	return &BlockProducer{
		memoryHandle:   mem,
		blockchainInfo: bc,
		wallet:         w,
		executer:       exe,
	}
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
		bp.blockchainInfo.NextHeight(),
		bp.wallet.PublicKey(),
		bundle,
		bp.blockchainInfo.PreviousBlockHash(),
		state,
	)
	err = blk.HashBlock()
	if err != nil {
		return nil, nil, err
	}

	blk.Info.Signature = bp.wallet.SignRaw(blk.Info.Hash)
	return blk, finCh, nil
}

func (bp *BlockProducer) Verify(
	blk *blocks.Block, errCh chan<- error,
) (bool, chan<- bool, error) {
	//check height and prevhash
	if blk.Info.Height != bp.blockchainInfo.NextHeight() {
		log.Println("block height is wrong, rejected")
		return false, nil, nil
	}
	if !bytes.Equal(
		blk.PreviousBlockHash, bp.blockchainInfo.PreviousBlockHash(),
	) {
		log.Println("block previous hash is wrong, rejected")
		return false, nil, nil
	}

	// execute transactions
	finalizeCh, err := bp.executer.Execute(
		blk.Bundle.Transactions, errCh,
	)
	if err != nil {
		// this error is not from wrong transactions
		// transaction error should be reverted and returns ok
		log.Printf("error occured on execution: %s\n", err.Error())
		return false, finalizeCh, nil
	}
	// check state
	// !!
	// here still has bug from order of states
	stateHash, err := bp.memoryHandle.GetStateHash()
	if err != nil {
		return false, finalizeCh, err
	}
	if !bytes.Equal(stateHash, blk.StateHash) {
		log.Println("executed state is wrong, rejected")
		return false, finalizeCh, nil
	}

	//check hash
	txHash, err := blk.Bundle.HashTransactions()
	if err != nil {
		return false, finalizeCh, err
	}
	prevHash := bp.blockchainInfo.PreviousBlockHash()
	add := make(
		[]byte, 0, len(txHash)+len(prevHash)+len(stateHash),
	)
	add = append(add, txHash...)
	add = append(add, prevHash...)
	add = append(add, stateHash...)
	hash := sha3.Sum256(add)
	if !bytes.Equal(blk.Info.Hash, hash[:]) {
		log.Println("hash is wrong, rejected")
		return false, finalizeCh, nil
	}
	return true, finalizeCh, nil
}
