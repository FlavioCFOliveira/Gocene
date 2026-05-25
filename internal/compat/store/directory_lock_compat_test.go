// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// directory_lock_compat_test.go addresses the audit row
// "Lock file payload" (NativeFSLockFactory) from docs/compat-coverage.tsv.
//
// Class structure (per Sprint 114 T6 guidance):
//
//	(a) isolated  : a Gocene-acquired write.lock obeys Lucene's documented
//	                contract — file is created, exclusive on the same path
//	                in-process, and is NOT deleted when the lock is closed
//	                (Lucene NEVER deletes the lock file; the OS releases the
//	                advisory lock when the fd closes).
//	(b) combined  : a write.lock acquired by Gocene does NOT confuse the
//	                Java harness's ability to read fixtures from the same
//	                directory (CodecUtil-framed payloads are independent of
//	                the lock).
//	(c) cross-engine: NOT FEASIBLE at this layer without adding a new Java
//	                scenario whose `verify` would attempt to acquire the
//	                same write.lock. The harness deliberately ships no such
//	                scenario per task spec; the NativeFSLockFactory
//	                contract is well-defined (POSIX OFD advisory locks)
//	                and is exercised cross-process by Lucene's own test
//	                suite. Adding a new Java scenario would expand the
//	                harness API beyond the T6 scope.
package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/internal/compat"
	gostore "github.com/FlavioCFOliveira/Gocene/store"
)

// TestDirectoryLock_IsolatedContract is class (a): the Gocene lock matches
// Lucene's documented contract for file naming, exclusivity and lifecycle.
func TestDirectoryLock_IsolatedContract(t *testing.T) {
	tmp := t.TempDir()
	d, err := gostore.NewNIOFSDirectory(tmp)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	lock, err := d.ObtainLock("write.lock")
	if err != nil {
		t.Fatalf("ObtainLock: %v", err)
	}

	// File-naming contract: the lock file MUST be created at the
	// directory root with the exact name passed to ObtainLock.
	lockPath := filepath.Join(tmp, "write.lock")
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file not created at %s: %v", lockPath, err)
	}

	// Exclusivity contract: a second ObtainLock for the same name from the
	// same process MUST fail (NativeFSLockFactory in-process registry).
	if _, err := d.ObtainLock("write.lock"); err == nil {
		t.Fatal("expected second ObtainLock to fail while first is held, got nil")
	}

	// Liveness contract.
	if !lock.IsLocked() {
		t.Fatal("IsLocked returned false on a freshly-obtained lock")
	}
	if err := lock.EnsureValid(); err != nil {
		t.Fatalf("EnsureValid on live lock: %v", err)
	}

	// Release contract.
	if err := lock.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if lock.IsLocked() {
		t.Fatal("IsLocked returned true after Close")
	}

	// Lucene contract: the lock file is NOT deleted on release; the
	// advisory lock is released by closing the fd. Asserting this matches
	// Lucene's documented behaviour exactly.
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lock file disappeared after Close, but Lucene's NativeFSLockFactory never deletes it: %v", err)
	}

	// After release, a new ObtainLock from the same process MUST succeed.
	d2, err := gostore.NewNIOFSDirectory(tmp)
	if err != nil {
		t.Fatalf("reopen dir: %v", err)
	}
	defer d2.Close()
	lock2, err := d2.ObtainLock("write.lock")
	if err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	if err := lock2.Close(); err != nil {
		t.Fatalf("close re-acquired lock: %v", err)
	}
}

// TestDirectoryLock_CoexistsWithLuceneFixtures is class (b): a Gocene
// write.lock in a directory does not interfere with the Java harness's
// ability to read CodecUtil-framed fixtures from the same directory.
func TestDirectoryLock_CoexistsWithLuceneFixtures(t *testing.T) {
	requireHarness(t)

	tmp := t.TempDir()
	// Generate a fixture into tmp.
	if err := compat.GenerateInto("store-primitives", 12648430, tmp); err != nil {
		t.Fatalf("harness gen: %v", err)
	}

	// Now have Gocene hold a write.lock in the same directory.
	d, err := gostore.NewNIOFSDirectory(tmp)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()
	lock, err := d.ObtainLock("write.lock")
	if err != nil {
		t.Fatalf("ObtainLock: %v", err)
	}
	defer lock.Close()

	// Lucene must still be able to read the fixture. This proves the
	// write.lock payload (its mere presence) doesn't perturb CodecUtil
	// framing — i.e. Lucene's directory scan tolerates the same lock
	// file Gocene produces.
	if err := compat.Verify("store-primitives", 12648430, tmp); err != nil {
		// If the harness's verify path picks up write.lock by mistake the
		// error message will mention it; surface that to make a future
		// regression easy to diagnose.
		if strings.Contains(err.Error(), "write.lock") {
			t.Fatalf("Lucene tripped on Gocene write.lock: %v", err)
		}
		t.Fatalf("Lucene verify: %v", err)
	}
}
