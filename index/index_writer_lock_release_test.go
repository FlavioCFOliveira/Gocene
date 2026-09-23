// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterLockRelease.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// isFileNotFoundOrNoSuchFile renders catching FileNotFoundException |
// NoSuchFileException.
func isFileNotFoundOrNoSuchFile(err error) bool {
	return errors.Is(err, store.ErrFileNotFound) || errors.Is(err, fs.ErrNotExist)
}

// This tests the patch for issue #LUCENE-715 (IndexWriter does not release
// its write lock when trying to open an index which does not yet exist).
func TestIndexWriterLockReleaseIndexWriterLockRelease(t *testing.T) {
	dir := newFSDirectory(t)
	defer mustClose(t, dir)

	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(index.Append)
	w, err := index.NewIndexWriter(dir, conf)
	if err == nil {
		// Java leaves the writer open here; its write lock then fails
		// dir.close(). The Go rendering reports it directly.
		mustClose(t, w)
		t.Fatal("new IndexWriter(APPEND) on a missing index did not throw FileNotFoundException/NoSuchFileException")
	}
	if !isFileNotFoundOrNoSuchFile(err) {
		t.Fatalf("new IndexWriter(APPEND): %v", err)
	}

	conf2 := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf2.SetOpenMode(index.Append)
	w, err = index.NewIndexWriter(dir, conf2)
	if err == nil {
		mustClose(t, w)
		t.Fatal("second new IndexWriter(APPEND) on a missing index did not throw FileNotFoundException/NoSuchFileException")
	}
	if !isFileNotFoundOrNoSuchFile(err) {
		t.Fatalf("second new IndexWriter(APPEND): %v", err)
	}
}
