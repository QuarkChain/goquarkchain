package qkcdb

import (
	"errors"
	"os"
	"sync"

	"github.com/ethereum/go-ethereum/log"
	"github.com/tecbot/gorocksdb"
)

const (
	cache   uint64 = 128
	handles        = 1024
)

type RDBDatabase struct {
	fn         string        // filename for reporting
	db         *gorocksdb.DB // RocksDB instance
	ro         *gorocksdb.ReadOptions
	wo         *gorocksdb.WriteOptions
	isReadOnly bool
	closeOnce  sync.Once
	log        log.Logger // Contextual logger tracking the database path
}

// NewRDBDatabase returns a rocksdb wrapped object.
func NewRDBDatabase(file string, clean, isReadOnly bool) (*RDBDatabase, error) {
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
	opts.SetMaxFileOpeningThreads(handles)
	// 128 MiB
	opts.SetMaxTotalWalSize(uint64(cache * 1024 * 1024))
	// sets the maximum number of write buffers that are built up in memory.
	opts.SetMaxWriteBufferNumber(3)
	// sets the target file size for compaction.
	opts.SetTargetFileSizeBase(6710886)
	opts.SetCreateIfMissing(true)
	opts.IncreaseParallelism(3)
	opts.SetMaxBackgroundFlushes(1)
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

	return &RDBDatabase{
		fn:         file,
		db:         db,
		ro:         ro,
		wo:         wo,
		isReadOnly: isReadOnly,
		log:        logger,
	}, nil
}

// Path returns the path to the database directory.
func (db *RDBDatabase) Path() string {
	return db.fn
}

// Put puts the given key / value to the queue
func (db *RDBDatabase) Put(key []byte, value []byte) error {
	if db.isReadOnly {
		return nil
	}
	if len(key) == 0 || len(value) == 0 {
		return errors.New("failed to put data, key or value can't be empty")
	}
	return db.db.Put(db.wo, key, value)
}

func (db *RDBDatabase) Has(key []byte) (bool, error) {
	_, err := db.Get(key)
	if err != nil {
		return false, err
	}
	return true, nil
}

// Get returns the given key if it's present.
func (db *RDBDatabase) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, errors.New("failed to get data from database, key can't be empty")
	}
	dat, err := db.db.Get(db.ro, key)
	if err != nil {
		return nil, err
	}
	defer dat.Free()
	rawData := dat.Data()
	result := make([]byte, len(rawData))
	copy(result, rawData)
	if dat.Size() == 0 {
		return nil, errors.New("failed to get data from rocksdb, return empty data")
	}
	return result, nil
}

// Delete deletes the key from the queue and database
func (db *RDBDatabase) Delete(key []byte) error {
	return db.db.Delete(db.wo, key)
}

func (db *RDBDatabase) NewIterator() *gorocksdb.Iterator {
	return db.db.NewIterator(db.ro)
}

// NewIteratorWithPrefix returns a iterator to iterate over subset of database content with a particular prefix.
func (db *RDBDatabase) NewIteratorWithPrefix(prefix []byte) *gorocksdb.Iterator {
	it := db.NewIterator()
	it.Seek(prefix)
	return it
}

func (db *RDBDatabase) Close() {
	db.closeOnce.Do(func() {
		db.db.Close()
	})
}

func (db *RDBDatabase) NewBatch() Batch {
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
