package sync

import (
	"encoding/json"
	"errors"
	"log"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/p2p"
	"simple-blockchain-go2/rpc"
	"simple-blockchain-go2/storage"
	"simple-blockchain-go2/txs"
	"sync/atomic"
	"time"

	"golang.org/x/exp/slices"
)

type TxBroadcaster interface {
	BroadcastTx(tx *txs.Transaction, origin []p2p.NodeInfo) error
}

type SyncEventHandle interface {
	IsSyncing() bool
	StartSync(targetHeight uint64)
	NewBlock(blk *blocks.Block) chan<- bool
}

type SyncService struct {
	p2pService     *p2p.P2pService
	storageHandle  storage.StorageHandle
	memoryHandle   memory.MemoryHandle
	blockchainInfo blockchain.BlockchainInfo

	chs map[uint64]chan common.Result[*blocks.Block]
	eCh chan error

	isSyncing        atomic.Bool
	blockStoredEvent *common.BlockchainEvent[*blocks.Block]
}

func NewSyncService(
	port string,
	storage storage.StorageHandle,
	memory memory.MemoryHandle,
	bcInfo blockchain.BlockchainInfo,
	onBlockStored *common.BlockchainEvent[*blocks.Block],
) *SyncService {
	return &SyncService{
		p2pService:       p2p.NewP2pService(port, p2p.SyncNode),
		storageHandle:    storage,
		memoryHandle:     memory,
		blockchainInfo:   bcInfo,
		chs:              nil,
		eCh:              make(chan error),
		isSyncing:        atomic.Bool{},
		blockStoredEvent: onBlockStored,
	}
}

func (sys *SyncService) E() <-chan error {
	return sys.eCh
}

func (sys *SyncService) Run() {
	go sys.catch()
	sys.isSyncing.Store(false)
	sys.p2pService.Run(sys.handleSyncMessage)
	sys.p2pService.Discover(
		p2p.DiscoveryConfig{
			PortMin: 3000,
			PortMax: 3002,
		},
	)
}

func (sys *SyncService) IsSyncing() bool {
	return sys.isSyncing.Load()
}

func (sys *SyncService) BroadcastTx(tx *txs.Transaction, origin []p2p.NodeInfo) error {
	origin = append(origin, sys.p2pService.Self())
	msg := TransactionMsg{
		Origins: origin,
		Tx:      *tx,
	}
	payload, err := p2p.PackPayload(msg, byte(p2p.SyncMessage), byte(TxMsg))
	if err != nil {
		return err
	}
	sys.p2pService.Broadcast(payload, origin...)
	log.Println("broadcasting tx...")
	return nil
}

func (sys *SyncService) NewBlock(blk *blocks.Block) chan<- bool {
	ch := make(chan bool)
	go sys.waitForFinality(ch, blk)
	return ch
}

func (sys *SyncService) StartSync(targetHeight uint64) {
	// this very simple state machine can be problem
	// if iterations are fast
	if sys.isSyncing.Load() {
		return
	}
	sys.isSyncing.Swap(true)
	go sys.sync(targetHeight)
}

func (sys *SyncService) waitForFinality(finCh <-chan bool, blk *blocks.Block) {
	keys := blk.Bundle.ToKeys()
	ok := <-finCh
	if ok {
		log.Println("putting block to storage")
		resCh := sys.storageHandle.Put(storage.Blocks, blk)
		res := <-resCh
		if res.Err != nil {
			sys.eCh <- res.Err
			return
		}

		sys.memoryHandle.RemoveTxs(keys)

		err := sys.blockStoredEvent.Event(blk)
		if err != nil {
			sys.eCh <- err
		}
	}
}

func (sys *SyncService) sync(target uint64) {
	current := sys.blockchainInfo.NextHeight()
	for {
		cache := current
		sys.chs = make(map[uint64]chan common.Result[*blocks.Block])
		peerLen := sys.p2pService.LenPeers()
		for i := 0; i < peerLen; i++ {
			c := make(chan common.Result[*blocks.Block])
			sys.chs[current] = c
			go sys.blockRequest(current, i, c)
			current++
			if current > target {
				break
			}
		}

		chLen := len(sys.chs)
		for i := 0; i < chLen; i++ {
			// if peer does not send block or peer sends wrong block,
			// sync process will deadlock.
			timeout := time.After(time.Second * 5)

			log.Printf("waiting for block %d...\n", cache)
			select {
			case <-timeout:
				log.Println("timeout. returning...")
				return
			case res := <-sys.chs[cache]:
				if res.Err != nil {
					sys.eCh <- res.Err
					return
				}
				if res.Value == nil {
					log.Panic("receiving nil pointer is not expected")
					return
				}

				// put block
				// TODO:
				// execute block txs
				putCh := sys.storageHandle.Put(storage.Blocks, res.Value)
				putres := <-putCh
				if putres.Err != nil {
					sys.eCh <- putres.Err
					return
				}
				err := sys.blockStoredEvent.Event(res.Value)
				if err != nil {
					sys.eCh <- err
					return
				}
				log.Printf("block sync %d done\n", cache)
				cache++
			}
		}

		if current > target {
			sys.chs = nil
			break
		}
	}
	sys.isSyncing.Swap(false)
	log.Println("all block sync done")
}

