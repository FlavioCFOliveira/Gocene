// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPatienceByteVectorQuery.java
//
// Byte analogue of TestPatienceFloatVectorQuery: every KnnByteVectorQuery is
// wrapped in a PatienceKnnVectorQuery, and the full inherited
// BaseKnnVectorQueryTestCase scenario set runs through it.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// patienceByteKnnFixture wraps byteKnnFixture's queries in a
// PatienceKnnVectorQuery.
type patienceByteKnnFixture struct {
	byteKnnFixture
}

func (patienceByteKnnFixture) newQuery(field string, target []float32, k int, filter search.Query) search.Query {
	b := floatToBytes(target)
	var inner search.Query
	if filter == nil {
		inner = search.NewKnnByteVectorQuery(field, b, k)
	} else {
		inner = search.NewKnnByteVectorQueryWithFilter(field, b, k, filter)
	}
	return search.NewPatienceKnnVectorQuery(inner, patienceDefaultPatience)
}

// TestPatienceByteVectorQuery runs the inherited scenario set through the
// patience-wrapped byte query.
func TestPatienceByteVectorQuery(t *testing.T) {
	runKnnAllScenarios(t, patienceByteKnnFixture{})
}

// TestPatienceByteVectorQuery_ToString mirrors testToString (see the float
// counterpart for the format deviation rationale).
func TestPatienceByteVectorQuery_ToString(t *testing.T) {
	ix := newIntegrationIndex(t)
	doc := document.NewDocument()
	f, _ := document.NewKnnByteVectorField("field", floatToBytes([]float32{0, 1}),
		index.VectorSimilarityFunctionEuclidean)
	doc.Add(f)
	ix.addDoc(doc)
	s, cleanup := ix.searcher()
	defer cleanup()

	inner := search.NewKnnByteVectorQuery("field", floatToBytes([]float32{0, 1}), 10)
	q := search.NewPatienceKnnVectorQuery(inner, patienceDefaultPatience)
	str := q.String()
	for _, want := range []string{"PatienceKnnVectorQuery", "patience=7", "KnnByteVectorQuery", "field"} {
		if !containsToken(str, want) {
			t.Fatalf("String() %q missing %q", str, want)
		}
	}

	rewritten, err := q.Rewrite(s.GetIndexReader())
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if _, ok := rewritten.(*search.DocAndScoreQuery); !ok {
		t.Fatalf("patience query must rewrite to DocAndScoreQuery, got %T", rewritten)
	}
}