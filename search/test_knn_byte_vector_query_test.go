// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestKnnByteVectorQuery.java
//
// Concrete byte specialisation of BaseKnnVectorQueryTestCase. It supplies the
// byteKnnFixture (used by the shared scenario runners, the MMap subclass and the
// seeded / patience subclass suites) and the byte-only @Test methods.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// byteKnnFixture builds KnnByteVectorQuery instances and byte vector fields.
// Float test vectors are converted to bytes (the shared scenarios only use
// small integral non-negative components, which round-trip losslessly), exactly
// like TestKnnByteVectorQuery.floatToBytes.
type byteKnnFixture struct{}

func (byteKnnFixture) newQuery(field string, target []float32, k int, filter search.Query) search.Query {
	b := floatToBytes(target)
	if filter == nil {
		return search.NewKnnByteVectorQuery(field, b, k)
	}
	return search.NewKnnByteVectorQueryWithFilter(field, b, k, filter)
}

func (byteKnnFixture) addVectorDoc(ix *integrationIndex, field string, vec []float32,
	sim index.VectorSimilarityFunction, extra ...document.IndexableField) {
	doc := document.NewDocument()
	f, err := document.NewKnnByteVectorField(field, floatToBytes(vec), sim)
	if err != nil {
		ix.t.Fatalf("NewKnnByteVectorField: %v", err)
	}
	doc.Add(f)
	for _, e := range extra {
		doc.Add(e)
	}
	ix.addDoc(doc)
}

func (byteKnnFixture) queryTypeName() string { return "KnnByteVectorQuery" }

func (byteKnnFixture) newIndex(t *testing.T) *integrationIndex { return newIntegrationIndex(t) }

// floatToBytes converts a float vector with small integral components to a byte
// vector. Mirrors TestKnnByteVectorQuery.floatToBytes.
func floatToBytes(query []float32) []byte {
	b := make([]byte, len(query))
	for i, v := range query {
		if v > 127 || v < -128 || v != float32(int32(v)) {
			panic("float value cannot be converted to byte")
		}
		b[i] = byte(int8(v))
	}
	return b
}

// TestKnnByteVectorQuery runs the full inherited BaseKnnVectorQueryTestCase
// scenario set against the byte fixture.
func TestKnnByteVectorQuery(t *testing.T) {
	runKnnAllScenarios(t, byteKnnFixture{})
}

// TestKnnByteVectorQuery_GetTarget mirrors testGetTarget.
func TestKnnByteVectorQuery_GetTarget(t *testing.T) {
	target := floatToBytes([]float32{0, 1})
	q := search.NewKnnByteVectorQuery("f1", target, 10)
	got := q.GetTargetCopy()
	if len(got) != len(target) {
		t.Fatalf("target length = %d, want %d", len(got), len(target))
	}
	for i := range target {
		if got[i] != target[i] {
			t.Fatalf("target[%d] = %d, want %d", i, got[i], target[i])
		}
	}
	got[0] = 99
	if q.GetTargetCopy()[0] == 99 {
		t.Fatalf("GetTargetCopy must return an independent copy")
	}
}

// TestKnnByteVectorQuery_ToString mirrors testToString's field-prefix check.
// Deviation (see the float counterpart): Gocene's String() format differs from
// Lucene's "KnnByteVectorQuery:field[0,...][10]".
func TestKnnByteVectorQuery_ToString(t *testing.T) {
	q := search.NewKnnByteVectorQuery("field", floatToBytes([]float32{0, 1}), 10)
	s := q.String()
	for _, want := range []string{"KnnByteVectorQuery", "field", "10"} {
		if !containsToken(s, want) {
			t.Fatalf("String() %q missing %q", s, want)
		}
}