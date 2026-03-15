// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter force merge operations.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterForceMerge
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterForceMerge.java
//
// GC-189: Port TestIndexWriterForceMerge from Apache Lucene to Go
// Focus: Force merge to single segment, max segments, concurrent writes
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriterForceMerge_PartialMerge tests partial merging to a specified number of segments.
// Ported from: TestIndexWriterForceMerge.testPartialMerge()
// Purpose: Verifies that forceMerge(maxSegments) correctly reduces segment count to at most maxSegments
func TestIndexWriterForceMerge_PartialMerge(t *testing.T) {
	t.Run("partial merge to 3 segments", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create a document with a single field
		doc := document.NewDocument()
		field, err := document.NewStringField("content", "aaa", false)
		if err != nil {
			t.Fatalf("Failed to create StringField: %v", err)
		}
		doc.Add(field)

		// Test with increasing number of documents
		// Using smaller increments for faster tests (vs Java's 15/40 min and 5*incr max)
		incrMin := 10
		for numDocs := 10; numDocs < 200; numDocs += incrMin * 3 {
			// Create writer with merge policy
			config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			config.SetOpenMode(index.CREATE)
			config.SetMaxBufferedDocs(2)

			// Use TieredMergePolicy since LogDocMergePolicy is not yet implemented
			policy := index.NewTieredMergePolicy()
			config.SetMergePolicy(policy)

			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("NewIndexWriter() error = %v", err)
			}

			// Add documents
			for j := 0; j < numDocs; j++ {
				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument() error = %v", err)
				}
			}
			writer.Close()

			// Read segment count
			sis, err := index.ReadSegmentInfos(dir)
			if err != nil {
				t.Fatalf("ReadSegmentInfos() error = %v", err)
			}
			segCount := sis.Size()

			// Reopen writer and force merge to 3 segments
			config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			config2.SetMergePolicy(index.NewTieredMergePolicy())
			writer, err = index.NewIndexWriter(dir, config2)
			if err != nil {
				t.Fatalf("NewIndexWriter() error = %v", err)
			}

			// TODO: Implement ForceMerge when available
			// writer.ForceMerge(3)
			t.Skip("ForceMerge not yet implemented")

			writer.Close()

			// Verify segment count
			sis, err = index.ReadSegmentInfos(dir)
			if err != nil {
				t.Fatalf("ReadSegmentInfos() error = %v", err)
			}
			optSegCount := sis.Size()

			if segCount < 3 {
				if optSegCount != segCount {
					t.Errorf("Expected %d segments, got %d", segCount, optSegCount)
				}
			} else {
				if optSegCount != 3 {
					t.Errorf("Expected 3 segments, got %d", optSegCount)
				}
			}
		}
	})
}

// TestIndexWriterForceMerge_MaxNumSegments2 tests max num segments with concurrent scheduler.
// Ported from: TestIndexWriterForceMerge.testMaxNumSegments2()
// Purpose: Verifies forceMerge works correctly with ConcurrentMergeScheduler
func TestIndexWriterForceMerge_MaxNumSegments2(t *testing.T) {
	t.Run("max num segments with concurrent scheduler", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create document
		doc := document.NewDocument()
		field, err := document.NewStringField("content", "aaa", false)
		if err != nil {
			t.Fatalf("Failed to create StringField: %v", err)
		}
		doc.Add(field)

		// Create writer with concurrent merge scheduler
		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		config.SetMaxBufferedDocs(2)
		config.SetMergePolicy(index.NewTieredMergePolicy())
		config.SetMergeScheduler(index.NewConcurrentMergeScheduler())

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// Run multiple iterations
		for iter := 0; iter < 10; iter++ {
			// Add 19 documents per iteration
			for i := 0; i < 19; i++ {
				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument() error = %v", err)
				}
			}

			// Commit and wait for merges
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit() error = %v", err)
			}

			// TODO: Implement WaitForMerges when available
			// writer.WaitForMerges()

			// Get segment count before force merge
			sis, err := index.ReadSegmentInfos(dir)
			if err != nil {
				t.Fatalf("ReadSegmentInfos() error = %v", err)
			}
			segCount := sis.Size()

			// Force merge to 7 segments
			// TODO: Implement ForceMerge when available
			// writer.ForceMerge(7)
			t.Skip("ForceMerge not yet implemented")

			// Commit and wait
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit() error = %v", err)
			}
			// TODO: writer.WaitForMerges()

			// Verify segment count
			sis, err = index.ReadSegmentInfos(dir)
			if err != nil {
				t.Fatalf("ReadSegmentInfos() error = %v", err)
			}
			optSegCount := sis.Size()

			if segCount < 7 {
				if optSegCount != segCount {
					t.Errorf("Iteration %d: Expected %d segments, got %d", iter, segCount, optSegCount)
				}
			} else {
				if optSegCount != 7 {
					t.Errorf("Iteration %d: Expected 7 segments, got %d", iter, optSegCount)
				}
			}
		}

		writer.Close()
	})
}

