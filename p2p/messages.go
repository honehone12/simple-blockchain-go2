package p2p

import (
	"encoding/json"
)

type MessageKind byte

const (
	HelloMessage MessageKind = iota + 1
	WelcomeMessage
	SyncMessage
	ConsensusMessage
)

func (mk MessageKind) PackPayload(data interface{}) ([]byte, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	p := make([]byte, 0, len(b)+1)
	p = append(p, byte(mk))
	p = append(p, b...)
	return p, nil
}

type HelloMsg struct {
	From NodeInfo
}

func (hm *HelloMsg) Verify() bool {
	return hm.From.Verify()
}

type WelcomeMsg struct {
	From      NodeInfo
	KnownPeer []NodeInfo
}

func (wm *WelcomeMsg) Verify() bool {
	if !wm.From.Verify() {
		return false
	}
	for _, p := range wm.KnownPeer {
		if !p.Verify() {
			return false
		}
	}
	return true
}
