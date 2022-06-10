package main

import (
	"errors"
	"os"
	"sync"

	"github.com/QuarkChain/goquarkchain/qkcdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tecbot/gorocksdb"
)

const (
	cache   int = 128
	handles     = 256
)

type QKCDataBase struct {
	fn         string        // filename for reporting
	db         *gorocksdb.DB // RocksDB instance
	ro         *gorocksdb.ReadOptions
	wo         *gorocksdb.WriteOptions
	isReadOnly bool
	closeOnce  sync.Once
	log        log.Logger // Contextual logger tracking the database path
}

// NewDatabase returns a rocksdb wrapped object.
func NewDatabase(file string, clean, isReadOnly bool) (*QKCDataBase, error) {
	log.Info("create db", "db type", "rocksdb", "file", file)
	logger := log.New("database", file)

	if err := os.MkdirAll(file, 0700); err != nil {
		return nil, err
	}
	opts := gorocksdb.NewDefaultOptions()
	if clean {
		if err := gorocksdb.DestroyDb(file, opts); err != nil {
			return nil, err
		}
	}
	// ubuntu 16.04 max files descriptors 524288
	// opts.SetMaxFileOpeningThreads(handles)
	// 128 MiB
	// sets the maximum number of write buffers that are built up in memory.
	opts.SetMaxWriteBufferNumber(3)
	// sets the target file size for compaction.
	opts.SetTargetFileSizeBase(67108864)
	opts.SetCreateIfMissing(true)
	opts.SetMaxOpenFiles(100000)
	opts.SetWriteBufferSize(int(cache) * 1024 * 1024)
	opts.SetCompression(gorocksdb.SnappyCompression)
	// Open the db and recover any potential corruptions
	db, err := gorocksdb.OpenDb(opts, file)
	// check for errors and abort if opening of the db failed
	if err != nil {
		return nil, err
	}

	ro := gorocksdb.NewDefaultReadOptions()
	wo := gorocksdb.NewDefaultWriteOptions()

	logger.Info("Allocated cache and file handles", "cache", cache, "handles", handles)

	return &QKCDataBase{
		fn:         file,
		db:         db,
		ro:         ro,
		wo:         wo,
		isReadOnly: isReadOnly,
		log:        logger,
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
	if len(key) == 0 || len(value) == 0 {
		return errors.New("failed to put data, key or value can't be empty")
	}
	return db.db.Put(db.wo, key, value)
}

func (db *QKCDataBase) Has(key []byte) (bool, error) {
	_, err := db.Get(key)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Get returns the given key if it's present.
func (db *QKCDataBase) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, errors.New("failed to get data from database, key can't be empty")
	}
	dat, err := db.db.Get(db.ro, key)
	if err != nil {
		return nil, err
	}
	defer dat.Free()
	if dat.Size() == 0 {
		return nil, errors.New("failed to get data from rocksdb, return empty data")
	}
	result := make([]byte, len(dat.Data()))
	copy(result, dat.Data())
	return result, nil
}

// Delete deletes the key from the queue and database
func (db *QKCDataBase) Delete(key []byte) error {
	return db.db.Delete(db.wo, key)
}

func (db *QKCDataBase) NewIterator() *QKCIterator {
	return &QKCIterator{
		db.db.NewIterator(db.ro),
	}
}

func (db *QKCDataBase) Close() {
	db.closeOnce.Do(func() {
		db.db.Close()
	})
}

func (db *QKCDataBase) NewBatch() qkcdb.Batch {
	return &rdbBatch{db: db.db /*ro: db.ro,*/, wo: db.wo, w: gorocksdb.NewWriteBatch()}
}

type rdbBatch struct {
	db *gorocksdb.DB
	wo *gorocksdb.WriteOptions
	w  *gorocksdb.WriteBatch
}

func (b *rdbBatch) Put(key, value []byte) error {
	b.w.Put(key, value)
	return nil
}

func (b *rdbBatch) Delete(key []byte) error {
	b.w.Delete(key)
	return nil
}

func (b *rdbBatch) Write() error {
	defer b.w.Destroy()
	return b.db.Write(b.wo, b.w)
}

func (b *rdbBatch) ValueSize() int {
	return b.w.Count()
}

func (b *rdbBatch) Reset() {
	b.w.Clear()
}

type QKCIterator struct {
	it *gorocksdb.Iterator
}

func (q *QKCIterator) Key() []byte {
	return q.it.Key().Data()
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
