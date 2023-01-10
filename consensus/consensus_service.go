package consensus

import (
	"log"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/p2p"
)

type ConsensusService struct {
	p2pService *p2p.P2pService
	memHandle  memory.MemoryHandle
	eCh        chan error
}

func NewConsensusService(port string, mem memory.MemoryHandle) *ConsensusService {
	return &ConsensusService{
		p2pService: p2p.NewP2pService(port, p2p.ConsensusNode),
		memHandle:  mem,
		eCh:        make(chan error),
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
}

func (cs *ConsensusService) catch() {
	p2pECh := cs.p2pService.E()
	err := <-p2pECh
	cs.eCh <- err
}

func (cs *ConsensusService) handleConsensusMessage(raw []byte) error {
	if raw[0] != byte(p2p.ConsensusMessage) {
		log.Println("skipping a message...")
		return nil
	}

	return nil
}
