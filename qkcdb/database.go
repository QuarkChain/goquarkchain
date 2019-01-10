package qkcdb

import (
	"errors"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tecbot/gorocksdb"
	"sync"
)

type RDBDatabase struct {
	fn string        // filename for reporting
	db *gorocksdb.DB // RocksDB instance
	ro *gorocksdb.ReadOptions
	wo *gorocksdb.WriteOptions

	quitLock sync.Mutex      // Mutex protecting the quit channel access
	quitChan chan chan error // Quit channel to stop the metrics collection before closing the database

	log log.Logger // Contextual logger tracking the database path
}

// NewRDBDatabase returns a rocksdb wrapped object.
// sets the maximum total wal size in bytes.
// Once write-ahead logs exceed this size,
// we will start forcing the flush of column families whose memtables are backed by the oldest live
// WAL file (i.e. the ones that are causing all the space amplification).
// If set to 0 (default),
// we will dynamically choose the WAL size limit to be [sum of all write_buffer_size * //max_write_buffer_number] * 4 Default: 0
func NewRDBDatabase(file string, cache int) (*RDBDatabase, error) {
	logger := log.New("database", file)

	// Ensure we have some minimal caching and file guarantees
	if cache < 16 {
		cache = 16
	}
	opts := gorocksdb.NewDefaultOptions()
	// ubuntu 16.04 max files descriptors 524288
	opts.SetMaxFileOpeningThreads(1024)
	// 128 MiB
	opts.SetMaxTotalWalSize(uint64(cache * 1024 * 1024))
	// sets the maximum number of write buffers that are built up in memory.
	opts.SetMaxWriteBufferNumber(3)
	// sets the target file size for compaction.
	opts.SetTargetFileSizeBase(6710886)
	opts.SetCreateIfMissing(true)

	// Open the db and recover any potential corruptions
	db, err := gorocksdb.OpenDb(opts, file)
	// check for errors and abort if opening of the db failed
	if err != nil {
		return nil, err
	}

	ro := gorocksdb.NewDefaultReadOptions()
	wo := gorocksdb.NewDefaultWriteOptions()

	logger.Info("Allocated cache and file handles", "cache", cache)

	return &RDBDatabase{
		fn:  file,
		db:  db,
		ro:  ro,
		wo:  wo,
		log: logger,
	}, nil
}

// Path returns the path to the database directory.
func (db *RDBDatabase) Path() string {
	return db.fn
}

// Put puts the given key / value to the queue
func (db *RDBDatabase) Put(key []byte, value []byte) error {
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
	if dat.Size() == 0 {
		return nil, errors.New("failed to get data from rocksdb, return empty data")
	}
	return dat.Data(), nil
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
	// Stop the metrics collection to avoid internal database races
	db.quitLock.Lock()
	defer db.quitLock.Unlock()

	if db.quitChan != nil {
		errc := make(chan error)
		db.quitChan <- errc
		if err := <-errc; err != nil {
			db.log.Error("Metrics collection failed", "err", err)
		}
		db.quitChan = nil
	}
	err := db.db.Close
	if err == nil {
		db.log.Info("Database closed")
	} else {
		db.log.Error("Failed to close database", "err", err)
	}
}

func (db *RDBDatabase) NewBatch() ethdb.Batch {
	return &rdbBatch{db: db.db, /*ro: db.ro,*/ wo: db.wo, w: gorocksdb.NewWriteBatch()}
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
	return b.db.Write(b.wo, b.w)
}

func (b *rdbBatch) ValueSize() int {
	return b.w.Count()
}

func (b *rdbBatch) Reset() {
	b.w.Clear()
}
