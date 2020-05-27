// +build !rdb

package qkcdb

import (
	"os"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
)

type QKCDataBase struct {
	*ethdb.LDBDatabase
	isReadOnly bool
}

// NewLDBDatabase returns a LevelDB wrapped object.
func NewDatabase(file string, clean, isReadOnly bool) (*QKCDataBase, error) {
	if clean {
		if err := os.RemoveAll(file); err != nil {
			return nil, err
		}
	}
	log.Info("create db", "db type", "leveldb", "file", file)
	db, err := ethdb.NewLDBDatabase(file, 16, 16)
	return &QKCDataBase{LDBDatabase: db, isReadOnly: isReadOnly}, err
}

// Put puts the given key / value to the queue
func (db *QKCDataBase) Put(key []byte, value []byte) error {
	if db.isReadOnly {
		return nil
	}
	return db.LDBDatabase.Put(key, value)
}
