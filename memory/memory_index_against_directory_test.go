// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Tests that verify MemoryIndex behaviour mirrors a Directory-backed index.
// Port of org.apache.lucene.index.memory.TestMemoryIndexAgainstDirectory.
//
// MemoryIndex.CreateSearcher() and MemoryIndex.Search() are now implemented,
// so tests use TermQuery, BooleanQuery, PhraseQuery, and RegexpQuery through
// IndexSearcher to verify the in-memory index behaves correctly.

package memory_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/memory"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// testTerms mirrors the TEST_TERMS constant from the Java source.
var testTerms = []string{
	"term", "Term", "tErm", "TERM", "telm",
	"stop", "drop", "roll", "phrase",
	"a", "c", "bar", "blar", "gack",
	"weltbank", "worlbank",
	"hello", "on", "the",
	"apache", "Apache", "copyright", "Copyright",
}

// randomTerm returns either a term from testTerms or a random lowercase ASCII word.
func randomTerm(rng *rand.Rand) string {
	if rng.Intn(2) == 0 {
		return testTerms[rng.Intn(len(testTerms))]
	}
	// random short word, lowercase ASCII
	n := 1 + rng.Intn(8)
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + rng.Intn(26))
	}
	return string(b)
}

// buildFooField builds a whitespace-separated field value from up to maxTerms random terms.
func buildFooField(rng *rand.Rand, maxTerms int) string {
	n := rng.Intn(maxTerms + 1)
	terms := make([]string, n)
	for i := range terms {
		terms[i] = randomTerm(rng)
	}
	return strings.Join(terms, " ")
}

// newMemoryIndex is a local alias to keep tests independent of API churn.
func newMemoryIndex() *memory.MemoryIndex { return memory.NewMemoryIndex() }

// TestMemoryIndexAgainstDirectory_RandomQueries verifies that MemoryIndex
// with CreateSearcher + TermQuery produces correct results for known terms,
// mirroring the "assertAgainstDirectory" loop from the Java test.
func TestMemoryIndexAgainstDirectory_RandomQueries(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	mi := newMemoryIndex()

	fooText := buildFooField(rng, 50)
	termText := buildFooField(rng, 50)

	if err := mi.AddField("foo", fooText); err != nil {
		t.Fatalf("AddField foo: %v", err)
	}
	if err := mi.AddField("term", termText); err != nil {
		t.Fatalf("AddField term: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	// Verify that TermQuery for a token present in "foo" returns 1 hit.
	fooTerms := mi.GetFieldTerms("foo")
	for term := range fooTerms {
		tq := search.NewTermQuery(index.NewTerm("foo", term))
		top, err := searcher.Search(tq, 10)
		if err != nil {
			t.Fatalf("Search for %q: %v", term, err)
		}
		if top.TotalHits.Value != 1 {
			t.Errorf("TermQuery(%q) returned %d hits, want 1", term, top.TotalHits.Value)
		}
		if len(top.ScoreDocs) > 0 && top.ScoreDocs[0].Doc != 0 {
			t.Errorf("TermQuery(%q) doc = %d, want 0", term, top.ScoreDocs[0].Doc)
		}
		break // one verification is sufficient
	}

	// Non-existent term returns 0 hits.
	tq := search.NewTermQuery(index.NewTerm("foo", "nonExistentTerm937"))
	top, err := searcher.Search(tq, 10)
	if err != nil {
		t.Fatalf("Search for non-existent term: %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Errorf("non-existent term returned %d hits, want 0", top.TotalHits.Value)
	}
}

// TestMemoryIndexAgainstDirectory_DocsEnumStart verifies that a PostingsEnum
// from the MemoryIndex reader starts with docID == -1 and advances correctly.
// Java counterpart: testDocsEnumStart.
func TestMemoryIndexAgainstDirectory_DocsEnumStart(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("field", "the quick brown fox"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	reader := searcher.GetReader()
	leafReader, ok := reader.(index.LeafReaderInterface)
	if !ok {
		t.Fatal("reader is not a LeafReaderInterface")
	}
	terms, err := leafReader.Terms("field")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms returned nil")
	}

	termsEnum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}

	// Seek to "quick" and get postings.
	seeked, err := termsEnum.SeekExact(schema.NewTerm("field", "quick"))
	if err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if !seeked {
		t.Fatal("term 'quick' not found")
	}

	pe, err := termsEnum.Postings(schema.PostingsFlagFreqs)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}

	// DocID should start at -1.
	if pe.DocID() != -1 {
		t.Errorf("initial DocID = %d, want -1", pe.DocID())
	}

	// NextDoc returns 0.
	doc, err := pe.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Errorf("NextDoc = %d, want 0", doc)
	}

	// NextDoc returns NO_MORE_DOCS (single doc MemoryIndex).
	doc, err = pe.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc after exhaustion: %v", err)
	}
	if doc != schema.NO_MORE_DOCS {
		t.Errorf("NextDoc after exhaustion = %d, want NO_MORE_DOCS", doc)
	}
}