// TestIndexWriterForceMerge_TempSpaceUsage tests temporary space usage during force merge.
// Ported from: TestIndexWriterForceMerge.testForceMergeTempSpaceUsage()
// Purpose: Verifies forceMerge doesn't use more than 4X the starting index size as temporary space
func TestIndexWriterForceMerge_TempSpaceUsage(t *testing.T) {
	t.Run("force merge temp space usage", func(t *testing.T) {
		// Use a tracking directory wrapper to monitor disk usage
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Create writer with specific settings
		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		config.SetMaxBufferedDocs(10)
		config.SetMergePolicy(index.NewTieredMergePolicy())

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// Add 500 documents
		for j := 0; j < 500; j++ {
			doc := document.NewDocument()
			field, _ := document.NewStringField("index", string(rune('a'+j%26)), false)
			doc.Add(field)
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument() error = %v", err)
			}
		}

		// Force one extra segment with different content
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		doc := document.NewDocument()
		field, _ := document.NewStringField("index", "extra", false)
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument() error = %v", err)
		}

		writer.Close()

		// Calculate starting disk usage
		// TODO: Implement file length tracking when available
		// startDiskUsage := calculateDiskUsage(dir)

		// Reopen writer and force merge to 1 segment
		config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		config2.SetOpenMode(index.APPEND)
		config2.SetMergePolicy(index.NewTieredMergePolicy())

		writer, err = index.NewIndexWriter(dir, config2)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// TODO: Implement ForceMerge when available
		// writer.ForceMerge(1)
		t.Skip("ForceMerge not yet implemented")

		writer.Close()

		// Calculate final disk usage
		// finalDiskUsage := calculateDiskUsage(dir)

		// Verify temp space usage is within limits (max 4X starting or final size)
		// maxDiskUsage := dir.GetMaxUsedSizeInBytes()
		// maxStartFinal := max(startDiskUsage, finalDiskUsage)
		// if maxDiskUsage > 4 * maxStartFinal {
		//     t.Errorf("forceMerge used too much temporary space: %d bytes (max allowed: %d)",
		//         maxDiskUsage, 4*maxStartFinal)
		// }
	})
}

// TestIndexWriterForceMerge_BackgroundForceMerge tests background force merge behavior.
// Ported from: TestIndexWriterForceMerge.testBackgroundForceMerge()
// Purpose: Verifies forceMerge(1, false) kicks off merge but doesn't wait, and writer.close() waits
func TestIndexWriterForceMerge_BackgroundForceMerge(t *testing.T) {
	t.Run("background force merge", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		// Test two passes
		for pass := 0; pass < 2; pass++ {
			config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
			config.SetOpenMode(index.CREATE)
			config.SetMaxBufferedDocs(2)
			config.SetMergePolicy(index.NewTieredMergePolicy())

			writer, err := index.NewIndexWriter(dir, config)
			if err != nil {
				t.Fatalf("NewIndexWriter() error = %v", err)
			}

			// Create document
			doc := document.NewDocument()
			field, _ := document.NewStringField("field", "aaa", false)
			doc.Add(field)

			// Add 100 documents
			for i := 0; i < 100; i++ {
				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument() error = %v", err)
				}
			}

			// Force merge to 1 segment without waiting (background)
			// TODO: Implement ForceMerge with doWait parameter when available
			// writer.ForceMerge(1, false)
			t.Skip("ForceMerge with doWait parameter not yet implemented")

			if pass == 0 {
				// Close writer - should wait for merges
				writer.Close()

				// Verify single segment
				reader, err := index.OpenDirectoryReader(dir)
				if err != nil {
					t.Fatalf("OpenDirectoryReader() error = %v", err)
				}

				leaves, err := reader.Leaves()
				if err != nil {
					t.Fatalf("Leaves() error = %v", err)
				}
				if len(leaves) != 1 {
					t.Errorf("Expected 1 segment, got %d", len(leaves))
				}
				reader.Close()
			} else {
				// Add more documents before closing
				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument() error = %v", err)
				}
				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("AddDocument() error = %v", err)
				}
				writer.Close()

				// Verify more than 1 segment
				reader, err := index.OpenDirectoryReader(dir)
				if err != nil {
					t.Fatalf("OpenDirectoryReader() error = %v", err)
				}

				leaves, err := reader.Leaves()
				if err != nil {
					t.Fatalf("Leaves() error = %v", err)
				}
				if len(leaves) <= 1 {
					t.Errorf("Expected more than 1 segment, got %d", len(leaves))
				}
				reader.Close()

				// Verify segment infos count
				sis, err := index.ReadSegmentInfos(dir)
				if err != nil {
					t.Fatalf("ReadSegmentInfos() error = %v", err)
				}
				if sis.Size() != 2 {
					t.Errorf("Expected 2 segments in SegmentInfos, got %d", sis.Size())
				}
			}
		}
	})
}

