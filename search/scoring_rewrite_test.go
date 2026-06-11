// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ScoringRewrite.java
//
// No dedicated Java test peer found (TestScoringRewrite / ScoringRewriteTest
// do not exist in Lucene 10.4.0 core tests).  These tests cover the Go public
// contract of the structural port.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestScoringRewrite_MaxClauseCount verifies getter/setter round-trip.
func TestScoringRewrite_MaxClauseCount(t *testing.T) {
	orig := search.GetMaxClauseCount()
	defer search.SetMaxClauseCount(orig)

	search.SetMaxClauseCount(512)
	if got := search.GetMaxClauseCount(); got != 512 {
		t.Errorf("GetMaxClauseCount()=%d, want 512", got)
	}
}

// TestScoringRewrite_DefaultMaxClauseCount verifies the default is 1024.
func TestScoringRewrite_DefaultMaxClauseCount(t *testing.T) {
	if search.DefaultMaxClauseCount != 1024 {
		t.Errorf("DefaultMaxClauseCount=%d, want 1024", search.DefaultMaxClauseCount)
	}
}

// TestScoringRewrite_ErrTooManyClauses verifies the sentinel error is non-nil
// and has a useful message.
func TestScoringRewrite_ErrTooManyClauses(t *testing.T) {
	if search.ErrTooManyClauses == nil {
		t.Fatal("ErrTooManyClauses is nil")
	}
	if search.ErrTooManyClauses.Error() == "" {
		t.Fatal("ErrTooManyClauses has empty message")
	}
}

// TestScoringRewrite_ScoringBooleanRewriteMethodNotNil verifies the sentinel instance exists.
func TestScoringRewrite_ScoringBooleanRewriteMethodNotNil(t *testing.T) {
	if search.ScoringBooleanRewriteMethod == nil {
		t.Fatal("ScoringBooleanRewriteMethod is nil")
	}
}

// TestScoringRewrite_ConstantScoreBooleanRewriteMethodNotNil verifies the sentinel instance exists.
func TestScoringRewrite_ConstantScoreBooleanRewriteMethodNotNil(t *testing.T) {
	if search.ConstantScoreBooleanRewriteMethod == nil {
		t.Fatal("ConstantScoreBooleanRewriteMethod is nil")
	}
}

// TestScoringRewrite_NewTermFreqBoostByteStart verifies construction.
func TestScoringRewrite_NewTermFreqBoostByteStart(t *testing.T) {
	arr := search.NewTermFreqBoostByteStart(4)
	if arr == nil {
		t.Fatal("NewTermFreqBoostByteStart returned nil")
	}
}

// TestScoringRewrite_NewParallelArraysTermCollector verifies construction.
func TestScoringRewrite_NewParallelArraysTermCollector(t *testing.T) {
	col := search.NewParallelArraysTermCollector()
	if col == nil {
		t.Fatal("NewParallelArraysTermCollector returned nil")
	}
	if col.Terms == nil {
		t.Fatal("col.Terms is nil")
	}
	if col.Array == nil {
		t.Fatal("col.Array is nil")
	}
}

// TestScoringRewrite_RewriteDegraded verifies that Rewrite returns the original
// query unchanged (degraded path until TermsEnum.TermState() lands).
func TestScoringRewrite_RewriteDegraded(t *testing.T) {
	q := search.NewMultiTermQuery("field", nil)
	got, err := search.ScoringBooleanRewriteMethod.Rewrite(q, nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if got != q {
		t.Errorf("expected original query returned in degraded path")
	}

// TestScoringRewrite_ConstantScoreBooleanRewriteWraps verifies that the
// constant-score variant wraps the result in a ConstantScoreQuery.
func TestScoringRewrite_ConstantScoreBooleanRewriteWraps(t *testing.T) {
	q := search.NewMultiTermQuery("field", nil)
	got, err := search.ConstantScoreBooleanRewriteMethod.Rewrite(q, nil)
	if err != nil {
		t.Fatalf("Rewrite error: %v", err)
	}
	if got == nil {
		t.Fatal("Rewrite returned nil")
	}
	if _, ok := got.(*search.ConstantScoreQuery); !ok {
		t.Errorf("expected *search.ConstantScoreQuery, got %T", got)
	}
}