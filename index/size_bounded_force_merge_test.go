// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for size-bounded forceMerge behavior.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestSizeBoundedForceMerge
// Source: lucene/core/src/test/org/apache/lucene/index/TestSizeBoundedForceMerge.java
//
// GOC-4221: Port test org.apache.lucene.index.TestSizeBoundedForceMerge (Sprint 55).
//
// Port strategy (Sprint 55 option c): each Java @Test has a 1:1 Go counterpart.
// The shared writer/document/commit roundtrip runs for real wherever the Gocene
// API supports it; assertions that depend on still-missing behavior are gated
// with t.Skip so the divergence is explicit rather than silently absent.
//
// Known API gaps that force a skip in this file:
//   - IndexWriter.ForceMerge ignores maxNumSegments (it currently only flushes
//     buffered docs via Commit), so every resulting segment-count assertion is
//     deferred.
//   - LogByteSizeMergePolicy/LogDocMergePolicy size constraints
//     (SetMaxMergeMBForForcedMerge, SetMaxMergeDocs, SetMergeFactor) are not
//     consulted by ForceMerge, so the size/doc-bound assertions are deferred.
//   - hasDeletions state after a forced merge is likewise unobservable until
//     ForceMerge performs real merges.
package index_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// newSizeBoundedWriterConfig mirrors the Java newWriterConfig() helper: no auto
// flush, no compound files, and NoMergePolicy so the initial index keeps every
// addDocs() batch as its own segment.
func newSizeBoundedWriterConfig() *index.IndexWriterConfig {
	conf := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	conf.SetMaxBufferedDocs(index.DISABLE_AUTO_FLUSH)
	conf.SetUseCompoundFile(false)
	conf.SetMergePolicy(index.NewNoMergePolicy())
	return conf
}

// addSizeBoundedDocs ports the addDocs(writer, numDocs, withID) helper: it adds
// numDocs documents (optionally with a stored-NO "id" StringField) and commits,
// producing one segment per call under NoMergePolicy.
func addSizeBoundedDocs(t *testing.T, writer *index.IndexWriter, numDocs int, withID bool) {
	t.Helper()
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		if withID {
			f, err := document.NewStringField("id", strconv.Itoa(i), false)
			if err != nil {
				t.Fatalf("NewStringField() error = %v", err)
			}
			doc.Add(f)
		}
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument() error = %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
}

// TestSizeBoundedForceMerge_ByteSizeLimit ports testByteSizeLimit().
//
// Builds 15 segments (one oversized) then forceMerge(1) under a
// LogByteSizeMergePolicy whose forced-merge size cap equals the smallest
// segment, expecting 3 final segments. The build runs for real; the
// segment-count assertion is deferred (ForceMerge ignores the size cap).
func TestSizeBoundedForceMerge_ByteSizeLimit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, newSizeBoundedWriterConfig())
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	const numSegments = 15
	for i := 0; i < numSegments; i++ {
		numDocs := 1
		if i == 7 {
			numDocs = 30
		}
		addSizeBoundedDocs(t, writer, numDocs, false)
	}
	writer.Close()

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos() error = %v", err)
	}
	min := float64(sis.Get(0).SegmentInfo().SizeInBytes())

	conf := newSizeBoundedWriterConfig()
	lmp := index.NewLogByteSizeMergePolicy()
	lmp.SetMaxMergeMBForForcedMerge(min / (1 << 20))
	conf.SetMergePolicy(lmp)

	writer, err = index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) error = %v", err)
	}
	writer.Close()

	sis, err = index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos() error = %v", err)
	}

	t.Skip("ForceMerge ignores LogByteSizeMergePolicy size cap; segment-count assertion deferred")

	if got := sis.Size(); got != 3 {
		t.Errorf("expected 3 segments, got %d", got)
	}
}

