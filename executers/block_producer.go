package executers

import (
	"bytes"
	"log"
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/txs"

	"golang.org/x/crypto/sha3"
)

type BlockVerifier struct {
	memoryHandle   memory.MemoryHandle
	blockchainInfo blockchain.BlockchainInfo
	ExecutionHandle
}

type BlockProducer struct {
	BlockVerifier
	wallet *wallets.Wallet
}

func NewBlockVerifier(
	mem memory.MemoryHandle,
	bc blockchain.BlockchainInfo,
	exe ExecutionHandle,
) *BlockVerifier {
	return &BlockVerifier{
		memoryHandle:    mem,
		blockchainInfo:  bc,
		ExecutionHandle: exe,
	}
}

func NewBlockProducer(
	mem memory.MemoryHandle,
	bc blockchain.BlockchainInfo,
	w *wallets.Wallet,
	exe ExecutionHandle,
) *BlockProducer {
	return &BlockProducer{
		BlockVerifier: *NewBlockVerifier(mem, bc, exe),
		wallet:        w,
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
	finCh, err := bp.Execute(txsInBlock, errCh)
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
	if !bp.CheckBlockInfo(blk) {
		return false, nil, nil
	}

	// execute transactions
	finalizeCh, err := bp.Execute(
		blk.Bundle.Transactions, errCh,
	)
	if err != nil {
		// this error is not from wrong transactions
		// transaction error should be reverted and returns ok
		log.Printf("error occured on execution: %s\n", err.Error())
		return false, finalizeCh, nil
	}
	ok, state, err := bp.CheckStateHash(blk)
	if err != nil {
		return false, finalizeCh, err
	}
	if !ok {
		return false, finalizeCh, nil
	}

	ok, err = bp.CheckBlockHash(blk, state)
	if err != nil {
		return false, finalizeCh, err
	}
	if !ok {
		return false, finalizeCh, nil
	}
	return true, finalizeCh, nil
}

// check height and prevhash
func (bv *BlockVerifier) CheckBlockInfo(blk *blocks.Block) bool {
	if blk.Info.Height != bv.blockchainInfo.NextHeight() {
		log.Println("block height is wrong, rejected")
		return false
	}
	if !bytes.Equal(
		blk.PreviousBlockHash, bv.blockchainInfo.PreviousBlockHash(),
	) {
		log.Println("block previous hash is wrong, rejected")
		return false
	}
	return true
}

// check state
func (bv *BlockVerifier) CheckStateHash(
	blk *blocks.Block,
) (bool, []byte, error) {
	stateHash, err := bv.memoryHandle.GetStateHash()
	if err != nil {
		return false, nil, err
	}
	if !bytes.Equal(stateHash, blk.StateHash) {
		log.Println("executed state is wrong, rejected")
		return false, stateHash, nil
	}
	return true, stateHash, nil
}

// check hash
func (bv *BlockVerifier) CheckBlockHash(
	blk *blocks.Block, stateHash []byte,
) (bool, error) {
	txHash, err := blk.Bundle.HashTransactions()
	if err != nil {
		return false, err
	}
	prevHash := bv.blockchainInfo.PreviousBlockHash()
	add := make(
		[]byte, 0, len(txHash)+len(prevHash)+len(stateHash),
	)
	add = append(add, txHash...)
	add = append(add, prevHash...)
	add = append(add, stateHash...)
	hash := sha3.Sum256(add)
	if !bytes.Equal(blk.Info.Hash, hash[:]) {
		log.Println("hash is wrong, rejected")
		return false, nil
	}
	return true, nil
}
