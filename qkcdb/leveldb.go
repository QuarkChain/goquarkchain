// +build !rdb

package qkcdb

import (
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type QKCDataBase struct {
	fn         string          // filename for reporting
	db         *leveldb.DB     // LevelDB instance
	quitLock   sync.Mutex      // Mutex protecting the quit channel access
	quitChan   chan chan error // Quit channel to stop the metrics collection before closing the database
	isReadOnly bool
}

// NewLDBDatabase returns a LevelDB wrapped object.
func NewDatabase(file string, clean, isReadOnly bool) (*QKCDataBase, error) {
	log.Info("create db", "db type", "leveldb", "file", file)
	logger := log.New("database", file)

	// Ensure we have some minimal caching and file guarantees

	logger.Info("Allocated cache and file handles", "cache", cache, "handles", handles)

	opt := &opt.Options{
		OpenFilesCacheCapacity: handles,
		BlockCacheCapacity:     cache / 2 * opt.MiB,
		WriteBuffer:            cache / 4 * opt.MiB, // Two of these are used internally
		Filter:                 filter.NewBloomFilter(10),
	}

	// Open the db and recover any potential corruptions
	if clean {
		if err := os.RemoveAll(file); err != nil {
			return nil, err
		}
	}

	db, err := leveldb.OpenFile(file, opt)
	if err != nil {
		return nil, err
	}

	if _, corrupted := err.(*errors.ErrCorrupted); corrupted {
		db, err = leveldb.RecoverFile(file, nil)
	}
	// (Re)check for errors and abort if opening of the db failed
	if err != nil {
		return nil, err
	}
	return &QKCDataBase{
		fn:         file,
		db:         db,
		isReadOnly: isReadOnly,
	}, nil
}

// Path returns the path to the database directory.
func (db *QKCDataBase) Path() string {
	return db.fn
}

// Put puts the given key / value to the queue
func (db *QKCDataBase) Put(key []byte, value []byte) error {
	if db.isReadOnly {
		return nil
	}
	return db.db.Put(key, value, nil)
}

func (db *QKCDataBase) Has(key []byte) (bool, error) {
	return db.db.Has(key, nil)
}

// Get returns the given key if it's present.
func (db *QKCDataBase) Get(key []byte) ([]byte, error) {
	dat, err := db.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	return dat, nil
}

// Delete deletes the key from the queue and database
func (db *QKCDataBase) Delete(key []byte) error {
	return db.db.Delete(key, nil)
}

func (db *QKCDataBase) NewIterator() *QKCIterator {
	return &QKCIterator{
		db.db.NewIterator(nil, nil),
	}
}

func (db *QKCDataBase) Close() {
	// Stop the metrics collection to avoid internal database races
	db.quitLock.Lock()
	defer db.quitLock.Unlock()

	if db.quitChan != nil {
		errc := make(chan error)
		db.quitChan <- errc
		if err := <-errc; err != nil {
			log.Error("Metrics collection failed", "err", err)
		}
		db.quitChan = nil
	}
	err := db.db.Close()
	if err == nil {
		log.Info("Database closed")
	} else {
		log.Error("Failed to close database", "err", err)
	}
}

func (db *QKCDataBase) LDB() *leveldb.DB {
	return db.db
}

func (db *QKCDataBase) NewBatch() Batch {
	return &ldbBatch{db: db.db, b: new(leveldb.Batch)}
}

type ldbBatch struct {
	db   *leveldb.DB
	b    *leveldb.Batch
	size int
}

func (b *ldbBatch) Put(key, value []byte) error {
	b.b.Put(key, value)
	b.size += len(value)
	return nil
}

func (b *ldbBatch) Delete(key []byte) error {
	b.b.Delete(key)
	b.size += 1
	return nil
}

func (b *ldbBatch) Write() error {
	return b.db.Write(b.b, nil)
}

func (b *ldbBatch) ValueSize() int {
	return b.size
}

func (b *ldbBatch) Reset() {
	b.b.Reset()
	b.size = 0
}

type QKCIterator struct {
	it iterator.Iterator
}

func (q *QKCIterator) Key() []byte {
	return q.it.Key()
}

func (q *QKCIterator) Next() {
	q.it.Next()
}

func (q *QKCIterator) Valid() bool {
	return q.it.Valid()
}

func (q *QKCIterator) Seek(b []byte) {
	q.it.Seek(b)
}
