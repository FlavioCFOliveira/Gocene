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
