package p2p

import (
	"io"
	"log"
	"net"
	"simple-blockchain-go2/common"

	"golang.org/x/exp/slices"
)

func handleUnexpectedData(err error) {
	log.Printf("skipping unexpected data. orignal error: %s\n", err.Error())
}

type P2pService struct {
	server      *Server
	transporter *Transporter
	handlerFn   func([]byte) error
	peers       []NodeInfo
	failCh      chan NodeInfo
	eCh         chan error
}

func NewP2pService(port string, network NetworkKind) *P2pService {
	onFail := make(chan NodeInfo)
	return &P2pService{
		server:      NewServer(port, network),
		transporter: NewTransporter(onFail),
		peers:       make([]NodeInfo, 0),
		failCh:      onFail,
		eCh:         make(chan error),
	}
}

func (ps *P2pService) E() <-chan error {
	return ps.eCh
}

func (ps *P2pService) Run(fn func([]byte) error) {
	go ps.catch()
	go ps.onFail()
	ps.handlerFn = fn
	ps.server.Listen(ps.handleInbound)
}

func (ps *P2pService) Self() NodeInfo {
	return ps.server.Self()
}

func (ps *P2pService) Send(to NodeInfo, data []byte) {
	if !to.IsSameIp(ps.server.Self()) {
		ps.transporter.Send(to, data)
	}
}

func (ps *P2pService) Broadcast(data []byte, except ...NodeInfo) {
	if len(except) == 0 {
		for _, p := range ps.peers {
			ps.transporter.Send(p, data)
		}
	} else {
		for _, p := range ps.peers {
			for _, exc := range except {
				if !p.IsSameIp(exc) {
					ps.transporter.Send(p, data)
				}
			}
		}
	}
}

func (ps *P2pService) LenPeers() int {
	return len(ps.peers)
}

func (ps *P2pService) KnowsPeer(peer NodeInfo) bool {
	return slices.ContainsFunc(ps.peers, func(p NodeInfo) bool {
		return peer.IsSameIp(p)
	})
}

func (ps *P2pService) PeerByIndex(idx int) *NodeInfo {
	peerLen := len(ps.peers)
	if peerLen == 0 {
		return nil
	}
	if idx < 0 || idx > peerLen {
		return nil
	}

	return &ps.peers[idx]
}

func (ps *P2pService) catch() {
	var err error
	select {
	case err = <-ps.server.eCh:
	case err = <-ps.transporter.eCh:
	}

	ps.eCh <- err
}

func (ps *P2pService) onFail() {
	for failed := range ps.failCh {
		ps.removePeer(failed)
	}
}

func (ps *P2pService) addPeer(peer NodeInfo) {
	self := ps.server.Self()
	if !ps.KnowsPeer(peer) && !self.IsSameIp(peer) {
		log.Printf("%s is now known peer\n", peer.Ip4)
		ps.peers = append(ps.peers, peer)
	}
}

func (ps *P2pService) removePeer(peer NodeInfo) {
	idx := slices.IndexFunc(ps.peers, func(p NodeInfo) bool {
		return peer.IsSameIp(p)
	})
	if idx >= 0 {
		log.Printf("%s is out from peers\n", ps.peers[idx].Ip4)
		ps.peers = slices.Delete(ps.peers, idx, idx+1)
	}
}

func (ps *P2pService) handleInbound(conn net.Conn) error {
	defer conn.Close()
	raw, err := io.ReadAll(conn)
	if err != nil {
		return err
	}
	if len(raw) > common.MaxPayloadSize {
		log.Println("skipping too big inbound...")
		return nil
	}

	kind := MessageKind(raw[0])
	switch kind {
	case HelloMessage:
		err = ps.handleJoin(raw[1:])
	case WelcomeMessage:
		err = ps.handleWelcome(raw[1:])
	default:
		err = ps.handlerFn(raw)
	}
	return err
}
