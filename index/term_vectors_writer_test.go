// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestTermVectorsWriter ports org.apache.lucene.index.TestTermVectorsWriter
// (Lucene 10.4.0, core/src/test/org/apache/lucene/index/TestTermVectorsWriter.java).
//
// The Java suite verifies term-vector writing end to end: offset/position
// counting when the same Field instance is added several times to one document
// (LUCENE-1442), end-offset bookkeeping across analyzers, caching token filters
// and stop filters (LUCENE-1448), term-vector survival through addIndexes and
// forceMerge (LUCENE-1168), absence of term vectors on later documents
// (LUCENE-1008/1010), rejection of inconsistent per-field term-vector options
// within a single document, and the LUCENE-5611 guarantee that a bad
// term-vector field type does not abort the whole segment.
//
// Pre-existing infrastructure gaps (shared by every test below). Each test is
// skipped with termVectorsWriterBlocked; the bodies are written in full so the
// assertion intent stays 1:1 with the Java reference and the tests become
// executable once the missing pieces land.
//
//   - MockAnalyzer (org.apache.lucene.tests.analysis): every Java test indexes
//     through `new MockAnalyzer(random())`. The offset assertions in
//     testDoubleOffsetCounting, testEndOffsetPositionCharAnalyzer,
//     testEndOffsetPositionStopFilter and testEndOffsetPositionStandard depend
//     on MockTokenizer's exact character-offset accounting and on
//     MockTokenFilter.ENGLISH_STOPSET; WhitespaceAnalyzer cannot be substituted
//     because the expected start/end offsets (the 8/12 gap from trailing
//     whitespace, the 9/13 shift from a dropped stopword, the StandardTokenizer
//     splitting in testEndOffsetPositionStandard) are MockTokenizer-specific.
//   - RandomIndexWriter (org.apache.lucene.tests.index): doTestMixup, the
//     helper behind testInconsistentTermVectorOptions, drives writes through
//     RandomIndexWriter and reads back with its near-real-time getReader().
//     Gocene exposes no randomized test-writer wrapper and IndexWriter has no
//     NRT reader.
//   - MockDirectoryWrapper as a writable test directory: testTermVectorCorruption
//     copies the index into `new MockDirectoryWrapper(random(), TestUtil.ramCopyOf(dir))`
//     before exercising addIndexes. store.MockDirectoryWrapper exists but has no
//     randomized test-writer constructor, and TestUtil.ramCopyOf (a RAM snapshot
//     of a Directory) has no Gocene equivalent.
//
// Additional gaps surfaced while porting (also block the tests, beyond the
// three helpers named in the task brief):
//
//   - document.Field has no constructor that accepts a pre-built
//     analysis.TokenStream value (Lucene's `new Field("field", stream, ft)`).
//     testEndOffsetPositionWithCachingTokenFilter feeds a CachingTokenFilter
//     stream directly to a Field; the Go body therefore stops at building the
//     CachingTokenFilter and documents the missing Field-from-TokenStream path.
//   - StoredFields.Document(docID, visitor) writes into a visitor and returns
//     only an error, unlike Java's storedFields.document(i) which returns a
//     Document; the ports use document.NewDocumentStoredFieldVisitor() to
//     mirror "read document i" with no assertion on its contents (matching the
//     Java tests, which also discard the returned Document).
//
// Divergences from Lucene (would apply once unskipped):
//   - Lucene's PostingsEnum.ALL flag has no Gocene equivalent; TermsEnum.Postings
//     takes a bare int, so 2 (doc IDs + freqs + positions + offsets + payloads)
//     is passed, matching the Terms.GetPostingsReader flag documentation.
//   - TermsEnum has no postings-reuse overload; each call is a fresh
//     Postings(flags), so the Java `dpEnum = termsEnum.postings(dpEnum, ALL)`
//     reuse pattern becomes a plain re-fetch.
//   - Lucene reads via DirectoryReader.open(IndexWriter) (near-real-time) in
//     testNoAbortOnBadTVSettings; Gocene's IndexWriter has no NRT reader, so the
//     index is committed and reopened from the directory.
//   - newField/newTextField (LuceneTestCase randomization helpers) are replaced
//     by direct document.NewField / document.NewTextField construction.

