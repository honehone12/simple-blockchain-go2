package consensus

import (
	"bytes"
	"log"
	"math/big"
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/executer"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/nodes/sync"
	"simple-blockchain-go2/p2p"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/sha3"
	"golang.org/x/exp/slices"
)

const (
	SlotIdle time.Duration = time.Millisecond * 500
)

type ConsensusService struct {
	p2pService     *p2p.P2pService
	memHandle      memory.MemoryHandle
	syncHandle     sync.SyncEventHandle
	blockchainInfo blockchain.BlockchainInfo
	producer       BlockProducer
	nodeWallet     *wallets.Wallet
	eCh            chan error

	// !!
	// actually these are not thread safe...
	nextValidators    [][]byte
	nextBlockProducer []byte
	nextAggregator    []byte
	finalizeCh        chan<- bool
	blockCh           chan<- bool
	numVoted          atomic.Int64
	numVotedOk        atomic.Int64

	peersStatus map[string]bool
	badPeers    [][]byte
}

func NewConsensusService(
	port string,
	mem memory.MemoryHandle,
	sync sync.SyncEventHandle,
	bcInfo blockchain.BlockchainInfo,
	wallet *wallets.Wallet,
	exe executer.ExecutionHandle,
) *ConsensusService {
	return &ConsensusService{
		p2pService:        p2p.NewP2pService(port, p2p.ConsensusNode),
		memHandle:         mem,
		syncHandle:        sync,
		blockchainInfo:    bcInfo,
		producer:          *NewBlockProducer(mem, bcInfo, wallet, exe),
		nodeWallet:        wallet,
		eCh:               make(chan error),
		nextValidators:    [][]byte{},
		nextBlockProducer: nil,
		nextAggregator:    nil,
		finalizeCh:        nil,
		blockCh:           nil,
		numVoted:          atomic.Int64{},
		numVotedOk:        atomic.Int64{},
		peersStatus:       make(map[string]bool),
		badPeers:          [][]byte{},
	}
}

func (cs *ConsensusService) E() <-chan error {
	return cs.eCh
}

func (cs *ConsensusService) Run() {
	go cs.catch()
	cs.p2pService.Run(cs.handleConsensusMessage)
	cs.p2pService.Discover(
		p2p.DiscoveryConfig{
			PortMin: 5000,
			PortMax: 5002,
		},
	)

	cs.numVoted.Store(0)
	cs.numVotedOk.Store(0)
	t := time.NewTicker(SlotIdle)
	go cs.reminder(t)
}

func (cs *ConsensusService) reset() {
	cs.nextValidators = [][]byte{}
	cs.nextBlockProducer = nil
	cs.nextAggregator = nil
	cs.finalizeCh = nil
	cs.blockCh = nil
	cs.numVoted.Store(0)
	cs.numVotedOk.Store(0)
}

func (cs *ConsensusService) catch() {
	p2pECh := cs.p2pService.E()
	err := <-p2pECh
	cs.eCh <- err
}

func (cs *ConsensusService) removeFromNextValidators(pk []byte) {
	if len(pk) != common.PublicKeySize {
		return
	}

	idx := slices.IndexFunc(cs.nextValidators, func(p []byte) bool {
		return bytes.Equal(pk, p)
	})
	if idx >= 0 {
		cs.nextValidators = slices.Delete(cs.nextValidators, idx, idx+1)
	}
}

func (cs *ConsensusService) addToNextValidators(pk []byte) {
	if len(pk) != common.PublicKeySize {
		return
	}

	if !slices.ContainsFunc(cs.nextValidators, func(p []byte) bool {
		return bytes.Equal(pk, p)
	}) {
		cs.nextValidators = append(cs.nextValidators, pk)
	}
}

func (cs *ConsensusService) addToBadPeer(pk []byte) {
	if len(pk) != common.PublicKeySize {
		return
	}

	if !slices.ContainsFunc(cs.badPeers, func(p []byte) bool {
		return bytes.Equal(pk, p)
	}) {
		log.Printf("%x is added to bad peers\n", pk)
		cs.badPeers = append(cs.badPeers, pk)
	}
}

