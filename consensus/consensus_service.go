package consensus

import (
	"bytes"
	"encoding/json"
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
	numVotedOk        atomic.Uint64

	peersStatus map[string]bool
	badPeers    [][]byte
}

func NewConsensusService(
	port string,
	mem memory.MemoryHandle,
	sync sync.SyncEventHandle,
	bcInfo blockchain.BlockchainInfo,
	wallet *wallets.Wallet,
	exe executer.ExecuteHandle,
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
		numVotedOk:        atomic.Uint64{},
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

	cs.numVotedOk.Store(0)
	t := time.NewTicker(SlotIdle)
	go cs.reminder(t)
}

func (cs *ConsensusService) reset() {
	cs.nextValidators = [][]byte{}
	cs.nextBlockProducer = nil
	cs.nextAggregator = nil
	cs.finalizeCh = nil
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
		cs.badPeers = append(cs.badPeers, pk)
	}
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
			if cs.allPeerReady() {
				err := cs.broadcastStartBlockProcessing()
				if err != nil {
					cs.eCh <- err
					return
				}

				// if the node is producer
				if bytes.Equal(cs.nextBlockProducer, cs.nodeWallet.PublicKey()) {
					cs.finalizeCh, err = cs.produceBlock()
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
	// sort nodes with ip4 address
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

func (cs *ConsensusService) produceBlock() (chan<- bool, error) {
	blk, finCh, err := cs.producer.NewBlock(cs.eCh)
	if err != nil {
		return nil, err
	}
	self := cs.p2pService.Self()
	msg := ProposeBlockMsg{
		From:          self,
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
	cs.p2pService.Broadcast(payload, self)
	return finCh, nil
}

func (cs *ConsensusService) broadcastStartBlockProcessing() error {
	self := cs.p2pService.Self()
	msg := StartBlockProcessingMsg{
		From:              self,
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
	cs.p2pService.Broadcast(payload, self)
	return nil
}

func (cs *ConsensusService) broadcastGossip() error {
	self := cs.p2pService.Self()
	ready := (cs.nextBlockProducer != nil && cs.nextAggregator != nil) &&
		cs.memHandle.LenTx() > 0
	msg := GossipMsg{
		From:          self,
		FromPublicKey: cs.nodeWallet.PublicKey(),
		IsSyncing:     cs.syncHandle.IsSyncing(),
		IsReady:       ready,
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

	cs.p2pService.Broadcast(payload, self)
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

func (cs *ConsensusService) handleVote(raw []byte) error {
	if !bytes.Equal(cs.nextAggregator, cs.nodeWallet.PublicKey()) {
		return nil
	}

	msg := VoteMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}
	if !slices.ContainsFunc(cs.nextValidators, func(pk []byte) bool {
		return bytes.Equal(pk, msg.FromPublicKey)
	}) {
		return nil
	}

	if msg.Ok {
		cs.numVotedOk.Add(1)
	}

	return nil
}

func (cs *ConsensusService) handleFinalize(raw []byte) error {
	msg := FinalizeMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}
	if !bytes.Equal(msg.FromPublicKey, cs.nextAggregator) {
		cs.addToBadPeer(msg.FromPublicKey)
		return nil
	}

	return nil
}

func (cs *ConsensusService) handleProposeBlock(raw []byte) error {
	msg := ProposeBlockMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}
	if !bytes.Equal(msg.FromPublicKey, cs.nextBlockProducer) {
		cs.addToBadPeer(msg.FromPublicKey)
		return nil
	}

	log.Printf("received block height: %d\n", msg.Block.Info.Height)

	cs.finalizeCh, err = cs.producer.ExecuteTxs(
		msg.Block.Bundle.Transactions, cs.eCh,
	)
	ok := true
	if err != nil {
		log.Printf("error occured: %s\n", err.Error())
		ok = false
	}

	self := cs.p2pService.Self()
	vote := VoteMsg{
		From:          self,
		FromPublicKey: cs.nodeWallet.PublicKey(),
		Ok:            ok,
	}
	payload, err := p2p.PackPayload(
		vote, byte(p2p.ConsensusMessage), byte(VoteMessage),
	)
	if err != nil {
		return err
	}
	cs.p2pService.Broadcast(payload, self)
	log.Println("broadcasting vote")
	return nil
}

func (cs *ConsensusService) handleStartBlockProcessing(raw []byte) error {
	msg := StartBlockProcessingMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}

	if !(bytes.Equal(msg.NextBlockProducer, cs.nextBlockProducer) &&
		bytes.Equal(msg.NextAggregator, cs.nextAggregator)) {

		log.Printf("%x is added to bad peers\n", msg.FromPublicKey)
		cs.addToBadPeer(msg.FromPublicKey)
		return nil
	}

	log.Printf("confirmed comittee, peer: %s", msg.From.Ip4)
	return nil
}

func (cs *ConsensusService) handleGossip(raw []byte) error {
	if cs.syncHandle.IsSyncing() {
		// this node does not go further
		log.Println("syncing...")
		return nil
	}

	msg := GossipMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}

	if msg.IsSyncing {
		// peer is syncing
		return nil
	}
	if len(cs.badPeers) > 0 &&
		slices.ContainsFunc(cs.badPeers, func(pk []byte) bool {
			return bytes.Equal(msg.FromPublicKey, pk)
		}) {

		// peer is bad node
		return nil
	}

	// TODO:
	// add known peer from msg
	// add bad peer from msg

	selfNextHeight := cs.blockchainInfo.NextHeight()
	if msg.NextHeight > selfNextHeight {
		// need sync
		log.Printf("node %d is behind %d\n", selfNextHeight, msg.NextHeight)
		cs.syncHandle.StartSync(msg.NextHeight - 1)
		return nil
	} else if msg.NextHeight < selfNextHeight {
		// this is from new node
		log.Println("received lower height")
		cs.removeFromNextValidators(msg.FromPublicKey)
		return nil
	}

	if msg.NextEpoch != cs.blockchainInfo.NextEpoch() {
		// this is not expected to happen normally
		log.Println("recieved different epoch")
		cs.removeFromNextValidators(msg.FromPublicKey)
		return nil
	}
	if msg.NextSlot != cs.blockchainInfo.NextSlot() {
		// this is not expected to happen normally
		log.Println("recieved different slot")
		cs.removeFromNextValidators(msg.FromPublicKey)
		return nil
	}

	cs.peersStatus[msg.From.Ip4] = msg.IsReady
	cs.addToNextValidators(msg.FromPublicKey)
	return nil
}
