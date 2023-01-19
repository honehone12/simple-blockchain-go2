package consensus

import (
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/p2p"
)

// maybe should use double broadcast like sync.TxMsg
// or add peers from other, means cache all the nodes

type ConsensuMsgKind byte

const (
	GossipMessage ConsensuMsgKind = iota + 1
	StartBlockProcessingMessage
	ProposeBlockMessage
	VoteMessage
	FinalizeMessage
)

type GossipMsg struct {
	From          p2p.NodeInfo
	FromPublicKey []byte
	IsSyncing     bool
	IsReady       bool
	NextHeight    uint64
	NextEpoch     uint64
	NextSlot      uint32

	BadNodes [][]byte
}

func (gm *GossipMsg) Verify() bool {
	if gm.BadNodes != nil && len(gm.BadNodes) > 0 {
		for _, pk := range gm.BadNodes {
			if len(pk) != common.PublicKeySize {
				return false
			}
		}
	}

	return gm.From.Verify() && len(gm.FromPublicKey) == common.PublicKeySize
}

type StartBlockProcessingMsg struct {
	From              p2p.NodeInfo
	FromPublicKey     []byte
	NextBlockProducer []byte
	NextAggregator    []byte
}

func (sbpm *StartBlockProcessingMsg) Verify() bool {
	return sbpm.From.Verify() &&
		sbpm.FromPublicKey != nil &&
		len(sbpm.FromPublicKey) == common.PublicKeySize &&
		sbpm.NextBlockProducer != nil &&
		len(sbpm.NextBlockProducer) == common.PublicKeySize &&
		sbpm.NextAggregator != nil &&
		len(sbpm.NextAggregator) == common.PublicKeySize
}

type ProposeBlockMsg struct {
	From          p2p.NodeInfo
	FromPublicKey []byte

	Block blocks.Block
}

func (pbm *ProposeBlockMsg) Verify() bool {
	return pbm.From.Verify() &&
		pbm.FromPublicKey != nil && len(pbm.FromPublicKey) == common.PublicKeySize &&
		pbm.Block.Verify()
}

type VoteMsg struct {
	From          p2p.NodeInfo
	FromPublicKey []byte
	Height        uint64
	Ok            bool
}

func (vm *VoteMsg) Verify() bool {
	return vm.From.Verify() &&
		vm.FromPublicKey != nil && len(vm.FromPublicKey) == common.PublicKeySize
}

type FinalizeMsg struct {
	From          p2p.NodeInfo
	FromPublicKey []byte
	Height        uint64
	Ok            bool
}

func (fm *FinalizeMsg) Verify() bool {
	return fm.From.Verify() &&
		fm.FromPublicKey != nil && len(fm.FromPublicKey) == common.PublicKeySize
}
