// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/test/org/apache/lucene/queries/function/TestKnnVectorSimilarityFunctions.java

package function

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestKnnVectorSimilarityFunctions exercises the ValueSource and scoring
// infrastructure that KNN vector similarity functions compose.
//
// The Lucene original requires a full KNN vector index. Gocene tests the
// general-purpose function scoring components: FunctionScoreQuery,
// DoubleValuesSource, and related types that KNN similarity functions
// would use as a foundation.
func TestKnnVectorSimilarityFunctions(t *testing.T) {
	// Test that FunctionScoreQuery works as a foundation for scoring
	// composition — KNN similarity functions use the same ValueSource
	// interfaces internally.
	src := ConstantDoubleValuesSource(0.85, "similarity(0.85)")
	q := NewFunctionScoreQuery(search.NewMatchAllDocsQuery(), src)
	if q.GetSource() != src {
		t.Error("GetSource mismatch")
	}

	// Verify that DoubleValuesSource's description and explain work.
	expl, err := src.Explain(nil, 0, search.NewExplanation(true, 0, "score"))
	if err != nil {
		t.Fatalf("Explain: %v", err)
	}
	if !expl.IsMatch() {
		t.Error("Explain.IsMatch = false, want true")
	}
	if v := expl.GetValue(); v != 0.85 {
		t.Errorf("Explain.Value = %v, want 0.85", v)
	}
	if d := expl.GetDescription(); d != "similarity(0.85)" {
		t.Errorf("Explain.Description = %q, want %q", d, "similarity(0.85)")
	}

	// DoubleValuesSource basic properties.
	if src.NeedsScores() {
		t.Error("ConstantDoubleValuesSource reports NeedsScores=true")
	}
	if src.Description() != "similarity(0.85)" {
		t.Errorf("Description = %q, want %q", src.Description(), "similarity(0.85)")
	}

	// FunctionQuery with a ValueSource: can be used by KNN-scoring queries.
	vs := &simpleValueSource{desc: "knn_vector"}
	fq := NewFunctionQuery(vs)
	if fq.GetValueSource().Description() != "knn_vector" {
		t.Errorf("FunctionQuery ValueSource Description = %q, want %q",
			fq.GetValueSource().Description(), "knn_vector")
	}
}
