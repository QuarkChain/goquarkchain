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
)

func newTestLDB() (*qkcdb.LDBDatabase, func()) {
	dirname, err := ioutil.TempDir(os.TempDir(), "qkcdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	db, err := qkcdb.NewLDBDatabase(dirname, 0, 0)
	if err != nil {
		panic("failed to create test database: " + err.Error())
	}

	return db, func() {
		db.Close()
		os.RemoveAll(dirname)
	}
}

func testData() map[string]string {
	var test_values = make(map[string]string)
	test_values["1"] = "a"
	test_values["23"] = "12"
	test_values[""] = "a3#8"
	test_values["ac"] = "a3<9"

	return test_values
}

func TestLDB_PutGet(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	testPutGet(db, t)
}

func TestMemoryDB_PutGet(t *testing.T) {
	testPutGet(qkcdb.NewMemDatabase(), t)
}

func testPutGet(db qkcdb.Database, t *testing.T) {
	t.Parallel()

	test_values := testData()

	for k, v := range test_values {
		err := db.Put([]byte(k), []byte(v))
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
	}

	for k, v := range test_values {
		data, err := db.Get([]byte(k))
		// fmt.Println("db.Get: ", "key: ", k, "data: ", string(data))
		if err != nil || v != string(data) {
			t.Fatalf("get failed: %v", err)
		}
	}

	data, err := db.Get([]byte("non-exist-key"))
	if err == nil && len(data) > 0 {
		t.Fatalf("expect to return a not found error")
	}

	for k, v := range test_values {
		err := db.Put([]byte(k), []byte(v))
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
	}

	for k, v := range test_values {
		data, err := db.Get([]byte(k))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte(v)) {
			t.Fatalf("get returned wrong result, got %q expected %q", string(data), v)
		}
	}

	for _, v := range test_values {
		err := db.Put([]byte(v), []byte("?"))
		if err != nil {
			t.Fatalf("put override failed: %v", err)
		}
	}

	for _, v := range test_values {
		data, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte("?")) {
			t.Fatalf("get returned wrong result, got %q expected ?", string(data))
		}
	}

	for _, v := range test_values {
		orig, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		orig[0] = byte(0xff)
		data, err := db.Get([]byte(v))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if !bytes.Equal(data, []byte("?")) {
			t.Fatalf("get returned wrong result, got %q expected ?", string(data))
		}
	}

	for _, v := range test_values {
		err := db.Delete([]byte(v))
		if err != nil {
			t.Fatalf("delete %q failed: %v", v, err)
		}
	}

	for _, v := range test_values {
		data, err := db.Get([]byte(v))
		if err == nil && len(data) > 0 {
			t.Fatalf("got deleted value %q", v)
		}
	}
}

func Test_batch(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	batch := db.NewBatch()
	testBatchPutGet(db, batch, t)
}

func testBatchPutGet(db qkcdb.Database, batch qkcdb.Batch, t *testing.T) {
	t.Parallel()
	test_values := testData()

	for k, v := range test_values {
		batch.Put([]byte(k), []byte(v))
	}
	if batch.ValueSize() != len(test_values) {
		t.Fatalf("batch operation put error.")
	}

	batch.Reset()
	if batch.ValueSize() != 0 {
		t.Fatalf("clean rocksdb falied, %d", batch.ValueSize())
	}

	for k, v := range test_values {
		batch.Put([]byte(k), []byte(v))
	}
	batch.Write()

	for k, v := range test_values {
		data, err := db.Get([]byte(k))
		if err != nil {
			t.Fatalf("After batch write, db get error: %v", err)
		}
		if string(data) != v {
			t.Fatalf("After batch write, not exist, real : %v,return : %v", v, string(data))
		}
	}
}

func TestLDB_ParallelPutGet(t *testing.T) {
	db, remove := newTestLDB()
	defer remove()
	testParallelPutGet(db, t)
}

func TestMemoryDB_ParallelPutGet(t *testing.T) {
	testParallelPutGet(qkcdb.NewMemDatabase(), t)
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

	/*pending.Add(n)
	for i := 0; i < n; i++ {
		go func(key string) {
			defer pending.Done()
			_, err := db.Get([]byte(key))
			if err == nil {
				panic("get succeeded")
			}
		}(strconv.Itoa(i))
	}
	pending.Wait()*/
}
