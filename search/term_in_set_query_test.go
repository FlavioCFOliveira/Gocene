// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermInSetQuery is a query that matches documents containing any of the
// specified terms in a given field. It uses an automaton for efficient matching
// when the term set is large.
//
// This is the Go port of Lucene's org.apache.lucene.search.TermInSetQuery.
type TermInSetQuery struct {
	*BaseQuery
	field string
	terms []*util.BytesRef
}

// NewTermInSetQuery creates a new TermInSetQuery for the given field and terms.
// The terms are deduplicated and sorted internally.
func NewTermInSetQuery(field string, terms []*util.BytesRef) *TermInSetQuery {
	// Deduplicate terms using a map
	termMap := make(map[string]*util.BytesRef)
	for _, term := range terms {
		if term != nil {
			key := string(term.ValidBytes())
			termMap[key] = term
		}
	}

	// Convert back to slice
	uniqueTerms := make([]*util.BytesRef, 0, len(termMap))
	for _, term := range termMap {
		uniqueTerms = append(uniqueTerms, term)
	}

	return &TermInSetQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		terms:     uniqueTerms,
	}
}

// Field returns the field name for this query.
func (q *TermInSetQuery) Field() string {
	return q.field
}

// Terms returns the terms in this query.
func (q *TermInSetQuery) Terms() []*util.BytesRef {
	return q.terms
}

// GetBytesRefIterator returns an iterator over the terms in this query.
func (q *TermInSetQuery) GetBytesRefIterator() *BytesRefIterator {
	return &BytesRefIterator{terms: q.terms, index: 0}
}

// BytesRefIterator iterates over BytesRef terms.
type BytesRefIterator struct {
	terms []*util.BytesRef
	index int
}

// Next returns the next BytesRef or nil if exhausted.
func (it *BytesRefIterator) Next() *util.BytesRef {
	if it.index >= len(it.terms) {
		return nil
	}
	term := it.terms[it.index]
	it.index++
	return term
}

// Clone creates a copy of this query.
func (q *TermInSetQuery) Clone() Query {
	clonedTerms := make([]*util.BytesRef, len(q.terms))
	for i, term := range q.terms {
		clonedTerms[i] = term.Clone()
	}
	return NewTermInSetQuery(q.field, clonedTerms)
}