func (cs *ConsensusService) ready() bool {
	return (cs.nextBlockProducer != nil && cs.nextAggregator != nil) &&
		cs.memHandle.LenTx() > 0 &&
		cs.finalizeCh == nil
}

func (cs *ConsensusService) allPeerReady() bool {
	for _, ready := range cs.peersStatus {
		if !ready {
			return false
		}
	}
	return true
}

func (cs *ConsensusService) reminder(ticker *time.Ticker) {
	defer ticker.Stop()
	var tick int32 = 0

	for range ticker.C {
		tick++

		if cs.p2pService.LenPeers() == 0 {
			tick %= 3
			continue
		}

		if !cs.syncHandle.IsSyncing() && tick == 3 {
			// add self
			cs.addToNextValidators(cs.nodeWallet.PublicKey())
			if len(cs.nextValidators) < 2 {
				// need at least 2 validators
				cs.reset()
				tick %= 3
				continue
			}

			// expected to every node has same result
			cs.decideCommittee()

			// need to wait all peers are ready
			if cs.ready() && cs.allPeerReady() {
				err := cs.broadcastStartBlockProcessing()
				if err != nil {
					cs.eCh <- err
					return
				}

				// if the node is producer
				if bytes.Equal(cs.nextBlockProducer, cs.nodeWallet.PublicKey()) {
					cs.finalizeCh, err = cs.proposeBlock()
					if err != nil {
						cs.eCh <- err
					}
				}
			}
			tick %= 3
			continue
		}

		err := cs.broadcastGossip()
		if err != nil {
			cs.eCh <- err
			return
		}

		tick %= 3
	}
}

func (cs *ConsensusService) decideCommittee() {
	// sort nodes with public key
	// use hash for adding little bit fairness
	slices.SortFunc(cs.nextValidators, func(a, b []byte) bool {
		aInt := big.NewInt(0)
		ah := sha3.Sum256(a)
		aInt.SetBytes(ah[:])
		bInt := big.NewInt(0)
		bh := sha3.Sum256(b)
		bInt.SetBytes(bh[:])
		return aInt.Cmp(bInt) == -1
	})

	// each role is just sequential index
	numValidators := uint64(len(cs.nextValidators))
	nextHeight := cs.blockchainInfo.NextHeight()
	nextBlockProducerIdx := nextHeight % numValidators
	nextAggregatorIdx := (nextBlockProducerIdx + 1) % numValidators
	cs.nextBlockProducer = cs.nextValidators[nextBlockProducerIdx]
	cs.nextAggregator = cs.nextValidators[nextAggregatorIdx]
}

func (cs *ConsensusService) aggregate(ok bool) {
	if !bytes.Equal(cs.nextAggregator, cs.nodeWallet.PublicKey()) {
		return
	}

	cs.numVoted.Add(1)
	if ok {
		cs.numVotedOk.Add(1)
	}

	numValidators := len(cs.nextValidators)
	if numValidators%2 != 0 {
		numValidators++
	}
	log.Printf(
		"current vote status, yes: %d total: %d\n",
		cs.numVotedOk.Load(), cs.numVoted.Load(),
	)

	if cs.numVoted.Load() == int64(numValidators)-1 {
		if cs.numVotedOk.Load() > int64((numValidators/2)-1) {
			cs.broadcastFinality(true)
		} else {
			cs.broadcastFinality(false)
		}
	}
}

func (cs *ConsensusService) Finalize(ok bool) {
	if cs.finalizeCh != nil {
		cs.finalizeCh <- ok
	}
	if cs.blockCh != nil {
		cs.blockCh <- ok
	}
	cs.reset()
}