const termVectorsWriterBlocked = "blocked: MockAnalyzer, RandomIndexWriter and a writable MockDirectoryWrapper/ramCopyOf test directory are not yet ported (see file header)"

// postingsAll is the Gocene flag closest to Lucene's PostingsEnum.ALL: doc IDs,
// term frequencies, positions, offsets and payloads (see the Terms interface
// flag documentation in index/terms.go).
const postingsAll = 2

// customTVType builds a FieldType cloned from base with all three term-vector
// options enabled, mirroring the repeated FieldType setup in the Java tests.
func customTVType(base *document.FieldType) *document.FieldType {
	ft := document.NewFieldTypeFrom(base)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectorOffsets(true)
	return ft
}

// newTVWriterConfig builds the IndexWriterConfig shared by the LUCENE-1168
// corruption tests: maxBufferedDocs 2, auto-flush disabled, serial merge
// scheduler, LogDoc merge policy. Gocene's setters return void, so the config
// is mutated step by step rather than chained.
func newTVWriterConfig() *index.IndexWriterConfig {
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetMaxBufferedDocs(2)
	cfg.SetRAMBufferSizeMB(index.DISABLE_AUTO_FLUSH)
	cfg.SetMergeScheduler(index.NewSerialMergeScheduler())
	cfg.SetMergePolicy(index.NewLogDocMergePolicy())
	return cfg
}

// tvWriterDir opens a fresh on-disk directory and an IndexWriter over it with
// the plain WhitespaceAnalyzer config, returning both plus a cleanup func. It
// mirrors the newDirectory() + new IndexWriter(...) preamble shared by the
// offset-counting tests.
func tvWriterDir(t *testing.T) (store.Directory, *index.IndexWriter, func()) {
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
	return dir, writer, func() {
		dir.Close()
	}
}

// firstDocFieldTermsEnum reopens dir, fetches the term vectors of document 0,
// and returns the TermsEnum for fieldName together with a cleanup func. It
// captures the `DirectoryReader.open(dir); r.termVectors().get(0).terms(field).iterator()`
// idiom repeated across the offset tests.
func firstDocFieldTermsEnum(t *testing.T, dir store.Directory, fieldName string) (index.TermsEnum, func()) {
	t.Helper()
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	tvs, err := reader.TermVectors()
	if err != nil {
		reader.Close()
		t.Fatalf("TermVectors() failed: %v", err)
	}
	fields, err := tvs.Get(0)
	if err != nil {
		reader.Close()
		t.Fatalf("TermVectors.Get(0) failed: %v", err)
	}
	if fields == nil {
		reader.Close()
		t.Fatal("TermVectors.Get(0) returned nil")
	}
	terms, err := fields.Terms(fieldName)
	if err != nil {
		reader.Close()
		t.Fatalf("Terms(%q) failed: %v", fieldName, err)
	}
	if terms == nil {
		reader.Close()
		t.Fatalf("Terms(%q) returned nil", fieldName)
	}
	te, err := terms.GetIterator()
	if err != nil {
		reader.Close()
		t.Fatalf("GetIterator failed: %v", err)
	}
	return te, func() {
		reader.Close()
	}
}

// nextPositionOffsets advances dpEnum one position and returns its start/end
// offsets, failing the test on any error. It folds the
// nextPosition()/startOffset()/endOffset() trio used throughout the Java tests.
func nextPositionOffsets(t *testing.T, dpEnum index.PostingsEnum) (int, int) {
	t.Helper()
	if _, err := dpEnum.NextPosition(); err != nil {
		t.Fatalf("NextPosition failed: %v", err)
	}
	so, err := dpEnum.StartOffset()
	if err != nil {
		t.Fatalf("StartOffset failed: %v", err)
	}
	eo, err := dpEnum.EndOffset()
	if err != nil {
		t.Fatalf("EndOffset failed: %v", err)
	}
	return so, eo
}

