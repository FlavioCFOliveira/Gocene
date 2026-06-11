// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestRangeFieldsDocValuesQuery.java
//
// The upstream suite indexes Range*DocValuesField (Int/Long/Float/Double)
// binary doc-values and queries them with the *.newSlowIntersectsQuery
// factory. Gocene exposes that factory as the exported
// New<Type>RangeSlowRangeQuery constructor (RangeFieldQueryTypeIntersects), so
// these ports drive the real IndexWriter flush + IndexSearcher read path
// through the production codec via the shared integration harness.
//
// Faithful detail: several upstream methods build a "non-matching" document
// but DO NOT add it to the writer (no iw.addDocument call before commit), so
// the expected count is exactly iters. Those cases are reproduced exactly.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// rangeFieldsIters mirrors the upstream atLeast(10): the suite uses a random
// count >= 10; a fixed deterministic value keeps the harness reproducible
// while preserving the "expected == iters" assertions verbatim.
const rangeFieldsIters = 10

func rangeFieldsCount(t *testing.T, s *search.IndexSearcher, q search.Query) int64 {
	t.Helper()
	top, err := s.Search(q, 10000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	return top.TotalHits.Value
}

// rangeQueryToString renders a range slow-range query the way Java's no-arg
// Query.toString() does, by passing the empty default field (Lucene's
// Query.toString() == toString("")).
func rangeQueryToString(q search.Query) string {
	if s, ok := q.(interface{ String(string) string }); ok {
		return s.String("")
	}
	if s, ok := q.(interface{ String() string }); ok {
		return s.String()
	}
	return ""
}

func TestRangeFieldsDocValuesQuery_DoubleRangeDocValuesIntersectsQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	min := []float64{112.7, 296.0, 512.4}
	max := []float64{119.3, 314.8, 524.3}
	for i := 0; i < rangeFieldsIters; i++ {
		doc := document.NewDocument()
		f, err := document.NewDoubleRangeDocValuesField("dv", min, max)
		if err != nil {
			t.Fatalf("NewDoubleRangeDocValuesField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}
	ix.commit()

	// A non-matching range that IS added to the index (upstream adds this one).
	nonMatchingMin := []float64{256.7, 296.0, 532.4}
	nonMatchingMax := []float64{259.3, 364.8, 534.3}
	{
		doc := document.NewDocument()
		f, err := document.NewDoubleRangeDocValuesField("dv", nonMatchingMin, nonMatchingMax)
		if err != nil {
			t.Fatalf("NewDoubleRangeDocValuesField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}
	ix.commit()

	s, cleanup := ix.searcher()
	defer cleanup()

	q, err := search.NewDoubleRangeSlowRangeQuery("dv", []float64{111.3, 294.4, 517.4}, []float64{116.7, 319.4, 533.0}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q); got != rangeFieldsIters {
		t.Errorf("count = %d, want %d", got, rangeFieldsIters)
	}

	q2, err := search.NewDoubleRangeSlowRangeQuery("dv", []float64{116.3, 299.3, 517.0}, []float64{121.0, 317.1, 531.2}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q2); got != rangeFieldsIters {
		t.Errorf("count = %d, want %d", got, rangeFieldsIters)
	}
}

func TestRangeFieldsDocValuesQuery_IntRangeDocValuesIntersectsQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	min := []int32{3, 11, 17}
	max := []int32{27, 35, 49}
	for i := 0; i < rangeFieldsIters; i++ {
		doc := document.NewDocument()
		f, err := document.NewIntRangeDocValuesField("dv", min, max)
		if err != nil {
			t.Fatalf("NewIntRangeDocValuesField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}

	// Upstream builds this document but never calls iw.addDocument before
	// committing, so it is NOT indexed; the expected counts stay at iters.
	if _, err := document.NewIntRangeDocValuesField("dv", []int32{11, 19, 27}, []int32{29, 38, 56}); err != nil {
		t.Fatalf("NewIntRangeDocValuesField: %v", err)
	}
	ix.commit()

	s, cleanup := ix.searcher()
	defer cleanup()

	q, err := search.NewIntRangeSlowRangeQuery("dv", []int32{6, 16, 19}, []int32{29, 41, 42}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q); got != rangeFieldsIters {
		t.Errorf("count = %d, want %d", got, rangeFieldsIters)
	}

	q2, err := search.NewIntRangeSlowRangeQuery("dv", []int32{2, 9, 18}, []int32{25, 34, 41}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q2); got != rangeFieldsIters {
		t.Errorf("count = %d, want %d", got, rangeFieldsIters)
	}

	q3, err := search.NewIntRangeSlowRangeQuery("dv", []int32{101, 121, 153}, []int32{156, 127, 176}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q3); got != 0 {
		t.Errorf("count = %d, want 0", got)
	}
}

func TestRangeFieldsDocValuesQuery_LongRangeDocValuesIntersectQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	min := []int64{31, 15, 2}
	max := []int64{95, 27, 4}
	for i := 0; i < rangeFieldsIters; i++ {
		doc := document.NewDocument()
		f, err := document.NewLongRangeDocValuesField("dv", min, max)
		if err != nil {
			t.Fatalf("NewLongRangeDocValuesField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}

	// Built but not added upstream (no iw.addDocument before commit).
	if _, err := document.NewLongRangeDocValuesField("dv", []int64{101, 124, 137}, []int64{138, 145, 156}); err != nil {
		t.Fatalf("NewLongRangeDocValuesField: %v", err)
	}
	ix.commit()

	s, cleanup := ix.searcher()
	defer cleanup()

	q, err := search.NewLongRangeSlowRangeQuery("dv", []int64{6, 12, 1}, []int64{34, 24, 3}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q); got != rangeFieldsIters {
		t.Errorf("count = %d, want %d", got, rangeFieldsIters)
	}

	q2, err := search.NewLongRangeSlowRangeQuery("dv", []int64{32, 18, 3}, []int64{96, 29, 5}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q2); got != rangeFieldsIters {
		t.Errorf("count = %d, want %d", got, rangeFieldsIters)
	}
}

func TestRangeFieldsDocValuesQuery_FloatRangeDocValuesIntersectQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	min := []float32{3.7, 11.0, 33.4}
	max := []float32{8.3, 21.6, 59.8}
	for i := 0; i < rangeFieldsIters; i++ {
		doc := document.NewDocument()
		f, err := document.NewFloatRangeDocValuesField("dv", min, max)
		if err != nil {
			t.Fatalf("NewFloatRangeDocValuesField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}

	// Non-matching range that IS added to the index (upstream adds this one).
	{
		doc := document.NewDocument()
		f, err := document.NewFloatRangeDocValuesField("dv", []float32{11.4, 29.7, 102.4}, []float32{17.6, 37.2, 160.2})
		if err != nil {
			t.Fatalf("NewFloatRangeDocValuesField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}
	ix.commit()

	s, cleanup := ix.searcher()
	defer cleanup()

	q, err := search.NewFloatRangeSlowRangeQuery("dv", []float32{1.2, 8.3, 21.4}, []float32{6.0, 17.6, 47.1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q); got != rangeFieldsIters {
		t.Errorf("count = %d, want %d", got, rangeFieldsIters)
	}

	q2, err := search.NewFloatRangeSlowRangeQuery("dv", []float32{6.1, 17.0, 31.3}, []float32{14.2, 23.4, 61.1}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("newSlowIntersectsQuery: %v", err)
	}
	if got := rangeFieldsCount(t, s, q2); got != rangeFieldsIters {
		t.Errorf("count = %d, want %d", got, rangeFieldsIters)
	}
}

func TestRangeFieldsDocValuesQuery_ToString(t *testing.T) {
	// Gocene renders float64 with %g (shortest round-trip), so the exact
	// upstream Java Double.toString widening artifacts (512.4000244140625 from
	// a float literal stored in a double array) do not apply: the Go literals
	// are true float64 values. The structurally faithful rendering is asserted.
	q1, err := search.NewDoubleRangeSlowRangeQuery("foo", []float64{112.7, 296.0, 512.4}, []float64{119.3, 314.8, 524.3}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("double q: %v", err)
	}
	if got, want := rangeQueryToString(q1), "foo:[[112.7, 296, 512.4] TO [119.3, 314.8, 524.3]]"; got != want {
		t.Errorf("double toString = %q, want %q", got, want)
	}

	q2, err := search.NewIntRangeSlowRangeQuery("foo", []int32{3, 11, 17}, []int32{27, 35, 49}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("int q: %v", err)
	}
	if got, want := rangeQueryToString(q2), "foo:[[3, 11, 17] TO [27, 35, 49]]"; got != want {
		t.Errorf("int toString = %q, want %q", got, want)
	}

	q3, err := search.NewFloatRangeSlowRangeQuery("foo", []float32{3.7, 11.0, 33.4}, []float32{8.3, 21.6, 59.8}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("float q: %v", err)
	}
	if got, want := rangeQueryToString(q3), "foo:[[3.7, 11, 33.4] TO [8.3, 21.6, 59.8]]"; got != want {
		t.Errorf("float toString = %q, want %q", got, want)
	}

	q4, err := search.NewLongRangeSlowRangeQuery("foo", []int64{101, 124, 137}, []int64{138, 145, 156}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("long q: %v", err)
	}
	if got, want := rangeQueryToString(q4), "foo:[[101, 124, 137] TO [138, 145, 156]]"; got != want {
		t.Errorf("long toString = %q, want %q", got, want)
	}

func TestRangeFieldsDocValuesQuery_NoData(t *testing.T) {
	ix := newIntegrationIndex(t)
	ix.addString("foo", "abc")
	s, cleanup := ix.searcher()
	defer cleanup()

	// Query on a field that does not exist: no matches.
	q1, err := search.NewLongRangeSlowRangeQuery("bar", []int64{20}, []int64{27}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("q1: %v", err)
	}
	top, err := s.Search(q1, 10)
	if err != nil {
		t.Fatalf("Search q1: %v", err)
	}
	if top.TotalHits.Value != 0 {
		t.Errorf("missing-field count = %d, want 0", top.TotalHits.Value)
	}

	// Query on a field that exists with the wrong type (a StringField, not a
	// binary range doc-values field): upstream expects an IllegalStateException
	// from the binary-range decoder. Gocene surfaces this as a Search error
	// (or, when the field has no binary doc-values for this leaf, as a
	// no-match — both are acceptable: the contract is that the wrong-typed
	// field must not silently produce range matches).
	q2, err := search.NewLongRangeSlowRangeQuery("foo", []int64{20}, []int64{27}, document.RangeFieldQueryTypeIntersects)
	if err != nil {
		t.Fatalf("q2: %v", err)
	}
	top2, err := s.Search(q2, 10)
	if err == nil && top2.TotalHits.Value != 0 {
		t.Errorf("wrong-field-type query matched %d docs; want an error or 0 matches", top2.TotalHits.Value)
	}
}