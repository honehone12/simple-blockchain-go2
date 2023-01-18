package nodes

import (
	"encoding/json"
	"log"
	"simple-blockchain-go2/blockchain"
	"simple-blockchain-go2/blocks"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/executer"
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
	executer      *executer.Executer
	eCh           chan error
}

func NewNode(
	name,
	syncPort,
	rpcPort string,
	memory memory.MemoryHandle,
) (*Node, error) {
	storage, err := storage.NewStorageService(name)
	if err != nil {
		return nil, err
	}
	onBlockStored := common.BlockchainEvent[*blocks.Block]{}
	bc := blockchain.NewBlockchain()
	sync := sync.NewSyncService(
		syncPort,
		storage,
		memory,
		bc,
		&onBlockStored,
	)
	rpc := rpc.NewRpcServer(rpcPort, memory, storage, sync)
	exe := executer.NewExecuter(storage, memory)

	n := Node{
		blockchain:    bc,
		syncService:   sync,
		rpcServer:     rpc,
		memoryHandle:  memory,
		storageSevice: storage,
		executer:      exe,
		eCh:           make(chan error),
	}
	onBlockStored.Event = n.OnBlockStored
	return &n, nil
}

func (n *Node) E() <-chan error {
	return n.eCh
}

func (n *Node) Blockchain() blockchain.BlockchainInfo {
	return n.blockchain
}

func (n *Node) SyncHandle() sync.SyncEventHandle {
	return n.syncService
}

func (n *Node) ExecuteHandle() executer.ExecuteHandle {
	return n.executer
}

func (n *Node) Run() {
	go n.catch()
	n.storageSevice.Run()
	n.syncService.Run()
	n.rpcServer.Listen()

	err := n.executer.Init()
	if err != nil {
		n.eCh <- err
		return
	}

	resCh := n.storageSevice.Get(storage.Blocks, storage.Latest, nil)
	res := <-resCh
	if res.Err != nil {
		n.eCh <- res.Err
		return
	}
	if res.Value != nil {
		blk := blocks.Block{}
		err := json.Unmarshal(res.Value, &blk)
		if err != nil {
			n.eCh <- err
			return
		}

		n.blockchain.Init(blk.Info.Height+1, blk.Info.Hash)
	}

	err = n.storageSevice.FetchAllAccounts(n.memoryHandle)
	if err != nil {
		n.eCh <- err
		return
	}

	log.Printf(
		"node is running from\n height: %d\n epoch: %d\n slot: %d\n",
		n.blockchain.NextHeight(),
		n.blockchain.NextEpoch(),
		n.blockchain.NextSlot(),
	)
}

func (n *Node) OnBlockStored(b *blocks.Block) error {
	if b.Info.Height == 0 {
		// push generator account to storage
		err := n.executer.Init()
		if err != nil {
			return err
		}
		// push generator account to memory
		fetch := [][]byte{b.Info.PublicKey}
		err = n.storageSevice.FetchAccounts(n.memoryHandle, fetch)
		if err != nil {
			return err
		}
	}

	n.blockchain.Increment(b.Info.Hash)
	return nil
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