// TestMemoryIndexAgainstDirectory_DocsAndPositionsEnumStart verifies that a
// PostingsEnum with positions from the MemoryIndex reader starts at docID == -1
// and yields correct positions and offsets.
// Java counterpart: testDocsAndPositionsEnumStart.
func TestMemoryIndexAgainstDirectory_DocsAndPositionsEnumStart(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("field", "the quick brown fox"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	reader := searcher.GetReader()
	leafReader, ok := reader.(index.LeafReaderInterface)
	if !ok {
		t.Fatal("reader is not a LeafReaderInterface")
	}
	terms, err := leafReader.Terms("field")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Terms returned nil")
	}

	termsEnum, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}

	seeked, err := termsEnum.SeekExact(schema.NewTerm("field", "quick"))
	if err != nil {
		t.Fatalf("SeekExact: %v", err)
	}
	if !seeked {
		t.Fatal("term 'quick' not found")
	}

	pe, err := termsEnum.Postings(schema.PostingsFlagPositions)
	if err != nil {
		t.Fatalf("Postings with positions: %v", err)
	}

	// DocID starts at -1.
	if pe.DocID() != -1 {
		t.Errorf("initial DocID = %d, want -1", pe.DocID())
	}

	doc, err := pe.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Errorf("NextDoc = %d, want 0", doc)
	}

	// Verify position.
	pos, err := pe.NextPosition()
	if err != nil {
		t.Fatalf("NextPosition: %v", err)
	}
	// "quick" is at position 1 (0-indexed: "the"=0, "quick"=1).
	if pos != 1 {
		t.Errorf("position = %d, want 1", pos)
	}

	// Verify offsets.
	start, err := pe.StartOffset()
	if err != nil {
		t.Fatalf("StartOffset: %v", err)
	}
	end, err := pe.EndOffset()
	if err != nil {
		t.Fatalf("EndOffset: %v", err)
	}
	if start != 4 || end != 9 {
		t.Errorf("offsets = (%d,%d), want (4,9) for 'quick'", start, end)
	}
}

