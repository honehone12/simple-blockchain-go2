package consensus

import (
	"bytes"
	"log"
	"math/big"
	"simple-blockchain-go2/accounts/wallets"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/executers"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/nodes/sync"
	"simple-blockchain-go2/p2p"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/sha3"
	"golang.org/x/exp/slices"
)

const (
	// 1sec total
	SlotIdle time.Duration = time.Millisecond * 250
)

type ConsensusService struct {
	p2pService     *p2p.P2pService
	memHandle      memory.MemoryHandle
	syncHandle     sync.SyncEventHandle
	blockchainInfo blockchain.BlockchainInfo
	producer       *executers.BlockProducer
	nodeWallet     *wallets.Wallet
	eCh            chan error

	// !!
	// actually these are not thread safe...
	nextValidators    [][]byte
	nextBlockProducer []byte
	nextAggregator    []byte
	finalizeCh        chan bool
	accountCh         chan<- bool
	blockCh           chan<- bool
	numVoted          atomic.Int64
	numVotedOk        atomic.Int64
	waitingFianlity   atomic.Bool
	timer             *time.Timer

	peersStatus map[string]bool
	badPeers    [][]byte
}

func NewConsensusService(
	port string,
	mem memory.MemoryHandle,
	sync sync.SyncEventHandle,
	bcInfo blockchain.BlockchainInfo,
	wallet *wallets.Wallet,
	exe executers.ExecutionHandle,
) *ConsensusService {
	return &ConsensusService{
		p2pService:        p2p.NewP2pService(port, p2p.ConsensusNode),
		memHandle:         mem,
		syncHandle:        sync,
		blockchainInfo:    bcInfo,
		producer:          executers.NewBlockProducer(mem, bcInfo, wallet, exe),
		nodeWallet:        wallet,
		eCh:               make(chan error),
		nextValidators:    make([][]byte, 0),
		nextBlockProducer: nil,
		nextAggregator:    nil,
		finalizeCh:        nil,
		accountCh:         nil,
		blockCh:           nil,
		timer:             nil,
		waitingFianlity:   atomic.Bool{},
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
	cs.p2pService.SetOnFail(cs.onPeerDisapear)
	cs.p2pService.Run(cs.handleConsensusMessage)
	cs.p2pService.Discover(
		p2p.DiscoveryConfig{
			PortMin: 5000,
			PortMax: 5002,
		},
	)
	cs.numVoted.Store(0)
	cs.numVotedOk.Store(0)
	cs.waitingFianlity.Store(false)
	t := time.NewTicker(SlotIdle)
	go cs.loop(t)
}

func (cs *ConsensusService) onPeerDisapear(n p2p.NodeInfo) {
	delete(cs.peersStatus, n.Ip4)
	cs.reset()
}

func (cs *ConsensusService) reset() {
	cs.nextValidators = make([][]byte, 0)
	cs.nextBlockProducer = nil
	cs.nextAggregator = nil
	cs.finalizeCh = nil
	cs.accountCh = nil
	cs.blockCh = nil
	cs.numVoted.Store(0)
	cs.numVotedOk.Store(0)
	cs.waitingFianlity.Store(false)
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
	return (cs.nextBlockProducer != nil && cs.nextAggregator != nil)
}

func (cs *ConsensusService) allPeerReady() bool {
	for _, ready := range cs.peersStatus {
		if !ready {
			return false
		}
	}
	return true
}

func (cs *ConsensusService) loop(ticker *time.Ticker) {
	defer ticker.Stop()
	var tick int32 = 0

	for range ticker.C {
		if cs.p2pService.LenPeers() == 0 {
			continue
		}
		if cs.waitingFianlity.Load() {
			log.Println("waiting for fainality, skipping tick...")
			continue
		}

		if tick < 2 {
			err := cs.broadcastGossip()
			if err != nil {
				cs.eCh <- err
				return
			}
		} else if tick == 2 {
			if cs.syncHandle.IsSyncing() {
				continue
			}
			if len(cs.nextValidators) == 0 {
				continue
			}

			// add self
			cs.addToNextValidators(cs.nodeWallet.PublicKey())
			// expected to every node has same result
			cs.decideCommittee()

			// need to wait all peers are ready
			if cs.ready() && cs.allPeerReady() {
				err := cs.broadcastStartBlockProcessing()
				if err != nil {
					cs.eCh <- err
					return
				}

				if cs.memHandle.LenTx() > 0 {
					// if the node is producer
					if bytes.Equal(cs.nextBlockProducer, cs.nodeWallet.PublicKey()) {
						err = cs.proposeBlock()
						if err != nil {
							cs.eCh <- err
						}
						cs.waitingFianlity.Store(true)
						cs.startTimer()
					}
				}
			}
		}
		tick = (tick + 1) % 4
	}
}

func (cs *ConsensusService) decideCommittee() {
	// sort nodes with public key
	// use hash for adding little bit fairness

	// !!
	// sha256 is too much
	// lighter hasing is ok
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
	modTotal := numValidators
	if modTotal%2 != 0 {
		modTotal++
	}
	log.Printf(
		"current vote status, yes: %d total: %d\n",
		cs.numVotedOk.Load(), cs.numVoted.Load(),
	)

	// except block producer always vote yes
	if cs.numVoted.Load() == int64(numValidators)-1 {
		if cs.numVotedOk.Load() > int64((modTotal/2)-1) {
			cs.broadcastFinality(true)
		} else {
			cs.broadcastFinality(false)
		}
	}
}

func (cs *ConsensusService) startTimer() {
	log.Println("setting timeout")
	cs.finalizeCh = make(chan bool)
	cs.timer = time.NewTimer(time.Millisecond * common.FinalityTimeoutMilSec)
	go cs.timeout()
}

func (cs *ConsensusService) timeout() {
	select {
	case <-cs.finalizeCh:
		cs.timer.Stop()
	case <-cs.timer.C:
		log.Println("timeout, reject block...")
		cs.finalize(false)
	}
}

func (cs *ConsensusService) finalize(ok bool) {
	cs.finalizeCh <- ok
	log.Println("finalizing")
	if cs.accountCh != nil {
		cs.accountCh <- ok
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
		Signature:     cs.nodeWallet.SignRaw(ConsensusMsgSigContent()),
		Height:        cs.blockchainInfo.NextHeight(),
		Ok:            ok,
	}
	payload, err := p2p.PackPayload(
		vote, byte(p2p.ConsensusMessage), byte(FinalizeMessage),
	)
	if err != nil {
		return err
	}

	// !!
	// block producer can be rewarded now

	// !!
	// should aggregator also sign to the block??
	// (also other validators, like multisig ??)

	cs.p2pService.Broadcast(payload)
	log.Println("broadcasting finality...")
	// for self
	cs.finalize(ok)
	return nil
}

func (cs *ConsensusService) vote(ok bool) error {
	cs.startTimer()
	if bytes.Equal(cs.nextAggregator, cs.nodeWallet.PublicKey()) {
		cs.aggregate(ok)
	} else {
		vote := VoteMsg{
			From:          cs.p2pService.Self(),
			FromPublicKey: cs.nodeWallet.PublicKey(),
			Signature:     cs.nodeWallet.SignRaw(ConsensusMsgSigContent()),
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

func (cs *ConsensusService) proposeBlock() error {
	blk, ch, err := cs.producer.NewBlock(cs.eCh)
	if err != nil {
		return err
	}

	msg := ProposeBlockMsg{
		From:          cs.p2pService.Self(),
		FromPublicKey: cs.nodeWallet.PublicKey(),
		Signature:     cs.nodeWallet.SignRaw(ConsensusMsgSigContent()),
		Block:         *blk,
	}
	payload, err := p2p.PackPayload(
		msg, byte(p2p.ConsensusMessage), byte(ProposeBlockMessage),
	)
	if err != nil {
		return err
	}
	log.Println("proposing block...")
	cs.p2pService.Broadcast(payload)
	cs.blockCh = cs.syncHandle.NewBlock(blk)
	cs.accountCh = ch
	return nil
}

func (cs *ConsensusService) broadcastStartBlockProcessing() error {
	msg := StartBlockProcessingMsg{
		From:              cs.p2pService.Self(),
		FromPublicKey:     cs.nodeWallet.PublicKey(),
		Signature:         cs.nodeWallet.SignRaw(ConsensusMsgSigContent()),
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
		Signature:     cs.nodeWallet.SignRaw(ConsensusMsgSigContent()),
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
