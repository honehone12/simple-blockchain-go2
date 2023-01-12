package p2p

import (
	"log"
	"net"
	"simple-blockchain-go2/common"
)

type Server struct {
	info NodeInfo
	eCh  chan error
	f    func(net.Conn) error
}

func NewServer(port string, network NetworkKind) *Server {
	return &Server{
		info: NewNodeInfo(port, network),
		eCh:  make(chan error),
	}
}

func (s *Server) E() <-chan error {
	return s.eCh
}

func (s *Server) Self() NodeInfo {
	return s.info
}

func (s *Server) Listen(handlerFunc func(net.Conn) error) {
	s.f = handlerFunc
	go s.listen()
}

func (s *Server) listen() {
	listener, err := net.Listen(common.Tcp, s.info.Ip4)
	if err != nil {
		s.eCh <- err
		return
	}
	defer listener.Close()
	log.Printf("listening at: %s\n", s.info.Ip4)

	// here is very weak for fast inbound
	for {
		conn, err := listener.Accept()
		if err != nil {
			s.eCh <- err
			return
		}

		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	err := s.f(conn)
	if err != nil {
		s.eCh <- err
	}
}
