// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestKnnFloatVectorQuery.java
//
// Concrete float specialisation of BaseKnnVectorQueryTestCase. It supplies the
// floatKnnFixture (used by the shared scenario runners and by the seeded /
// patience subclass suites) and the float-only @Test methods (testToString,
// testGetTarget, testScoreDotProduct, …).

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// floatKnnFixture builds KnnFloatVectorQuery instances and float vector fields.
type floatKnnFixture struct{}

func (floatKnnFixture) newQuery(field string, target []float32, k int, filter search.Query) search.Query {
	if filter == nil {
		return search.NewKnnFloatVectorQuery(field, target, k)
	}
	return search.NewKnnFloatVectorQueryWithFilter(field, target, k, filter)
}

func (floatKnnFixture) addVectorDoc(ix *integrationIndex, field string, vec []float32,
	sim index.VectorSimilarityFunction, extra ...document.IndexableField) {
	doc := document.NewDocument()
	f, err := document.NewKnnFloatVectorField(field, vec, sim)
	if err != nil {
		ix.t.Fatalf("NewKnnFloatVectorField: %v", err)
	}
	doc.Add(f)
	for _, e := range extra {
		doc.Add(e)
	}
	ix.addDoc(doc)
}

func (floatKnnFixture) queryTypeName() string { return "KnnFloatVectorQuery" }

func (floatKnnFixture) newIndex(t *testing.T) *integrationIndex { return newIntegrationIndex(t) }

// TestKnnFloatVectorQuery runs the full inherited BaseKnnVectorQueryTestCase
// scenario set against the float fixture.
func TestKnnFloatVectorQuery(t *testing.T) {
	runKnnAllScenarios(t, floatKnnFixture{})
}

// TestKnnFloatVectorQuery_GetTarget mirrors testGetTarget.
func TestKnnFloatVectorQuery_GetTarget(t *testing.T) {
	target := []float32{0, 1}
	q := search.NewKnnFloatVectorQuery("f1", target, 10)
	got := q.GetTargetCopy()
	if len(got) != len(target) {
		t.Fatalf("target length = %d, want %d", len(got), len(target))
	}
	for i := range target {
		if got[i] != target[i] {
			t.Fatalf("target[%d] = %f, want %f", i, got[i], target[i])
		}
	}
	// GetTargetCopy must return a defensive copy, not the same backing array.
	got[0] = 99
	if q.GetTargetCopy()[0] == 99 {
		t.Fatalf("GetTargetCopy must return an independent copy")
	}
}

// TestKnnFloatVectorQuery_ToString mirrors testToString's field-prefix
// assertion. Deviation: Gocene's KnnFloatVectorQuery.String() formats as
// "KnnFloatVectorQuery(field=field, k=10, dim=2)" rather than Lucene's
// "KnnFloatVectorQuery:field[0.0,...][10]"; the format differs but conveys the
// same field/k/dimension, so the assertion checks those components are present.
func TestKnnFloatVectorQuery_ToString(t *testing.T) {
	q := search.NewKnnFloatVectorQuery("field", []float32{0.0, 1.0}, 10)
	s := q.String()
	for _, want := range []string{"KnnFloatVectorQuery", "field", "10"} {
		if !containsToken(s, want) {
			t.Fatalf("String() %q missing %q", s, want)
		}
	}
}

// TestKnnFloatVectorQuery_ScoreDotProduct mirrors testScoreDotProduct: the
// per-doc DOT_PRODUCT scores must match the analytic values.
func TestKnnFloatVectorQuery_ScoreDotProduct(t *testing.T) {
	ix := newIntegrationIndex(t)
	for j := 1; j <= 5; j++ {
		vec := l2normalize([]float32{float32(j), float32(j * j)})
		doc := document.NewDocument()
		f, err := document.NewKnnFloatVectorField("field", vec, index.VectorSimilarityFunctionDotProduct)
		if err != nil {
			t.Fatalf("field: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}
	ix.forceMerge(1)
	s, cleanup := ix.searcher()
	defer cleanup()

	q := search.NewKnnFloatVectorQuery("field", l2normalize([]float32{2, 3}), 3)
	rewritten, err := q.Rewrite(s.GetIndexReader())
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	weight, err := s.CreateWeight(rewritten, search.COMPLETE, 1.0)
	if err != nil {
		t.Fatalf("weight: %v", err)
	}
	leaves, _ := s.GetIndexReader().Leaves()
	if len(leaves) != 1 {
		t.Fatalf("expected single segment, got %d", len(leaves))
	}
	scorer, err := weight.Scorer(leaves[0])
	if err != nil || scorer == nil {
		t.Fatalf("scorer: %v", err)
	}

	// score0 = ((2,3)·(1,1)=5)/(||2,3||·||1,1||=sqrt(26)), normalized (1+x)/2.
	score0 := float32((1 + (2*1+3*1)/math.Sqrt((2*2+3*3)*(1*1+1*1))) / 2)
	// score1 = ((2,3)·(2,4)=16)/(||2,3||·||2,4||=sqrt(260)), normalized (1+x)/2.
	score1 := float32((1 + (2*2+3*4)/math.Sqrt((2*2+3*3)*(2*2+4*4))) / 2)

	if got := scorer.GetMaxScore(search.NO_MORE_DOCS); math.Abs(float64(got-score1)) > 1e-4 {
		t.Fatalf("getMaxScore = %f, want %f", got, score1)
	}
	doc, _ := scorer.NextDoc()
	if doc != 0 {
		t.Fatalf("first doc = %d, want 0", doc)
	}
	if got := scorer.Score(); math.Abs(float64(got-score0)) > 1e-4 {
		t.Fatalf("doc0 score = %f, want %f", got, score0)
	}
	adv, _ := scorer.Advance(1)
	if adv != 1 {
		t.Fatalf("advance(1) = %d, want 1", adv)
	}
	if got := scorer.Score(); math.Abs(float64(got-score1)) > 1e-4 {
		t.Fatalf("doc1 score = %f, want %f", got, score1)
	}
	end, _ := scorer.Advance(4)
	if end != search.NO_MORE_DOCS {
		t.Fatalf("advance(4) = %d, want NO_MORE_DOCS", end)
	}
}

// containsToken reports whether s contains the substring sub.
func containsToken(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// l2normalize returns a unit-length copy of v (matching VectorUtil.l2normalize).
func l2normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sum))
	out := make([]float32, len(v))
	if norm == 0 {
		copy(out, v)
		return out
	}
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}