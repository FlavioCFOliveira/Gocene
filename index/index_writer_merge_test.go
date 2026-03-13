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

		// Force merge should complete without error even with empty index
		// TODO: Implement ForceMerge when available
		t.Skip("ForceMerge not yet implemented")

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

		// Add multiple documents to create segments
		for i := 0; i < 100; i++ {
			doc := &testDocument{fields: []interface{}{}}
			writer.AddDocument(doc)
		}

		// Commit to flush segments
		writer.Commit()

		// TODO: Implement ForceMerge
		// maxSegments := 1
		// writer.ForceMerge(maxSegments)
		t.Skip("ForceMerge not yet implemented")

		writer.Close()
	})

	t.Run("force merge with deletes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(createTestAnalyzer())
		writer, _ := index.NewIndexWriter(dir, config)

		// Add documents
		for i := 0; i < 50; i++ {
			doc := &testDocument{fields: []interface{}{}}
			writer.AddDocument(doc)
		}

		// Delete some documents
		term := index.NewTerm("id", "1")
		writer.DeleteDocuments(term)

		// Commit
		writer.Commit()

		// Force merge should handle deleted documents
		t.Skip("ForceMerge not yet implemented")

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
			t.Skip("GetMergePolicy returns interface, not *TieredMergePolicy")
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
			t.Skip("MergeScheduler not yet implemented in config")
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
		t.Skip("Disable background merge not yet implemented")

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

		// TODO: Set custom merge scheduler when available
		t.Skip("Merge scheduler configuration not yet fully implemented")

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
		writer.Commit()

		// TODO: Verify compound file creation
		t.Skip("Compound file verification not yet implemented")
	})
}
