// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter merge operations.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriter
// Merge-related test methods:
//   - testForceMerge
//   - testMerging
//   - testMergeAfterCommit
//
// GC-116: Index Tests - IndexWriter Merging
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriterForceMerge tests force merge operations.
// Ported from: TestIndexWriter.testForceMerge()
func TestIndexWriterForceMerge(t *testing.T) {
	t.Run("force merge with empty index", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// Force merge completes without error even on an empty index.
		if err := writer.ForceMerge(1); err != nil {
			t.Errorf("ForceMerge() error = %v", err)
		}

		err = writer.Close()
		if err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	t.Run("force merge reduces segment count", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		// Add documents in three commits to create three segments.
		for b := 0; b < 3; b++ {
			for i := 0; i < 5; i++ {
				if err := writer.AddDocument(&testDocument{fields: []interface{}{}}); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
			}
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
		}
		if c := writer.GetSegmentCount(); c < 2 {
			t.Fatalf("expected multiple segments before merge, got %d", c)
		}

		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("ForceMerge() error = %v", err)
		}
		if c := writer.GetSegmentCount(); c != 1 {
			t.Errorf("segment count after ForceMerge = %d, want 1", c)
		}

		writer.Close()
	})

	t.Run("force merge with deletes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		// Two segments of empty documents.
		for b := 0; b < 2; b++ {
			for i := 0; i < 5; i++ {
				if err := writer.AddDocument(&testDocument{fields: []interface{}{}}); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
			}
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
		}

		// A delete-by-term (no-match on field-less docs) followed by ForceMerge
		// must still merge cleanly to a single segment. Exact delete compaction
		// is covered by TestForceMerge_CompactsDeletes.
		if err := writer.DeleteDocuments(index.NewTerm("id", "1")); err != nil {
			t.Fatalf("DeleteDocuments: %v", err)
		}
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("ForceMerge() error = %v", err)
		}
		if c := writer.GetSegmentCount(); c != 1 {
			t.Errorf("segment count after ForceMerge = %d, want 1", c)
		}

		writer.Close()
	})
}

// TestIndexWriterMergePolicy tests merge policy integration.
func TestIndexWriterMergePolicy(t *testing.T) {
	t.Run("writer with tiered merge policy", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		policy := index.NewTieredMergePolicy()
		config.SetMergePolicy(policy)

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}
		defer writer.Close()

		// Verify policy was set
		if config.GetMergePolicy() == nil {
			t.Error("MergePolicy should be set in config")
		}
	})

	t.Run("writer with custom merge policy settings", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		policy := index.NewTieredMergePolicy()
		policy.SetMaxMergeAtOnce(5)
		policy.SetMaxMergedSegmentMB(1024)
		config.SetMergePolicy(policy)

		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Verify settings
		setPolicy := config.GetMergePolicy()
		if setPolicy == nil {
			t.Fatal("MergePolicy should be set")
		}

		tieredPolicy, ok := setPolicy.(*index.TieredMergePolicy)
		if !ok {
			t.Fatal("GetMergePolicy returns interface, not *TieredMergePolicy")
		}

		if tieredPolicy.GetMaxMergeAtOnce() != 5 {
			t.Errorf("GetMaxMergeAtOnce() = %d, want 5", tieredPolicy.GetMaxMergeAtOnce())
		}
		if tieredPolicy.GetMaxMergedSegmentMB() != 1024 {
			t.Errorf("GetMaxMergedSegmentMB() = %f, want 1024", tieredPolicy.GetMaxMergedSegmentMB())
		}
	})
}

// TestIndexWriterBackgroundMerge tests background merge behavior.
func TestIndexWriterBackgroundMerge(t *testing.T) {
	t.Run("background merge enabled by default", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())

		// Check that merge scheduler is set
		if config.GetMergeScheduler() == nil {
			t.Fatal("MergeScheduler not yet implemented in config")
		}

		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		t.Log("Background merge is enabled by default")
	})

	t.Run("disable background merge", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())

		// TODO: Disable background merge when API available
		t.Fatal("Disable background merge not yet implemented")

		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()
	})
}

// TestIndexWriterMergeAfterCommit tests merging behavior after commit.
func TestIndexWriterMergeAfterCommit(t *testing.T) {
	t.Run("merge after commit", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		// Add documents
		for i := 0; i < 50; i++ {
			doc := &testDocument{fields: []interface{}{}}
			writer.AddDocument(doc)
		}

		// Commit
		if err := writer.Commit(); err != nil {
			t.Errorf("Commit() error = %v", err)
		}

		// After commit, merges may be triggered
		t.Log("Commit completed, segments may be merged")

		writer.Close()
	})
}

// TestIndexWriterMergeScheduling tests merge scheduling.
func TestIndexWriterMergeScheduling(t *testing.T) {
	t.Run("merge scheduler configuration", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		config.SetMergeScheduler(index.NewSerialMergeScheduler())

		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()
	})
}

// TestIndexWriterCompoundFiles tests compound file handling during merge.
func TestIndexWriterCompoundFiles(t *testing.T) {
	t.Run("compound file creation on merge", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		policy := index.NewTieredMergePolicy()
		policy.SetNoCFSRatio(0.0) // Always use compound files
		config.SetMergePolicy(policy)

		writer, _ := index.NewIndexWriter(dir, config)
		defer writer.Close()

		// Add documents
		for i := 0; i < 20; i++ {
			doc := &testDocument{fields: []interface{}{}}
			writer.AddDocument(doc)
		}

		// Commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}

		// With NoCFSRatio=0 every segment should be packed into a compound file.
		files, err := dir.ListAll()
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}
		var cfsCount int
		for _, f := range files {
			if index.GetExtension(f) == "cfs" {
				cfsCount++
			}
		}
		if cfsCount == 0 {
			t.Fatal("expected at least one .cfs compound file after commit")
		}
	})
}
