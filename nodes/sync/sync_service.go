package sync

import (
	"log"
	"simple-blockchain-go2/p2p"
	"simple-blockchain-go2/storage"
)

type SyncService struct {
	p2pService    *p2p.P2pService
	storageHandle storage.StorageHandle
	eCh           chan error
}

func NewSyncService(port string, storage storage.StorageHandle) *SyncService {
	return &SyncService{
		p2pService:    p2p.NewP2pService(port, p2p.SyncNode),
		storageHandle: storage,
		eCh:           make(chan error),
	}
}

func (sys *SyncService) E() <-chan error {
	return sys.eCh
}

func (sys *SyncService) Run() {
	sys.p2pService.Run(sys.handleSyncMessage)
	go sys.catch()
	sys.p2pService.Discover(
		p2p.DiscoveryConfig{
			PortMin: 3000,
			PortMax: 3002,
		},
	)
}

func (sys *SyncService) catch() {
	p2pECh := sys.p2pService.E()
	err := <-p2pECh
	sys.eCh <- err
}

func (sys *SyncService) handleSyncMessage(raw []byte) error {
	if raw[0] != byte(p2p.SyncMessage) {
		log.Println("skipping a message...")
		return nil
	}
	return nil
}
