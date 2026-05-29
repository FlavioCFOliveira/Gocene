// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestConstantScoreQuery_Basics(t *testing.T) {
	tq := NewTermQuery(index.NewTerm("field", "value"))
	csq := NewConstantScoreQuery(tq)

	if csq.Query() != tq {
		t.Error("Query() should return the wrapped query")
	}
	if csq.Score() != 1.0 {
		t.Errorf("Expected default score 1.0, got %f", csq.Score())
	}

	csq.SetScore(2.5)
	if csq.Score() != 2.5 {
		t.Errorf("Expected score 2.5, got %f", csq.Score())
	}
}

func TestConstantScoreQuery_Rewrite(t *testing.T) {
	// Nested BooleanQuery that should be rewritten
	inner := NewBooleanQuery()
	tq := NewTermQuery(index.NewTerm("f", "v"))
	inner.Add(tq, MUST)

	csq := NewConstantScoreQuery(inner)
	rewritten, err := csq.Rewrite(nil)
	if err != nil {
		t.Fatalf("Rewrite failed: %v", err)
	}

	// inner rewrites to tq, so csq should now wrap tq
	if rcsq, ok := rewritten.(*ConstantScoreQuery); ok {
		if !rcsq.Query().Equals(tq) {
			t.Errorf("Expected wrapped query to be rewritten to TermQuery, got %T", rcsq.Query())
		}
	} else {
		t.Errorf("Expected ConstantScoreQuery, got %T", rewritten)
	}
}

// TestConstantScoreQuery_CreateWeightNotNil guards the root-cause bug behind
// rmp #4760 / #4767: ConstantScoreQuery must override CreateWeight rather than
// inheriting BaseQuery.CreateWeight (which returns a nil Weight). A nil Weight
// makes the query silently match nothing.
func TestConstantScoreQuery_CreateWeightNotNil(t *testing.T) {
	tq := NewTermQuery(index.NewTerm("field", "value"))
	csq := NewConstantScoreQuery(tq)

	// needsScores=false returns the inner Weight directly (TermWeight here).
	wNoScores, err := csq.CreateWeight(nil, false, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight(needsScores=false) failed: %v", err)
	}
	if wNoScores == nil {
		t.Fatal("CreateWeight(needsScores=false) returned a nil Weight")
	}

	// needsScores=true wraps the inner Weight in a ConstantScoreWeight.
	wScores, err := csq.CreateWeight(nil, true, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight(needsScores=true) failed: %v", err)
	}
	if wScores == nil {
		t.Fatal("CreateWeight(needsScores=true) returned a nil Weight")
	}
	if _, ok := wScores.(*ConstantScoreWeight); !ok {
		t.Errorf("Expected a *ConstantScoreWeight when scores are needed, got %T", wScores)
	}
}