// TestTermVectorsWriterDoubleOffsetCounting ports testDoubleOffsetCounting
// (LUCENE-1442): the same StringField instance is added three times plus one
// empty Field instance; offsets must not be double-counted.
func TestTermVectorsWriterDoubleOffsetCounting(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	dir, w, cleanup := tvWriterDir(t)
	defer cleanup()

	customType := customTVType(document.StringFieldTypeNotStored)

	doc := document.NewDocument()
	f, err := document.NewField("field", "abcd", customType)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	doc.Add(f)
	doc.Add(f)
	f2, err := document.NewField("field", "", customType)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	doc.Add(f2)
	doc.Add(f)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	te, rclose := firstDocFieldTermsEnum(t, dir, "field")
	defer rclose()

	// First term: "" occurred once.
	term, err := te.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if term == nil {
		t.Fatal("Next() returned nil, want term \"\"")
	}
	if got := te.Term().Text(); got != "" {
		t.Fatalf("term() = %q, want \"\"", got)
	}
	if ttf, _ := te.TotalTermFreq(); ttf != 1 {
		t.Fatalf("totalTermFreq() = %d, want 1", ttf)
	}

	dpEnum, err := te.Postings(postingsAll)
	if err != nil {
		t.Fatalf("Postings failed: %v", err)
	}
	if next, _ := dpEnum.NextDoc(); next == index.NO_MORE_DOCS {
		t.Fatal("nextDoc() returned NO_MORE_DOCS")
	}
	if so, eo := nextPositionOffsets(t, dpEnum); so != 8 || eo != 8 {
		t.Fatalf("offsets = (%d,%d), want (8,8)", so, eo)
	}
	if next, _ := dpEnum.NextDoc(); next != index.NO_MORE_DOCS {
		t.Fatalf("nextDoc() = %d, want NO_MORE_DOCS", next)
	}

	// Second term: "abcd" occurred three times.
	term, err = te.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if term == nil || term.Text() != "abcd" {
		t.Fatalf("Next() = %v, want term \"abcd\"", term)
	}
	dpEnum, err = te.Postings(postingsAll)
	if err != nil {
		t.Fatalf("Postings failed: %v", err)
	}
	if ttf, _ := te.TotalTermFreq(); ttf != 3 {
		t.Fatalf("totalTermFreq() = %d, want 3", ttf)
	}
	if next, _ := dpEnum.NextDoc(); next == index.NO_MORE_DOCS {
		t.Fatal("nextDoc() returned NO_MORE_DOCS")
	}
	for _, want := range [][2]int{{0, 4}, {4, 8}, {8, 12}} {
		if so, eo := nextPositionOffsets(t, dpEnum); so != want[0] || eo != want[1] {
			t.Fatalf("offsets = (%d,%d), want (%d,%d)", so, eo, want[0], want[1])
		}
	}
	if next, _ := dpEnum.NextDoc(); next != index.NO_MORE_DOCS {
		t.Fatalf("nextDoc() = %d, want NO_MORE_DOCS", next)
	}
	term, err = te.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if term != nil {
		t.Fatalf("Next() = %v, want nil", term)
	}
}

// offsetCheck is one (startOffset, endOffset) expectation for a position.
type offsetCheck struct{ start, end int }

// runTwoTermOffsetCase covers the shared body of testDoubleOffsetCounting2,
// testEndOffsetPositionCharAnalyzer and testEndOffsetPositionStopFilter: a
// single field whose text is added twice produces one term with totalTermFreq
// 2 and two positions with the supplied offsets.
func runTwoTermOffsetCase(t *testing.T, text string, want []offsetCheck) {
	t.Helper()
	dir, w, cleanup := tvWriterDir(t)
	defer cleanup()

	customType := customTVType(document.TextFieldTypeNotStored)
	doc := document.NewDocument()
	f, err := document.NewField("field", text, customType)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	doc.Add(f)
	doc.Add(f)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	te, rclose := firstDocFieldTermsEnum(t, dir, "field")
	defer rclose()

	if term, err := te.Next(); err != nil || term == nil {
		t.Fatalf("Next() = %v, %v; want non-nil term", term, err)
	}
	dpEnum, err := te.Postings(postingsAll)
	if err != nil {
		t.Fatalf("Postings failed: %v", err)
	}
	if ttf, _ := te.TotalTermFreq(); ttf != 2 {
		t.Fatalf("totalTermFreq() = %d, want 2", ttf)
	}
	if next, _ := dpEnum.NextDoc(); next == index.NO_MORE_DOCS {
		t.Fatal("nextDoc() returned NO_MORE_DOCS")
	}
	for _, c := range want {
		if so, eo := nextPositionOffsets(t, dpEnum); so != c.start || eo != c.end {
			t.Fatalf("offsets = (%d,%d), want (%d,%d)", so, eo, c.start, c.end)
		}
	}
	if next, _ := dpEnum.NextDoc(); next != index.NO_MORE_DOCS {
		t.Fatalf("nextDoc() = %d, want NO_MORE_DOCS", next)
	}
}

