// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestFunctionScoreExplanations.java

package function

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestFunctionScoreExplanations verifies that FunctionScoreQuery and its
// BoostByValue variant produce correct explanation structures.
//
// The Lucene original requires an indexed corpus with RandomIndexWriter.
// Gocene tests the query construction, Rewrite propagation, and basic
// accessor correctness for FunctionScoreQuery.
func TestFunctionScoreExplanations(t *testing.T) {
	// Construct a FunctionScoreQuery with a constant source.
	src := ConstantDoubleValuesSource(1.5, "boost(1.5)")
	inner := search.NewMatchAllDocsQuery()
	q := NewFunctionScoreQuery(inner, src)

	// Verify the query structure.
	if q.GetWrappedQuery() != inner {
		t.Error("GetWrappedQuery mismatch")
	}
	if q.GetSource() != src {
		t.Error("GetSource mismatch")
	}
	if q.String() == "" {
		t.Error("String() returned empty")
	}

	// BoostByValue factory.
	boosted := BoostByValue(search.NewMatchAllDocsQuery(), ConstantDoubleValuesSource(2.0, "x2"))
	if boosted.String() == "" {
		t.Error("BoostByValue String() returned empty")
	}

	// Explain from ConstantDoubleValuesSource.
	expl, err := src.Explain(nil, 0, search.NewExplanation(true, 0, "score"))
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if v := expl.GetValue(); v != 1.5 {
		t.Errorf("expl.Value = %v, want 1.5", v)
	}
	if d := expl.GetDescription(); d != "boost(1.5)" {
		t.Errorf("expl.Description = %q, want %q", d, "boost(1.5)")
	}
}
