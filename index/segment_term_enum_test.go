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

// segmentTermEnumAddDoc indexes a single document whose "content" field holds
// value. It ports the private addDoc helper of
// org.apache.lucene.index.TestSegmentTermEnum.
func segmentTermEnumAddDoc(t *testing.T, writer *index.IndexWriter, value string) {
	t.Helper()
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

// verifySegmentTermEnumDocFreq reopens dir and asserts the term dictionary of
// the "content" field: term "aaa" with docFreq 200 followed by "bbb" with
// docFreq 100, walking the enumeration first with Next and then re-seeking via
// SeekCeil. It ports TestSegmentTermEnum#verifyDocFreq.
func verifySegmentTermEnumDocFreq(t *testing.T, dir store.Directory) {
	t.Helper()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	// Lucene reads the term dictionary through MultiTerms.getTerms, which
	// flattens every segment into one TermsEnum. Gocene exposes no such
	// reader-level helper, so the leaves are obtained directly; the docFreq
	// invariant is independent of how many segments back the enumeration.
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}

	subs := make([]index.Terms, 0, len(leaves))
	for _, leaf := range leaves {
		terms, err := leaf.LeafReader().Terms("content")
		if err != nil {
			t.Fatalf("Failed to get terms: %v", err)
		}
		if terms != nil {
			subs = append(subs, terms)
		}
	}

	multi, err := index.NewMultiTerms(subs, nil)
	if err != nil {
		t.Fatalf("Failed to build MultiTerms: %v", err)
	}
	termEnum, err := multi.Iterator()
	if err != nil {
		t.Fatalf("Failed to get terms iterator: %v", err)
	}

	// Walk to the first term (aaa).
	term, err := termEnum.Next()
	if err != nil {
		t.Fatalf("TermsEnum.Next() failed: %v", err)
	}
	if term == nil || term.Text() != "aaa" {
		t.Fatalf("first term = %v, want \"aaa\"", term)
	}
	if df, err := termEnum.DocFreq(); err != nil {
		t.Fatalf("DocFreq() failed: %v", err)
	} else if df != 200 {
		t.Fatalf("docFreq(aaa) = %d, want 200", df)
	}

	// Walk to the second term (bbb).
	term, err = termEnum.Next()
	if err != nil {
		t.Fatalf("TermsEnum.Next() failed: %v", err)
	}
	if term == nil || term.Text() != "bbb" {
		t.Fatalf("second term = %v, want \"bbb\"", term)
	}
	if df, err := termEnum.DocFreq(); err != nil {
		t.Fatalf("DocFreq() failed: %v", err)
	} else if df != 100 {
		t.Fatalf("docFreq(bbb) = %d, want 100", df)
	}

	// Re-seek to "aaa" (inclusive) and walk forward again.
	aaa := index.NewTerm("content", "aaa")
	term, err = termEnum.SeekCeil(aaa)
	if err != nil {
		t.Fatalf("SeekCeil(aaa) failed: %v", err)
	}
	if term == nil || term.Text() != "aaa" {
		t.Fatalf("SeekCeil(aaa) landed on %v, want \"aaa\"", term)
	}
	if df, err := termEnum.DocFreq(); err != nil {
		t.Fatalf("DocFreq() failed: %v", err)
	} else if df != 200 {
		t.Fatalf("docFreq(aaa) = %d, want 200", df)
	}

	term, err = termEnum.Next()
	if err != nil {
		t.Fatalf("TermsEnum.Next() failed: %v", err)
	}
	if term == nil || term.Text() != "bbb" {
		t.Fatalf("term after aaa = %v, want \"bbb\"", term)
	}
	if df, err := termEnum.DocFreq(); err != nil {
		t.Fatalf("DocFreq() failed: %v", err)
	} else if df != 100 {
		t.Fatalf("docFreq(bbb) = %d, want 100", df)
	}
}

