// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPatienceFloatVectorQuery.java
//
// TestPatienceFloatVectorQuery extends BaseKnnVectorQueryTestCase, wrapping each
// KnnFloatVectorQuery in a PatienceKnnVectorQuery (the saturation-based early
// termination wrapper). The Go port supplies a patience fixture and runs the
// full inherited scenario set through it, plus the patience-specific toString.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// patienceDefaultPatience matches the patience Lucene's
// PatienceKnnVectorQuery.fromFloatQuery derives by default (max(7, k/10)); the
// shared scenarios use small k, so 7 is the effective value.
const patienceDefaultPatience = 7

// patienceFloatKnnFixture wraps floatKnnFixture's queries in a
// PatienceKnnVectorQuery.
type patienceFloatKnnFixture struct {
	floatKnnFixture
}

func (patienceFloatKnnFixture) newQuery(field string, target []float32, k int, filter search.Query) search.Query {
	var inner search.Query
	if filter == nil {
		inner = search.NewKnnFloatVectorQuery(field, target, k)
	} else {
		inner = search.NewKnnFloatVectorQueryWithFilter(field, target, k, filter)
	}
	return search.NewPatienceKnnVectorQuery(inner, patienceDefaultPatience)
}

// TestPatienceFloatVectorQuery runs the inherited BaseKnnVectorQueryTestCase
// scenario set through the patience-wrapped float query. Because Gocene's
// patience saturation collector is not yet wired into the leaf-level search,
// the wrapper produces the same final top-K as the underlying query — which the
// shared scenarios assert.
func TestPatienceFloatVectorQuery(t *testing.T) {
	runKnnAllScenarios(t, patienceFloatKnnFixture{})
}

// TestPatienceFloatVectorQuery_ToString mirrors testToString.
//
// Deviation: Gocene's PatienceKnnVectorQuery.String() formats as
// "PatienceKnnVectorQuery(patience=7, inner=...)" rather than Lucene's
// "PatienceKnnVectorQuery{saturationThreshold=0.995, patience=7, delegate=...}"
// — Gocene does not carry an explicit saturationThreshold field. The
// patience value and the wrapped query identity (the load-bearing facts) are
// asserted instead of the exact byte string.
func TestPatienceFloatVectorQuery_ToString(t *testing.T) {
	ix := newIntegrationIndex(t)
	doc := document.NewDocument()
	f, _ := document.NewKnnFloatVectorField("field", []float32{0, 1}, index.VectorSimilarityFunctionEuclidean)
	doc.Add(f)
	ix.addDoc(doc)
	s, cleanup := ix.searcher()
	defer cleanup()

	inner := search.NewKnnFloatVectorQuery("field", []float32{0.0, 1.0}, 10)
	q := search.NewPatienceKnnVectorQuery(inner, patienceDefaultPatience)
	str := q.String()
	for _, want := range []string{"PatienceKnnVectorQuery", "patience=7", "KnnFloatVectorQuery", "field"} {
		if !containsToken(str, want) {
			t.Fatalf("String() %q missing %q", str, want)
		}

	// The wrapped query must still rewrite to a runnable DocAndScoreQuery.
	rewritten, err := q.Rewrite(s.GetIndexReader())
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if _, ok := rewritten.(*search.DocAndScoreQuery); !ok {
		t.Fatalf("patience query must rewrite to DocAndScoreQuery, got %T", rewritten)
	}
}