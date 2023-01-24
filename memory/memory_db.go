package memory

import (
	"math/big"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/common/merkle"

	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/sha3"
	"golang.org/x/exp/slices"
)

const (
	fakeKey = "fakekey"
)

type DbContent interface {
	ToBytes() ([]byte, error)
}

type dbBox[C DbContent] struct {
	content C
}

func newDbBox[C DbContent](data C) dbBox[C] {
	return dbBox[C]{
		content: data,
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
	node := merkle.MerkleNode{Data: hash[:]}
	return &node, nil
}

type MemoryDb[C DbContent] struct {
	dbMap       map[string]dbBox[C]
	makeDefault func() C
}

func NewMemoryDb[C DbContent](defaultFunc func() C) *MemoryDb[C] {
	return &MemoryDb[C]{
		dbMap:       make(map[string]dbBox[C]),
		makeDefault: defaultFunc,
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

func (md *MemoryDb[C]) Get(key string) (C, bool) {
	box, ok := md.dbMap[key]
	if ok {
		return box.content, true
	}
	return md.makeDefault(), false
}

func (md *MemoryDb[C]) GetMerkleNodes() ([]*merkle.MerkleNode, error) {
	total := common.NextPowerOf2(len(md.dbMap))
	nodes := make([]*merkle.MerkleNode, total)
	i := 0
	for k, box := range md.dbMap {
		m, err := box.getMerkleNode(k)
		if err != nil {
			return nil, err
		}
		nodes[i] = m
		i++
	}

	slices.SortFunc(nodes[:i], func(a, b *merkle.MerkleNode) bool {
		aInt := big.NewInt(0)
		bInt := big.NewInt(0)

		// !!
		// this data is json bytes
		// not sure having fixed length
		// better using light hashing??
		aInt.SetBytes(a.Data)
		bInt.SetBytes(b.Data)
		return aInt.Cmp(bInt) == -1
	})

	if i < total {
		fakeBox := newDbBox(md.makeDefault())
		fake, err := fakeBox.getMerkleNode(fakeKey)
		if err != nil {
			return nil, err
		}
		for i < total {
			nodes[i] = fake
			i++
		}
	}
	return nodes, nil
}