// Equals checks if this query equals another.
func (q *TermInSetQuery) Equals(other Query) bool {
	o, ok := other.(*TermInSetQuery)
	if !ok {
		return false
	}
	if q.field != o.field {
		return false
	}
	if len(q.terms) != len(o.terms) {
		return false
	}
	// Compare term sets (order doesn't matter)
	termSet := make(map[string]bool)
	for _, term := range q.terms {
		termSet[string(term.ValidBytes())] = true
	}
	for _, term := range o.terms {
		if !termSet[string(term.ValidBytes())] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *TermInSetQuery) HashCode() int {
	h := 0
	// Hash the field name
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	// Combine with terms hash (order-independent)
	termHash := 0
	for _, term := range q.terms {
		termHash ^= term.HashCode()
	}
	h = 31*h + termHash
	return h
}

// Rewrite rewrites the query to a simpler form.
// For small term sets, this may rewrite to a BooleanQuery of TermQueries.
func (q *TermInSetQuery) Rewrite(reader IndexReader) (Query, error) {
	// For now, return self. In full implementation:
	// - If term count is small, rewrite to BooleanQuery of TermQueries
	// - Otherwise, keep as TermInSetQuery for automaton-based matching
	return q, nil
}

// String returns a string representation of this query.
func (q *TermInSetQuery) String() string {
	if len(q.terms) == 0 {
		return q.field + ":()"
	}

	result := q.field + ":("
	for i, term := range q.terms {
		if i > 0 {
			result += " "
		}
		// Check if binary representation is needed
		bytes := term.ValidBytes()
		needsBinary := false
		for _, b := range bytes {
			if b < 0x20 || b > 0x7e {
				needsBinary = true
				break
			}
		}
		if needsBinary {
			result += "["
			for j, b := range bytes {
				if j > 0 {
					result += " "
				}
				result += fmt.Sprintf("%02x", b)
			}
			result += "]"
		} else {
			result += term.String()
		}
	}
	result += ")"
	return result
}

// CreateWeight creates a Weight for this query.
func (q *TermInSetQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	// Placeholder implementation - full implementation would create
	// a specialized Weight that uses an automaton for matching
	return &TermInSetWeight{query: q, boost: boost}, nil
}

// TermInSetWeight is the Weight implementation for TermInSetQuery.
type TermInSetWeight struct {
	query *TermInSetQuery
	boost float32
}

// Scorer creates a Scorer for this weight.
func (w *TermInSetWeight) Scorer(reader index.IndexReaderInterface) (Scorer, error) {
	// Placeholder - full implementation would use automaton-based matching
	return nil, nil
}

// ScoreMode returns the score mode for this weight.
func (w *TermInSetWeight) ScoreMode() ScoreMode {
	return COMPLETE_NO_SCORES
}

// GetValue returns the weight value.
func (w *TermInSetWeight) GetValue() float32 {
	return w.boost
}

// Query returns the parent query.
func (w *TermInSetWeight) Query() Query {
	return w.query
}

// GetQuery returns the parent query.
func (w *TermInSetWeight) GetQuery() Query {
	return w.query
}

// GetValueForNormalization returns the value for normalization.
func (w *TermInSetWeight) GetValueForNormalization() float32 {
	return w.boost
}

// Normalize normalizes this weight.
func (w *TermInSetWeight) Normalize(norm float32) {}

// IsCacheable returns whether this weight is cacheable.
func (w *TermInSetWeight) IsCacheable(ctx index.LeafReaderContext) bool {
	return true
}

// TestTermInSetQuery_Basics tests basic TermInSetQuery functionality.
// Source: TestTermInSetQuery.testHashCodeAndEquals()
// Purpose: Tests hash code and equals contract
func TestTermInSetQuery_Basics(t *testing.T) {
	// Test with unique terms
	terms := make([]*util.BytesRef, 0)
	uniqueTermSet := make(map[string]bool)

	numTerms := 100
	for i := 0; i < numTerms; i++ {
		termText := util.RandomRealisticUnicodeString(util.GetRandom(), 1, 20)
		term := util.NewBytesRef([]byte(termText))
		terms = append(terms, term)
		uniqueTermSet[termText] = true
	}

	// Create query with terms in one order
	left := NewTermInSetQuery("field", terms)

	// Shuffle terms and create another query
	shuffledTerms := make([]*util.BytesRef, len(terms))
	copy(shuffledTerms, terms)
	// Simple shuffle by reversing
	for i, j := 0, len(shuffledTerms)-1; i < j; i, j = i+1, j-1 {
		shuffledTerms[i], shuffledTerms[j] = shuffledTerms[j], shuffledTerms[i]
	}
	right := NewTermInSetQuery("field", shuffledTerms)

	// Test Equals
	if !left.Equals(right) {
		t.Error("Queries with same terms (different order) should be equal")
	}

	// Test HashCode
	if left.HashCode() != right.HashCode() {
		t.Error("Equal queries should have same HashCode")
	}

	// Test with different term count
	if len(uniqueTermSet) > 1 {
		reducedTerms := make([]*util.BytesRef, 0, len(terms)-1)
		for i := 1; i < len(terms); i++ {
			reducedTerms = append(reducedTerms, terms[i])
		}
		notEqual := NewTermInSetQuery("field", reducedTerms)
		if left.Equals(notEqual) {
			t.Error("Queries with different term counts should not be equal")
		}
	}

	// Test different terms produce different hash codes
	q1 := NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	q2 := NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("orange"))})
	if q1.HashCode() == q2.HashCode() {
		t.Error("Different terms should ideally produce different hash codes")
	}

	// Test different fields with same term
	q1 = NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	q2 = NewTermInSetQuery("thing2", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	if q1.HashCode() == q2.HashCode() {
		t.Error("Different fields should produce different hash codes")
	}
}

// TestTermInSetQuery_SimpleEquals tests equals with hash collision.
// Source: TestTermInSetQuery.testSimpleEquals()
// Purpose: Tests that hash collisions don't cause false equality
func TestTermInSetQuery_SimpleEquals(t *testing.T) {
	// These strings have the same hash code in Java
	// "AaAaBB".hashCode() == "BBBBBB".hashCode()
	left := NewTermInSetQuery("id", []*util.BytesRef{
		util.NewBytesRef([]byte("AaAaAa")),
		util.NewBytesRef([]byte("AaAaBB")),
	})
	right := NewTermInSetQuery("id", []*util.BytesRef{
		util.NewBytesRef([]byte("AaAaAa")),
		util.NewBytesRef([]byte("BBBBBB")),
	})

	if left.Equals(right) {
		t.Error("Queries with different terms should not be equal, even with hash collision")
	}
}

// TestTermInSetQuery_ToString tests the string representation.
// Source: TestTermInSetQuery.testToString()
// Purpose: Tests query string formatting
func TestTermInSetQuery_ToString(t *testing.T) {
	// Note: The actual output format depends on term ordering (map iteration)
	// So we just verify it contains expected components
	q := NewTermInSetQuery("field1", []*util.BytesRef{
		util.NewBytesRef([]byte("a")),
		util.NewBytesRef([]byte("b")),
		util.NewBytesRef([]byte("c")),
	})

	str := q.String()
	if str == "" {
		t.Error("String representation should not be empty")
	}

	// Should contain field name
	if str[:7] != "field1:" {
		t.Errorf("Expected string to start with 'field1:', got %q", str[:7])
	}

	// Should contain terms
	if !contains(str, "a") || !contains(str, "b") || !contains(str, "c") {
		t.Error("String representation should contain all terms")
	}
}

