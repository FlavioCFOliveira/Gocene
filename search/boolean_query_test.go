// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestBooleanQuery_Basics(t *testing.T) {
	bq1 := NewBooleanQuery()
	bq1.Add(NewTermQuery(index.NewTerm("field", "v1")), MUST)
	bq1.Add(NewTermQuery(index.NewTerm("field", "v2")), SHOULD)

	if len(bq1.Clauses()) != 2 {
		t.Errorf("Expected 2 clauses, got %d", len(bq1.Clauses()))
	}

	// Test Clone
	cloned := bq1.Clone().(*BooleanQuery)
	if len(cloned.Clauses()) != 2 {
		t.Error("Cloned query should have same number of clauses")
	}
	if cloned.Clauses()[0].Occur != MUST {
		t.Error("Cloned clause should have same Occur")
	}

	// Test Equals
	bq2 := NewBooleanQuery()
	bq2.Add(NewTermQuery(index.NewTerm("field", "v1")), MUST)
	bq2.Add(NewTermQuery(index.NewTerm("field", "v2")), SHOULD)
	if !bq1.Equals(bq2) {
		t.Error("Queries with same clauses should be equal")
	}

	// Test Not Equals (different occur)
	bq3 := NewBooleanQuery()
	bq3.Add(NewTermQuery(index.NewTerm("field", "v1")), SHOULD)
	bq3.Add(NewTermQuery(index.NewTerm("field", "v2")), SHOULD)
	if bq1.Equals(bq3) {
		t.Error("Queries with different Occur should not be equal")
	}

	// Test HashCode
	if bq1.HashCode() != bq2.HashCode() {
		t.Error("Equal queries should have same HashCode")
	}
}

func TestBooleanQuery_MinShouldMatch(t *testing.T) {
	bq := NewBooleanQuery()
	bq.SetMinimumNumberShouldMatch(2)
	if bq.MinimumNumberShouldMatch() != 2 {
		t.Errorf("Expected minShouldMatch 2, got %d", bq.MinimumNumberShouldMatch())
	}

	cloned := bq.Clone().(*BooleanQuery)
	if cloned.MinimumNumberShouldMatch() != 2 {
		t.Error("Cloned query should preserve minShouldMatch")
	}
}

func TestBooleanQuery_OccurString(t *testing.T) {
	tests := []struct {
		occur    Occur
		expected string
	}{
		{MUST, "MUST"},
		{SHOULD, "SHOULD"},
		{MUST_NOT, "MUST_NOT"},
		{FILTER, "FILTER"},
		{Occur(99), "Occur(99)"},
	}

	for _, tc := range tests {
		if tc.occur.String() != tc.expected {
			t.Errorf("Expected %s, got %s", tc.expected, tc.occur.String())
		}
	}
}

func TestBooleanQuery_Rewrite(t *testing.T) {
	// Empty query rewrites to MatchNoDocsQuery (Lucene 10.4.0 behaviour).
	bqEmpty := NewBooleanQuery()
	rewritten, _ := bqEmpty.Rewrite(nil)
	if _, ok := rewritten.(*MatchNoDocsQuery); !ok {
		t.Errorf("Empty BooleanQuery should rewrite to MatchNoDocsQuery, got %T", rewritten)
	}

	// Single MUST clause rewrites to subquery.
	tq := NewTermQuery(index.NewTerm("f", "v"))
	bqSingle := NewBooleanQuery()
	bqSingle.Add(tq, MUST)

	rewritten, _ = bqSingle.Rewrite(nil)
	if _, ok := rewritten.(*TermQuery); !ok {
		t.Errorf("Single MUST clause BooleanQuery should rewrite to subquery type, got %T", rewritten)
	}
	if !rewritten.Equals(tq) {
		t.Error("Rewritten query should equal original subquery")
	}

	// Single FILTER clause rewrites to BoostQuery(ConstantScoreQuery(inner), 0)
	// per Lucene 10.4.0: boost=0 propagates needsScores=false to the inner scorer.
	bqFilter := NewBooleanQuery()
	bqFilter.Add(tq, FILTER)
	rewritten, _ = bqFilter.Rewrite(nil)
	bqr, ok := rewritten.(*BoostQuery)
	if !ok {
		t.Fatalf("Single FILTER clause BooleanQuery should rewrite to BoostQuery, got %T", rewritten)
	}
	if bqr.Boost() != 0.0 {
		t.Errorf("FILTER BoostQuery should have boost 0, got %f", bqr.Boost())
	}
	if _, ok := bqr.Query().(*ConstantScoreQuery); !ok {
		t.Errorf("Inner query should be ConstantScoreQuery, got %T", bqr.Query())
	}

	// Nested BooleanQuery rewrite.
	bqNested := NewBooleanQuery()
	bqInner := NewBooleanQuery()
	bqInner.Add(tq, MUST)
	bqNested.Add(bqInner, MUST)

	rewritten, _ = bqNested.Rewrite(nil)
	if _, ok := rewritten.(*TermQuery); !ok {
		t.Errorf("Nested single MUST clause should rewrite to innermost query type, got %T", rewritten)
	}
}