func (cs *ConsensusService) broadcastFinality(ok bool) error {
	if !bytes.Equal(cs.nextAggregator, cs.nodeWallet.PublicKey()) {
		return nil
	}

	vote := FinalizeMsg{
		From:          cs.p2pService.Self(),
		FromPublicKey: cs.nodeWallet.PublicKey(),
		Height:        cs.blockchainInfo.NextHeight(),
		Ok:            ok,
	}
	payload, err := p2p.PackPayload(
		vote, byte(p2p.ConsensusMessage), byte(FinalizeMessage),
	)
	if err != nil {
		return err
	}

	cs.p2pService.Broadcast(payload)
	log.Println("broadcasting finality...")
	// for self
	cs.Finalize(ok)
	return nil
}

func (cs *ConsensusService) vote(ok bool) error {
	if bytes.Equal(cs.nextAggregator, cs.nodeWallet.PublicKey()) {
		cs.aggregate(ok)
	} else {
		vote := VoteMsg{
			From:          cs.p2pService.Self(),
			FromPublicKey: cs.nodeWallet.PublicKey(),
			Height:        cs.blockchainInfo.NextHeight(),
			Ok:            ok,
		}
		payload, err := p2p.PackPayload(
			vote, byte(p2p.ConsensusMessage), byte(VoteMessage),
		)
		if err != nil {
			return err
		}

		cs.p2pService.Broadcast(payload)
		log.Println("broadcasting vote...")
		if !ok {
			cs.addToBadPeer(cs.nextBlockProducer)
		}
	}
	return nil
}

func (cs *ConsensusService) proposeBlock() (chan<- bool, error) {
	blk, finCh, err := cs.producer.NewBlock(cs.eCh)
	if err != nil {
		return nil, err
	}

	msg := ProposeBlockMsg{
		From:          cs.p2pService.Self(),
		FromPublicKey: cs.nodeWallet.PublicKey(),
		Block:         *blk,
	}
	payload, err := p2p.PackPayload(
		msg, byte(p2p.ConsensusMessage), byte(ProposeBlockMessage),
	)
	if err != nil {
		return nil, err
	}
	log.Println("proposing block...")
	cs.p2pService.Broadcast(payload)
	cs.blockCh = cs.syncHandle.NewBlock(blk)
	return finCh, nil
}

func (cs *ConsensusService) broadcastStartBlockProcessing() error {
	msg := StartBlockProcessingMsg{
		From:              cs.p2pService.Self(),
		FromPublicKey:     cs.nodeWallet.PublicKey(),
		NextBlockProducer: cs.nextBlockProducer,
		NextAggregator:    cs.nextAggregator,
	}
	payload, err := p2p.PackPayload(
		msg, byte(p2p.ConsensusMessage), byte(StartBlockProcessingMessage),
	)
	if err != nil {
		return err
	}
	cs.p2pService.Broadcast(payload)
	return nil
}

func (cs *ConsensusService) broadcastGossip() error {
	msg := GossipMsg{
		From:          cs.p2pService.Self(),
		FromPublicKey: cs.nodeWallet.PublicKey(),
		IsSyncing:     cs.syncHandle.IsSyncing(),
		IsReady:       cs.ready(),
		NextHeight:    cs.blockchainInfo.NextHeight(),
		NextEpoch:     cs.blockchainInfo.NextEpoch(),
		NextSlot:      cs.blockchainInfo.NextSlot(),
		BadNodes:      cs.badPeers,
	}
	payload, err := p2p.PackPayload(
		msg, byte(p2p.ConsensusMessage), byte(GossipMessage),
	)
	if err != nil {
		return err
	}

	cs.p2pService.Broadcast(payload)
	return nil
}

func (cs *ConsensusService) handleConsensusMessage(raw []byte) error {
	if raw[0] != byte(p2p.ConsensusMessage) {
		log.Println("skipping a message...")
		return nil
	}

	switch ConsensuMsgKind(raw[1]) {
	case GossipMessage:
		return cs.handleGossip(raw[2:])
	case StartBlockProcessingMessage:
		return cs.handleStartBlockProcessing(raw[2:])
	case ProposeBlockMessage:
		return cs.handleProposeBlock(raw[2:])
	case VoteMessage:
		return cs.handleVote(raw[2:])
	case FinalizeMessage:
		return cs.handleFinalize(raw[2:])
	default:
		return nil
	}
}
