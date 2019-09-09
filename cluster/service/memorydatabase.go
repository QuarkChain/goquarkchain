package service

import (
	"github.com/ethereum/go-ethereum/ethdb"
)

type QkcMemoryDB struct {
	*ethdb.MemDatabase
	isReadOnly bool
}

func NewQkcMemoryDB(isReadOnly bool) *QkcMemoryDB {
	return &QkcMemoryDB{
		MemDatabase: ethdb.NewMemDatabase(),
		isReadOnly:  isReadOnly,
	}
}

func (db *QkcMemoryDB) Put(key []byte, value []byte) error {
	if db.isReadOnly {
		return nil
	}

	return db.MemDatabase.Put(key, value)
}

func (db *QkcMemoryDB) SetIsReadOnly(isReadOnly bool) {
	db.isReadOnly = isReadOnly
}
