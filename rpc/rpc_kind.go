package rpc

import (
	"encoding/json"
	"simple-blockchain-go2/common"
)

type RpcKind byte

const (
	Airdrop RpcKind = iota + 1
	Transfer
	GetAccountState
)

const (
	UnsignedRpc byte = iota
	SifnedRpc
)

func VerifyGetAccountStateCall(call Call) bool {
	return call.Call == GetAccountState &&
		len(call.Params) == common.PublicKeySize
}

func VerifyTransferCall(call Call) bool {
	if call.Call != Transfer {
		return false
	}

	param := TransferParam{}
	err := json.Unmarshal(call.Params, &param)
	if err != nil {
		return false
	}

	return param.Amount > 0 &&
		param.Amount < common.GeneratorBalance &&
		len(param.To) == common.PublicKeySize
}

func VerifyAirdropCall(call Call) bool {
	if call.Call != Airdrop {
		return false
	}

	param := AirdropParam{}
	err := json.Unmarshal(call.Params, &param)
	if err != nil {
		return false
	}
	return param.Amount > 0 && param.Amount < common.GeneratorBalance
}
