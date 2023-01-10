package memory

import (
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/common/merkle"

	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/sha3"
)

const (
	fakeKey = "fakekey"
)

type DbContent interface {
	ToBytes() ([]byte, error)
}

type dbBox[C DbContent] struct {
	content C
	node    merkle.MerkleNode
}

func newDbBox[C DbContent](data C) dbBox[C] {
	return dbBox[C]{
		content: data,
		node:    merkle.MerkleNode{},
	}
}

func (box *dbBox[C]) getMerkleNode(pubKey string) (*merkle.MerkleNode, error) {
	raw := base58.Decode(pubKey)
	b, err := box.content.ToBytes()
	if err != nil {
		return nil, err
	}
	raw = append(raw, b...)
	hash := sha3.Sum256(raw)
	box.node.Data = hash[:]
	return &box.node, nil
}

type MemoryDb[C DbContent] struct {
	dbMap       map[string]dbBox[C]
	fakeContent dbBox[C]
}

func NewMemoryDb[C DbContent](fakeData C) *MemoryDb[C] {
	return &MemoryDb[C]{
		dbMap:       make(map[string]dbBox[C]),
		fakeContent: newDbBox(fakeData),
	}
}

func (md *MemoryDb[C]) Put(key string, data C) {
	box, ok := md.dbMap[key]
	if ok {
		box.content = data
		md.dbMap[key] = box
		return
	}

	box = newDbBox(data)
	md.dbMap[key] = box
}

func (md *MemoryDb[C]) Get(key string) (*C, bool) {
	box, ok := md.dbMap[key]
	if ok {
		return &box.content, true
	}
	return nil, false
}

func (md *MemoryDb[C]) GetMerkleNodes() ([]*merkle.MerkleNode, error) {
	mapLen := len(md.dbMap)
	total := common.NextPowerOf2(mapLen)
	merkle := make([]*merkle.MerkleNode, total)
	i := 0
	for k, box := range md.dbMap {
		m, err := box.getMerkleNode(k)
		if err != nil {
			return nil, err
		}
		merkle[i] = m
		i++
	}

	fake, err := md.fakeContent.getMerkleNode(fakeKey)
	if err != nil {
		return nil, err
	}
	for i < total {
		merkle[i] = fake
		i++
	}
	return merkle, nil
}