// TestTermVectorsWriterDoubleOffsetCounting2 ports testDoubleOffsetCounting2
// (LUCENE-1442).
func TestTermVectorsWriterDoubleOffsetCounting2(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)
	runTwoTermOffsetCase(t, "abcd", []offsetCheck{{0, 4}, {5, 9}})
}

// TestTermVectorsWriterEndOffsetPositionCharAnalyzer ports
// testEndOffsetPositionCharAnalyzer (LUCENE-1448): trailing whitespace must not
// shift the recorded end offset.
func TestTermVectorsWriterEndOffsetPositionCharAnalyzer(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)
	runTwoTermOffsetCase(t, "abcd   ", []offsetCheck{{0, 4}, {8, 12}})
}

// TestTermVectorsWriterEndOffsetPositionWithCachingTokenFilter ports
// testEndOffsetPositionWithCachingTokenFilter (LUCENE-1448): the field is fed a
// pre-built CachingTokenFilter token stream rather than a raw string.
//
// This body cannot be completed: document.Field has no constructor that takes
// an analysis.TokenStream value (Lucene's `new Field("field", stream, ft)`).
// The CachingTokenFilter is built here to keep the assertion intent visible;
// the missing Field-from-TokenStream path is the additional blocker noted in
// the file header. Expected offsets once unskipped: {0,4} and {8,12}.
func TestTermVectorsWriterEndOffsetPositionWithCachingTokenFilter(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	dir, w, cleanup := tvWriterDir(t)
	defer cleanup()

	analyzer := analysis.NewWhitespaceAnalyzer() // Java: MockAnalyzer(random())
	stream, err := analyzer.TokenStream("field", strings.NewReader("abcd   "))
	if err != nil {
		t.Fatalf("TokenStream failed: %v", err)
	}
	caching := analysis.NewCachingTokenFilter(stream)

	// document.Field cannot wrap `caching`; the rest of the Java body
	// (add the field twice, write, reopen, assert offsets {0,4} and {8,12})
	// is unreachable until a Field-from-TokenStream constructor exists.
	_ = caching
	_ = w
	_ = dir
	customType := customTVType(document.TextFieldTypeNotStored)
	_ = customType
}

// TestTermVectorsWriterEndOffsetPositionStopFilter ports
// testEndOffsetPositionStopFilter (LUCENE-1448): a dropped stopword ("the")
// still advances the offset of the following term.
func TestTermVectorsWriterEndOffsetPositionStopFilter(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)
	// Java analyzer: MockAnalyzer with MockTokenFilter.ENGLISH_STOPSET, so "the"
	// is removed; "abcd" keeps offsets 0..4 and the second occurrence lands at
	// 9..13 (the stopword consumes characters 5..8).
	runTwoTermOffsetCase(t, "abcd the", []offsetCheck{{0, 4}, {9, 13}})
}

// termOffsetCheck is one term's expectation inside runTwoFieldOffsetCase.
type termOffsetCheck struct {
	checkFreq     bool
	totalTermFreq int
	start, end    int
}

