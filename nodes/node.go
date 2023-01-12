package nodes

import (
	"log"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/nodes/rpc"
	"simple-blockchain-go2/nodes/sync"
	"simple-blockchain-go2/storage"
)

type Node struct {
	blockchain    *blockchain.Blockchain
	syncService   *sync.SyncService
	rpcServer     *rpc.RpcServer
	storageSevice *storage.StorageService
	memoryHandle  memory.MemoryHandle
	eCh           chan error
}

func NewNode(
	name,
	syncPort,
	rpcPort string,
	memory memory.MemoryHandle,
	blockchain *blockchain.Blockchain,
) (*Node, error) {
	storage, err := storage.NewStorageService(name)
	if err != nil {
		return nil, err
	}
	onBlockStored := common.BlockchainEvent{}
	sync := sync.NewSyncService(
		syncPort,
		storage,
		memory,
		blockchain,
		&onBlockStored,
	)
	rpc := rpc.NewRpcServer(rpcPort, memory, storage, sync)

	n := Node{
		blockchain:    blockchain,
		syncService:   sync,
		rpcServer:     rpc,
		memoryHandle:  memory,
		storageSevice: storage,
		eCh:           make(chan error),
	}
	onBlockStored.Event = n.OnBlockStored
	return &n, nil
}

func (n *Node) E() <-chan error {
	return n.eCh
}

func (n *Node) SyncHandle() sync.SyncEventHandle {
	return n.syncService
}

func (n *Node) Run() {
	n.storageSevice.Run()
	n.syncService.Run()
	n.rpcServer.Listen()
	go n.catch()

	resCh := n.storageSevice.Get(storage.Blocks, storage.LatestIndex, nil)
	res := <-resCh
	if res.Err != nil {
		n.eCh <- res.Err
		return
	}
	if res.Value != nil {
		lastHeight, err := common.FromHex[uint64](res.Value)
		if err != nil {
			n.eCh <- res.Err
			return
		}
		n.blockchain.Init(lastHeight + 1)
	}

	err := n.storageSevice.FetchAllAccounts(n.memoryHandle)
	if err != nil {
		n.eCh <- err
	}

	log.Printf(
		"node is running from\n height: %d\n epoch: %d\n slot: %d\n",
		n.blockchain.NextHeight(),
		n.blockchain.NextEpoch(),
		n.blockchain.NextSlot(),
	)
}

func (n *Node) OnBlockStored() {
	n.blockchain.Increment()
}

func (n *Node) catch() {
	var err error
	sysEch := n.syncService.E()
	rpcEch := n.rpcServer.E()
	select {
	case err = <-sysEch:
	case err = <-rpcEch:
	}
	n.eCh <- err
}
