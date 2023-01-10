package merkle

import (
	"errors"
	"simple-blockchain-go2/common"

	"golang.org/x/crypto/sha3"
)

type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

type MerkleTree struct {
	RootNode *MerkleNode
}

func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	mNode := MerkleNode{
		Left:  left,
		Right: right,
	}

	if left == nil && right == nil {
		hash := sha3.Sum256(data)
		mNode.Data = hash[:]
	} else {
		prevHash := make([]byte, 0, len(left.Data)+len(right.Data))
		prevHash = append(prevHash, left.Data...)
		prevHash = append(prevHash, right.Data...)
		hash := sha3.Sum256(prevHash)
		mNode.Data = hash[:]
	}
	return &mNode
}

func NewMerkleTree(data [][]byte) (*MerkleTree, error) {
	leafLen := len(data)
	if !common.IsPowerOf2(leafLen) {
		return nil, errors.New("data length has to be power of 2")
	}

	var level []*MerkleNode
	// leaf
	for _, bs := range data {
		n := NewMerkleNode(nil, nil, bs)
		level = append(level, n)
	}
	// level
	for len(level) > 1 {
		var newLevel []*MerkleNode
		for j := 0; j < len(level); j += 2 {
			n := NewMerkleNode(level[j], level[j+1], nil)
			newLevel = append(newLevel, n)
		}
		level = newLevel
	}
	mTree := MerkleTree{level[0]}
	return &mTree, nil
}