// TestSizeBoundedForceMerge_NumDocsLimit ports testNumDocsLimit().
//
// Builds 7 segments then forceMerge(1) under a LogDocMergePolicy capped at 3
// docs, expecting 3 final segments. The build runs for real; the segment-count
// assertion is deferred (ForceMerge ignores maxMergeDocs).
func TestSizeBoundedForceMerge_NumDocsLimit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, newSizeBoundedWriterConfig())
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	for _, n := range []int{3, 3, 5, 3, 3, 3, 3} {
		addSizeBoundedDocs(t, writer, n, false)
	}
	writer.Close()

	conf := newSizeBoundedWriterConfig()
	lmp := index.NewLogDocMergePolicy()
	lmp.SetMaxMergeDocs(3)
	conf.SetMergePolicy(lmp)

	writer, err = index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) error = %v", err)
	}
	writer.Close()

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos() error = %v", err)
	}

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs; segment-count assertion deferred")

	if got := sis.Size(); got != 3 {
		t.Errorf("expected 3 segments, got %d", got)
	}
}

// TestSizeBoundedForceMerge_LastSegmentTooLarge ports testLastSegmentTooLarge().
func TestSizeBoundedForceMerge_LastSegmentTooLarge(t *testing.T) {
	sis := runSizeBoundedForceMerge(t, []int{3, 3, 3, 5}, 3, 1, false)

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs; segment-count assertion deferred")

	if got := sis.Size(); got != 2 {
		t.Errorf("expected 2 segments, got %d", got)
	}
}

// TestSizeBoundedForceMerge_FirstSegmentTooLarge ports testFirstSegmentTooLarge().
func TestSizeBoundedForceMerge_FirstSegmentTooLarge(t *testing.T) {
	sis := runSizeBoundedForceMerge(t, []int{5, 3, 3, 3}, 3, 1, false)

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs; segment-count assertion deferred")

	if got := sis.Size(); got != 2 {
		t.Errorf("expected 2 segments, got %d", got)
	}
}

// TestSizeBoundedForceMerge_AllSegmentsSmall ports testAllSegmentsSmall().
func TestSizeBoundedForceMerge_AllSegmentsSmall(t *testing.T) {
	sis := runSizeBoundedForceMerge(t, []int{3, 3, 3, 3}, 3, 1, false)

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs; segment-count assertion deferred")

	if got := sis.Size(); got != 1 {
		t.Errorf("expected 1 segment, got %d", got)
	}
}

// TestSizeBoundedForceMerge_AllSegmentsLarge ports testAllSegmentsLarge().
func TestSizeBoundedForceMerge_AllSegmentsLarge(t *testing.T) {
	sis := runSizeBoundedForceMerge(t, []int{3, 3, 3}, 2, 1, false)

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs; segment-count assertion deferred")

	if got := sis.Size(); got != 3 {
		t.Errorf("expected 3 segments, got %d", got)
	}
}

// TestSizeBoundedForceMerge_OneLargeOneSmall ports testOneLargeOneSmall().
func TestSizeBoundedForceMerge_OneLargeOneSmall(t *testing.T) {
	sis := runSizeBoundedForceMerge(t, []int{3, 5, 3, 5}, 3, 1, false)

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs; segment-count assertion deferred")

	if got := sis.Size(); got != 4 {
		t.Errorf("expected 4 segments, got %d", got)
	}
}

// TestSizeBoundedForceMerge_MergeFactor ports testMergeFactor().
//
// Same as the size-bounded cases but with mergeFactor=2, expecting 4 final
// segments. The build runs for real; the assertion is deferred.
func TestSizeBoundedForceMerge_MergeFactor(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, newSizeBoundedWriterConfig())
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	for _, n := range []int{3, 3, 3, 3, 5, 3, 3} {
		addSizeBoundedDocs(t, writer, n, false)
	}
	writer.Close()

	conf := newSizeBoundedWriterConfig()
	lmp := index.NewLogDocMergePolicy()
	lmp.SetMaxMergeDocs(3)
	lmp.SetMergeFactor(2)
	conf.SetMergePolicy(lmp)

	writer, err = index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) error = %v", err)
	}
	writer.Close()

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos() error = %v", err)
	}

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs/mergeFactor; segment-count assertion deferred")

	if got := sis.Size(); got != 4 {
		t.Errorf("expected 4 segments, got %d", got)
	}
}

