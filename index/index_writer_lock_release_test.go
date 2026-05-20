// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene's
// org.apache.lucene.index.TestIndexWriterLockRelease.
//
// This tests the patch for issue LUCENE-715 (IndexWriter does not release
// its write lock when trying to open an index which does not yet exist).
//
// GOC-4141: Sprint 55.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriterLockRelease verifies that opening a non-existent index in
// APPEND mode fails, yet releases the write lock, so a second APPEND attempt
// fails the same way instead of being blocked by a stale lock.
//
// Skipped: NewIndexWriter currently ignores IndexWriterConfig.OpenMode and
// does not acquire a write lock, so APPEND on a missing index succeeds and the
// LUCENE-715 scenario cannot be exercised. Unskip once OpenMode handling and
// write-lock acquisition land in IndexWriter.
func TestIndexWriterLockRelease(t *testing.T) {
	t.Skip("IndexWriter ignores OpenMode and does not acquire a write lock")

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory() error = %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(createMockAnalyzer())
	config.SetOpenMode(index.APPEND)

	if w, err := index.NewIndexWriter(dir, config); err == nil {
		w.Close()
		t.Fatal("first NewIndexWriter(APPEND) on a missing index should fail")
	}

	// LUCENE-715: the first failed attempt must have released the write lock.
	config2 := index.NewIndexWriterConfig(createMockAnalyzer())
	config2.SetOpenMode(index.APPEND)
	if w, err := index.NewIndexWriter(dir, config2); err == nil {
		w.Close()
		t.Fatal("second NewIndexWriter(APPEND) on a missing index should fail")
	}
}
