package rpc

type AirdropParam struct {
	Amount uint64
}

func NewAirdropParam(amount uint64) AirdropParam {
	return AirdropParam{
		Amount: amount,
	}
}

type TransferParam struct {
	Amount uint64
	To     []byte
}

func NewTransferParam(amount uint64, to []byte) TransferParam {
	return TransferParam{
		Amount: amount,
		To:     to,
	}
}
