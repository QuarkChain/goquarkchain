package qkcdb

import (
	"github.com/ethereum/go-ethereum/ethdb"
	_ "github.com/ethereum/go-ethereum/ethdb"
)

// Putter wraps the database write operation supported by both batches and regular databases.
type Putter ethdb.Putter

// Deleter wraps the database delete operation supported by both batches and regular databases.
type Deleter ethdb.Deleter

// Database wraps all database operations. All methods are safe for concurrent use.
type Database ethdb.Database

// Batch is a write-only database that commits changes to its host database
// when Write is called. Batch cannot be used concurrently.
type Batch ethdb.Batch
