package qkcdb

import (
	"errors"
	"strings"
)

// Similar to GetProperty(), but only works for a subset of properties whose
// return value is an integer. Return the value by integer. Supported
// properties:
//	"rocksdb.stats"
//  "rocksdb.num-immutable-mem-table"
//  "rocksdb.mem-table-flush-pending"
//  "rocksdb.compaction-pending"
//  "rocksdb.background-errors"
//  "rocksdb.cur-size-active-mem-table"
//  "rocksdb.cur-size-all-mem-tables"
//  "rocksdb.size-all-mem-tables"
//  "rocksdb.num-entries-active-mem-table"
//  "rocksdb.num-entries-imm-mem-tables"
//  "rocksdb.num-deletes-active-mem-table"
//  "rocksdb.num-deletes-imm-mem-tables"
//  "rocksdb.estimate-num-keys"
//  "rocksdb.estimate-table-readers-mem"
//  "rocksdb.is-file-deletions-enabled"
//  "rocksdb.num-snapshots"
//  "rocksdb.oldest-snapshot-time"
//  "rocksdb.num-live-versions"
//  "rocksdb.current-super-version-number"
//  "rocksdb.estimate-live-data-size"
//  "rocksdb.min-log-number-to-keep"
//  "rocksdb.min-obsolete-sst-number-to-keep"
//  "rocksdb.total-sst-files-size"
//  "rocksdb.live-sst-files-size"
//  "rocksdb.base-level"
//  "rocksdb.estimate-pending-compaction-bytes"
//  "rocksdb.num-running-compactions"
//  "rocksdb.num-running-flushes"
//  "rocksdb.actual-delayed-write-rate"
//  "rocksdb.is-write-stopped"
//  "rocksdb.estimate-oldest-key-time"
//  "rocksdb.block-cache-capacity"
//  "rocksdb.block-cache-usage"
//  "rocksdb.block-cache-pinned-usage"s
// TODO We need to disassemble the monitor data from gorocksdb.
func (db *RDBDatabase) GetProperty(name string) (value string, err error) {

	const prefix = "rocksdb."
	if !strings.HasPrefix(name, prefix) {
		return "", errors.New("rocksdb: not found")
	}
	p := name[len(prefix):]

	switch {
	case p == "stats":
		value = db.db.GetProperty(name)
		if len(value) == 0 {
			err = errors.New("rocksdb stats is empty.")
		}
		/*case p == "iostats":
			value = fmt.Sprintf("Read(MB):%.5f Write(MB):%.5f",
				float64(db.s.stor.reads())/1048576.0,
				float64(db.s.stor.writes())/1048576.0)
		case p == "writedelay":
			writeDelayN, writeDelay := atomic.LoadInt32(&db.cWriteDelayN), time.Duration(atomic.LoadInt64(&db.cWriteDelay))
			paused := atomic.LoadInt32(&db.inWritePaused) == 1
			value = fmt.Sprintf("DelayN:%d Delay:%s Paused:%t", writeDelayN, writeDelay, paused)
		case p == "sstables":
			for level, tables := range v.levels {
				value += fmt.Sprintf("--- level %d ---\n", level)
				for _, t := range tables {
					value += fmt.Sprintf("%d:%d[%q .. %q]\n", t.fd.Num, t.size, t.imin, t.imax)
				}
			}
		case p == "blockpool":
			value = fmt.Sprintf("%v", db.s.tops.bpool)
		case p == "cachedblock":
			if db.s.tops.bcache != nil {
				value = fmt.Sprintf("%d", db.s.tops.bcache.Size())
			} else {
				value = "<nil>"
			}
		case p == "openedtables":
			value = fmt.Sprintf("%d", db.s.tops.cache.Size())
		case p == "alivesnaps":
			value = fmt.Sprintf("%d", atomic.LoadInt32(&db.aliveSnaps))
		case p == "aliveiters":
			value = fmt.Sprintf("%d", atomic.LoadInt32(&db.aliveIters))*/
	default:
		err = errors.New("rocksdb: not found")
	}

	return
}
