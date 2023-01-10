package p2p

import (
	"net"
	"simple-blockchain-go2/common"
)

type Transporter struct {
	eCh    chan error
	failCh chan<- NodeInfo
}

func NewTransporter(onFail chan<- NodeInfo) *Transporter {
	return &Transporter{
		eCh:    make(chan error),
		failCh: onFail,
	}
}

func (t *Transporter) E() <-chan error {
	return t.eCh
}

func (t *Transporter) Send(to NodeInfo, data []byte) {
	go t.send(to, data)
}

func (t *Transporter) send(to NodeInfo, data []byte) {
	conn, err := net.Dial(common.Tcp, to.Ip4)
	if err != nil {
		t.failCh <- to
		return
	}
	defer conn.Close()
	_, err = conn.Write(data)
	if err != nil {
		t.eCh <- err
	}
}