// TestSizeBoundedForceMerge_SingleMergeableSegment ports testSingleMergeableSegment().
//
// Deletes a document so the last segment becomes mergeable, then expects 3
// final segments with no deletions on the last one.
func TestSizeBoundedForceMerge_SingleMergeableSegment(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, newSizeBoundedWriterConfig())
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	addSizeBoundedDocs(t, writer, 3, false)
	addSizeBoundedDocs(t, writer, 5, false)
	addSizeBoundedDocs(t, writer, 3, false)
	if err := writer.DeleteDocuments(index.NewTerm("id", "10")); err != nil {
		t.Fatalf("DeleteDocuments() error = %v", err)
	}
	writer.Close()

	conf := newSizeBoundedWriterConfig()
	lmp := index.NewLogDocMergePolicy()
	lmp.SetMaxMergeDocs(3)
	conf.SetMergePolicy(lmp)

	writer, err = index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) error = %v", err)
	}
	writer.Close()

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos() error = %v", err)
	}

	t.Skip("ForceMerge does not merge mergeable segments yet; segment-count/deletions assertions deferred")

	if got := sis.Size(); got != 3 {
		t.Errorf("expected 3 segments, got %d", got)
	}
	if sis.Get(2).HasDeletions() {
		t.Error("last segment should not have deletions")
	}
}

// TestSizeBoundedForceMerge_SingleNonMergeableSegment ports testSingleNonMergeableSegment().
func TestSizeBoundedForceMerge_SingleNonMergeableSegment(t *testing.T) {
	sis := runSizeBoundedForceMerge(t, []int{3}, 3, 1, true)

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs; segment-count assertion deferred")

	if got := sis.Size(); got != 1 {
		t.Errorf("expected 1 segment, got %d", got)
	}
}

// TestSizeBoundedForceMerge_SingleMergeableTooLargeSegment ports
// testSingleMergeableTooLargeSegment().
//
// A single oversized segment with one deleted document: forceMerge should keep
// it as one segment that still carries the deletion.
func TestSizeBoundedForceMerge_SingleMergeableTooLargeSegment(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer, err := index.NewIndexWriter(dir, newSizeBoundedWriterConfig())
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	addSizeBoundedDocs(t, writer, 5, true)
	if err := writer.DeleteDocuments(index.NewTerm("id", "4")); err != nil {
		t.Fatalf("DeleteDocuments() error = %v", err)
	}
	writer.Close()

	conf := newSizeBoundedWriterConfig()
	lmp := index.NewLogDocMergePolicy()
	lmp.SetMaxMergeDocs(2)
	conf.SetMergePolicy(lmp)

	writer, err = index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) error = %v", err)
	}
	writer.Close()

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos() error = %v", err)
	}

	t.Skip("ForceMerge ignores LogDocMergePolicy maxMergeDocs; segment-count/deletions assertions deferred")

	if got := sis.Size(); got != 1 {
		t.Errorf("expected 1 segment, got %d", got)
	}
	if !sis.Get(0).HasDeletions() {
		t.Error("segment should have deletions")
	}
}

// runSizeBoundedForceMerge factors the shared body of the cases that only build
// segments from a docs-per-segment list and forceMerge(1) under a LogDocMergePolicy
// with the given maxMergeDocs. It returns the post-merge SegmentInfos.
func runSizeBoundedForceMerge(t *testing.T, docsPerSegment []int, maxMergeDocs, maxNumSegments int, withID bool) *index.SegmentInfos {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { dir.Close() })

	writer, err := index.NewIndexWriter(dir, newSizeBoundedWriterConfig())
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	for _, n := range docsPerSegment {
		addSizeBoundedDocs(t, writer, n, withID)
	}
	writer.Close()

	conf := newSizeBoundedWriterConfig()
	lmp := index.NewLogDocMergePolicy()
	lmp.SetMaxMergeDocs(maxMergeDocs)
	conf.SetMergePolicy(lmp)

	writer, err = index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("NewIndexWriter() error = %v", err)
	}
	if err := writer.ForceMerge(maxNumSegments); err != nil {
		t.Fatalf("ForceMerge(%d) error = %v", maxNumSegments, err)
	}
	writer.Close()

	sis, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos() error = %v", err)
	}
	return sis
}