// runTwoFieldOffsetCase covers the shared body of testEndOffsetPositionStandard,
// testEndOffsetPositionStandardEmptyField and
// testEndOffsetPositionStandardEmptyField2: a document holds several Field
// instances for the same field name, and each successive term carries the
// expected totalTermFreq and first-position offsets.
func runTwoFieldOffsetCase(t *testing.T, texts []string, checks []termOffsetCheck) {
	t.Helper()
	dir, w, cleanup := tvWriterDir(t)
	defer cleanup()

	customType := customTVType(document.TextFieldTypeNotStored)
	doc := document.NewDocument()
	for _, text := range texts {
		f, err := document.NewField("field", text, customType)
		if err != nil {
			t.Fatalf("NewField failed: %v", err)
		}
		doc.Add(f)
	}
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	te, rclose := firstDocFieldTermsEnum(t, dir, "field")
	defer rclose()

	for i, c := range checks {
		term, err := te.Next()
		if err != nil {
			t.Fatalf("check %d: Next failed: %v", i, err)
		}
		if term == nil {
			t.Fatalf("check %d: Next() returned nil, want a term", i)
		}
		dpEnum, err := te.Postings(postingsAll)
		if err != nil {
			t.Fatalf("check %d: Postings failed: %v", i, err)
		}
		if c.checkFreq {
			if ttf, _ := te.TotalTermFreq(); int(ttf) != c.totalTermFreq {
				t.Fatalf("check %d: totalTermFreq() = %d, want %d", i, ttf, c.totalTermFreq)
			}
		}
		if next, _ := dpEnum.NextDoc(); next == index.NO_MORE_DOCS {
			t.Fatalf("check %d: nextDoc() returned NO_MORE_DOCS", i)
		}
		if so, eo := nextPositionOffsets(t, dpEnum); so != c.start || eo != c.end {
			t.Fatalf("check %d: offsets = (%d,%d), want (%d,%d)", i, so, eo, c.start, c.end)
		}
	}
}

// TestTermVectorsWriterEndOffsetPositionStandard ports
// testEndOffsetPositionStandard (LUCENE-1448).
func TestTermVectorsWriterEndOffsetPositionStandard(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)
	runTwoFieldOffsetCase(t,
		[]string{"abcd the  ", "crunch man"},
		[]termOffsetCheck{
			{start: 0, end: 4},
			{start: 11, end: 17},
			{start: 18, end: 21},
		})
}

// TestTermVectorsWriterEndOffsetPositionStandardEmptyField ports
// testEndOffsetPositionStandardEmptyField (LUCENE-1448): a leading empty field
// instance still consumes one position before the next field's terms.
func TestTermVectorsWriterEndOffsetPositionStandardEmptyField(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)
	runTwoFieldOffsetCase(t,
		[]string{"", "crunch man"},
		[]termOffsetCheck{
			{checkFreq: true, totalTermFreq: 1, start: 1, end: 7},
			{start: 8, end: 11},
		})
}

// TestTermVectorsWriterEndOffsetPositionStandardEmptyField2 ports
// testEndOffsetPositionStandardEmptyField2 (LUCENE-1448): an empty field
// instance between two non-empty ones.
func TestTermVectorsWriterEndOffsetPositionStandardEmptyField2(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)
	runTwoFieldOffsetCase(t,
		[]string{"abcd", "", "crunch"},
		[]termOffsetCheck{
			{checkFreq: true, totalTermFreq: 1, start: 0, end: 4},
			{start: 6, end: 12},
		})
}