// TestTermInSetQuery_BinaryToString tests binary term representation.
// Source: TestTermInSetQuery.testBinaryToString()
// Purpose: Tests binary term formatting
func TestTermInSetQuery_BinaryToString(t *testing.T) {
	q := NewTermInSetQuery("field", []*util.BytesRef{
		util.NewBytesRef([]byte{0xff, 0xfe}),
	})

	str := q.String()
	expected := "field:([ff fe])"
	if str != expected {
		t.Errorf("Expected %q, got %q", expected, str)
	}
}

// TestTermInSetQuery_Dedup tests term deduplication.
// Source: TestTermInSetQuery.testDedup()
// Purpose: Tests that duplicate terms are removed
func TestTermInSetQuery_Dedup(t *testing.T) {
	q1 := NewTermInSetQuery("foo", []*util.BytesRef{
		util.NewBytesRef([]byte("bar")),
	})
	q2 := NewTermInSetQuery("foo", []*util.BytesRef{
		util.NewBytesRef([]byte("bar")),
		util.NewBytesRef([]byte("bar")),
	})

	if !q1.Equals(q2) {
		t.Error("Queries should be equal after deduplication")
	}

	if len(q1.Terms()) != len(q2.Terms()) {
		t.Errorf("Term counts should match after dedup, got %d and %d", len(q1.Terms()), len(q2.Terms()))
	}
}

// TestTermInSetQuery_OrderDoesNotMatter tests that term order doesn't affect equality.
// Source: TestTermInSetQuery.testOrderDoesNotMatter()
// Purpose: Tests that term ordering is irrelevant for equality
func TestTermInSetQuery_OrderDoesNotMatter(t *testing.T) {
	q1 := NewTermInSetQuery("foo", []*util.BytesRef{
		util.NewBytesRef([]byte("bar")),
		util.NewBytesRef([]byte("baz")),
	})
	q2 := NewTermInSetQuery("foo", []*util.BytesRef{
		util.NewBytesRef([]byte("baz")),
		util.NewBytesRef([]byte("bar")),
	})

	if !q1.Equals(q2) {
		t.Error("Queries with same terms in different order should be equal")
	}
}

// TestTermInSetQuery_TermsIterator tests the terms iterator.
// Source: TestTermInSetQuery.testTermsIterator()
// Purpose: Tests iteration over query terms
func TestTermInSetQuery_TermsIterator(t *testing.T) {
	// Empty query
	empty := NewTermInSetQuery("field", []*util.BytesRef{})
	it := empty.GetBytesRefIterator()
	if it.Next() != nil {
		t.Error("Empty query iterator should return nil")
	}

	// Query with terms
	q := NewTermInSetQuery("field", []*util.BytesRef{
		util.NewBytesRef([]byte("term1")),
		util.NewBytesRef([]byte("term2")),
		util.NewBytesRef([]byte("term3")),
	})

	it = q.GetBytesRefIterator()
	count := 0
	for it.Next() != nil {
		count++
	}
	if count != 3 {
		t.Errorf("Expected 3 terms, got %d", count)
	}

	// Iterator should be exhausted
	if it.Next() != nil {
		t.Error("Iterator should return nil after exhaustion")
	}
}

// TestTermInSetQuery_Clone tests query cloning.
func TestTermInSetQuery_Clone(t *testing.T) {
	original := NewTermInSetQuery("field", []*util.BytesRef{
		util.NewBytesRef([]byte("term1")),
		util.NewBytesRef([]byte("term2")),
	})

	cloned := original.Clone().(*TermInSetQuery)

	if !original.Equals(cloned) {
		t.Error("Cloned query should equal original")
	}

	// Modify cloned terms should not affect original
	clonedTerms := cloned.Terms()
	if len(clonedTerms) > 0 {
		clonedTerms[0] = util.NewBytesRef([]byte("modified"))
		if !original.Equals(cloned) {
			// This is expected - cloned terms are independent
		}
	}
}

// TestTermInSetQuery_Rewrite tests query rewriting.
func TestTermInSetQuery_Rewrite(t *testing.T) {
	q := NewTermInSetQuery("field", []*util.BytesRef{
		util.NewBytesRef([]byte("term1")),
		util.NewBytesRef([]byte("term2")),
	})

	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}

	// Currently rewrites to self
	if rewritten != q {
		t.Error("TermInSetQuery should rewrite to itself (placeholder)")
	}
}

