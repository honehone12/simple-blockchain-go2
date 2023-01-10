package fullnode

import (
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/consensus"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/nodes"
)

type FullNode struct {
	node       *nodes.Node
	memory     *memory.MemoryService
	consensus  *consensus.ConsensusService
	blockchain *blockchain.Blockchain
}

func NewFullNode(
	name,
	syncPort,
	consensusPort,
	rpcPort string,
) (*FullNode, error) {
	bc := blockchain.NewBlockchain()
	ms := memory.NewMemoryService()
	cs := consensus.NewConsensusService(consensusPort, ms)
	n, err := nodes.NewNode(name, syncPort, rpcPort, ms)
	if err != nil {
		return nil, err
	}

	return &FullNode{
		node:       n,
		memory:     ms,
		consensus:  cs,
		blockchain: bc,
	}, nil
}

func (fn *FullNode) Run() error {
	fn.consensus.Run()
	fn.memory.Run()
	fn.node.Run()

	var err error
	nodeECh := fn.node.E()
	consECh := fn.consensus.E()
	select {
	case err = <-nodeECh:
	case err = <-consECh:
	}
	return err
}
