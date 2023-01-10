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
