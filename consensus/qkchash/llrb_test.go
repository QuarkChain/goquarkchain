// Copyright 2010 Petar Maymounkov. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package qkchash

import (
	"math/rand"
	"testing"
)

func TestCases(t *testing.T) {
	tree := NewLLRB()
	tree.ReplaceOrInsert(uint64(1))
	tree.ReplaceOrInsert(uint64(1))
	if tree.Len() != 1 {
		t.Errorf("expecting len 1")
	}
	if !tree.Has(uint64(1)) {
		t.Errorf("expecting to find key=1")
	}

	tree.Delete(uint64(1))
	if tree.Len() != 0 {
		t.Errorf("expecting len 0")
	}
	if tree.Has(uint64(1)) {
		t.Errorf("not expecting to find key=1")
	}

	tree.Delete(uint64(1))
	if tree.Len() != 0 {
		t.Errorf("expecting len 0")
	}
	if tree.Has(uint64(1)) {
		t.Errorf("not expecting to find key=1")
	}
}

func TestRandomReplace(t *testing.T) {
	tree := NewLLRB()
	n := 100
	perm := rand.Perm(n)
	for i := 0; i < n; i++ {
		tree.ReplaceOrInsert(uint64(perm[i]))
	}
	perm = rand.Perm(n)
	for i := 0; i < n; i++ {
		if replaced := tree.ReplaceOrInsert(uint64(perm[i])); replaced == Null_Value || replaced != uint64(perm[i]) {
			t.Errorf("error replacing")
		}
	}
}

func TestRandomInsertSequentialDelete(t *testing.T) {
	tree := NewLLRB()
	n := 1000
	perm := rand.Perm(n)
	for i := 0; i < n; i++ {
		tree.ReplaceOrInsert(uint64(perm[i]))
	}
	for i := 0; i < n; i++ {
		tree.Delete(uint64(i))
	}
}

func TestRandomInsertDeleteNonExistent(t *testing.T) {
	tree := NewLLRB()
	n := 100
	perm := rand.Perm(n)
	for i := 0; i < n; i++ {
		tree.ReplaceOrInsert(uint64(perm[i]))
	}
	if tree.Delete(uint64(200)) != Null_Value {
		t.Errorf("deleted non-existent item")
	}
	if tree.Delete(uint64(2000)) != Null_Value {
		t.Errorf("deleted non-existent item")
	}
	for i := 0; i < n; i++ {
		if u := tree.Delete(uint64(i)); u == Null_Value || u != uint64(i) {
			t.Errorf("delete failed")
		}
	}
	if tree.Delete(uint64(200)) != Null_Value {
		t.Errorf("deleted non-existent item")
	}
	if tree.Delete(uint64(2000)) != Null_Value {
		t.Errorf("deleted non-existent item")
	}
}

func BenchmarkInsert(b *testing.B) {
	tree := NewLLRB()
	for i := 0; i < b.N; i++ {
		tree.ReplaceOrInsert(uint64(b.N - i))
	}
}

func BenchmarkDelete(b *testing.B) {
	b.StopTimer()
	tree := NewLLRB()
	for i := 0; i < b.N; i++ {
		tree.ReplaceOrInsert(uint64(b.N - i))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tree.Delete(uint64(i))
	}
}

func BenchmarkDeleteMin(b *testing.B) {
	b.StopTimer()
	tree := NewLLRB()
	for i := 0; i < b.N; i++ {
		tree.ReplaceOrInsert(uint64(b.N - i))
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tree.DeleteMin()
	}
}