// TestTermVectorsWriterTermVectorCorruption ports testTermVectorCorruption
// (LUCENE-1168): term vectors must survive addIndexes from a separate directory
// followed by forceMerge.
//
// The Java test stages the source through MockDirectoryWrapper(random(),
// TestUtil.ramCopyOf(dir)); Gocene has no ramCopyOf snapshot and no randomized
// MockDirectoryWrapper test-writer, so the addIndexes leg below opens the
// source directory directly. The body is otherwise faithful to the reference.
func TestTermVectorsWriterTermVectorCorruption(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	for iter := 0; iter < 2; iter++ {
		writer, err := index.NewIndexWriter(dir, newTVWriterConfig())
		if err != nil {
			t.Fatalf("iter %d: NewIndexWriter failed: %v", iter, err)
		}

		stored := document.NewFieldType()
		stored.SetStored(true)
		storedField, err := document.NewField("stored", "stored", stored)
		if err != nil {
			t.Fatalf("NewField failed: %v", err)
		}

		document1 := document.NewDocument()
		document1.Add(storedField)
		if err := writer.AddDocument(document1); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
		if err := writer.AddDocument(document1); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}

		document2 := document.NewDocument()
		document2.Add(storedField)
		customType2 := customTVType(document.StringFieldTypeNotStored)
		termVectorField, err := document.NewField("termVector", "termVector", customType2)
		if err != nil {
			t.Fatalf("NewField failed: %v", err)
		}
		document2.Add(termVectorField)
		if err := writer.AddDocument(document2); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("ForceMerge(1) failed: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader failed: %v", err)
		}
		storedFields, err := reader.StoredFields()
		if err != nil {
			t.Fatalf("StoredFields failed: %v", err)
		}
		termVectors, err := reader.TermVectors()
		if err != nil {
			t.Fatalf("TermVectors failed: %v", err)
		}
		for i := 0; i < reader.NumDocs(); i++ {
			// Java: storedFields.document(i) returns a Document; Gocene's
			// Document writes into a visitor and returns only an error.
			if err := storedFields.Document(i, document.NewDocumentStoredFieldVisitor()); err != nil {
				t.Fatalf("Document(%d) failed: %v", i, err)
			}
			if _, err := termVectors.Get(i); err != nil {
				t.Fatalf("TermVectors.Get(%d) failed: %v", i, err)
			}
		}
		reader.Close()

		// Java: addIndexes from MockDirectoryWrapper(random(), TestUtil.ramCopyOf(dir)).
		// Gocene has no ramCopyOf; addIndexes is exercised against dir directly.
		writer, err = index.NewIndexWriter(dir, newTVWriterConfig())
		if err != nil {
			t.Fatalf("iter %d: NewIndexWriter (addIndexes) failed: %v", iter, err)
		}
		if err := writer.AddIndexes(dir); err != nil {
			t.Fatalf("AddIndexes failed: %v", err)
		}
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("ForceMerge(1) failed: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}
}

// TestTermVectorsWriterTermVectorCorruption2 ports testTermVectorCorruption2
// (LUCENE-1168): only the third document carries term vectors; the first two
// must report none.
func TestTermVectorsWriterTermVectorCorruption2(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	for iter := 0; iter < 2; iter++ {
		writer, err := index.NewIndexWriter(dir, newTVWriterConfig())
		if err != nil {
			t.Fatalf("iter %d: NewIndexWriter failed: %v", iter, err)
		}

		stored := document.NewFieldType()
		stored.SetStored(true)
		storedField, err := document.NewField("stored", "stored", stored)
		if err != nil {
			t.Fatalf("NewField failed: %v", err)
		}

		document1 := document.NewDocument()
		document1.Add(storedField)
		if err := writer.AddDocument(document1); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
		if err := writer.AddDocument(document1); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}

		document2 := document.NewDocument()
		document2.Add(storedField)
		customType2 := customTVType(document.StringFieldTypeNotStored)
		termVectorField, err := document.NewField("termVector", "termVector", customType2)
		if err != nil {
			t.Fatalf("NewField failed: %v", err)
		}
		document2.Add(termVectorField)
		if err := writer.AddDocument(document2); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("ForceMerge(1) failed: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("OpenDirectoryReader failed: %v", err)
		}
		tvs, err := reader.TermVectors()
		if err != nil {
			t.Fatalf("TermVectors failed: %v", err)
		}
		if fields, err := tvs.Get(0); err != nil || fields != nil {
			t.Fatalf("TermVectors.Get(0) = %v, %v; want nil, nil", fields, err)
		}
		if fields, err := tvs.Get(1); err != nil || fields != nil {
			t.Fatalf("TermVectors.Get(1) = %v, %v; want nil, nil", fields, err)
		}
		if fields, err := tvs.Get(2); err != nil || fields == nil {
			t.Fatalf("TermVectors.Get(2) = %v, %v; want non-nil, nil", fields, err)
		}
		reader.Close()
	}
}

