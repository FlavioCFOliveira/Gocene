// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSegmentTermDocs ports org.apache.lucene.index.TestSegmentTermDocs.
//
// The Java suite exercises PostingsEnum traversal over a freshly written
// segment: resolving a TermsEnum from a leaf reader, seeking a term, and
// reading docID/freq plus advance/nextDoc navigation (including the skip-list
// boundary cases at exactly skipInterval and well beyond it).
//
// Pre-existing infrastructure gap: every test routes through
// LeafReader.Terms(field).iterator()..., and OpenDirectoryReader materialises
// each segment via NewSegmentReader (index/directory_reader.go:462/497), which
// leaves SegmentReader.coreReaders nil. Without the codec-side wiring that
// loads SegmentCoreReaders from disk, LeafReader.Terms returns the
// "core readers are nil" error and none of the assertions can run. Each test
// therefore skips with the same blocker as TestDocsAndPositions /
// TestBinaryTerms; unskip once OpenDirectoryReader uses
// NewSegmentReaderWithCore.
//
// Divergences from Lucene shared by every test below:
//   - Lucene drives writes through DocHelper.writeDoc / RandomIndexWriter with
//     a randomized merge policy and MockAnalyzer; Gocene exposes no randomized
//     test-writer wrapper, so these ports use the plain IndexWriter with
//     WhitespaceAnalyzer (the indexed text is purely space-separated tokens,
//     so tokenization is identical).
//   - Lucene reads via "new SegmentReader(info, ...)" / IndexWriter.getReader;
//     Gocene's IndexWriter has no NRT reader and no public single-segment
//     reader constructor, so the index is reopened from the directory after
//     commit, matching TestDocsAndPositions / TestBinaryTerms.
//   - Lucene's PostingsEnum.FREQS flag constant has no Gocene equivalent;
//     TermsEnum.Postings takes a bare int, so 0 is passed.
//   - Lucene's DocIdSetIterator.NO_MORE_DOCS sentinel is Integer.MAX_VALUE;
//     Gocene's PostingsEnum sentinel (index.NO_MORE_DOCS) is -1. The ports
//     compare against index.NO_MORE_DOCS.
//   - testBadSeek's second case ("junk", a never-indexed field) is preserved:
//     a missing field yields a nil PostingsEnum, mirroring the Java assertNull.

const segmentTermDocsBlocked = "blocked: OpenDirectoryReader builds SegmentReader without core readers (index/directory_reader.go:462/497); fix is NewSegmentReaderWithCore"

// segmentTermDocsSeekCeil mirrors the Java pattern
// "reader.terms(field).iterator(); terms.seekCeil(term); terms.postings(...)".
// It resolves the PostingsEnum positioned at the first term >= seekTerm in
// fieldName, or returns nil when the field is absent or no such term exists.
func segmentTermDocsSeekCeil(t *testing.T, air index.LeafReaderInterface, fieldName, seekTerm string) index.PostingsEnum {
	t.Helper()
	terms, err := air.Terms(fieldName)
	if err != nil {
		t.Fatalf("Terms(%q) failed: %v", fieldName, err)
	}
	if terms == nil {
		return nil
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator failed: %v", err)
	}
	found, err := te.SeekCeil(index.NewTerm(fieldName, seekTerm))
	if err != nil {
		t.Fatalf("SeekCeil(%q) failed: %v", seekTerm, err)
	}
	if found == nil {
		return nil
	}
	pe, err := te.Postings(0) // Java: PostingsEnum.FREQS
	if err != nil {
		t.Fatalf("Postings failed: %v", err)
	}
	return pe
}

// segmentTermDocsSeekExact mirrors the Java helper TestUtil.docs(reader,
// field, term, ...): it resolves the PostingsEnum for an exact term, or
// returns nil when the field or the term is absent (the assertNull contract
// of testBadSeek).
func segmentTermDocsSeekExact(t *testing.T, air index.LeafReaderInterface, fieldName, term string) index.PostingsEnum {
	t.Helper()
	terms, err := air.Terms(fieldName)
	if err != nil {
		t.Fatalf("Terms(%q) failed: %v", fieldName, err)
	}
	if terms == nil {
		return nil
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator failed: %v", err)
	}
	found, err := te.SeekExact(index.NewTerm(fieldName, term))
	if err != nil {
		t.Fatalf("SeekExact(%q) failed: %v", term, err)
	}
	if !found {
		return nil
	}
	pe, err := te.Postings(0)
	if err != nil {
		t.Fatalf("Postings failed: %v", err)
	}
	return pe
}

// segmentTermDocsTestDocLeaf writes the shared DocHelper document, commits,
// reopens the directory and returns the single leaf reader. It is the Gocene
// analogue of the Java setUp() body (DocHelper.setupDoc + DocHelper.writeDoc).
func segmentTermDocsTestDocLeaf(t *testing.T) (index.LeafReaderInterface, func()) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		dir.Close()
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	if err := writer.AddDocument(readerSetupTestDoc()); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		dir.Close()
		t.Fatalf("Failed to open reader: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		reader.Close()
		dir.Close()
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) != 1 {
		reader.Close()
		dir.Close()
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	return leaves[0].LeafReader(), func() {
		reader.Close()
		dir.Close()
	}
}

