package p2p

import (
	"fmt"
	"simple-blockchain-go2/common"
	"strings"
)

type NetworkKind byte

const (
	SyncNode NetworkKind = iota + 1
	ConsensusNode
)

type NodeInfo struct {
	Ip4     string
	Network NetworkKind
}

func NewNodeInfo(port string, network NetworkKind) NodeInfo {
	return NodeInfo{
		Ip4:     fmt.Sprintf(common.Ip4Format, port),
		Network: network,
	}
}

func (ni *NodeInfo) IsSameIp(other NodeInfo) bool {
	return strings.Compare(ni.Ip4, other.Ip4) == 0
}

// localhost:1234 only
func (ni *NodeInfo) Verify() bool {
	return len(ni.Ip4) == 14 &&
		strings.Compare(ni.Ip4[:10], "localhost:") == 0
}