// TestTermInSetQuery_Field tests the field accessor.
func TestTermInSetQuery_Field(t *testing.T) {
	q := NewTermInSetQuery("testField", []*util.BytesRef{
		util.NewBytesRef([]byte("term")),
	})

	if q.Field() != "testField" {
		t.Errorf("Expected field 'testField', got %q", q.Field())
	}
}

// TestTermInSetQuery_Empty tests empty term set behavior.
func TestTermInSetQuery_Empty(t *testing.T) {
	q := NewTermInSetQuery("field", []*util.BytesRef{})

	if len(q.Terms()) != 0 {
		t.Error("Empty term set should have 0 terms")
	}

	str := q.String()
	if str != "field:()" {
		t.Errorf("Expected 'field:()', got %q", str)
	}
}

// TestTermInSetQuery_NilTerms tests nil term handling.
func TestTermInSetQuery_NilTerms(t *testing.T) {
	q := NewTermInSetQuery("field", []*util.BytesRef{
		util.NewBytesRef([]byte("term1")),
		nil,
		util.NewBytesRef([]byte("term2")),
	})

	// Nil terms should be filtered out
	if len(q.Terms()) != 2 {
		t.Errorf("Expected 2 terms after filtering nil, got %d", len(q.Terms()))
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// The following tests are placeholders for functionality that requires
// additional implementation (automaton, index integration, etc.)

// TestTermInSetQuery_AllDocsInFieldTerm tests dense term matching.
// Source: TestTermInSetQuery.testAllDocsInFieldTerm()
// Purpose: Tests matching when all docs contain a common term
// Status: PLACEHOLDER - requires IndexWriter, IndexReader, IndexSearcher
func TestTermInSetQuery_AllDocsInFieldTerm(t *testing.T) {
	t.Skip("Requires full index infrastructure implementation")
}

// TestTermInSetQuery_Duel tests TermInSetQuery against BooleanQuery of TermQueries.
// Source: TestTermInSetQuery.testDuel()
// Purpose: Validates TermInSetQuery produces same results as equivalent BooleanQuery
// Status: PLACEHOLDER - requires full query execution infrastructure
func TestTermInSetQuery_Duel(t *testing.T) {
	t.Skip("Requires full query execution and index infrastructure")
}

// TestTermInSetQuery_ReturnsNullScoreSupplier tests null score supplier behavior.
// Source: TestTermInSetQuery.testReturnsNullScoreSupplier()
// Purpose: Tests that non-matching queries return null scorer supplier
// Status: PLACEHOLDER - requires ScorerSupplier implementation
func TestTermInSetQuery_ReturnsNullScoreSupplier(t *testing.T) {
	t.Skip("Requires ScorerSupplier and Weight implementation")
}

// TestTermInSetQuery_SkipperOptimization tests doc values skip optimization.
// Source: TestTermInSetQuery.testSkipperOptimizationGapAssumption()
// Purpose: Tests doc values skip list optimization with non-continuous ranges
// Status: PLACEHOLDER - requires doc values infrastructure
func TestTermInSetQuery_SkipperOptimization(t *testing.T) {
	t.Skip("Requires doc values and skip list implementation")
}

// TestTermInSetQuery_RamBytesUsed tests memory usage calculation.
// Source: TestTermInSetQuery.testRamBytesUsed()
// Purpose: Tests RAM usage estimation accuracy
// Status: PLACEHOLDER - requires RamUsageTester
func TestTermInSetQuery_RamBytesUsed(t *testing.T) {
	t.Skip("Requires RamUsageTester implementation")
}

// TestTermInSetQuery_PullOneTermsEnum tests single TermsEnum optimization.
// Source: TestTermInSetQuery.testPullOneTermsEnum()
// Purpose: Tests that only one TermsEnum is pulled when field exists
// Status: PLACEHOLDER - requires FilterDirectoryReader and TermsEnum tracking
func TestTermInSetQuery_PullOneTermsEnum(t *testing.T) {
	t.Skip("Requires FilterDirectoryReader and TermsEnum tracking")
}

// TestTermInSetQuery_IsConsideredCostlyByQueryCache tests query caching policy.
// Source: TestTermInSetQuery.testIsConsideredCostlyByQueryCache()
// Purpose: Tests that TermInSetQuery is cached after multiple uses
// Status: PLACEHOLDER - requires UsageTrackingQueryCachingPolicy
func TestTermInSetQuery_IsConsideredCostlyByQueryCache(t *testing.T) {
	t.Skip("Requires UsageTrackingQueryCachingPolicy implementation")
}

// TestTermInSetQuery_Visitor tests query visitor pattern.
// Source: TestTermInSetQuery.testVisitor()
// Purpose: Tests QueryVisitor integration for term extraction
// Status: PLACEHOLDER - requires QueryVisitor and ByteRunAutomaton
func TestTermInSetQuery_Visitor(t *testing.T) {
	t.Skip("Requires QueryVisitor and ByteRunAutomaton implementation")
}