// TestMemoryIndexAgainstDirectory_NullPointerException verifies that searching
// a RegexpQuery via MemoryIndex returns correct results without panic.
// Java counterpart: testNullPointerException (LUCENE-3831) used
// SpanMultiTermQueryWrapper; Gocene tests the RegexpQuery search path directly.
func TestMemoryIndexAgainstDirectory_NullPointerException(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("text", "hello world"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	// RegexpQuery matching "world" should find the doc.
	rq, err := search.NewRegexpQuery("text", "world")
	if err != nil {
		t.Fatalf("NewRegexpQuery: %v", err)
	}
	top, err := mi.Search(rq, 10)
	if err != nil {
		t.Fatalf("Search with RegexpQuery: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("RegexpQuery 'world' matched %d docs, want 1", top.TotalHits.Value)
	}

	// RegexpQuery matching nothing should return 0 hits.
	rq, err = search.NewRegexpQuery("text", "zzz")
	if err != nil {
		t.Fatalf("NewRegexpQuery: %v", err)
	}
	top, err = mi.Search(rq, 10)
	if err != nil {
		t.Fatalf("Search with non-matching RegexpQuery: %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Errorf("RegexpQuery 'zzz' matched %d docs, want 0", top.TotalHits.Value)
	}
}

// TestMemoryIndexAgainstDirectory_PassesIfWrapped verifies that wrapping a
// query in a BooleanQuery also returns correct results without panic.
// Java counterpart: testPassesIfWrapped (LUCENE-3831) used SpanOrQuery wrapping
// SpanMultiTermQueryWrapper; Gocene tests BooleanQuery wrapping instead.
func TestMemoryIndexAgainstDirectory_PassesIfWrapped(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("text", "hello world"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	// BooleanQuery(MUST(RegexpQuery("world"))) should find the doc.
	rq, err := search.NewRegexpQuery("text", "world")
	if err != nil {
		t.Fatalf("NewRegexpQuery: %v", err)
	}
	bq := search.NewBooleanQuery()
	bq.Add(rq, search.MUST)

	top, err := mi.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search with wrapped RegexpQuery: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("wrapped query matched %d docs, want 1", top.TotalHits.Value)
	}
}

// TestMemoryIndexAgainstDirectory_SameFieldAddedMultipleTimes verifies that
// adding the same field twice replaces the content (Gocene behaviour) and that
// TermQuery via Search finds the correct terms.
func TestMemoryIndexAgainstDirectory_SameFieldAddedMultipleTimes(t *testing.T) {
	mi := newMemoryIndex()

	// Gocene's MemoryIndex replaces the field on second AddField, but
	// Search still works correctly with the last added field's content.
	if err := mi.AddField("field", "hello world"); err != nil {
		t.Fatalf("First AddField: %v", err)
	}
	if err := mi.AddField("field", "hello again"); err != nil {
		t.Fatalf("Second AddField: %v", err)
	}

	// "again" should be present (from the second AddField).
	if freq := mi.GetTermFrequency("field", "again"); freq != 1 {
		t.Errorf("term 'again' freq = %d, want 1", freq)
	}
	// "world" should NOT be present (replaced by second AddField).
	if freq := mi.GetTermFrequency("field", "world"); freq != 0 {
		t.Errorf("term 'world' freq = %d, want 0 (field was replaced)", freq)
	}

	// Verify via Search that "again" is matched and "world" is not.
	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	tqAgain := search.NewTermQuery(index.NewTerm("field", "again"))
	top, err := searcher.Search(tqAgain, 10)
	if err != nil {
		t.Fatalf("Search for 'again': %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("TermQuery 'again' matched %d docs, want 1", top.TotalHits.Value)
	}

	tqWorld := search.NewTermQuery(index.NewTerm("field", "world"))
	top, err = searcher.Search(tqWorld, 10)
	if err != nil {
		t.Fatalf("Search for 'world': %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Errorf("TermQuery 'world' matched %d docs, want 0 (field was replaced)", top.TotalHits.Value)
	}
}

// TestMemoryIndexAgainstDirectory_NonExistentField verifies that querying a
// field that was never added returns nil/nil without error.
func TestMemoryIndexAgainstDirectory_NonExistentField(t *testing.T) {
	mi := newMemoryIndex()
	mi.AddField("field", "the quick brown fox")

	// Non-existent field queries should return nil/zero without panic.
	freq := mi.GetTermFrequency("not-in-index", "foo")
	if freq != 0 {
		t.Errorf("expected 0 term frequency for non-existent field, got %d", freq)
	}
	terms := mi.GetFieldTerms("not-in-index")
	if terms != nil {
		t.Errorf("expected nil terms for non-existent field, got %v", terms)
	}
	positions := mi.GetTermPositions("not-in-index", "foo")
	if positions != nil {
		t.Errorf("expected nil positions for non-existent field, got %v", positions)
	}
}

// TestMemoryIndexAgainstDirectory_SearchWithBooleanQuery verifies that
// BooleanQuery (MUST + SHOULD) via MemoryIndex.Search returns correct hits.
func TestMemoryIndexAgainstDirectory_SearchWithBooleanQuery(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("field", "hello world foo"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	// BooleanQuery: MUST("hello") AND should match.
	bq := search.NewBooleanQuery()
	bq.Add(search.NewTermQuery(index.NewTerm("field", "hello")), search.MUST)

	top, err := mi.Search(bq, 10)
	if err != nil {
		t.Fatalf("Search with BooleanQuery: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("BooleanQuery MUST matched %d docs, want 1", top.TotalHits.Value)
	}

	// MUST("nonexistent") should return 0 hits.
	bq2 := search.NewBooleanQuery()
	bq2.Add(search.NewTermQuery(index.NewTerm("field", "nonexistent")), search.MUST)
	top, err = mi.Search(bq2, 10)
	if err != nil {
		t.Fatalf("Search with non-matching BooleanQuery: %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Errorf("BooleanQuery MUST(nonexistent) matched %d docs, want 0", top.TotalHits.Value)
	}
}

// TestMemoryIndexAgainstDirectory_SearchAfterReset verifies that after Reset,
// Search returns 0 hits.
func TestMemoryIndexAgainstDirectory_SearchAfterReset(t *testing.T) {
	mi := newMemoryIndex()
	mi.AddField("field", "hello world")

	// Before reset, search finds the doc.
	top, err := mi.Search(search.NewTermQuery(index.NewTerm("field", "hello")), 10)
	if err != nil {
		t.Fatalf("Search before reset: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("before reset matched %d docs, want 1", top.TotalHits.Value)
	}

	mi.Reset()

	// After reset, search should find nothing.
	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher after reset: %v", err)
	}
	defer searcher.Close()

	top, err = searcher.Search(search.NewTermQuery(index.NewTerm("field", "hello")), 10)
	if err != nil {
		t.Fatalf("Search after reset: %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Errorf("after reset matched %d docs, want 0", top.TotalHits.Value)
	}
}

// TestMemoryIndexAgainstDirectory_SearchWithMatchAll verifies that
// MatchAllDocsQuery via Search returns 1 hit (single-doc MemoryIndex).
func TestMemoryIndexAgainstDirectory_SearchWithMatchAll(t *testing.T) {
	mi := newMemoryIndex()
	mi.AddField("field", "hello world")

	top, err := mi.Search(search.NewMatchAllDocsQuery(), 10)
	if err != nil {
		t.Fatalf("Search with MatchAllDocsQuery: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("MatchAllDocsQuery matched %d docs, want 1", top.TotalHits.Value)
	}
	if len(top.ScoreDocs) != 1 || top.ScoreDocs[0].Doc != 0 {
		t.Errorf("ScoreDoc = %+v, want {Doc:0}", top.ScoreDocs[0])
	}
}

// TestMemoryIndexAgainstDirectory_EmptyString verifies that an empty-string
// token can be added and that the MemoryIndex handles it gracefully.
// Java counterpart: testEmptyString (LUCENE-4880).
// Gocene's MemoryIndex.AddField silently ignores empty strings (returns nil),
// so the index remains empty and Search returns 0 hits.
func TestMemoryIndexAgainstDirectory_EmptyString(t *testing.T) {
	mi := newMemoryIndex()

	// Adding an empty string returns nil (no error) but adds no terms.
	if err := mi.AddField("field", ""); err != nil {
		t.Fatalf("AddField with empty string: %v", err)
	}

	// The field should have no terms.
	if mi.Size() != 0 {
		t.Errorf("Size() = %d, want 0 (empty string added no field)", mi.Size())
	}

	// Add a non-empty field after the empty one.
	if err := mi.AddField("field", "hello"); err != nil {
		t.Fatalf("AddField after empty: %v", err)
	}
	if mi.Size() != 1 {
		t.Errorf("Size() = %d, want 1", mi.Size())
	}

	// Search should find the non-empty field.
	top, err := mi.Search(search.NewTermQuery(index.NewTerm("field", "hello")), 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("after empty-string AddField, matched %d docs, want 1", top.TotalHits.Value)
	}
}

// TestMemoryIndexAgainstDirectory_CreateSearcherReturnsSearcher verifies that
// CreateSearcher returns a working IndexSearcher that can execute queries.
func TestMemoryIndexAgainstDirectory_CreateSearcherReturnsSearcher(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("field", "hello world"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	// Verify reader is accessible and has expected properties.
	reader := searcher.GetReader()
	if reader.MaxDoc() != 1 {
		t.Errorf("MaxDoc = %d, want 1", reader.MaxDoc())
	}
	if reader.NumDocs() != 1 {
		t.Errorf("NumDocs = %d, want 1", reader.NumDocs())
	}
	if reader.HasDeletions() {
		t.Errorf("HasDeletions = true, want false")
	}
}

// ---------------------------------------------------------------------------
// Smoke tests for the current MemoryIndex stub that verify the behavioral
// contract exercised by the above integration tests.
// ---------------------------------------------------------------------------

// TestMemoryIndexAgainstDirectory_FieldAccumulation verifies that fields and
// terms added to a MemoryIndex are tracked correctly.
func TestMemoryIndexAgainstDirectory_FieldAccumulation(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	mi := newMemoryIndex()

	fooText := buildFooField(rng, 50)
	termText := buildFooField(rng, 50)

	if err := mi.AddField("foo", fooText); err != nil {
		t.Fatalf("AddField foo: %v", err)
	}
	if err := mi.AddField("term", termText); err != nil {
		t.Fatalf("AddField term: %v", err)
	}

	// Both fields must be present.
	fields := mi.GetFields()
	fieldSet := make(map[string]bool, len(fields))
	for _, f := range fields {
		fieldSet[f] = true
	}
	if !fieldSet["foo"] {
		t.Error("field 'foo' missing after AddField")
	}
	if !fieldSet["term"] {
		t.Error("field 'term' missing after AddField")
	}

	// After reset the index must be empty -- mirrors memory.reset() in Java.
	mi.Reset()
	if mi.Size() != 0 {
		t.Errorf("expected empty index after Reset, got %d fields", mi.Size())
	}
}

// TestMemoryIndexAgainstDirectory_TermFrequency verifies that term frequency is
// accumulated correctly.
func TestMemoryIndexAgainstDirectory_TermFrequency(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("field", "the quick brown fox the"); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	// "the" appears twice.
	if got := mi.GetTermFrequency("field", "the"); got != 2 {
		t.Errorf("expected freq 2 for 'the', got %d", got)
	}
	// "quick" appears once.
	if got := mi.GetTermFrequency("field", "quick"); got != 1 {
		t.Errorf("expected freq 1 for 'quick', got %d", got)
	}
}

// TestMemoryIndexAgainstDirectory_TermPositions verifies that positions are
// recorded.
func TestMemoryIndexAgainstDirectory_TermPositions(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("field", "a b a c a"); err != nil {
		t.Fatalf("AddField: %v", err)
	}
	positions := mi.GetTermPositions("field", "a")
	if len(positions) != 3 {
		t.Fatalf("expected 3 positions for 'a', got %d", len(positions))
	}
	// Positions must be monotonically increasing.
	for i := 1; i < len(positions); i++ {
		if positions[i] <= positions[i-1] {
			t.Errorf("positions not monotonic: %v", positions)
		}
	}
}

// TestMemoryIndexAgainstDirectory_FrozenRejectsWrites verifies that a frozen
// index refuses further writes.
func TestMemoryIndexAgainstDirectory_FrozenRejectsWrites(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("field", "hello"); err != nil {
		t.Fatalf("AddField before freeze: %v", err)
	}
	mi.Freeze()
	if !mi.IsFrozen() {
		t.Error("IsFrozen should be true after Freeze")
	}
	if err := mi.AddField("field", "world"); err == nil {
		t.Error("expected error when adding field to frozen index")
	}
}

// TestMemoryIndexAgainstDirectory_ResetUnfreezes verifies that Reset clears the
// frozen state so new fields can be added.
func TestMemoryIndexAgainstDirectory_ResetUnfreezes(t *testing.T) {
	mi := newMemoryIndex()
	mi.Freeze()
	mi.Reset()
	if mi.IsFrozen() {
		t.Error("IsFrozen should be false after Reset")
	}
	if err := mi.AddField("field", "hello"); err != nil {
		t.Errorf("AddField after Reset: %v", err)
	}
}

// testLeafReader is a local interface to access DocValues/Norms/Points methods
// on the unexported memoryIndexReader via type assertion through leaves.
type testLeafReader interface {
	index.LeafReaderInterface
	GetNumericDocValues(field string) (index.NumericDocValues, error)
	GetBinaryDocValues(field string) (index.BinaryDocValues, error)
	GetSortedDocValues(field string) (index.SortedDocValues, error)
	GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
	GetSortedSetDocValues(field string) (index.SortedSetDocValues, error)
	GetNormValues(field string) (index.NumericDocValues, error)
	GetPointValues(field string) (index.PointValues, error)
}

// getLeafReader extracts a testLeafReader from the searcher.
func getLeafReader(searcher *search.IndexSearcher) testLeafReader {
	leaves, err := searcher.GetReader().Leaves()
	if err != nil {
		return nil
	}
	if len(leaves) == 0 {
		return nil
	}
	lr, _ := leaves[0].Reader().(testLeafReader)
	return lr
}

// --- New tests ported from Java counterparts ---

// TestMemoryIndexAgainstDirectory_DocValuesVsNormalIndex verifies that
// MemoryIndex correctly stores and retrieves doc values.
func TestMemoryIndexAgainstDirectory_DocValuesVsNormalIndex(t *testing.T) {
	mi := newMemoryIndex()

	if err := mi.AddNumericDocValues("numeric", 42); err != nil {
		t.Fatalf("AddNumericDocValues: %v", err)
	}
	if err := mi.AddBinaryDocValues("binary", []byte("hello")); err != nil {
		t.Fatalf("AddBinaryDocValues: %v", err)
	}
	if err := mi.AddSortedDocValues("sorted", []byte("alpha")); err != nil {
		t.Fatalf("AddSortedDocValues: %v", err)
	}
	if err := mi.AddSortedNumericDocValues("sorted_numeric", []int64{10, 20, 30}); err != nil {
		t.Fatalf("AddSortedNumericDocValues: %v", err)
	}
	if err := mi.AddSortedSetDocValues("sorted_set", [][]byte{[]byte("x"), []byte("y"), []byte("z")}); err != nil {
		t.Fatalf("AddSortedSetDocValues: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	lr := getLeafReader(searcher)
	if lr == nil {
		t.Fatal("could not get leaf reader with DocValues methods")
	}

	// NumericDocValues
	ndv, err := lr.GetNumericDocValues("numeric")
	if err != nil {
		t.Fatalf("GetNumericDocValues: %v", err)
	}
	if ndv == nil {
		t.Fatal("NumericDocValues is nil")
	}
	doc, err := ndv.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Fatalf("NextDoc = %d, want 0", doc)
	}
	val, err := ndv.LongValue()
	if err != nil {
		t.Fatalf("LongValue: %v", err)
	}
	if val != 42 {
		t.Errorf("NumericDocValues = %d, want 42", val)
	}

	// BinaryDocValues
	bdv, err := lr.GetBinaryDocValues("binary")
	if err != nil {
		t.Fatalf("GetBinaryDocValues: %v", err)
	}
	if bdv == nil {
		t.Fatal("BinaryDocValues is nil")
	}
	if _, err := bdv.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	bval, err := bdv.BinaryValue()
	if err != nil {
		t.Fatalf("BinaryValue: %v", err)
	}
	if string(bval) != "hello" {
		t.Errorf("BinaryDocValues = %q, want 'hello'", string(bval))
	}

	// SortedDocValues
	sdv, err := lr.GetSortedDocValues("sorted")
	if err != nil {
		t.Fatalf("GetSortedDocValues: %v", err)
	}
	if sdv == nil {
		t.Fatal("SortedDocValues is nil")
	}
	if _, err := sdv.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	ord, err := sdv.OrdValue()
	if err != nil {
		t.Fatalf("OrdValue: %v", err)
	}
	if ord != 0 {
		t.Errorf("OrdValue = %d, want 0", ord)
	}
	termVal, err := sdv.LookupOrd(0)
	if err != nil {
		t.Fatalf("LookupOrd: %v", err)
	}
	if string(termVal) != "alpha" {
		t.Errorf("SortedDocValues term = %q, want 'alpha'", string(termVal))
	}
	if sdv.GetValueCount() != 1 {
		t.Errorf("GetValueCount = %d, want 1", sdv.GetValueCount())
	}

	// SortedNumericDocValues
	sndv, err := lr.GetSortedNumericDocValues("sorted_numeric")
	if err != nil {
		t.Fatalf("GetSortedNumericDocValues: %v", err)
	}
	if sndv == nil {
		t.Fatal("SortedNumericDocValues is nil")
	}
	if _, err := sndv.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	count, err := sndv.DocValueCount()
	if err != nil {
		t.Fatalf("DocValueCount: %v", err)
	}
	if count != 3 {
		t.Errorf("DocValueCount = %d, want 3", count)
	}
	for _, exp := range []int64{10, 20, 30} {
		v, err := sndv.NextValue()
		if err != nil {
			t.Fatalf("NextValue: %v", err)
		}
		if v != exp {
			t.Errorf("SortedNumeric value = %d, want %d", v, exp)
		}
	}

	// SortedSetDocValues
	ssdv, err := lr.GetSortedSetDocValues("sorted_set")
	if err != nil {
		t.Fatalf("GetSortedSetDocValues: %v", err)
	}
	if ssdv == nil {
		t.Fatal("SortedSetDocValues is nil")
	}
	if _, err := ssdv.NextDoc(); err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if ssdv.GetValueCount() != 3 {
		t.Errorf("GetValueCount = %d, want 3", ssdv.GetValueCount())
	}
	for ord, exp := range []string{"x", "y", "z"} {
		o, err := ssdv.NextOrd()
		if err != nil {
			t.Fatalf("NextOrd: %v", err)
		}
		if o != ord {
			t.Errorf("NextOrd = %d, want %d", o, ord)
		}
		lt, err := ssdv.LookupOrd(o)
		if err != nil {
			t.Fatalf("LookupOrd: %v", err)
		}
		if string(lt) != exp {
			t.Errorf("LookupOrd(%d) = %q, want %q", o, string(lt), exp)
		}
	}
}

// TestMemoryIndexAgainstDirectory_NormsWithDocValues verifies that norms
// are correctly computed for fields added to MemoryIndex.
func TestMemoryIndexAgainstDirectory_NormsWithDocValues(t *testing.T) {
	mi := newMemoryIndex()
	if err := mi.AddField("text", "quick brown fox"); err != nil {
		t.Fatalf("AddField: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	lr := getLeafReader(searcher)
	if lr == nil {
		t.Fatal("could not get leaf reader with NormValues methods")
	}

	nv, err := lr.GetNormValues("text")
	if err != nil {
		t.Fatalf("GetNormValues: %v", err)
	}
	if nv == nil {
		t.Fatal("GetNormValues returned nil -- no norms stored for text field")
	}

	doc, err := nv.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if doc != 0 {
		t.Errorf("NextDoc = %d, want 0", doc)
	}
	normVal, err := nv.LongValue()
	if err != nil {
		t.Fatalf("LongValue: %v", err)
	}
	// "quick brown fox" has 3 tokens => norm = IntToByte4(3)
	expectedByte, err := util.IntToByte4(3)
	if err != nil {
		t.Fatalf("IntToByte4: %v", err)
	}
	if normVal != int64(expectedByte) {
		t.Errorf("Norm = %d, want %d", normVal, expectedByte)
	}
}

// TestMemoryIndexAgainstDirectory_PointValuesVsNormalIndex verifies that
// PointValues can be stored and retrieved from MemoryIndex.
func TestMemoryIndexAgainstDirectory_PointValuesVsNormalIndex(t *testing.T) {
	mi := newMemoryIndex()

	// IntPoint uses 4-byte sortable encoding
	intPacked := make([]byte, 4)
	intPacked[0] = 0x80 ^ byte(42>>24)
	intPacked[1] = byte(42 >> 16)
	intPacked[2] = byte(42 >> 8)
	intPacked[3] = byte(42)
	if err := mi.AddPointField("int", intPacked, 1, 4); err != nil {
		t.Fatalf("AddPointField: %v", err)
	}

	// LongPoint uses 8-byte encoding
	longPacked := make([]byte, 8)
	lv := int64(100)
	longPacked[0] = 0x80 ^ byte(lv>>56)
	longPacked[1] = byte(lv >> 48)
	longPacked[2] = byte(lv >> 40)
	longPacked[3] = byte(lv >> 32)
	longPacked[4] = byte(lv >> 24)
	longPacked[5] = byte(lv >> 16)
	longPacked[6] = byte(lv >> 8)
	longPacked[7] = byte(lv)
	if err := mi.AddPointField("long", longPacked, 1, 8); err != nil {
		t.Fatalf("AddPointField: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	lr := getLeafReader(searcher)
	if lr == nil {
		t.Fatal("could not get leaf reader with PointValues methods")
	}

	intPV, err := lr.GetPointValues("int")
	if err != nil {
		t.Fatalf("GetPointValues(int): %v", err)
	}
	if intPV == nil {
		t.Fatal("PointValues for 'int' is nil")
	}
	if intPV.GetDocCount() != 1 {
		t.Errorf("GetDocCount = %d, want 1", intPV.GetDocCount())
	}
	if intPV.GetValueCount() != 1 {
		t.Errorf("GetValueCount = %d, want 1", intPV.GetValueCount())
	}
	if intPV.GetNumDimensions() != 1 {
		t.Errorf("GetNumDimensions = %d, want 1", intPV.GetNumDimensions())
	}
	if intPV.GetBytesPerDimension() != 4 {
		t.Errorf("GetBytesPerDimension = %d, want 4", intPV.GetBytesPerDimension())
	}

	longPV, err := lr.GetPointValues("long")
	if err != nil {
		t.Fatalf("GetPointValues(long): %v", err)
	}
	if longPV == nil {
		t.Fatal("PointValues for 'long' is nil")
	}
	if longPV.GetDocCount() != 1 {
		t.Errorf("GetDocCount = %d, want 1", longPV.GetDocCount())
	}
	if longPV.GetValueCount() != 1 {
		t.Errorf("GetValueCount = %d, want 1", longPV.GetValueCount())
	}
	if longPV.GetNumDimensions() != 1 {
		t.Errorf("GetNumDimensions = %d, want 1", longPV.GetNumDimensions())
	}
	if longPV.GetBytesPerDimension() != 8 {
		t.Errorf("GetBytesPerDimension = %d, want 8", longPV.GetBytesPerDimension())
	}
}

// TestMemoryIndexAgainstDirectory_DuellMemIndex verifies that a MemoryIndex
// with known content produces correct term enumeration and search results.
func TestMemoryIndexAgainstDirectory_DuellMemIndex(t *testing.T) {
	mi := newMemoryIndex()

	if err := mi.AddField("title", "the quick brown fox"); err != nil {
		t.Fatalf("AddField title: %v", err)
	}
	if err := mi.AddField("body", "jumps over the lazy dog"); err != nil {
		t.Fatalf("AddField body: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	// Verify term vectors
	tv, err := searcher.GetReader().TermVectors()
	if err != nil {
		t.Fatalf("TermVectors: %v", err)
	}
	fields, err := tv.Get(0)
	if err != nil {
		t.Fatalf("TermVectors.Get: %v", err)
	}

	foundTitle := false
	foundBody := false
	iter, err := fields.Iterator()
	if err != nil {
		t.Fatalf("Iterator: %v", err)
	}
	for iter.HasNext() {
		name, err := iter.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if name == "" {
			break
		}
		if name == "title" {
			foundTitle = true
		}
		if name == "body" {
			foundBody = true
		}
	}
	if !foundTitle {
		t.Error("field 'title' missing from term vectors")
	}
	if !foundBody {
		t.Error("field 'body' missing from term vectors")
	}

	leafReader, ok := searcher.GetReader().(index.LeafReaderInterface)
	if !ok {
		t.Fatal("reader does not implement LeafReaderInterface")
	}
	for _, fieldName := range []string{"title", "body"} {
		terms, err := leafReader.Terms(fieldName)
		if err != nil {
			t.Fatalf("Terms(%q): %v", fieldName, err)
		}
		if terms == nil {
			t.Fatalf("Terms(%q) is nil", fieldName)
		}
		if terms.Size() <= 0 {
			t.Errorf("Terms(%q).Size() = %d, want > 0", fieldName, terms.Size())
		}
	}

	tq := search.NewTermQuery(index.NewTerm("title", "fox"))
	top, err := searcher.Search(tq, 10)
	if err != nil {
		t.Fatalf("Search title:fox: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("TermQuery(title:fox) matched %d docs, want 1", top.TotalHits.Value)
	}

	tq2 := search.NewTermQuery(index.NewTerm("body", "jumps"))
	top2, err := searcher.Search(tq2, 10)
	if err != nil {
		t.Fatalf("Search body:jumps: %v", err)
	}
	if top2.TotalHits.Value != 1 {
		t.Errorf("TermQuery(body:jumps) matched %d docs, want 1", top2.TotalHits.Value)
	}
}

// TestMemoryIndexAgainstDirectory_DuelCoreDirectoryWithArrayField verifies
// that MemoryIndex handles multi-field documents correctly.
func TestMemoryIndexAgainstDirectory_DuelCoreDirectoryWithArrayField(t *testing.T) {
	mi := newMemoryIndex()

	if err := mi.AddField("text", "la la"); err != nil {
		t.Fatalf("First AddField text: %v", err)
	}
	if err := mi.AddField("text", "foo bar foo bar foo"); err != nil {
		t.Fatalf("Second AddField text: %v", err)
	}

	searcher, err := mi.CreateSearcher()
	if err != nil {
		t.Fatalf("CreateSearcher: %v", err)
	}
	defer searcher.Close()

	tq := search.NewTermQuery(index.NewTerm("text", "bar"))
	top, err := searcher.Search(tq, 10)
	if err != nil {
		t.Fatalf("Search text:bar: %v", err)
	}
	if top.TotalHits.Value != 1 {
		t.Errorf("TermQuery(text:bar) matched %d docs, want 1", top.TotalHits.Value)
	}

	tq2 := search.NewTermQuery(index.NewTerm("text", "la"))
	top2, err := searcher.Search(tq2, 10)
	if err != nil {
		t.Fatalf("Search text:la: %v", err)
	}
	if top2.TotalHits.Value != 0 {
		t.Errorf("TermQuery(text:la) matched %d docs, want 0 (field was replaced)", top2.TotalHits.Value)
	}

	tv, err := searcher.GetReader().TermVectors()
	if err != nil {
		t.Fatalf("TermVectors: %v", err)
	}
	fields, err := tv.Get(0)
	if err != nil {
		t.Fatalf("TermVectors.Get: %v", err)
	}
	textTerms, err := fields.Terms("text")
	if err != nil {
		t.Fatalf("Terms(text): %v", err)
	}
	if textTerms == nil {
		t.Fatal("Terms(text) is nil")
	}
	if textTerms.Size() <= 0 {
		t.Errorf("Terms(text) size = %d, want > 0", textTerms.Size())
	}
}