func TestBooleanQuery_MinShouldMatchRewrite(t *testing.T) {
	tq1 := NewTermQuery(index.NewTerm("f", "v1"))
	tq2 := NewTermQuery(index.NewTerm("f", "v2"))

	bq := NewBooleanQuery()
	bq.Add(tq1, SHOULD)
	bq.Add(tq2, SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	// When SHOULD count equals minShouldMatch, Lucene converts SHOULDs to MUSTs
	// and resets minShouldMatch to 0 (step 14 of BooleanQuery.rewriteStep).
	rewritten, _ := bq.Rewrite(nil)
	bqRewritten, ok := rewritten.(*BooleanQuery)
	if !ok {
		t.Fatalf("Expected BooleanQuery, got %T", rewritten)
	}
	if bqRewritten.MinimumNumberShouldMatch() != 0 {
		t.Errorf("Expected minShouldMatch 0 after SHOULD→MUST conversion, got %d", bqRewritten.MinimumNumberShouldMatch())
	}
	for _, c := range bqRewritten.Clauses() {
		if c.Occur != MUST {
			t.Errorf("Expected all clauses to be MUST after conversion, got %v", c.Occur)
		}
	}
}

func TestBooleanQuery_ConvenienceMethods(t *testing.T) {
	t1 := NewTermQuery(index.NewTerm("f", "v1"))
	t2 := NewTermQuery(index.NewTerm("f", "v2"))

	bqAnd := NewBooleanQueryAndWithQueries(t1, t2)
	for _, c := range bqAnd.Clauses() {
		if c.Occur != MUST {
			t.Error("NewBooleanQueryAndWithQueries should use MUST")
		}
	}

	bqOr := NewBooleanQueryOrWithQueries(t1, t2)
	for _, c := range bqOr.Clauses() {
		if c.Occur != SHOULD {
			t.Error("NewBooleanQueryOrWithQueries should use SHOULD")
		}
	}

	bqNot := NewBooleanQueryNotWithQuery(t1)
	if len(bqNot.Clauses()) != 1 || bqNot.Clauses()[0].Occur != MUST_NOT {
		t.Error("NewBooleanQueryNotWithQuery should use MUST_NOT")
	}
}

func TestBooleanQuery_String(t *testing.T) {
	t1 := NewTermQuery(index.NewTerm("f", "v1"))
	t2 := NewTermQuery(index.NewTerm("f", "v2"))

	bq := NewBooleanQuery()
	bq.Add(t1, MUST)
	bq.Add(t2, SHOULD)

	expected := "+f:v1 f:v2"
	if bq.String() != expected {
		t.Errorf("Expected %q, got %q", expected, bq.String())
	}

	bq2 := NewBooleanQuery()
	bq2.Add(t1, MUST_NOT)
	bq2.Add(t2, FILTER)
	expected2 := "-f:v1 #f:v2"
	if bq2.String() != expected2 {
		t.Errorf("Expected '%s', got '%s'", expected2, bq2.String())
	}
}

func TestBooleanQuery_EqualityComplexity(t *testing.T) {
	tq1 := NewTermQuery(index.NewTerm("f", "v1"))
	tq2 := NewTermQuery(index.NewTerm("f", "v2"))
	tq3 := NewTermQuery(index.NewTerm("f", "v3"))

	bq1 := NewBooleanQuery()
	bq1.Add(tq1, MUST)
	bq1.Add(tq2, SHOULD)

	bq2 := NewBooleanQuery()
	bq2.Add(tq1, MUST)
	bq2.Add(tq2, SHOULD)

	if !bq1.Equals(bq2) {
		t.Error("Simple equality failed")
	}

	bq3 := NewBooleanQuery()
	bq3.Add(tq1, MUST)
	bq3.Add(tq3, SHOULD)
	if bq1.Equals(bq3) {
		t.Error("Should not be equal with different subquery")
	}

	bq4 := NewBooleanQuery()
	bq4.Add(tq1, MUST)
	bq4.Add(tq2, MUST)
	if bq1.Equals(bq4) {
		t.Error("Should not be equal with different occur")
	}

	bq5 := NewBooleanQuery()
	bq5.Add(tq1, MUST)
	bq5.Add(tq2, SHOULD)
	bq5.SetMinimumNumberShouldMatch(2)
	if bq1.Equals(bq5) {
		t.Error("Should not be equal with different minShouldMatch")
	}
}
