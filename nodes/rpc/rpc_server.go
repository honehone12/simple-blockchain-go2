package rpc

import (
	"encoding/json"
	"io"
	"log"
	"net"
	"simple-blockchain-go2/accounts"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/memory"
	"simple-blockchain-go2/p2p"
	"simple-blockchain-go2/rpc"
	"simple-blockchain-go2/storage"
	"simple-blockchain-go2/txs"
)

type RpcServer struct {
	server        *p2p.Server
	memoryHandle  memory.MemoryHandle
	storageHandle storage.StorageHandle
}

func NewRpcServer(
	port string,
	memory memory.MemoryHandle,
	storage storage.StorageHandle,
) *RpcServer {
	s := p2p.NewServer(port, 0)
	return &RpcServer{
		server:        s,
		memoryHandle:  memory,
		storageHandle: storage,
	}
}

func (rs *RpcServer) E() <-chan error {
	return rs.server.E()
}

func (rs *RpcServer) Listen() {
	rs.server.Listen(rs.handleRpc)
}

func (rs *RpcServer) handleRpc(conn net.Conn) error {
	defer conn.Close()

	buff := make([]byte, 4)
	_, err := conn.Read(buff)
	if err != nil {
		return err
	}
	reqLen, err := common.FromHex[int32](buff)
	if err != nil {
		return err
	}
	if reqLen > common.MaxPayloadSize || reqLen < 1 {
		log.Printf("received payload size: %d\n", reqLen)
		log.Println("skipping too big call...")
		return nil
	}
	buff = make([]byte, reqLen)
	_, err = conn.Read(buff)
	if err != nil {
		return err
	}

	var res []byte = nil
	switch buff[0] {
	case rpc.SifnedRpc:
		res = rs.handleTransaction(buff[1:])
	case rpc.UnsignedRpc:
		res, err = rs.handleCall(buff[1:])
	default:
	}
	if err != nil {
		return err
	}
	if res == nil {
		log.Println("skipping wrong format call...")
		return nil
	}

	resLen := len(res)
	prefixedData, err := common.ToHex(int32(resLen))
	if err != nil {
		return err
	}
	prefixedData = append(prefixedData, res...)
	_, err = conn.Write(prefixedData)
	if err != nil {
		return err
	}

	// wait until client cut conn
	_, err = io.ReadAll(conn)
	return err
}

func (rs *RpcServer) handleCall(raw []byte) ([]byte, error) {
	call := rpc.Call{}
	err := json.Unmarshal(raw, &call)
	if err != nil {
		return nil, nil
	}

	if rpc.VerifyGetAccountStateCall(call) {
		c := rs.storageHandle.Get(
			storage.Accounts,
			storage.Key,
			call.Params,
		)
		result := <-c

		if result.Err != nil {
			log.Printf("database error below has not returned as system error\n\n")
			log.Println(result.Err)
			return nil, nil
		}
		if result.Value == nil {
			state := accounts.AccountState{
				Nonce:   0,
				Balance: 0,
			}
			v, err := json.Marshal(state)
			if err != nil {
				return nil, err
			}
			return v, nil
		}
		return result.Value, nil
	}
	return nil, nil
}

func (rs *RpcServer) handleTransaction(raw []byte) []byte {
	tx := txs.Transaction{}
	err := json.Unmarshal(raw, &tx)
	if err != nil {
		return nil
	}

	ok, err := tx.Verify()
	if err != nil || !ok {
		return nil
	}

	call := rpc.Call{}
	err = json.Unmarshal(tx.InnerData.Data, &call)
	if err != nil {
		return nil
	}

	switch call.Call {
	case rpc.Airdrop:
		if !rpc.VerifyAirdropCall(call) {
			return nil
		}
	case rpc.Transfer:
		if !rpc.VerifyTransferCall(call) {
			return nil
		}
	default:
		return nil
	}

	rs.memoryHandle.AppendTx(&tx)
	return []byte("ok")
}