// segmentTermDocsSkipToLeaf writes three blocks of documents — 10 with "aaa",
// 16 with "bbb" and 50 with "ccc" — force-merges to a single segment, reopens
// the directory and returns its only leaf reader. It is the Gocene analogue of
// the testSkipTo() index-building preamble.
func segmentTermDocsSkipToLeaf(t *testing.T) (index.LeafReaderInterface, func()) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		dir.Close()
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	addDoc := func(value string) {
		doc := document.NewDocument()
		field, err := document.NewTextField("content", value, false)
		if err != nil {
			t.Fatalf("Failed to create field: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	for i := 0; i < 10; i++ {
		addDoc("aaa aaa aaa aaa")
	}
	for i := 0; i < 16; i++ {
		addDoc("bbb bbb bbb bbb")
	}
	for i := 0; i < 50; i++ {
		addDoc("ccc ccc ccc ccc")
	}

	// assure that we deal with a single segment
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) failed: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		dir.Close()
		t.Fatalf("Failed to open reader: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		reader.Close()
		dir.Close()
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) != 1 {
		reader.Close()
		dir.Close()
		t.Fatalf("expected 1 leaf after forceMerge(1), got %d", len(leaves))
	}
	return leaves[0].LeafReader(), func() {
		reader.Close()
		dir.Close()
	}
}

// expectAdvance asserts that pe.Advance(target) reaches a document and that the
// resulting docID equals want, mirroring the Java
// "assertTrue(tdocs.advance(target) != NO_MORE_DOCS); assertEquals(want, ...)".
func expectAdvance(t *testing.T, pe index.PostingsEnum, target, want int) {
	t.Helper()
	got, err := pe.Advance(target)
	if err != nil {
		t.Fatalf("Advance(%d) failed: %v", target, err)
	}
	if got == index.NO_MORE_DOCS {
		t.Fatalf("Advance(%d) returned NO_MORE_DOCS, want doc %d", target, want)
	}
	if pe.DocID() != want {
		t.Fatalf("Advance(%d): DocID() = %d, want %d", target, pe.DocID(), want)
	}
}

// expectNextDoc asserts that pe.NextDoc() reaches a document whose docID and
// freq match want/wantFreq, mirroring the Java "nextDoc()/docID()/freq()" trio.
func expectNextDoc(t *testing.T, pe index.PostingsEnum, want, wantFreq int) {
	t.Helper()
	got, err := pe.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc failed: %v", err)
	}
	if got == index.NO_MORE_DOCS {
		t.Fatalf("NextDoc returned NO_MORE_DOCS, want doc %d", want)
	}
	if pe.DocID() != want {
		t.Fatalf("NextDoc: DocID() = %d, want %d", pe.DocID(), want)
	}
	if freq, err := pe.Freq(); err != nil {
		t.Fatalf("Freq failed: %v", err)
	} else if freq != wantFreq {
		t.Fatalf("doc %d: Freq() = %d, want %d", want, freq, wantFreq)
	}
}

// expectExhausted asserts that pe.Advance(target) reports NO_MORE_DOCS,
// mirroring the Java "assertFalse(tdocs.advance(target) != NO_MORE_DOCS)".
func expectExhausted(t *testing.T, pe index.PostingsEnum, target int) {
	t.Helper()
	got, err := pe.Advance(target)
	if err != nil {
		t.Fatalf("Advance(%d) failed: %v", target, err)
	}
	if got != index.NO_MORE_DOCS {
		t.Fatalf("Advance(%d) = %d, want NO_MORE_DOCS", target, got)
	}
}

// TestSegmentTermDocs ports the test() method: it merely asserts the directory
// and its single leaf reader are non-nil after the document is written.
func TestSegmentTermDocs(t *testing.T) {
	t.Fatal(segmentTermDocsBlocked)

	air, cleanup := segmentTermDocsTestDocLeaf(t)
	defer cleanup()

	if air == nil {
		t.Fatal("leaf reader should not be nil")
	}
}

// TestSegmentTermDocsTermDocs ports testTermDocs: after writing the DocHelper
// document, the term "field" in textField2 must resolve to doc 0 with freq 3.
func TestSegmentTermDocsTermDocs(t *testing.T) {
	t.Fatal(segmentTermDocsBlocked)

	air, cleanup := segmentTermDocsTestDocLeaf(t)
	defer cleanup()

	termDocs := segmentTermDocsSeekCeil(t, air, srTextField2Key, "field")
	if termDocs == nil {
		t.Fatal("seekCeil(\"field\") returned nil PostingsEnum")
	}
	next, err := termDocs.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc failed: %v", err)
	}
	if next != index.NO_MORE_DOCS {
		if termDocs.DocID() != 0 {
			t.Fatalf("DocID() = %d, want 0", termDocs.DocID())
		}
		freq, err := termDocs.Freq()
		if err != nil {
			t.Fatalf("Freq failed: %v", err)
		}
		if freq != 3 {
			t.Fatalf("Freq() = %d, want 3", freq)
		}
	}
}

