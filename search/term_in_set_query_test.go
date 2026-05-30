// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestTermInSetQuery.java

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// contains reports whether substr appears within s.
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

// TestTermInSetQuery_Basics tests hash code and equals contract.
// Source: TestTermInSetQuery.testHashCodeAndEquals()
func TestTermInSetQuery_Basics(t *testing.T) {
	terms := make([]*util.BytesRef, 0)
	uniqueTermSet := make(map[string]bool)

	numTerms := 100
	for i := 0; i < numTerms; i++ {
		termText := util.RandomRealisticUnicodeString(util.GetRandom(), 1, 20)
		term := util.NewBytesRef([]byte(termText))
		terms = append(terms, term)
		uniqueTermSet[termText] = true
	}

	left := NewTermInSetQuery("field", terms)

	shuffledTerms := make([]*util.BytesRef, len(terms))
	copy(shuffledTerms, terms)
	for i, j := 0, len(shuffledTerms)-1; i < j; i, j = i+1, j-1 {
		shuffledTerms[i], shuffledTerms[j] = shuffledTerms[j], shuffledTerms[i]
	}
	right := NewTermInSetQuery("field", shuffledTerms)

	if !left.Equals(right) {
		t.Error("Queries with same terms (different order) should be equal")
	}
	if left.HashCode() != right.HashCode() {
		t.Error("Equal queries should have same HashCode")
	}

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

	q1 := NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	q2 := NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("orange"))})
	if q1.HashCode() == q2.HashCode() {
		t.Error("Different terms should ideally produce different hash codes")
	}

	q1 = NewTermInSetQuery("thing", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	q2 = NewTermInSetQuery("thing2", []*util.BytesRef{util.NewBytesRef([]byte("apple"))})
	if q1.HashCode() == q2.HashCode() {
		t.Error("Different fields should produce different hash codes")
	}
}

// TestTermInSetQuery_SimpleEquals tests that hash collisions don't cause false equality.
// Source: TestTermInSetQuery.testSimpleEquals()
func TestTermInSetQuery_SimpleEquals(t *testing.T) {
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
func TestTermInSetQuery_ToString(t *testing.T) {
	q := NewTermInSetQuery("field1", []*util.BytesRef{
		util.NewBytesRef([]byte("a")),
		util.NewBytesRef([]byte("b")),
		util.NewBytesRef([]byte("c")),
	})

	str := q.String()
	if str == "" {
		t.Error("String representation should not be empty")
	}
	if str[:7] != "field1:" {
		t.Errorf("Expected string to start with 'field1:', got %q", str[:7])
	}
	if !contains(str, "a") || !contains(str, "b") || !contains(str, "c") {
		t.Error("String representation should contain all terms")
	}
}

// TestTermInSetQuery_BinaryToString tests binary term representation.
// Source: TestTermInSetQuery.testBinaryToString()
func TestTermInSetQuery_BinaryToString(t *testing.T) {
	q := NewTermInSetQuery("field", []*util.BytesRef{
		util.NewBytesRef([]byte{0xff, 0xfe}),
	})
	expected := "field:([ff fe])"
	if got := q.String(); got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

// TestTermInSetQuery_Dedup tests term deduplication.
// Source: TestTermInSetQuery.testDedup()
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
func TestTermInSetQuery_TermsIterator(t *testing.T) {
	empty := NewTermInSetQuery("field", []*util.BytesRef{})
	it := empty.GetBytesRefIterator()
	if it.Next() != nil {
		t.Error("Empty query iterator should return nil")
	}

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
	if str := q.String(); str != "field:()" {
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
	if len(q.Terms()) != 2 {
		t.Errorf("Expected 2 terms after filtering nil, got %d", len(q.Terms()))
	}
}

// The following tests require infrastructure not yet ported.

// TestTermInSetQuery_AllDocsInFieldTerm tests dense term matching.
// Source: TestTermInSetQuery.testAllDocsInFieldTerm()
// Status: PLACEHOLDER - requires IndexWriter, IndexReader, IndexSearcher
func TestTermInSetQuery_AllDocsInFieldTerm(t *testing.T) {
	t.Fatal("Requires full index infrastructure implementation")
}

// TestTermInSetQuery_Duel tests TermInSetQuery against BooleanQuery of TermQueries.
// Source: TestTermInSetQuery.testDuel()
// Status: PLACEHOLDER - requires full query execution infrastructure
func TestTermInSetQuery_Duel(t *testing.T) {
	t.Fatal("Requires full query execution and index infrastructure")
}

// TestTermInSetQuery_ReturnsNullScoreSupplier tests null score supplier behavior.
// Source: TestTermInSetQuery.testReturnsNullScoreSupplier()
// Status: PLACEHOLDER - requires ScorerSupplier implementation
func TestTermInSetQuery_ReturnsNullScoreSupplier(t *testing.T) {
	t.Fatal("Requires ScorerSupplier and Weight implementation")
}

// TestTermInSetQuery_SkipperOptimization tests doc values skip optimization.
// Source: TestTermInSetQuery.testSkipperOptimizationGapAssumption()
// Status: PLACEHOLDER - requires doc values infrastructure
func TestTermInSetQuery_SkipperOptimization(t *testing.T) {
	t.Fatal("Requires doc values and skip list implementation")
}

// TestTermInSetQuery_RamBytesUsed tests memory usage calculation.
// Source: TestTermInSetQuery.testRamBytesUsed()
// Status: PLACEHOLDER - requires RamUsageTester
func TestTermInSetQuery_RamBytesUsed(t *testing.T) {
	t.Fatal("Requires RamUsageTester implementation")
}

// TestTermInSetQuery_PullOneTermsEnum tests single TermsEnum optimization.
// Source: TestTermInSetQuery.testPullOneTermsEnum()
// Status: PLACEHOLDER - requires FilterDirectoryReader and TermsEnum tracking
func TestTermInSetQuery_PullOneTermsEnum(t *testing.T) {
	t.Fatal("Requires FilterDirectoryReader and TermsEnum tracking")
}

// TestTermInSetQuery_IsConsideredCostlyByQueryCache tests query caching policy.
// Source: TestTermInSetQuery.testIsConsideredCostlyByQueryCache()
// Status: PLACEHOLDER - requires UsageTrackingQueryCachingPolicy
func TestTermInSetQuery_IsConsideredCostlyByQueryCache(t *testing.T) {
	t.Fatal("Requires UsageTrackingQueryCachingPolicy implementation")
}

// TestTermInSetQuery_Visitor tests query visitor pattern.
// Source: TestTermInSetQuery.testVisitor()
// Status: PLACEHOLDER - requires QueryVisitor and ByteRunAutomaton
func TestTermInSetQuery_Visitor(t *testing.T) {
	t.Fatal("Requires QueryVisitor and ByteRunAutomaton implementation")
}
