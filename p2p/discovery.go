package p2p

import (
	"encoding/json"
	"log"
	"strconv"
)

type DiscoveryConfig struct {
	PortMin int
	PortMax int
}

func (ps *P2pService) Discover(config DiscoveryConfig) {
	err := ps.localDiscovery(config.PortMin, config.PortMax)
	if err != nil {
		ps.eCh <- err
	}
}

func (ps *P2pService) localDiscovery(discoveryMin, discoveryMax int) error {
	self := ps.server.Self()
	hello := HelloMsg{
		From: self,
	}
	payload, err := PackPayload(hello, byte(HelloMessage))
	if err != nil {
		return err
	}

	for p := discoveryMin; p <= discoveryMax; p++ {
		to := NewNodeInfo(strconv.Itoa(p), 0)
		if self.IsSameIp(to) {
			continue
		}

		ps.transporter.Send(to, payload)
	}
	return nil
}

func (ps *P2pService) handleJoin(raw []byte) error {
	hello := HelloMsg{}
	err := json.Unmarshal(raw, &hello)
	if err != nil {
		handleUnexpectedData(err)
		return nil
	}
	if !hello.Verify() {
		return nil
	}
	if hello.From.Network != ps.server.Self().Network {
		return nil
	}

	log.Printf("%s joined \n", hello.From.Ip4)
	ps.addPeer(hello.From)
	// send welcome
	welcom := WelcomeMsg{
		From:      ps.server.Self(),
		KnownPeer: ps.peers,
	}
	payload, err := PackPayload(welcom, byte(WelcomeMessage))
	if err != nil {
		return err
	}

	ps.transporter.Send(hello.From, payload)
	return nil
}

func (ps *P2pService) handleWelcome(raw []byte) error {
	welcome := WelcomeMsg{}
	err := json.Unmarshal(raw, &welcome)
	if err != nil {
		handleUnexpectedData(err)
		return nil
	}
	if !welcome.Verify() {
		return nil
	}

	ps.addPeer(welcome.From)

	// send hello if peer is not known
	if len(welcome.KnownPeer) > 0 {
		self := ps.server.Self()
		hello := HelloMsg{
			From: self,
		}
		payload, err := PackPayload(hello, byte(HelloMessage))
		if err != nil {
			return err
		}

		for _, peer := range welcome.KnownPeer {
			if !ps.KnowsPeer(peer) && !self.IsSameIp(peer) {
				ps.transporter.Send(peer, payload)
			}
		}
	}
	return nil
}
