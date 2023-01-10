package storage

import (
	"fmt"
	"log"

	"simple-blockchain-go2/common"

	bolt "go.etcd.io/bbolt"
)

const (
	databaseFile = "%s.db"
	latestKey    = "latest"
	latestIndex  = "latestIdx"
)

type DbContent interface {
	ToBytes() ([]byte, error)
	GetKey() ([]byte, error)
	GetIndex() ([]byte, error)
}

type StorageDb struct {
	innerDb *bolt.DB
}

func DatabaseFileName(id string) string {
	return fmt.Sprintf(databaseFile, id)
}

func NewStorageDb(name string, buckets ...[]byte) (StorageDb, error) {
	if common.ExistFile(DatabaseFileName(name)) {
		log.Printf("found existing database for id: %s\n", name)
		db, err := bolt.Open(DatabaseFileName(name), 0600, nil)
		return StorageDb{db}, err
	}

	db, err := bolt.Open(DatabaseFileName(name), 0600, nil)
	if err != nil {
		return StorageDb{nil}, err
	}
	sdb := StorageDb{db}
	for i := 0; i < len(buckets); i++ {
		sdb.makeBucket(buckets[i])
	}

	log.Printf("database for id: %s is created\n", name)
	return sdb, err
}

func (db *StorageDb) Put(bucket []byte, content DbContent) error {
	return db.innerDb.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		enc, err := content.ToBytes()
		if err != nil {
			return err
		}
		key, err := content.GetKey()
		if err != nil {
			return err
		}
		idx, err := content.GetIndex()
		if err != nil {
			return err
		}
		err = b.Put(key, enc)
		if err != nil {
			return err
		}
		err = b.Put(idx, key)
		if err != nil {
			return err
		}
		err = b.Put([]byte(latestKey), key)
		if err != nil {
			return err
		}
		return b.Put([]byte(latestIndex), idx)
	})
}

func (db *StorageDb) GetByKey(bucket, key []byte) ([]byte, error) {
	var content []byte = nil
	err := db.innerDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		content = b.Get(key)
		return nil
	})
	return content, err
}

func (db *StorageDb) GetByIndex(bucket, index []byte) ([]byte, error) {
	var content []byte = nil
	err := db.innerDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		key := b.Get(index)
		content = b.Get(key)
		return nil
	})
	return content, err
}

func (db *StorageDb) GetLatestKey(bucket []byte) ([]byte, error) {
	var key []byte = nil
	err := db.innerDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		key = b.Get([]byte(latestKey))
		return nil
	})
	return key, err
}

func (db *StorageDb) GetLatestIndex(bucket []byte) ([]byte, error) {
	var idx []byte = nil
	err := db.innerDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		idx = b.Get([]byte(latestIndex))
		return nil
	})
	return idx, err
}

func (db *StorageDb) GetLatest(bucket []byte) ([]byte, error) {
	var content []byte = nil
	err := db.innerDb.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		key := b.Get([]byte(latestKey))
		content = b.Get(key)
		return nil
	})
	return content, err
}

func (db *StorageDb) makeBucket(name []byte) error {
	return db.innerDb.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(name)
		if err != nil {
			return err
		}
		return err
	})
}