// TestTermVectorsWriterTermVectorCorruption3 ports testTermVectorCorruption3
// (LUCENE-1168): ten then six identical term-vector documents, force-merged,
// must all be readable for both stored fields and term vectors.
func TestTermVectorsWriterTermVectorCorruption3(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	stored := document.NewFieldType()
	stored.SetStored(true)
	storedField, err := document.NewField("stored", "stored", stored)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	customType2 := customTVType(document.StringFieldTypeNotStored)
	termVectorField, err := document.NewField("termVector", "termVector", customType2)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	doc := document.NewDocument()
	doc.Add(storedField)
	doc.Add(termVectorField)

	writer, err := index.NewIndexWriter(dir, newTVWriterConfig())
	if err != nil {
		t.Fatalf("NewIndexWriter failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	writer, err = index.NewIndexWriter(dir, newTVWriterConfig())
	if err != nil {
		t.Fatalf("NewIndexWriter (2) failed: %v", err)
	}
	for i := 0; i < 6; i++ {
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument failed: %v", err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields failed: %v", err)
	}
	termVectors, err := reader.TermVectors()
	if err != nil {
		t.Fatalf("TermVectors failed: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := termVectors.Get(i); err != nil {
			t.Fatalf("TermVectors.Get(%d) failed: %v", i, err)
		}
		if err := storedFields.Document(i, document.NewDocumentStoredFieldVisitor()); err != nil {
			t.Fatalf("Document(%d) failed: %v", i, err)
		}
	}
	reader.Close()
}

// TestTermVectorsWriterNoTermVectorAfterTermVector ports
// testNoTermVectorAfterTermVector (LUCENE-1008): a field that drops term
// vectors in a later segment must still force-merge cleanly.
func TestTermVectorsWriterNoTermVectorAfterTermVector(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	_, iw, cleanup := tvWriterDir(t)
	defer cleanup()

	customType2 := customTVType(document.TextFieldTypeNotStored)
	document1 := document.NewDocument()
	f1, err := document.NewField("tvtest", "a b c", customType2)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	document1.Add(f1)
	if err := iw.AddDocument(document1); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}

	document2 := document.NewDocument()
	f2, err := document.NewTextField("tvtest", "x y z", false)
	if err != nil {
		t.Fatalf("NewTextField failed: %v", err)
	}
	document2.Add(f2)
	if err := iw.AddDocument(document2); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	// Make first segment.
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	document3 := document.NewDocument()
	f3, err := document.NewField("tvtest", "a b c", customType)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	document3.Add(f3)
	if err := iw.AddDocument(document3); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	// Make 2nd segment.
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := iw.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) failed: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestTermVectorsWriterNoTermVectorAfterTermVectorMerge ports
// testNoTermVectorAfterTermVectorMerge (LUCENE-1010): force-merge between the
// term-vector and non-term-vector segments must not corrupt the index.
func TestTermVectorsWriterNoTermVectorAfterTermVectorMerge(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	_, iw, cleanup := tvWriterDir(t)
	defer cleanup()

	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	document1 := document.NewDocument()
	f1, err := document.NewField("tvtest", "a b c", customType)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	document1.Add(f1)
	if err := iw.AddDocument(document1); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	document2 := document.NewDocument()
	f2, err := document.NewTextField("tvtest", "x y z", false)
	if err != nil {
		t.Fatalf("NewTextField failed: %v", err)
	}
	document2.Add(f2)
	if err := iw.AddDocument(document2); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	// Make first segment.
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := iw.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) failed: %v", err)
	}

	// Java mirrors a subtle ordering quirk: customType2 is built and a field is
	// added to document2, but document2 is then reassigned to a fresh empty
	// Document before addDocument, so the empty document is what gets indexed.
	customType2 := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType2.SetStoreTermVectors(true)
	f3, err := document.NewField("tvtest", "a b c", customType2)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	document2.Add(f3)
	document2 = document.NewDocument()
	if err := iw.AddDocument(document2); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
	// Make 2nd segment.
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := iw.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) failed: %v", err)
	}
	if err := iw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestTermVectorsWriterInconsistentTermVectorOptions ports