// TestSegmentTermDocsBadSeek ports testBadSeek: a bad term in an existing
// field, and any term in a never-indexed field, both yield a nil PostingsEnum.
func TestSegmentTermDocsBadSeek(t *testing.T) {
	t.Fatal(segmentTermDocsBlocked)

	air, cleanup := segmentTermDocsTestDocLeaf(t)
	defer cleanup()

	// Bad term in an existing field.
	if pe := segmentTermDocsSeekExact(t, air, "textField2", "bad"); pe != nil {
		t.Error("seekExact(textField2, \"bad\") should yield a nil PostingsEnum")
	}

	// Any term in a field that was never indexed.
	if pe := segmentTermDocsSeekExact(t, air, "junk", "bad"); pe != nil {
		t.Error("seekExact(junk, \"bad\") should yield a nil PostingsEnum")
	}
}

// TestSegmentTermDocsSkipTo ports testSkipTo: it walks nextDoc/advance over
// three terms whose posting lists straddle the skip-list interval — "aaa"
// (below skipInterval), "bbb" (exactly skipInterval) and "ccc" (well beyond).
func TestSegmentTermDocsSkipTo(t *testing.T) {
	t.Fatal(segmentTermDocsBlocked)

	air, cleanup := segmentTermDocsSkipToLeaf(t)
	defer cleanup()

	// without optimization (assumption skipInterval == 16)

	// with next
	tdocs := segmentTermDocsSeekExact(t, air, "content", "aaa")
	if tdocs == nil {
		t.Fatal("seekExact(content, \"aaa\") returned nil")
	}
	expectNextDoc(t, tdocs, 0, 4)
	expectNextDoc(t, tdocs, 1, 4)
	expectAdvance(t, tdocs, 2, 2)
	expectAdvance(t, tdocs, 4, 4)
	expectAdvance(t, tdocs, 9, 9)
	expectExhausted(t, tdocs, 10)

	// without next
	tdocs = segmentTermDocsSeekExact(t, air, "content", "aaa")
	if tdocs == nil {
		t.Fatal("seekExact(content, \"aaa\") returned nil")
	}
	expectAdvance(t, tdocs, 0, 0)
	expectAdvance(t, tdocs, 4, 4)
	expectAdvance(t, tdocs, 9, 9)
	expectExhausted(t, tdocs, 10)

	// exactly skipInterval documents and therefore with optimization

	// with next
	tdocs = segmentTermDocsSeekExact(t, air, "content", "bbb")
	if tdocs == nil {
		t.Fatal("seekExact(content, \"bbb\") returned nil")
	}
	expectNextDoc(t, tdocs, 10, 4)
	expectNextDoc(t, tdocs, 11, 4)
	expectAdvance(t, tdocs, 12, 12)
	expectAdvance(t, tdocs, 15, 15)
	expectAdvance(t, tdocs, 24, 24)
	expectAdvance(t, tdocs, 25, 25)
	expectExhausted(t, tdocs, 26)

	// without next
	tdocs = segmentTermDocsSeekExact(t, air, "content", "bbb")
	if tdocs == nil {
		t.Fatal("seekExact(content, \"bbb\") returned nil")
	}
	expectAdvance(t, tdocs, 5, 10)
	expectAdvance(t, tdocs, 15, 15)
	expectAdvance(t, tdocs, 24, 24)
	expectAdvance(t, tdocs, 25, 25)
	expectExhausted(t, tdocs, 26)

	// much more than skipInterval documents and therefore with optimization

	// with next
	tdocs = segmentTermDocsSeekExact(t, air, "content", "ccc")
	if tdocs == nil {
		t.Fatal("seekExact(content, \"ccc\") returned nil")
	}
	expectNextDoc(t, tdocs, 26, 4)
	expectNextDoc(t, tdocs, 27, 4)
	expectAdvance(t, tdocs, 28, 28)
	expectAdvance(t, tdocs, 40, 40)
	expectAdvance(t, tdocs, 57, 57)
	expectAdvance(t, tdocs, 74, 74)
	expectAdvance(t, tdocs, 75, 75)
	expectExhausted(t, tdocs, 76)

	// without next
	tdocs = segmentTermDocsSeekExact(t, air, "content", "ccc")
	if tdocs == nil {
		t.Fatal("seekExact(content, \"ccc\") returned nil")
	}
	expectAdvance(t, tdocs, 5, 26)
	expectAdvance(t, tdocs, 40, 40)
	expectAdvance(t, tdocs, 57, 57)
	expectAdvance(t, tdocs, 74, 74)
	expectAdvance(t, tdocs, 75, 75)
	expectExhausted(t, tdocs, 76)
}
