package nodes

import (
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/nodes/rpc"
	"simple-blockchain-go2/nodes/sync"
	"simple-blockchain-go2/storage"
)

type Node struct {
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
) (*Node, error) {
	sts, err := storage.NewStorageService(name)
	if err != nil {
		return nil, err
	}

	return &Node{
		syncService:   sync.NewSyncService(syncPort, sts),
		rpcServer:     rpc.NewRpcServer(rpcPort, memory, sts),
		memoryHandle:  memory,
		storageSevice: sts,
		eCh:           make(chan error),
	}, nil
}

func (n *Node) E() <-chan error {
	return n.eCh
}

func (n *Node) Run() {
	n.storageSevice.Run()
	n.syncService.Run()
	n.rpcServer.Listen()
	go n.catch()

	err := n.storageSevice.FetchAllAccounts(n.memoryHandle)
	if err != nil {
		n.eCh <- err
	}
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
