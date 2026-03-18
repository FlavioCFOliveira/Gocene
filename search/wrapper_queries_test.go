// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestBoostQuery_Clone(t *testing.T) {
	term := index.NewTerm("field", "value")
	innerQuery := NewTermQuery(term)
	query := NewBoostQuery(innerQuery, 2.5)

	cloned := query.Clone()
	if cloned == nil {
		t.Fatal("Clone returned nil")
	}

	boostQuery, ok := cloned.(*BoostQuery)
	if !ok {
		t.Fatal("Clone did not return *BoostQuery")
	}

	if boostQuery.Boost() != 2.5 {
		t.Errorf("Expected boost 2.5, got %f", boostQuery.Boost())
	}

	if boostQuery.Query() == nil {
		t.Error("Cloned query has nil inner query")
	}
}

func TestBoostQuery_Equals(t *testing.T) {
	term1 := index.NewTerm("field", "value1")
	term2 := index.NewTerm("field", "value2")

	q1 := NewBoostQuery(NewTermQuery(term1), 2.0)
	q2 := NewBoostQuery(NewTermQuery(term1), 2.0)
	q3 := NewBoostQuery(NewTermQuery(term2), 2.0)
	q4 := NewBoostQuery(NewTermQuery(term1), 3.0)

	if !q1.Equals(q2) {
		t.Error("Expected q1 and q2 to be equal")
	}

	if q1.Equals(q3) {
		t.Error("Expected q1 and q3 to be different (different inner query)")
	}

	if q1.Equals(q4) {
		t.Error("Expected q1 and q4 to be different (different boost)")
	}
}
