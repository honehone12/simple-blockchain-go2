package storage

import (
	"encoding/json"
	"log"
	"simple-blockchain-go2/accounts"
	"simple-blockchain-go2/common"
	"simple-blockchain-go2/memory"

	bolt "go.etcd.io/bbolt"
)

type RequestKind byte

const (
	Get RequestKind = iota + 1
	Put
)

type ReferenceKind byte

const (
	Key ReferenceKind = iota + 1
	Index
	Latest
	LatestKey
	LatestIndex
)

type BucketKind byte

const (
	Blocks BucketKind = iota + 1
	Accounts
)

type StorageRequest struct {
	RequestKind
	BucketKind
	Content DbContent
	ReferenceKind
	Reference []byte
	C         chan<- common.Result[[]byte]
}

type StorageService struct {
	db    StorageDb
	reqCh chan StorageRequest

	blocksBucket   []byte
	accountsBucket []byte
}

type StorageHandle interface {
	Get(
		bucket BucketKind,
		refKind ReferenceKind,
		ref []byte,
	) <-chan common.Result[[]byte]

	Put(bucket BucketKind, content DbContent) <-chan common.Result[[]byte]
}

func NewStorageService(name string) (*StorageService, error) {
	blkBucket := []byte("blocks")
	acntBucket := []byte("accounts")
	sdb, err := NewStorageDb(name, blkBucket, acntBucket)
	if err != nil {
		return nil, err
	}
	return &StorageService{
		db:             sdb,
		reqCh:          make(chan StorageRequest),
		blocksBucket:   blkBucket,
		accountsBucket: acntBucket,
	}, nil
}

func (sts *StorageService) Run() {
	go sts.run()
}

func (sts *StorageService) FetchAllAccounts(mem memory.MemoryHandle) error {
	i := 0
	err := sts.db.innerDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(sts.accountsBucket)
		c := b.Cursor()
		var err error
		state := accounts.AccountState{}
		for k, v := c.First(); k != nil; k, v = c.Next() {
			err = json.Unmarshal(v, &state)
			if err == nil {
				return err
			}
			mem.PutAccountState(k, &state)
			i++
		}
		return nil
	})
	if err != nil {
		return err
	}

	log.Printf("database fetched %d accounts\n", i)
	return nil
}

func (sts *StorageService) Get(
	bucket BucketKind,
	refKind ReferenceKind,
	ref []byte,
) <-chan common.Result[[]byte] {
	c := make(chan common.Result[[]byte], 1)
	req := StorageRequest{
		RequestKind:   Get,
		BucketKind:    bucket,
		ReferenceKind: refKind,
		Reference:     ref,
		C:             c,
	}
	sts.reqCh <- req
	return c
}

func (sts *StorageService) Put(bucket BucketKind, content DbContent,
) <-chan common.Result[[]byte] {
	c := make(chan common.Result[[]byte], 1)
	req := StorageRequest{
		RequestKind: Put,
		BucketKind:  bucket,
		Content:     content,
		C:           c,
	}
	sts.reqCh <- req
	return c
}

func (sts *StorageService) run() {
	for req := range sts.reqCh {
		if req.RequestKind == Put {
			err := sts.put(req.BucketKind, req.Content)
			result := common.Result[[]byte]{
				Value: nil,
				Err:   err,
			}
			req.C <- result
		} else if req.RequestKind == Get {
			v, err := sts.get(
				req.BucketKind,
				req.ReferenceKind,
				req.Reference,
			)
			result := common.Result[[]byte]{
				Value: v,
				Err:   err,
			}
			req.C <- result
		}
	}
}

func (sts *StorageService) put(bucket BucketKind, content DbContent) error {
	if content == nil {
		return nil
	}

	if bucket == Blocks {
		return sts.db.Put(sts.blocksBucket, content)
	} else if bucket == Accounts {
		return sts.db.Put(sts.accountsBucket, content)
	} else {
		return nil
	}
}

func (sts *StorageService) get(
	bucket BucketKind,
	refKind ReferenceKind,
	ref []byte,
) ([]byte, error) {
	if (refKind == Key || refKind == Index) && ref == nil {
		return nil, nil
	}

	var b []byte = nil
	if bucket == Blocks {
		b = sts.blocksBucket
	} else if bucket == Accounts {
		b = sts.accountsBucket
	} else {
		return nil, nil
	}

	switch refKind {
	case Key:
		return sts.db.GetByKey(b, ref)
	case Index:
		return sts.db.GetByIndex(b, ref)
	case Latest:
		return sts.db.GetLatest(b)
	case LatestKey:
		return sts.db.GetLatestKey(b)
	case LatestIndex:
		return sts.db.GetLatestIndex(b)
	default:
		return nil, nil
	}
}
