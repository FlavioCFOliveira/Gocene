// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestParentBlockJoinFloatKnnVectorQuery.
package join

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestParentBlockJoinFloatKnnVectorQuery_VectorEncodingMismatch corresponds to
// TestParentBlockJoinFloatKnnVectorQuery.testVectorEncodingMismatch.
// Skipped: requires DirectoryReader + IndexSearcher with byte-vector field.
func TestParentBlockJoinFloatKnnVectorQuery_VectorEncodingMismatch(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinFloatKnnVectorQuery_ScoreCosine corresponds to
// TestParentBlockJoinFloatKnnVectorQuery.testScoreCosine.
// Skipped: requires DirectoryReader + IndexSearcher with KNN float vectors.
func TestParentBlockJoinFloatKnnVectorQuery_ScoreCosine(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestParentBlockJoinFloatKnnVectorQuery_ToString corresponds to
// TestParentBlockJoinFloatKnnVectorQuery.testToString.
func TestParentBlockJoinFloatKnnVectorQuery_ToString(t *testing.T) {
	// Without filter.
	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{0, 1}, 10, nil, nil)
	s := q.String()
	if !strings.HasPrefix(s, "DiversifyingChildrenFloatKnnVectorQuery:field") {
		t.Errorf("String() prefix wrong: %q", s)
	}
	if !strings.Contains(s, "[10]") {
		t.Errorf("String() should contain [10]: %q", s)
	}
	if strings.Contains(s, "[") && strings.Count(s, "[") > 2 {
		// No filter → at most 2 brackets (vector + k).
		t.Errorf("String() without filter has unexpected extra bracket: %q", s)
	}

	// With filter: TermQuery(id=text) → "id:text" via Term.String().
	filter := search.NewTermQuery(index.NewTerm("id", "text"))
	q2 := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{0.0, 1.0}, 10, filter, nil)
	s2 := q2.String()
	if !strings.Contains(s2, "[id:text]") {
		t.Errorf("String() with filter should contain [id:text]: %q", s2)
	}
}

// TestParentBlockJoinFloatKnnVectorQuery_TargetCopy verifies the target vector
// is defensively copied on construction.
func TestParentBlockJoinFloatKnnVectorQuery_TargetCopy(t *testing.T) {
	orig := []float32{1, 2, 3}
	q := NewDiversifyingChildrenFloatKnnVectorQuery("f", orig, 5, nil, nil)
	orig[0] = 999
	if q.Target[0] == 999 {
		t.Error("target was not defensively copied")
	}
}
