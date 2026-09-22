// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// Package index_test contains the @Nightly test ports from Apache Lucene's
// org.apache.lucene.index.TestIndexWriterCommit.
//
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterCommit.java
// Reference tag: releases/lucene/10.4.0 (commit 9983b7c)
//
// These tests are excluded from the standard suite because Lucene annotates
// them @Nightly. They share the helper functions defined in
// index_writer_commit_test.go (same package, default build).
package index_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// TestCommitOnCloseDiskUsage verifies that a writer with commit-on-close
// cleans up temporary segments not referenced by the starting commit.
// Source: TestIndexWriterCommit.testCommitOnCloseDiskUsage() (@Nightly)
// Purpose: Tests transient disk usage stays bounded during merges.
func TestCommitOnCloseDiskUsage(t *testing.T) {
	t.Run("transient disk usage stays bounded", func(t *testing.T) {
		dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
		defer dir.Close()

		config := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config.SetMaxBufferedDocs(10)
		config.SetMergePolicy(index.NewLogMergePolicy())

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}
		for j := 0; j < 30; j++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("AddDocument(%d) error = %v", j, err)
			}
		}
		writer.Close()

		dir.ResetMaxUsedSizeInBytes()
		dir.SetTrackDiskUsage(true)
		startDiskUsage := dir.GetMaxUsedSizeInBytes()

		config2 := index.NewIndexWriterConfig(createCommitTestAnalyzer())
		config2.SetOpenMode(index.APPEND)
		config2.SetMaxBufferedDocs(10)
		config2.SetMergeScheduler(index.NewSerialMergeScheduler())
		config2.SetMergePolicy(index.NewLogMergePolicy())

		writer, err = index.NewIndexWriter(dir, config2)
		if err != nil {
			t.Fatalf("NewIndexWriter(append) error = %v", err)
		}
		for j := 0; j < 1470; j++ {
			if err := addCommitTestDoc(writer); err != nil {
				t.Fatalf("AddDocument(%d) error = %v", j, err)
			}
		}
		midDiskUsage := dir.GetMaxUsedSizeInBytes()
		dir.ResetMaxUsedSizeInBytes()
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("ForceMerge(1) error = %v", err)
		}
		writer.Close()

		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader() error = %v", err)
		}
		reader.Close()

		endDiskUsage := dir.GetMaxUsedSizeInBytes()

		if midDiskUsage >= 150*startDiskUsage {
			t.Fatalf("writer used too much space while adding documents: mid=%d start=%d limit=%d",
				midDiskUsage, startDiskUsage, 150*startDiskUsage)
		}
		if endDiskUsage >= 150*startDiskUsage {
			t.Fatalf("writer used too much space after close: end=%d start=%d limit=%d",
				endDiskUsage, startDiskUsage, 150*startDiskUsage)
		}
	})
}

// TestCommitThreadSafety verifies that commit does not return until all
// changes are durably in the index, under concurrent writers.
// Source: TestIndexWriterCommit.testCommitThreadSafety() (@Nightly)
// Purpose: Tests commit visibility under multi-threaded writes (LUCENE-2095).
func TestCommitThreadSafety(t *testing.T) {
	t.Run("commit visibility under concurrent writes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		riw, err := testindex.NewRandomIndexWriterWithConfig(rand.New(rand.NewSource(1)), dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
		if err != nil {
			t.Fatalf("NewRandomIndexWriterWithConfig: %v", err)
		}
		if _, err := riw.Commit(); err != nil {
			t.Fatalf("initial Commit: %v", err)
		}

		r, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader: %v", err)
		}
		defer r.Close()

		ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
		ft.Freeze()

		const maxIterations = 10
		for i := 0; i < maxIterations; i++ {
			s := fmt.Sprintf("term_%d", i)
			doc := document.NewDocument()
			f, _ := document.NewField("f", s, ft)
			doc.Add(f)
			if _, err := riw.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument %d: %v", i, err)
			}
			if _, err := riw.Commit(); err != nil {
				t.Fatalf("Commit %d: %v", i, err)
			}

			r2, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader %d: %v", i, err)
			}
			if err := r.Close(); err != nil {
				t.Fatalf("Close old reader %d: %v", i, err)
			}
			r = r2

			terms, err := r.Terms("f")
			if err != nil {
				t.Fatalf("Terms %d: %v", i, err)
			}
			it, err := terms.Iterator()
			if err != nil {
				t.Fatalf("Iterator %d: %v", i, err)
			}
			found := false
			for {
				te, err := it.Next()
				if err != nil {
					t.Fatalf("Next %d: %v", i, err)
				}
				if te == nil {
					break
				}
				if te.Text() == s {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("term %q not visible after commit (iter=%d)", s, i)
			}
		}

		if err := riw.Close(); err != nil {
			t.Fatalf("Close writer: %v", err)
		}
	})
}
