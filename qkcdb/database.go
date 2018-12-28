// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// +build !js

package qkcdb

import (
	"errors"
	"fmt"
	"github.com/tecbot/gorocksdb"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

const (
	writePauseWarningThrottler = 1 * time.Minute
)

type RDBDatabase struct {
	fn string        // filename for reporting
	db *gorocksdb.DB // RocksDB instance
	ro *gorocksdb.ReadOptions
	wo *gorocksdb.WriteOptions

	compTimeMeter    metrics.Meter // Meter for measuring the total time spent in database compaction
	compReadMeter    metrics.Meter // Meter for measuring the data read during compaction
	compWriteMeter   metrics.Meter // Meter for measuring the data written during compaction
	writeDelayNMeter metrics.Meter // Meter for measuring the write delay number due to database compaction
	writeDelayMeter  metrics.Meter // Meter for measuring the write delay duration due to database compaction
	diskReadMeter    metrics.Meter // Meter for measuring the effective amount of data read
	diskWriteMeter   metrics.Meter // Meter for measuring the effective amount of data written

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

// Meter configures the database metrics collectors and
func (db *RDBDatabase) Meter(prefix string) {
	// Initialize all the metrics collector at the requested prefix
	db.compTimeMeter = metrics.NewRegisteredMeter(prefix+"compact/time", nil)
	db.compReadMeter = metrics.NewRegisteredMeter(prefix+"compact/input", nil)
	db.compWriteMeter = metrics.NewRegisteredMeter(prefix+"compact/output", nil)
	db.diskReadMeter = metrics.NewRegisteredMeter(prefix+"disk/read", nil)
	db.diskWriteMeter = metrics.NewRegisteredMeter(prefix+"disk/write", nil)
	db.writeDelayMeter = metrics.NewRegisteredMeter(prefix+"compact/writedelay/duration", nil)
	db.writeDelayNMeter = metrics.NewRegisteredMeter(prefix+"compact/writedelay/counter", nil)

	// Create a quit channel for the periodic collector and run it
	db.quitLock.Lock()
	db.quitChan = make(chan chan error)
	db.quitLock.Unlock()

	go db.meter(3 * time.Second)
}

// meter periodically retrieves internal rocksdb	 counters and reports them to
// the metrics subsystem.
//
// This is how a stats table look like (currently):
// rocksdb.state:
// ** Compaction Stats [default] **
// Level    Files   Size     Score Read(GB)  Rn(GB) Rnp1(GB) Write(GB) Wnew(GB) Moved(GB) W-Amp Rd(MB/s) Wr(MB/s) Comp(sec) Comp(cnt) Avg(sec) KeyIn KeyDrop
//----------------------------------------------------------------------------------------------------------------------------------------------------------
// Sum      0/0    0.00 KB   0.0      0.0     0.0      0.0       0.0      0.0       0.0   0.0      0.0      0.0         0         0    0.000       0      0
// Int      0/0    0.00 KB   0.0      0.0     0.0      0.0       0.0      0.0       0.0   0.0      0.0      0.0         0         0    0.000       0      0
// Uptime(secs): 0.0 total, 0.0 interval
// Flush(GB): cumulative 0.000, interval 0.000
// AddFile(GB): cumulative 0.000, interval 0.000
// AddFile(Total Files): cumulative 0, interval 0
// AddFile(L0 Files): cumulative 0, interval 0
// AddFile(Keys): cumulative 0, interval 0
// Cumulative compaction: 0.00 GB write, 0.00 MB/s write, 0.00 GB read, 0.00 MB/s read, 0.0 seconds
// Interval compaction: 0.00 GB write, 0.00 MB/s write, 0.00 GB read, 0.00 MB/s read, 0.0 seconds
// Stalls(count): 0 level0_slowdown, 0 level0_slowdown_with_compaction, 0 level0_numfiles, 0 level0_numfiles_with_compaction, 0 stop for pending_compaction_bytes, 0 slowdown for pending_compaction_bytes, 0 memtable_compaction, 0 memtable_slowdown, interval 0 total count
//
//** File Read Latency Histogram By Level [default] **
//
//** DB Stats **
// Uptime(secs): 0.0 total, 0.0 interval
// Cumulative writes: 0 writes, 0 keys, 0 commit groups, 0.0 writes per commit group, ingest: 0.00 GB, 0.00 MB/s
// Cumulative WAL: 0 writes, 0 syncs, 0.00 writes per sync, written: 0.00 GB, 0.00 MB/s
// Cumulative stall: 00:00:0.000 H:M:S, 0.0 percent
// Interval writes: 0 writes, 0 keys, 0 commit groups, 0.0 writes per commit group, ingest: 0.00 MB, 0.00 MB/s
// Interval WAL: 0 writes, 0 syncs, 0.00 writes per sync, written: 0.00 MB, 0.00 MB/s
// Interval stall: 00:00:0.000 H:M:S, 0.0 percent
// TODO qkcdb's monitor module needs to be improved
func (db *RDBDatabase) meter(refresh time.Duration) {
	// Create the counters to store current and previous compaction values
	compactions := make([][]float64, 2)
	for i := 0; i < 2; i++ {
		compactions[i] = make([]float64, 3)
	}
	// Create storage for iostats.
	var iostats [2]float64

	// Create storage and warning log tracer for write delay.
	var (
		delaystats      [2]int64
		lastWritePaused time.Time
	)

	var (
		errc chan error
		merr error
	)

	// Iterate ad infinitum and collect the stats
	for i := 1; errc == nil && merr == nil; i++ {
		// Retrieve the database stats
		stats, err := db.GetProperty("rocksdb.stats")
		if err != nil {
			db.log.Error("Failed to read database stats", "err", err)
			merr = err
			continue
		}
		// Find the compaction table, skip the header
		lines := strings.Split(stats, "\n")
		for len(lines) > 0 && strings.TrimSpace(lines[0]) != "Compactions" {
			lines = lines[1:]
		}
		if len(lines) <= 3 {
			db.log.Error("Compaction table not found")
			merr = errors.New("compaction table not found")
			continue
		}
		lines = lines[3:]

		// Iterate over all the table rows, and accumulate the entries
		for j := 0; j < len(compactions[i%2]); j++ {
			compactions[i%2][j] = 0
		}
		for _, line := range lines {
			parts := strings.Split(line, "|")
			if len(parts) != 6 {
				break
			}
			for idx, counter := range parts[3:] {
				value, err := strconv.ParseFloat(strings.TrimSpace(counter), 64)
				if err != nil {
					db.log.Error("Compaction entry parsing failed", "err", err)
					merr = err
					continue
				}
				compactions[i%2][idx] += value
			}
		}
		// Update all the requested meters
		if db.compTimeMeter != nil {
			db.compTimeMeter.Mark(int64((compactions[i%2][0] - compactions[(i-1)%2][0]) * 1000 * 1000 * 1000))
		}
		if db.compReadMeter != nil {
			db.compReadMeter.Mark(int64((compactions[i%2][1] - compactions[(i-1)%2][1]) * 1024 * 1024))
		}
		if db.compWriteMeter != nil {
			db.compWriteMeter.Mark(int64((compactions[i%2][2] - compactions[(i-1)%2][2]) * 1024 * 1024))
		}

		// Retrieve the write delay statistic
		writedelay, err := db.GetProperty("rocksdb.writedelay")
		if err != nil {
			db.log.Error("Failed to read database write delay statistic", "err", err)
			merr = err
			continue
		}
		var (
			delayN        int64
			delayDuration string
			duration      time.Duration
			paused        bool
		)
		if n, err := fmt.Sscanf(writedelay, "DelayN:%d Delay:%s Paused:%t", &delayN, &delayDuration, &paused); n != 3 || err != nil {
			db.log.Error("Write delay statistic not found")
			merr = err
			continue
		}
		duration, err = time.ParseDuration(delayDuration)
		if err != nil {
			db.log.Error("Failed to parse delay duration", "err", err)
			merr = err
			continue
		}
		if db.writeDelayNMeter != nil {
			db.writeDelayNMeter.Mark(delayN - delaystats[0])
		}
		if db.writeDelayMeter != nil {
			db.writeDelayMeter.Mark(duration.Nanoseconds() - delaystats[1])
		}
		// If a warning that db is performing compaction has been displayed, any subsequent
		// warnings will be withheld for one minute not to overwhelm the user.
		if paused && delayN-delaystats[0] == 0 && duration.Nanoseconds()-delaystats[1] == 0 &&
			time.Now().After(lastWritePaused.Add(writePauseWarningThrottler)) {
			db.log.Warn("Database compacting, degraded performance")
			lastWritePaused = time.Now()
		}
		delaystats[0], delaystats[1] = delayN, duration.Nanoseconds()

		// Retrieve the database iostats.
		ioStats, err := db.GetProperty("rocksdb.iostats")
		if err != nil {
			db.log.Error("Failed to read database iostats", "err", err)
			merr = err
			continue
		}
		var nRead, nWrite float64
		parts := strings.Split(ioStats, " ")
		if len(parts) < 2 {
			db.log.Error("Bad syntax of ioStats", "ioStats", ioStats)
			merr = fmt.Errorf("bad syntax of ioStats %s", ioStats)
			continue
		}
		if n, err := fmt.Sscanf(parts[0], "Read(MB):%f", &nRead); n != 1 || err != nil {
			db.log.Error("Bad syntax of read entry", "entry", parts[0])
			merr = err
			continue
		}
		if n, err := fmt.Sscanf(parts[1], "Write(MB):%f", &nWrite); n != 1 || err != nil {
			db.log.Error("Bad syntax of write entry", "entry", parts[1])
			merr = err
			continue
		}
		if db.diskReadMeter != nil {
			db.diskReadMeter.Mark(int64((nRead - iostats[0]) * 1024 * 1024))
		}
		if db.diskWriteMeter != nil {
			db.diskWriteMeter.Mark(int64((nWrite - iostats[1]) * 1024 * 1024))
		}
		iostats[0], iostats[1] = nRead, nWrite

		// Sleep a bit, then repeat the stats collection
		select {
		case errc = <-db.quitChan:
			// Quit requesting, stop hammering the database
		case <-time.After(refresh):
			// Timeout, gather a new set of stats
		}
	}

	if errc == nil {
		errc = <-db.quitChan
	}
	errc <- merr
}

func (db *RDBDatabase) NewBatch() Batch {
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
