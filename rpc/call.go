package rpc

type Call struct {
	Call   RpcKind
	Params []byte
}

func NewCall(call RpcKind, param []byte) Call {
	return Call{
		Call:   call,
		Params: param,
	}
}

func (c Call) Verify() bool {
	switch c.Call {
	case GetAccountState:
		return VerifyGetAccountStateCall(c)
	case Airdrop:
		return VerifyAirdropCall(c)
	case Transfer:
		return VerifyTransferCall(c)
	default:
		return false
	}
}