// TestSegmentTermEnum ports org.apache.lucene.index.TestSegmentTermEnum#testTermEnum.
//
// It indexes 100 documents with term "aaa" and 100 with terms "aaa bbb", so
// "aaa" has docFreq 200 and "bbb" has docFreq 100, then verifies the term
// dictionary both as a multi-segment index and, after forceMerge(1), as a
// single-segment index.
//
// Divergences from Lucene:
//   - Lucene uses MockAnalyzer over LuceneTestCase's random Directory; Gocene
//     has no randomized test analyzer or directory wrapper, so a plain
//     WhitespaceAnalyzer and SimpleFSDirectory are used. Neither choice affects
//     docFreq, which depends only on the postings.
//   - Lucene reads the dictionary via the reader-level MultiTerms.getTerms
//     helper; Gocene has no such helper, so verifyDocFreq collects the leaf
//     Terms and assembles a MultiTerms explicitly.
func TestSegmentTermEnum(t *testing.T) {
	// Pre-existing infrastructure gap: OpenDirectoryReader materialises each
	// segment via NewSegmentReader (index/directory_reader.go), which leaves
	// SegmentReader.coreReaders nil, so LeafReader.Terms returns "core readers
	// are nil". Same blocker documented in bag_of_positions_test.go. Unskip
	// once OpenDirectoryReader uses NewSegmentReaderWithCore.
	t.Fatal("blocked: OpenDirectoryReader builds SegmentReader without core readers; fix is NewSegmentReaderWithCore")

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// 100 docs with "aaa" and 100 with "aaa bbb": docFreq(aaa)=200, docFreq(bbb)=100.
	for i := 0; i < 100; i++ {
		segmentTermEnumAddDoc(t, writer, "aaa")
		segmentTermEnumAddDoc(t, writer, "aaa bbb")
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Verify docFreq in a multi-segment index.
	verifySegmentTermEnumDocFreq(t, dir)

	// Merge to a single segment, reopening the index in APPEND mode.
	appendConfig := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	appendConfig.SetOpenMode(index.APPEND)
	writer, err = index.NewIndexWriter(dir, appendConfig)
	if err != nil {
		t.Fatalf("Failed to reopen IndexWriter: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Verify docFreq in a single-segment index.
	verifySegmentTermEnumDocFreq(t, dir)
}

// TestSegmentTermEnumPrevTermAtEnd ports
// org.apache.lucene.index.TestSegmentTermEnum#testPrevTermAtEnd.
//
// It indexes one document with terms "aaa bbb" and walks the term dictionary
// to its end, verifying the terms surface in order and that re-seeking back to
// the last term succeeds.
//
// Divergences from Lucene:
//   - The Java test exercises TermsEnum.ord() / seekExact(long); Gocene's
//     TermsEnum interface (index/terms_enum.go) exposes neither, so the
//     ordinal-based re-seek is replaced by a term-based SeekExact, which
//     verifies the same "land back on bbb" behavior the Java assertions check.
//   - Lucene pins the codec via TestUtil.alwaysPostingsFormat; Gocene has a
//     single postings format, so no codec override is needed.
//   - MockAnalyzer / random Directory are replaced by WhitespaceAnalyzer and
//     SimpleFSDirectory, as in TestSegmentTermEnum.
func TestSegmentTermEnumPrevTermAtEnd(t *testing.T) {
	// Same OpenDirectoryReader core-readers gap as TestSegmentTermEnum.
	t.Fatal("blocked: OpenDirectoryReader builds SegmentReader without core readers; fix is NewSegmentReaderWithCore")

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	segmentTermEnumAddDoc(t, writer, "aaa bbb")
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}

	terms, err := leaves[0].LeafReader().Terms("content")
	if err != nil {
		t.Fatalf("Failed to get terms: %v", err)
	}
	termsEnum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("Failed to get terms iterator: %v", err)
	}

	term, err := termsEnum.Next()
	if err != nil {
		t.Fatalf("TermsEnum.Next() failed: %v", err)
	}
	if term == nil || term.Text() != "aaa" {
		t.Fatalf("first term = %v, want \"aaa\"", term)
	}

	term, err = termsEnum.Next()
	if err != nil {
		t.Fatalf("TermsEnum.Next() failed: %v", err)
	}
	if term == nil || term.Text() != "bbb" {
		t.Fatalf("second term = %v, want \"bbb\"", term)
	}

	// End of enumeration.
	term, err = termsEnum.Next()
	if err != nil {
		t.Fatalf("TermsEnum.Next() failed: %v", err)
	}
	if term != nil {
		t.Fatalf("term after bbb = %v, want nil", term)
	}

	// Re-seek to the last term. Java seeks by ord; Gocene has no ord, so the
	// seek is by term value, asserting the same return to "bbb".
	bbb := index.NewTerm("content", "bbb")
	found, err := termsEnum.SeekExact(bbb)
	if err != nil {
		t.Fatalf("SeekExact(bbb) failed: %v", err)
	}
	if !found {
		t.Fatalf("SeekExact(bbb) = false, want true")
	}
	if got := termsEnum.Term(); got == nil || got.Text() != "bbb" {
		t.Fatalf("after SeekExact, term = %v, want \"bbb\"", got)
	}
}
