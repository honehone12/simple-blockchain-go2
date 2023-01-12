package consensus

import (
	"encoding/json"
	"log"
	"math/big"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/nodes/sync"
	"simple-blockchain-go2/p2p"
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
	eCh            chan error

	nextValidators    []p2p.NodeInfo
	nextBlockProducer p2p.NodeInfo
	nextAggregator    p2p.NodeInfo

	badPeer []p2p.NodeInfo
}

func NewConsensusService(
	port string,
	mem memory.MemoryHandle,
	sync sync.SyncEventHandle,
	bcInfo blockchain.BlockchainInfo,
) *ConsensusService {
	return &ConsensusService{
		p2pService:        p2p.NewP2pService(port, p2p.ConsensusNode),
		memHandle:         mem,
		syncHandle:        sync,
		blockchainInfo:    bcInfo,
		eCh:               make(chan error),
		nextValidators:    []p2p.NodeInfo{},
		nextBlockProducer: p2p.NodeInfo{},
		nextAggregator:    p2p.NodeInfo{},
		badPeer:           []p2p.NodeInfo{},
	}
}

func (cs *ConsensusService) E() <-chan error {
	return cs.eCh
}

func (cs *ConsensusService) Run() {
	cs.p2pService.Run(cs.handleConsensusMessage)
	go cs.catch()
	cs.p2pService.Discover(
		p2p.DiscoveryConfig{
			PortMin: 5000,
			PortMax: 5002,
		},
	)

	t := time.NewTicker(SlotIdle)
	go cs.reminder(t)
}

func (cs *ConsensusService) reset() {
	cs.nextValidators = []p2p.NodeInfo{}
	cs.nextBlockProducer = p2p.NodeInfo{}
	cs.nextAggregator = p2p.NodeInfo{}
}

func (cs *ConsensusService) decideCommittee() {
	// sort nodes with ip4 address
	// use hash for add little bit fairness
	slices.SortFunc(cs.nextValidators, func(a, b p2p.NodeInfo) bool {
		aInt := big.NewInt(0)
		ah := sha3.Sum256([]byte(a.Ip4))
		aInt.SetBytes(ah[:])
		bInt := big.NewInt(0)
		bh := sha3.Sum256([]byte(b.Ip4))
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
			// TODO:
			// need to wait peer at least two gossip

			// add self
			// expected to every node has same result
			cs.addToNextValidators(cs.p2pService.Self())
			if len(cs.nextValidators) < 2 {
				// need at least 2 validators
				cs.reset()
				tick %= 3
				continue
			}

			cs.decideCommittee()
			err := cs.broadcastStartBlockProcessing()
			if err != nil {
				cs.eCh <- err
				return
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

func (cs *ConsensusService) broadcastStartBlockProcessing() error {
	self := cs.p2pService.Self()
	msg := StartBlockProcessingMsg{
		From:              self,
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
	msg := GossipMsg{
		From:       self,
		IsSyncing:  cs.syncHandle.IsSyncing(),
		NextHeight: cs.blockchainInfo.NextHeight(),
		NextEpoch:  cs.blockchainInfo.NextEpoch(),
		NextSlot:   cs.blockchainInfo.NextSlot(),
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

func (cs *ConsensusService) catch() {
	p2pECh := cs.p2pService.E()
	err := <-p2pECh
	cs.eCh <- err
}

func (cs *ConsensusService) removeFromNextValidators(n p2p.NodeInfo) {
	idx := slices.IndexFunc(cs.nextValidators, func(p p2p.NodeInfo) bool {
		return n.IsSameIp(p)
	})
	if idx >= 0 {
		cs.nextValidators = slices.Delete(cs.nextValidators, idx, idx+1)
	}
}

func (cs *ConsensusService) addToNextValidators(n p2p.NodeInfo) {
	if !slices.ContainsFunc(cs.nextValidators, func(p p2p.NodeInfo) bool {
		return n.IsSameIp(p)
	}) {
		cs.nextValidators = append(cs.nextValidators, n)
	}
}

func (cs *ConsensusService) addToBadPeer(n p2p.NodeInfo) {
	if !slices.ContainsFunc(cs.badPeer, func(p p2p.NodeInfo) bool {
		return n.IsSameIp(p)
	}) {
		cs.badPeer = append(cs.badPeer, n)
	}
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
	default:
		return nil
	}
}

func (cs *ConsensusService) handleStartBlockProcessing(raw []byte) error {
	// if cs.nextBlockProducer.Network == 0 ||
	// 	cs.nextAggregator.Network == 0 {

	// 	// this node is not ready
	// 	return nil
	// }

	msg := StartBlockProcessingMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}
	if !msg.Verify() {
		return nil
	}

	log.Printf("%#v\n", msg)

	if !msg.NextBlockProducer.IsSameIp(cs.nextBlockProducer) ||
		!msg.NextAggregator.IsSameIp(cs.nextAggregator) {

		// actually we don't know From is true but ip address is useless
		// we need to use public key like id
		log.Printf("%s is added to bad peer\n", msg.From.Ip4)
		cs.addToBadPeer(msg.From)
	}
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

	selfNextHeight := cs.blockchainInfo.NextHeight()
	if msg.NextHeight > selfNextHeight {
		// need sync
		log.Printf("node %d is behind %d\n", selfNextHeight, msg.NextHeight)
		cs.syncHandle.StartSync(msg.NextHeight - 1)
		return nil
	} else if msg.NextHeight < selfNextHeight {
		// this is not expected to happen normally
		log.Println("received lower height")
		cs.removeFromNextValidators(msg.From)
		return nil
	}

	if msg.NextEpoch != cs.blockchainInfo.NextEpoch() {
		// this is not expected to happen normally
		log.Println("recieved different epoch")
		cs.removeFromNextValidators(msg.From)
		return nil
	}
	if msg.NextSlot != cs.blockchainInfo.NextSlot() {
		// this is not expected to happen normally
		log.Println("recieved different slot")
		cs.removeFromNextValidators(msg.From)
		return nil
	}

	cs.addToNextValidators(msg.From)
	return nil
}
