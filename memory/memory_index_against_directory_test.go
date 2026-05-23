// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Tests that verify MemoryIndex behaviour mirrors a Directory-backed index.
// Port of org.apache.lucene.index.memory.TestMemoryIndexAgainstDirectory.
//
// The Java original uses the full Lucene MemoryIndex (createSearcher, term vectors,
// DocValues, etc.) which is not yet implemented in the Gocene memory package.
// Each test covers the same logical scenario and assertion intent; tests that
// require unported infrastructure are explicitly skipped with a diagnostic message.
package memory_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/memory"
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

// TestMemoryIndexAgainstDirectory_RandomQueries verifies that MemoryIndex behaves
// consistently across repeated resets and field additions.
// Java counterpart: testRandomQueries / assertAgainstDirectory.
func TestMemoryIndexAgainstDirectory_RandomQueries(t *testing.T) {
	t.Skip("requires full MemoryIndex.createSearcher() + DirectoryReader — not yet ported")
}

// TestMemoryIndexAgainstDirectory_DocsEnumStart verifies that a PostingsEnum
// starts with docID == -1 and advances correctly.
// Java counterpart: testDocsEnumStart.
func TestMemoryIndexAgainstDirectory_DocsEnumStart(t *testing.T) {
	t.Skip("requires MemoryIndex.createSearcher() + LeafReader.terms() — not yet ported")
}

// TestMemoryIndexAgainstDirectory_DocsAndPositionsEnumStart verifies that a
// PostingsEnum with positions starts with docID == -1 and yields correct offsets.
// Java counterpart: testDocsAndPositionsEnumStart.
func TestMemoryIndexAgainstDirectory_DocsAndPositionsEnumStart(t *testing.T) {
	t.Skip("requires MemoryIndex.createSearcher() + PostingsEnum.ALL — not yet ported")
}

// TestMemoryIndexAgainstDirectory_NullPointerException verifies that searching a
// RegexpQuery wrapped in SpanMultiTermQueryWrapper returns 0 hits (not a panic).
// Java counterpart: testNullPointerException (LUCENE-3831).
func TestMemoryIndexAgainstDirectory_NullPointerException(t *testing.T) {
	t.Skip("requires MemoryIndex.search() + SpanMultiTermQueryWrapper — not yet ported")
}

// TestMemoryIndexAgainstDirectory_PassesIfWrapped verifies that wrapping a
// SpanMultiTermQueryWrapper in a SpanOrQuery also returns 0 hits without panic.
// Java counterpart: testPassesIfWrapped (LUCENE-3831).
func TestMemoryIndexAgainstDirectory_PassesIfWrapped(t *testing.T) {
	t.Skip("requires MemoryIndex.search() + SpanOrQuery — not yet ported")
}

// TestMemoryIndexAgainstDirectory_SameFieldAddedMultipleTimes verifies that adding
// the same field twice accumulates term frequencies and that phrase gap controls
// phrase-query matching.
// Java counterpart: testSameFieldAddedMultipleTimes.
func TestMemoryIndexAgainstDirectory_SameFieldAddedMultipleTimes(t *testing.T) {
	t.Skip("requires MemoryIndex.createSearcher() + PhraseQuery — not yet ported")
}

// TestMemoryIndexAgainstDirectory_NonExistentField verifies that querying a field
// that was never added returns nil/null without error.
// Java counterpart: testNonExistentField.
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

// TestMemoryIndexAgainstDirectory_DocValuesVsNormalIndex verifies that DocValues
// stored in a MemoryIndex match those from a Directory-backed index.
// Java counterpart: testDocValuesMemoryIndexVsNormalIndex.
func TestMemoryIndexAgainstDirectory_DocValuesVsNormalIndex(t *testing.T) {
	t.Skip("requires MemoryIndex.fromDocument() + DocValues APIs — not yet ported")
}

// TestMemoryIndexAgainstDirectory_NormsWithDocValues verifies norm values are
// consistent between MemoryIndex and a Directory-backed index when DocValues are present.
// Java counterpart: testNormsWithDocValues.
func TestMemoryIndexAgainstDirectory_NormsWithDocValues(t *testing.T) {
	t.Skip("requires MemoryIndex.createSearcher() + getNormValues() — not yet ported")
}

// TestMemoryIndexAgainstDirectory_PointValuesVsNormalIndex verifies that IntPoint,
// LongPoint, FloatPoint, and DoublePoint queries return the same results in
// MemoryIndex and a Directory-backed index.
// Java counterpart: testPointValuesMemoryIndexVsNormalIndex.
func TestMemoryIndexAgainstDirectory_PointValuesVsNormalIndex(t *testing.T) {
	t.Skip("requires MemoryIndex.fromDocument() + IntPoint/LongPoint queries — not yet ported")
}

// TestMemoryIndexAgainstDirectory_DuellMemIndex verifies that a MemoryIndex
// matches a Directory-backed index across multiple random documents.
// Java counterpart: testDuellMemIndex.
func TestMemoryIndexAgainstDirectory_DuellMemIndex(t *testing.T) {
	t.Skip("requires MemoryIndex.createSearcher() + DirectoryReader.duellReaders() — not yet ported")
}

// TestMemoryIndexAgainstDirectory_EmptyString verifies that an empty-string token
// can be indexed and found via TermQuery.
// Java counterpart: testEmptyString (LUCENE-4880).
func TestMemoryIndexAgainstDirectory_EmptyString(t *testing.T) {
	t.Skip("requires MemoryIndex.createSearcher() + CannedTokenStream — not yet ported")
}

// TestMemoryIndexAgainstDirectory_DuelCoreDirectoryWithArrayField verifies that
// term vectors (positions, offsets) match between MemoryIndex and a Directory
// when the same field is added multiple times.
// Java counterpart: testDuelMemoryIndexCoreDirectoryWithArrayField.
func TestMemoryIndexAgainstDirectory_DuelCoreDirectoryWithArrayField(t *testing.T) {
	t.Skip("requires MemoryIndex.createSearcher() + termVectors() — not yet ported")
}

// ---------------------------------------------------------------------------
// Smoke tests for the current MemoryIndex stub that verify the behavioral
// contract that will be exercised by the above integration tests once ported.
// ---------------------------------------------------------------------------

// newMemoryIndex is a local alias to keep tests independent of API churn.
func newMemoryIndex() *memory.MemoryIndex { return memory.NewMemoryIndex() }

// TestMemoryIndexAgainstDirectory_FieldAccumulation verifies that fields and
// terms added to a MemoryIndex are tracked correctly — the invariant relied on
// by the Java "assertAgainstDirectory" loop.
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

	// After reset the index must be empty — mirrors memory.reset() in Java.
	mi.Reset()
	if mi.Size() != 0 {
		t.Errorf("expected empty index after Reset, got %d fields", mi.Size())
	}
}

// TestMemoryIndexAgainstDirectory_TermFrequency verifies that term frequency is
// accumulated correctly, mirroring the "getSumTotalTermFreq" checks in the Java test.
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
// recorded, mirroring the position-equality assertions in duellReaders.
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

// TestMemoryIndexAgainstDirectory_FrozenRejectsWrites verifies that a frozen index
// refuses further writes — mirrors MemoryIndex's freeze/immutability contract.
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
// frozen state so new fields can be added — mirrors memory.reset() used in the
// Java "assertAgainstDirectory" loop.
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
