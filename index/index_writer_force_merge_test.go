// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for IndexWriter force merge operations.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexWriterForceMerge
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexWriterForceMerge.java
//
// GOC-4145: Port test org.apache.lucene.index.TestIndexWriterForceMerge (Sprint 55).
//
// Port strategy (Sprint 55 option c): each Java @Test has a 1:1 Go counterpart.
// The shared writer/document/commit roundtrip runs for real wherever the Gocene
// API supports it; assertions that depend on still-missing behavior are gated
// with t.Skip so the divergence is explicit rather than silently absent.
//
// Known API gaps that force a skip in this file:
//   - There is no background ForceMerge(maxNumSegments, doWait) overload.
//   - LogMergePolicy has no SetMinMergeDocs setter.
//   - PerField postings/doc-values formats with a merge barrier are not available
//     (the Java testMergePerField is itself @AwaitsFix upstream).
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newForceMergeDoc builds the single-field document reused by most cases,
// mirroring the `doc` local that the Java test shares across iterations.
func newForceMergeDoc(t *testing.T, field, value string) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	f, err := document.NewStringField(field, value, false)
	if err != nil {
		t.Fatalf("NewStringField(%q, %q) error = %v", field, value, err)
	}
	doc.Add(f)
	return doc
}

// TestIndexWriterForceMerge_PartialMerge ports testPartialMerge().
//
// Verifies that forceMerge(3) reduces the segment count to at most 3.
func TestIndexWriterForceMerge_PartialMerge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doc := newForceMergeDoc(t, "content", "aaa")

	// Java grows numDocs by a random increment; we use a fixed step for a
	// deterministic, fast test while still spanning several segment counts.
	const incrMin = 40
	for numDocs := 10; numDocs < 500; numDocs += incrMin {
		config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		config.SetOpenMode(index.CREATE)
		config.SetMaxBufferedDocs(2)
		ldmp := index.NewLogDocMergePolicy()
		ldmp.SetMergeFactor(5)
		config.SetMergePolicy(ldmp)

		writer, err := index.NewIndexWriter(dir, config)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}
		for j := 0; j < numDocs; j++ {
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument() error = %v", err)
			}
		}
		writer.Close()

		sis, err := index.ReadSegmentInfos(dir)
		if err != nil {
			t.Fatalf("ReadSegmentInfos() error = %v", err)
		}
		segCount := sis.Size()

		config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
		ldmp2 := index.NewLogDocMergePolicy()
		ldmp2.SetMergeFactor(5)
		config2.SetMergePolicy(ldmp2)
		writer, err = index.NewIndexWriter(dir, config2)
		if err != nil {
			t.Fatalf("NewIndexWriter() error = %v", err)
		}
		if err := writer.ForceMerge(3); err != nil {
			t.Fatalf("ForceMerge(3) error = %v", err)
		}
		writer.Close()

		sis, err = index.ReadSegmentInfos(dir)
		if err != nil {
			t.Fatalf("ReadSegmentInfos() error = %v", err)
		}
		optSegCount := sis.Size()

		if segCount < 3 {
			if optSegCount != segCount {
				t.Errorf("numDocs=%d: expected %d segments, got %d", numDocs, segCount, optSegCount)
			}
		} else if optSegCount > 3 {
			t.Errorf("numDocs=%d (segCount=%d): expected at most 3 segments, got %d", numDocs, segCount, optSegCount)
		}
	}
}