func (sys *SyncService) blockRequest(
	height uint64,
	index int,
	c chan<- common.Result[*blocks.Block],
) {
	msg := BlockRequestMsg{
		From:   sys.p2pService.Self(),
		Height: height,
	}
	payload, err := p2p.PackPayload(
		msg, byte(p2p.SyncMessage), byte(BlockRequestMessage),
	)
	if err != nil {
		c <- common.Result[*blocks.Block]{
			Err:   err,
			Value: nil,
		}
		return
	}

	p := sys.p2pService.PeerByIndex(index)
	if p == nil {
		c <- common.Result[*blocks.Block]{
			Err:   errors.New("index out of peers"),
			Value: nil,
		}
		return
	}
	sys.p2pService.Send(*p, payload)
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

	switch SyncMsgKind(raw[1]) {
	case TxMsg:
		return sys.handleTx(raw[2:])
	case BlockRequestMessage:
		return sys.handleBlockReq(raw[2:])
	case BlockResponseMessage:
		return sys.handleBlockRes(raw[2:])
	default:
		return nil
	}
}

func (sys *SyncService) handleBlockRes(raw []byte) error {
	msg := BlockResponseMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}

	if msg.Block.Info.Height != 0 {
		if !msg.Verify() {
			return nil
		}
	} else {
		// genesis block will not pass verification
		if msg.Block.Info.Hash == nil || len(msg.Block.Info.Hash) != 32 ||
			msg.Block.PreviousBlockHash == nil || len(msg.Block.PreviousBlockHash) != 32 ||
			msg.Block.StateHash == nil || len(msg.Block.StateHash) != 32 {

			return nil
		}
	}

	ch, ok := sys.chs[msg.Block.Info.Height]
	if !ok {
		log.Printf("could not find channel %d\n", msg.Block.Info.Height)
		return nil
	}

	ch <- common.Result[*blocks.Block]{
		Value: &msg.Block,
		Err:   nil,
	}
	return nil
}

func (sys *SyncService) handleBlockReq(raw []byte) error {
	reqmsg := BlockRequestMsg{}
	err := json.Unmarshal(raw, &reqmsg)
	if err != nil {
		return err
	}
	if !reqmsg.Verify() {
		return nil
	}
	if reqmsg.Height > sys.blockchainInfo.NextHeight()-1 {
		log.Printf("received higher block request: %d\n", reqmsg.Height)
		return nil
	}

	ref, err := common.ToHex(reqmsg.Height)
	if err != nil {
		return nil
	}
	resCh := sys.storageHandle.Get(storage.Blocks, storage.Index, ref)
	res := <-resCh
	if res.Err != nil {
		return res.Err
	}
	if res.Value == nil {
		log.Printf("could not find block index: %d\n", reqmsg.Height)
		return nil
	}

	resmsg := BlockResponseMsg{}
	err = json.Unmarshal(res.Value, &resmsg.Block)
	if err != nil {
		return err
	}
	resmsg.From = sys.p2pService.Self()
	payload, err := p2p.PackPayload(
		resmsg, byte(p2p.SyncMessage), byte(BlockResponseMessage),
	)
	if err != nil {
		return err
	}

	sys.p2pService.Send(reqmsg.From, payload)
	return nil
}

func (sys *SyncService) handleTx(raw []byte) error {
	msg := TransactionMsg{}
	err := json.Unmarshal(raw, &msg)
	if err != nil {
		return err
	}

	ok, err := msg.Verify()
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	call := rpc.Call{}
	err = json.Unmarshal(msg.Tx.InnerData.Data, &call)
	if err != nil {
		return nil
	}
	if !call.Verify() {
		return nil
	}

	if slices.ContainsFunc(msg.Origins, func(n p2p.NodeInfo) bool {
		return n.IsSameIp(sys.p2pService.Self())
	}) {
		return nil
	}
	if sys.memoryHandle.ContainsTx(msg.Tx.Hash[:]) {
		return nil
	}

	sys.memoryHandle.AppendTx(&msg.Tx)
	origins := msg.Origins
	origins = append(origins, sys.p2pService.Self())
	return sys.BroadcastTx(&msg.Tx, msg.Origins)
}
