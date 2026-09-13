// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestParentBlockJoinFloatKnnVectorQuery.
package join

import (
	"math"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TestParentBlockJoinFloatKnnVectorQuery_VectorEncodingMismatch corresponds to
// TestParentBlockJoinFloatKnnVectorQuery.testVectorEncodingMismatch: searching
// a float-encoded vector field with a byte-vector query must fail (Lucene
// throws IllegalStateException; Gocene surfaces a typed error from the codec
// reader's encoding check).
func TestParentBlockJoinFloatKnnVectorQuery_VectorEncodingMismatch(t *testing.T) {
	dir, w := newBlockWriter(t)
	d := document.NewDocument()
	vf, err := document.NewKnnFloatVectorField("field", []float32{1, 1}, index.VectorSimilarityFunctionCosine)
	if err != nil {
		t.Fatalf("NewKnnFloatVectorField: %v", err)
	}
	d.Add(vf)
	addBlock(t, w, d, pbjMakeParent(t, ""))
	r, s := commitAndOpen(t, dir, w)

	parentFilter := pbjParentsFilter()
	if err := Check(r, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}
	// A byte query over a float-encoded field must error.
	kvq := NewDiversifyingChildrenByteKnnVectorQuery("field", []byte{1, 2}, 2, nil, parentFilter)
	if _, err := s.Search(kvq, 3); err == nil {
		t.Errorf("byte query over float field: want error, got nil")
	}
}

// TestParentBlockJoinFloatKnnVectorQuery_ScoreCosine corresponds to
// TestParentBlockJoinFloatKnnVectorQuery.testScoreCosine: five single-child
// parent blocks with COSINE-similarity vectors {j, j*j}; the diversifying join
// keeps one best child per parent, and the scorer-level COSINE-normalized
// scores match.
func TestParentBlockJoinFloatKnnVectorQuery_ScoreCosine(t *testing.T) {
	dir, w := newBlockWriter(t)
	for j := 1; j <= 5; j++ {
		child := document.NewDocument()
		vf, err := document.NewKnnFloatVectorField("field", []float32{float32(j), float32(j * j)}, index.VectorSimilarityFunctionCosine)
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		child.Add(vf)
		child.Add(mustStringField(t, "id", pbjItoa(j), true))
		addBlock(t, w, child, pbjMakeParent(t, ""))
	}
	r, s := commitAndOpen(t, dir, w)
	parentFilter := pbjParentsFilter()
	if err := Check(r, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	// Query {2,3}. The two top parents are children "1" ({1,1}) and "2"
	// ({2,4}), with the COSINE-normalized scores below.
	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{2, 3}, 3, nil, parentFilter)
	score0 := float32((1 + (2*1+3*1)/math.Sqrt((2*2+3*3)*(1*1+1*1))) / 2)
	score1 := float32((1 + (2*2+3*4)/math.Sqrt((2*2+3*3)*(2*2+4*4))) / 2)
	pbjAssertScorerResults(t, s, r, q, map[string]float32{"1": score0, "2": score1}, 2)
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