// TestIndexWriterForceMerge_MaxNumSegments2 ports testMaxNumSegments2().
//
// Drives forceMerge(7) under a ConcurrentMergeScheduler across 10 iterations.
func TestIndexWriterForceMerge_MaxNumSegments2(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	doc := newForceMergeDoc(t, "content", "aaa")

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxBufferedDocs(2)
	ldmp := index.NewLogDocMergePolicy()
	ldmp.SetMergeFactor(4)
	config.SetMergePolicy(ldmp)
	config.SetMergeScheduler(index.NewConcurrentMergeScheduler())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}

	for iter := 0; iter < 10; iter++ {
		for i := 0; i < 19; i++ {
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("iter %d: AddDocument() error = %v", iter, err)
			}
		}

		if err := writer.Commit(); err != nil {
			t.Fatalf("iter %d: Commit() error = %v", iter, err)
		}
		if err := writer.WaitForMerges(); err != nil {
			t.Fatalf("iter %d: WaitForMerges() error = %v", iter, err)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("iter %d: Commit() error = %v", iter, err)
		}

		sis, err := index.ReadSegmentInfos(dir)
		if err != nil {
			t.Fatalf("iter %d: ReadSegmentInfos() error = %v", iter, err)
		}
		segCount := sis.Size()

		if err := writer.ForceMerge(7); err != nil {
			t.Fatalf("iter %d: ForceMerge(7) error = %v", iter, err)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("iter %d: Commit() error = %v", iter, err)
		}
		if err := writer.WaitForMerges(); err != nil {
			t.Fatalf("iter %d: WaitForMerges() error = %v", iter, err)
		}

		sis, err = index.ReadSegmentInfos(dir)
		if err != nil {
			t.Fatalf("iter %d: ReadSegmentInfos() error = %v", iter, err)
		}
		optSegCount := sis.Size()

		if segCount < 7 {
			if optSegCount != segCount {
				t.Errorf("iter %d: expected %d segments, got %d", iter, segCount, optSegCount)
			}
		} else if optSegCount > 7 {
			t.Errorf("iter %d (seg: %d): expected at most 7 segments, got %d", iter, segCount, optSegCount)
		}
	}

	writer.Close()
}

// TestIndexWriterForceMerge_TempSpaceUsage ports testForceMergeTempSpaceUsage().
//
// Asserts that forceMerge(1) uses at most 4X the larger of the starting and
// final index size as temporary disk space. Uses MockDirectoryWrapper to
// measure peak usage.
func TestIndexWriterForceMerge_TempSpaceUsage(t *testing.T) {
	dir := store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxBufferedDocs(10)
	config.SetMergePolicy(index.NewLogMergePolicy())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	for j := 0; j < 500; j++ {
		if err := writer.AddDocument(newForceMergeDoc(t, "content", "aaa")); err != nil {
			t.Fatalf("AddDocument() error = %v", err)
		}
	}
	// Force one extra segment with a separate doc store.
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if err := writer.AddDocument(newForceMergeDoc(t, "content", "aaa")); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}
	writer.Close()

	startDiskUsage, err := dirSize(dir)
	if err != nil {
		t.Fatalf("dirSize() error = %v", err)
	}

	dir.ResetMaxUsedSizeInBytes()
	dir.SetTrackDiskUsage(true)

	config2 := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config2.SetOpenMode(index.APPEND)
	config2.SetMergePolicy(index.NewLogMergePolicy())
	writer, err = index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) error = %v", err)
	}
	writer.Close()

	finalDiskUsage, err := dirSize(dir)
	if err != nil {
		t.Fatalf("dirSize() error = %v", err)
	}
	maxDiskUsage := dir.GetMaxUsedSizeInBytes()
	maxStartFinal := max(startDiskUsage, finalDiskUsage)

	if maxDiskUsage > 4*maxStartFinal {
		t.Fatalf("forceMerge used too much temporary space: maxUsage=%d start=%d final=%d limit=%d",
			maxDiskUsage, startDiskUsage, finalDiskUsage, 4*maxStartFinal)
	}
}

// dirSize returns the sum of the lengths of every file in dir.
func dirSize(dir store.Directory) (int64, error) {
	names, err := dir.ListAll()
	if err != nil {
		return 0, err
	}
	var total int64
	for _, name := range names {
		sz, err := dir.FileLength(name)
		if err != nil {
			return 0, err
		}
		total += sz
	}
	return total, nil
}

// TestIndexWriterForceMerge_BackgroundForceMerge ports testBackgroundForceMerge().
//
// The original calls forceMerge(1, false) to start a merge without waiting. No
// background ForceMerge overload exists, so the case is fully skipped.
func TestIndexWriterForceMerge_BackgroundForceMerge(t *testing.T) {
	t.Fatal("background ForceMerge(maxNumSegments, doWait) overload not implemented")
}

// TestIndexWriterForceMerge_MergePerField ports testMergePerField().
//
// The Java test is annotated @AwaitsFix (apache/lucene#13478) and additionally
// requires PerField postings/doc-values formats with a CyclicBarrier on merge.
// Neither the upstream fix nor those test hooks exist here, so it is skipped.
func TestIndexWriterForceMerge_MergePerField(t *testing.T) {
	t.Fatal("upstream @AwaitsFix (apache/lucene#13478); PerField merge-barrier formats unavailable")
}
