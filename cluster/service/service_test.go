// Modified from go-ethereum under GNU Lesser General Public License
package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// Tests that databases are correctly created persistent or ephemeral based on
// the configured service context.
func TestContextDatabases(t *testing.T) {
	// Create a temporary folder and ensure no database is contained within
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("failed to create temporary data directory: %v", err)
	}
	defer os.RemoveAll(dir)

	if _, err := os.Stat(filepath.Join(dir, "database")); err == nil {
		t.Fatalf("non-created database already exists")
	}
	// Request the opening/creation of a database and ensure it persists to disk
	ctx := &ServiceContext{config: &Config{Name: "unit-test", DataDir: dir}}
	db, err := ctx.OpenDatabase("persistent", false, false)
	if err != nil {
		t.Fatalf("failed to open persistent database: %v", err)
	}
	db.Close()

	if _, err := os.Stat(filepath.Join(dir, "unit-test", "persistent")); err != nil {
		t.Fatalf("persistent database doesn't exists: %v", err)
	}
	// Request th opening/creation of an ephemeral database and ensure it's not persisted
	ctx = &ServiceContext{config: &Config{DataDir: ""}}
	db, err = ctx.OpenDatabase("ephemeral", false, false)
	if err != nil {
		t.Fatalf("failed to open ephemeral database: %v", err)
	}
	db.Close()

	if _, err := os.Stat(filepath.Join(dir, "ephemeral")); err == nil {
		t.Fatalf("ephemeral database exists")
	}
}

// Tests that already constructed services can be retrieves by later ones.
func TestContextServices(t *testing.T) {
	stack, err := New(testNodeConfig())
	if err != nil {
		t.Fatalf("failed to create protocol stack: %v", err)
	}
	// Define a verifier that ensures a NoopA is before it and NoopB after
	verifier := func(ctx *ServiceContext) (Service, error) {
		var objA *NoopServiceA
		if ctx.Service(&objA) != nil {
			return nil, fmt.Errorf("former service not found")
		}
		var objB *NoopServiceB
		if err := ctx.Service(&objB); err != ErrServiceUnknown {
			return nil, fmt.Errorf("latters lookup error mismatch: have %v, want %v", err, ErrServiceUnknown)
		}
		return new(NoopService), nil
	}
	// Register the collection of services
	if err := stack.Register(NewNoopServiceA); err != nil {
		t.Fatalf("former failed to register service: %v", err)
	}
	if err := stack.Register(verifier); err != nil {
		t.Fatalf("failed to register service verifier: %v", err)
	}
	if err := stack.Register(NewNoopServiceB); err != nil {
		t.Fatalf("latter failed to register service: %v", err)
	}
	// Start the protocol stack and ensure services are constructed in order
	if err := stack.Start(); err != nil {
		t.Fatalf("failed to start stack: %v", err)
	}
	defer stack.Stop()
}