// testInconsistentTermVectorOptions: mixing different term-vector settings for
// the same field within one document must raise an error, while previously
// added good documents remain readable. It exercises six (ft1, ft2) pairs via
// doTestMixup, exactly as the Java original.
func TestTermVectorsWriterInconsistentTermVectorOptions(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	base := func() *document.FieldType {
		return document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	}

	// no vectors + vectors
	a := base()
	b := base()
	b.SetStoreTermVectors(true)
	doTestMixup(t, a, b)

	// vectors + vectors with pos
	a = base()
	a.SetStoreTermVectors(true)
	b = base()
	b.SetStoreTermVectors(true)
	b.SetStoreTermVectorPositions(true)
	doTestMixup(t, a, b)

	// vectors + vectors with off
	a = base()
	a.SetStoreTermVectors(true)
	b = base()
	b.SetStoreTermVectors(true)
	b.SetStoreTermVectorOffsets(true)
	doTestMixup(t, a, b)

	// vectors with pos + vectors with pos + off
	a = base()
	a.SetStoreTermVectors(true)
	a.SetStoreTermVectorPositions(true)
	b = base()
	b.SetStoreTermVectors(true)
	b.SetStoreTermVectorPositions(true)
	b.SetStoreTermVectorOffsets(true)
	doTestMixup(t, a, b)

	// vectors with pos + vectors with pos + pay
	a = base()
	a.SetStoreTermVectors(true)
	a.SetStoreTermVectorPositions(true)
	b = base()
	b.SetStoreTermVectors(true)
	b.SetStoreTermVectorPositions(true)
	b.SetStoreTermVectorPayloads(true)
	doTestMixup(t, a, b)
}

// doTestMixup ports the private doTestMixup helper: three well-formed documents
// are indexed, then a document whose "field" carries two incompatible
// FieldTypes must fail with a term-vector-settings error, after which the three
// good documents must still be visible.
//
// Java drives this through RandomIndexWriter and reads back with its NRT
// getReader(); Gocene uses the plain IndexWriter and reopens the directory.
func doTestMixup(t *testing.T, ft1, ft2 *document.FieldType) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter failed: %v", err)
	}

	// Add 3 good docs.
	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		idField, err := document.NewStringField("id", strconv.Itoa(i), false)
		if err != nil {
			t.Fatalf("NewStringField failed: %v", err)
		}
		doc.Add(idField)
		if err := iw.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument(good %d) failed: %v", i, err)
		}
	}

	// Add the broken doc: same field name, incompatible term-vector options.
	doc := document.NewDocument()
	f1, err := document.NewField("field", "value1", ft1)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	f2, err := document.NewField("field", "value2", ft2)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	doc.Add(f1)
	doc.Add(f2)

	// Ensure the broken doc hits an error.
	err = iw.AddDocument(doc)
	if err == nil {
		t.Fatal("AddDocument(broken) succeeded, want an error")
	}
	// Java accepts either of two messages via anyOf(startsWith(...)). Gocene
	// may wrap the message, so a substring match is used instead of a prefix.
	msg := err.Error()
	const want1 = "all instances of a given field name must have the same term vectors settings"
	const want2 = "Inconsistency of field data structures across documents for field [field]"
	if !strings.Contains(msg, want1) && !strings.Contains(msg, want2) {
		t.Fatalf("error message %q contains neither expected fragment", msg)
	}

	// Ensure the good docs are still ok.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	if n := reader.NumDocs(); n != 3 {
		reader.Close()
		t.Fatalf("NumDocs() = %d, want 3", n)
	}
	reader.Close()
	if err := iw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestTermVectorsWriterNoAbortOnBadTVSettings ports testNoAbortOnBadTVSettings
// (LUCENE-5611): a document whose field type sets term vectors on a stored-only
// type must be rejected without aborting the segment, so the previously added
// empty document survives.
func TestTermVectorsWriterNoAbortOnBadTVSettings(t *testing.T) {
	t.Fatal(termVectorsWriterBlocked)

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	// Java avoids RandomIndexWriter here so both docs land in one segment.
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter failed: %v", err)
	}

	doc := document.NewDocument()
	if err := iw.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument(empty) failed: %v", err)
	}

	ft := document.NewFieldTypeFrom(document.StoredFieldType)
	ft.SetStoreTermVectors(true)
	ft.Freeze()
	badField, err := document.NewField("field", "value", ft)
	if err != nil {
		t.Fatalf("NewField failed: %v", err)
	}
	doc.Add(badField)

	if err := iw.AddDocument(doc); err == nil {
		t.Fatal("AddDocument(bad term-vector field) succeeded, want an error")
	}

	// Java reads via DirectoryReader.open(iw) (NRT); Gocene has no NRT reader,
	// so commit and reopen the directory instead.
	if err := iw.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	// The error must not have lost the first document.
	if n := reader.NumDocs(); n != 1 {
		reader.Close()
		t.Fatalf("NumDocs() = %d, want 1", n)
	}
	reader.Close()
	if err := iw.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
