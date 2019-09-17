package qkcdb_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/QuarkChain/goquarkchain/qkcdb"
	"github.com/stretchr/testify/assert"
)

func newTestRDB() (*qkcdb.RDBDatabase, func()) {
	dirname, err := ioutil.TempDir(os.TempDir(), "qkcdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	db, err := qkcdb.NewRDBDatabase(dirname, false, false)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	return db, func() {
		db.Close()
		os.RemoveAll(dirname)
	}
}

func testData() map[string]string {
	var testValues = make(map[string]string)
	testValues["1"] = "a"
	testValues["23"] = "12"
	testValues["。A"] = "a3#8"
	testValues["ac"] = "a3<9"
	testValues[""] = "搜索"

	return testValues
}

func TestRDB_PutGet(t *testing.T) {
	db, remove := newTestRDB()
	defer remove()
	testPutGet(db, t)
}

func testPutGet(db qkcdb.Database, t *testing.T) {
	t.Parallel()

	testValues := testData()

	for k, v := range testValues {
		err := db.Put([]byte(k), []byte(v))
		if len(k)*len(k) != 0 && err != nil || len(k)*len(v) == 0 && err == nil {
			t.Fatalf("put failed: %v", err)
		}
	}

	for k := range testValues {
		data, err := db.Get([]byte(k))
		if len(k)*len(data) != 0 && err != nil || len(k)*len(data) == 0 && err == nil {
			t.Fatalf("get failed: %v", err)
		}
	}

	data, err := db.Get([]byte("non-exist-key"))
	if err == nil && len(data) > 0 {
		t.Fatalf("expect to return a not found error")
	}

	for k, v := range testValues {
		err := db.Put([]byte(k), []byte(v))
		if len(k)*len(v) != 0 && err != nil || len(k)*len(v) == 0 && err == nil {
			t.Fatalf("put failed: %v", err)
		}
	}

	for k, v := range testValues {
		data, err := db.Get([]byte(k))
		if len(k)*len(data) != 0 && err != nil || len(k)*len(data) == 0 && err == nil {
			t.Fatalf("get failed: %v", err)
		}
		if len(k) != 0 && !bytes.Equal(data, []byte(v)) {
			t.Fatalf("get returned wrong result, got %q expected ?", string(data))
		}
	}

	for k := range testValues {
		err := db.Delete([]byte(k))
		if err != nil {
			t.Fatalf("delete %q failed: %v", k, err)
		}
	}

	for k := range testValues {
		data, err := db.Get([]byte(k))
		if len(k)*len(data) != 0 && err != nil || len(k)*len(data) == 0 && err == nil {
			t.Fatalf("got deleted value %q", data)
		}
	}
}

func Test_batch(t *testing.T) {
	db, remove := newTestRDB()
	defer remove()
	batch := db.NewBatch()
	testBatchPutGet(db, batch, t)
}

func testBatchPutGet(db qkcdb.Database, batch qkcdb.Batch, t *testing.T) {
	t.Parallel()
	testValues := testData()

	for k, v := range testValues {
		batch.Put([]byte(k), []byte(v))
	}
	if batch.ValueSize() != len(testValues) {
		t.Fatalf("batch operation put error.")
	}

	batch.Reset()
	if batch.ValueSize() != 0 {
		t.Fatalf("clean rocksdb falied, %d", batch.ValueSize())
	}

	for k, v := range testValues {
		batch.Put([]byte(k), []byte(v))
	}
	batch.Write()

	for k, v := range testValues {
		data, err := db.Get([]byte(k))
		if len(k)*len(data) != 0 && err != nil || len(k)*len(data) == 0 && err == nil {
			t.Fatalf("After batch write, db get error: %v", err)
		}
		if len(k) != 0 && string(data) != v {
			t.Fatalf("After batch write, not exist, real : %v,return : %v", v, string(data))
		}
	}
}

func TestRDB_ParallelPutGet(t *testing.T) {
	db, remove := newTestRDB()
	defer remove()
	testParallelPutGet(db, t)
}

func testParallelPutGet(db qkcdb.Database, t *testing.T) {
	const n = 8
	var pending sync.WaitGroup

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			err := db.Put([]byte(key), []byte("v"+key))
			if err != nil {
				panic("put failed: " + err.Error())
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			data, err := db.Get([]byte(key))
			if err != nil {
				panic("get failed: " + err.Error())
			}
			if !bytes.Equal(data, []byte("v"+key)) {
				panic(fmt.Sprintf("get failed, got %q expected %q", []byte(data), []byte("v"+key)))
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			err := db.Delete([]byte(key))
			if err != nil {
				panic("delete failed: " + err.Error())
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()

	pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			_, err := db.Get([]byte(key))
			if err == nil {
				panic("get succeeded")
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()
}

func TestNewDBWithClean(t *testing.T) {
	dirname, err := ioutil.TempDir(os.TempDir(), "qkcdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	db, err := qkcdb.NewRDBDatabase(dirname, false, false)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}
	var (
		key1   = []byte("key1")
		value1 = []byte("value1")
	)

	err = db.Put(key1, value1)
	assert.NoError(t, err)
	getValue1, err := db.Get(key1)
	assert.NoError(t, err)
	assert.Equal(t, value1, getValue1)
	db.Close()

	//not clean
	dbNotClean, err := qkcdb.NewRDBDatabase(dirname, false, false)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	getValue1, err = dbNotClean.Get(key1)
	assert.NoError(t, err)
	assert.Equal(t, value1, getValue1)
	dbNotClean.Close()

	//clean
	dbClean, err := qkcdb.NewRDBDatabase(dirname, true, false)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	getValue1, err = dbClean.Get(key1)
	assert.Error(t, err)
	dbClean.Close()
	os.RemoveAll(dirname)
}
