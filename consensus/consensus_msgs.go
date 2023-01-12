package consensus

import "simple-blockchain-go2/p2p"

type ConsensuMsgKind byte

const (
	GossipMessage ConsensuMsgKind = iota + 1
	StartBlockProcessingMessage
)

type GossipMsg struct {
	From       p2p.NodeInfo
	IsSyncing  bool
	NextHeight uint64
	NextEpoch  uint64
	NextSlot   uint32

	BadNodes []p2p.NodeInfo
}

func (gm *GossipMsg) Verify() bool {
	return gm.From.Verify()
}

type StartBlockProcessingMsg struct {
	From              p2p.NodeInfo
	NextBlockProducer p2p.NodeInfo
	NextAggregator    p2p.NodeInfo
}

func (sbpm *StartBlockProcessingMsg) Verify() bool {
	return sbpm.From.Verify() &&
		sbpm.NextBlockProducer.Verify() &&
		sbpm.NextAggregator.Verify()
}