// TestIndexWriterForceMerge_SingleSegment tests force merge to a single segment.
// Additional test case for basic force merge functionality.
func TestIndexWriterForceMerge_SingleSegment(t *testing.T) {
	t.Run("force merge to single segment", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		config.SetMaxBufferedDocs(2)
		config.SetMergePolicy(index.NewTieredMergePolicy())

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// Add multiple documents to create multiple segments
		for i := 0; i < 50; i++ {
			doc := document.NewDocument()
			field, _ := document.NewStringField("id", string(rune('a'+i%26)), false)
			doc.Add(field)
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument() error = %v", err)
			}
		}

		// Commit to flush segments
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Get initial segment count
		sis, err := index.ReadSegmentInfos(dir)
		if err != nil {
			t.Fatalf("ReadSegmentInfos() error = %v", err)
		}
		initialSegCount := sis.Size()
		t.Logf("Initial segment count: %d", initialSegCount)

		// Force merge to 1 segment
		// TODO: Implement ForceMerge when available
		// if err := writer.ForceMerge(1); err != nil {
		//     t.Fatalf("ForceMerge() error = %v", err)
		// }
		t.Skip("ForceMerge not yet implemented")

		// Verify single segment
		sis, err = index.ReadSegmentInfos(dir)
		if err != nil {
			t.Fatalf("ReadSegmentInfos() error = %v", err)
		}
		if sis.Size() != 1 {
			t.Errorf("Expected 1 segment after force merge, got %d", sis.Size())
		}

		writer.Close()
	})
}

// TestIndexWriterForceMerge_WithDeletes tests force merge with deleted documents.
// Additional test case for force merge with deletes.
func TestIndexWriterForceMerge_WithDeletes(t *testing.T) {
	t.Run("force merge with deletes", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		config.SetMaxBufferedDocs(2)
		config.SetMergePolicy(index.NewTieredMergePolicy())

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// Add documents
		for i := 0; i < 50; i++ {
			doc := document.NewDocument()
			field, _ := document.NewStringField("id", string(rune('a'+i%26)), false)
			doc.Add(field)
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument() error = %v", err)
			}
		}

		// Delete some documents
		term := index.NewTerm("id", "a")
		if err := writer.DeleteDocuments(term); err != nil {
			t.Fatalf("DeleteDocuments() error = %v", err)
		}

		// Commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// Force merge
		// TODO: Implement ForceMerge when available
		// if err := writer.ForceMerge(1); err != nil {
		//     t.Fatalf("ForceMerge() error = %v", err)
		// }
		t.Skip("ForceMerge not yet implemented")

		// Verify document count after merge
		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader() error = %v", err)
		}
		defer reader.Close()

		// Should have 49 documents (50 - 1 deleted)
		if reader.NumDocs() != 49 {
			t.Errorf("Expected 49 documents after merge, got %d", reader.NumDocs())
		}

		writer.Close()
	})
}

// TestIndexWriterForceMerge_EmptyIndex tests force merge on empty index.
// Edge case: Force merge should complete without error on empty index.
func TestIndexWriterForceMerge_EmptyIndex(t *testing.T) {
	t.Run("force merge empty index", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// Force merge on empty index
		// TODO: Implement ForceMerge when available
		// if err := writer.ForceMerge(1); err != nil {
		//     t.Fatalf("ForceMerge() error = %v", err)
		// }
		t.Skip("ForceMerge not yet implemented")

		writer.Close()
	})
}

// TestIndexWriterForceMerge_AlreadyOptimized tests force merge on already optimized index.
// Edge case: Force merge should be a no-op on already optimized index.
func TestIndexWriterForceMerge_AlreadyOptimized(t *testing.T) {
	t.Run("force merge already optimized index", func(t *testing.T) {
		dir := store.NewByteBuffersDirectory()
		defer dir.Close()

		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		config.SetMaxBufferedDocs(2)
		config.SetMergePolicy(index.NewTieredMergePolicy())

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}

		// Add documents
		for i := 0; i < 10; i++ {
			doc := document.NewDocument()
			field, _ := document.NewStringField("id", string(rune('a'+i)), false)
			doc.Add(field)
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument() error = %v", err)
			}
		}

		// Commit
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}

		// First force merge to 1 segment
		// TODO: Implement ForceMerge when available
		// writer.ForceMerge(1)
		t.Skip("ForceMerge not yet implemented")

		// Get segment count
		sis, err := index.ReadSegmentInfos(dir)
		if err != nil {
			t.Fatalf("ReadSegmentInfos() error = %v", err)
		}
		segCountAfterFirst := sis.Size()

		// Second force merge should be no-op
		// writer.ForceMerge(1)

		sis, err = index.ReadSegmentInfos(dir)
		if err != nil {
			t.Fatalf("ReadSegmentInfos() error = %v", err)
		}
		segCountAfterSecond := sis.Size()

		if segCountAfterFirst != segCountAfterSecond {
			t.Errorf("Second force merge changed segment count from %d to %d",
				segCountAfterFirst, segCountAfterSecond)
		}

		writer.Close()
	})
}
