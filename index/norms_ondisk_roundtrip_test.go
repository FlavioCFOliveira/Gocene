// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test holds the on-disk NORMS round-trip acceptance test for
// rmp #120: norms written through IndexWriter (computed during indexing as
// Similarity.computeNorm(FieldInvertState)) must be serialised to the codec
// .nvd/.nvm files on commit, read back through OpenDirectoryReader's
// SegmentReader.GetNormValues, and preserved across a multi-segment
// forceMerge. The scoring subtest confirms shorter fields carry a smaller
// norm, the length-normalization signal BM25 consumes.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"

	// Blank-import the codecs so the production Lucene104 codec is registered
	// as the default. flushNorms is a no-op without a codec.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

// normsTextDoc builds a document with a single tokenized TextField "body" whose
// value is the given whitespace-separated string. Under the WhitespaceAnalyzer
// the token count equals the number of words, so the expected norm is
// SmallFloat.IntToByte4(wordCount) (the identity for word counts < 24).
func normsTextDoc(body string) *testDocument {
	f, _ := document.NewTextField("body", body, false)
	return &testDocument{fields: []interface{}{f}}
}

// wantNorm returns the norm value Lucene's default similarity computes for a
// field of the given token count: SmallFloat.IntToByte4(numTerms).
func wantNorm(t *testing.T, numTerms int) int64 {
	t.Helper()
	b, err := util.IntToByte4(numTerms)
	if err != nil {
		t.Fatalf("IntToByte4(%d): %v", numTerms, err)
	}
	return int64(b)
}

// readNorm fetches the norm value for (field, docID) from a SegmentReader,
// failing the test if the field has no norms or the document carries no value.
func readNorm(t *testing.T, r *index.SegmentReader, field string, docID int) int64 {
	t.Helper()
	nv, err := r.GetNormValues(field)
	if err != nil {
		t.Fatalf("GetNormValues(%q): %v", field, err)
	}
	if nv == nil {
		t.Fatalf("GetNormValues(%q) returned nil (norms not wired for this field)", field)
	}
	ok, err := nv.AdvanceExact(docID)
	if err != nil {
		t.Fatalf("AdvanceExact(%d): %v", docID, err)
	}
	if !ok {
		t.Fatalf("doc %d has no norm value for field %q", docID, field)
	}
	v, err := nv.LongValue()
	if err != nil {
		t.Fatalf("LongValue: %v", err)
	}
	return v
}

// TestNorms_OnDiskRoundTrip is the rmp #120 acceptance test (legs 1-3): write
// tokenized TextFields of varying length through IndexWriter, commit, reopen,
// and verify SegmentReader.GetNormValues returns the expected computeNorm
// value for every document.
func TestNorms_OnDiskRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// docID -> token count.
	lengths := []int{1, 2, 3, 5, 10}
	bodies := []string{
		"alpha",
		"alpha beta",
		"alpha beta gamma",
		"a b c d e",
		"a b c d e f g h i j",
	}
	for i, body := range bodies {
		if err := writer.AddDocument(normsTextDoc(body)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	r := segs[0]
	if got := r.MaxDoc(); got != len(bodies) {
		t.Fatalf("MaxDoc = %d, want %d", got, len(bodies))
	}

	// Iterate the dense norms stream and check every document.
	nv, err := r.GetNormValues("body")
	if err != nil {
		t.Fatalf("GetNormValues: %v", err)
	}
	if nv == nil {
		t.Fatal("GetNormValues returned nil; norms not wired to the codec")
	}
	for want := 0; want < len(bodies); want++ {
		doc, err := nv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if doc != want {
			t.Fatalf("norm stream doc = %d, want %d", doc, want)
		}
		v, err := nv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		if expect := wantNorm(t, lengths[want]); v != expect {
			t.Fatalf("doc %d: norm = %d, want %d (token count %d)", doc, v, expect, lengths[want])
		}
	}
}

// TestNorms_ForceMergePreservesNorms is the rmp #120 acceptance test (leg 4):
// build three single-document segments via three commits, forceMerge them into
// one, reopen, and verify every document's norm survives the merge with its
// docID remapped into the merged segment.
func TestNorms_ForceMergePreservesNorms(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// One document per commit so each lands in its own segment; the bodies
	// have distinct token counts so the merged norms are individually checkable.
	bodies := []string{"alpha", "alpha beta", "alpha beta gamma"}
	lengths := []int{1, 2, 3}
	for i, body := range bodies {
		if err := writer.AddDocument(normsTextDoc(body)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit(%d): %v", i, err)
		}
	}

	// Sanity: before the merge there must be more than one segment.
	preReader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (pre-merge): %v", err)
	}
	preSegs := len(preReader.GetSegmentReaders())
	preReader.Close()
	if preSegs < 2 {
		t.Fatalf("expected >= 2 segments before merge, got %d", preSegs)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1): %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit after ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (post-merge): %v", err)
	}
	defer reader.Close()

	segs := reader.GetSegmentReaders()
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment after forceMerge, got %d", len(segs))
	}
	r := segs[0]
	if got := r.MaxDoc(); got != len(bodies) {
		t.Fatalf("MaxDoc = %d, want %d", got, len(bodies))
	}

	// The merge appends segments in order, so merged docIDs are 0,1,2 in the
	// original commit order.
	for docID, length := range lengths {
		got := readNorm(t, r, "body", docID)
		if expect := wantNorm(t, length); got != expect {
			t.Fatalf("post-merge doc %d: norm = %d, want %d (token count %d)", docID, got, expect, length)
		}
	}
}

// TestNorms_ScoringReflectsLengthNormalization is the rmp #120 acceptance test
// (scoring leg): a shorter field must carry a smaller norm than a longer field
// containing the same term. BM25 applies a length penalty monotonic in the
// decoded norm (1 / (1 - b + b * docLen/avgLen)); a smaller norm therefore
// yields a higher score for the same term frequency, so this norm ordering is
// the exact signal that makes shorter documents score higher.
func TestNorms_ScoringReflectsLengthNormalization(t *testing.T) {
	dir := store.NewByteBuffersDirectory()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// doc 0: the query term in a 1-token field (short).
	// doc 1: the same term diluted in a 6-token field (long).
	if err := writer.AddDocument(normsTextDoc("lucene")); err != nil {
		t.Fatalf("AddDocument(0): %v", err)
	}
	if err := writer.AddDocument(normsTextDoc("lucene is a search engine library")); err != nil {
		t.Fatalf("AddDocument(1): %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	r := reader.GetSegmentReaders()[0]
	shortNorm := readNorm(t, r, "body", 0)
	longNorm := readNorm(t, r, "body", 1)

	if shortNorm >= longNorm {
		t.Fatalf("expected shorter field to carry a smaller norm: short=%d long=%d", shortNorm, longNorm)
	}
	// The decoded lengths must match the actual token counts (1 vs 6) since
	// both are below the SmallFloat free-value threshold.
	if want := wantNorm(t, 1); shortNorm != want {
		t.Fatalf("short field norm = %d, want %d", shortNorm, want)
	}
	if want := wantNorm(t, 6); longNorm != want {
		t.Fatalf("long field norm = %d, want %d", longNorm, want)
	}
}
