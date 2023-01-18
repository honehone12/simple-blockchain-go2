package fullnode

import (
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/consensus"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/nodes"
)

type FullNode struct {
	node      *nodes.Node
	memory    *memory.MemoryService
	consensus *consensus.ConsensusService
	eCh       chan error
}

func NewFullNode(
	name,
	syncPort,
	consensusPort,
	rpcPort string,
) (*FullNode, error) {
	ms := memory.NewMemoryService()
	n, err := nodes.NewNode(name, syncPort, rpcPort, ms)
	if err != nil {
		return nil, err
	}
	w, err := wallets.NewWallet(name)
	if err != nil {
		return nil, err
	}
	cs := consensus.NewConsensusService(
		consensusPort,
		ms,
		n.SyncHandle(),
		n.Blockchain(),
		w,
		n.ExecuteHandle(),
	)

	return &FullNode{
		node:      n,
		memory:    ms,
		consensus: cs,
		eCh:       make(chan error),
	}, nil
}

func (fn *FullNode) Run() error {
	go fn.catch()
	fn.memory.Run()
	fn.consensus.Run()
	fn.node.Run()

	err := <-fn.eCh
	return err
}

func (fn *FullNode) catch() {
	var err error
	nodeECh := fn.node.E()
	consECh := fn.consensus.E()
	select {
	case err = <-nodeECh:
	case err = <-consECh:
	}
	fn.eCh <- err
}
