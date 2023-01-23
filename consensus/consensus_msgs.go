package consensus

import (
	"crypto/ed25519"
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

const (
	// too lazy??
	// but only thing to do here is just to confirm keypair is real
	consensusMsgSigContent = "ConsensusMsgSignatureVersion2"
)

type GossipMsg struct {
	From          p2p.NodeInfo
	FromPublicKey []byte
	Signature     []byte
	IsSyncing     bool
	IsReady       bool
	NextHeight    uint64
	NextEpoch     uint64
	NextSlot      uint32

	// TODO:
	// known nodes
	// genesis hash
	// previous hash
	// mempool size
	BadNodes [][]byte
}

func ConsensusMsgSigContent() []byte {
	return []byte(consensusMsgSigContent)
}

func VerifyConsensusSignature(signature, pubKey []byte) bool {
	return ed25519.Verify(pubKey, ConsensusMsgSigContent(), signature)
}

func (gm *GossipMsg) Verify() bool {
	if gm.BadNodes != nil && len(gm.BadNodes) > 0 {
		for _, pk := range gm.BadNodes {
			if len(pk) != common.PublicKeySize {
				return false
			}
		}
	}

	return gm.From.Verify() &&
		VerifyConsensusSignature(gm.Signature, gm.FromPublicKey) &&
		len(gm.FromPublicKey) == common.PublicKeySize &&
		gm.Signature != nil && len(gm.Signature) > 0
}

type StartBlockProcessingMsg struct {
	From              p2p.NodeInfo
	FromPublicKey     []byte
	Signature         []byte
	NextBlockProducer []byte
	NextAggregator    []byte
}

func (sbpm *StartBlockProcessingMsg) Verify() bool {
	return sbpm.From.Verify() &&
		VerifyConsensusSignature(sbpm.Signature, sbpm.FromPublicKey) &&
		sbpm.FromPublicKey != nil &&
		len(sbpm.FromPublicKey) == common.PublicKeySize &&
		sbpm.Signature != nil && len(sbpm.Signature) > 0 &&
		sbpm.NextBlockProducer != nil &&
		len(sbpm.NextBlockProducer) == common.PublicKeySize &&
		sbpm.NextAggregator != nil &&
		len(sbpm.NextAggregator) == common.PublicKeySize
}

type ProposeBlockMsg struct {
	From          p2p.NodeInfo
	FromPublicKey []byte
	Signature     []byte

	Block blocks.Block
}

func (pbm *ProposeBlockMsg) Verify() bool {
	return pbm.From.Verify() &&
		VerifyConsensusSignature(pbm.Signature, pbm.FromPublicKey) &&
		pbm.FromPublicKey != nil && len(pbm.FromPublicKey) == common.PublicKeySize &&
		pbm.Signature != nil && len(pbm.Signature) > 0 &&
		pbm.Block.Verify()
}

type VoteMsg struct {
	From          p2p.NodeInfo
	FromPublicKey []byte
	Signature     []byte
	Height        uint64
	Ok            bool
}

func (vm *VoteMsg) Verify() bool {
	return vm.From.Verify() &&
		VerifyConsensusSignature(vm.Signature, vm.FromPublicKey) &&
		vm.FromPublicKey != nil && len(vm.FromPublicKey) == common.PublicKeySize &&
		vm.Signature != nil && len(vm.Signature) > 0
}

type FinalizeMsg struct {
	From          p2p.NodeInfo
	FromPublicKey []byte
	Signature     []byte
	Height        uint64
	Ok            bool
}

func (fm *FinalizeMsg) Verify() bool {
	return fm.From.Verify() &&
		VerifyConsensusSignature(fm.Signature, fm.FromPublicKey) &&
		fm.FromPublicKey != nil && len(fm.FromPublicKey) == common.PublicKeySize &&
		fm.Signature != nil && len(fm.Signature) > 0
}
