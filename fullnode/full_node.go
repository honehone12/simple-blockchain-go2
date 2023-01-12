package fullnode

import (
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/consensus"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/nodes"
)

type FullNode struct {
	node      *nodes.Node
	memory    *memory.MemoryService
	consensus *consensus.ConsensusService
}

func NewFullNode(
	name,
	syncPort,
	consensusPort,
	rpcPort string,
) (*FullNode, error) {
	bc := blockchain.NewBlockchain()
	ms := memory.NewMemoryService()
	n, err := nodes.NewNode(name, syncPort, rpcPort, ms, bc)
	if err != nil {
		return nil, err
	}
	cs := consensus.NewConsensusService(consensusPort, ms, n.SyncHandle(), bc)

	return &FullNode{
		node:      n,
		memory:    ms,
		consensus: cs,
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
