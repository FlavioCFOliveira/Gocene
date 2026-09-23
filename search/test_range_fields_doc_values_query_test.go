// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestRangeFieldsDocValuesQuery.java
// (Apache Lucene 10.5.0). XRangeDocValuesField.newSlowIntersectsQuery(field,
// min, max) is new XRangeSlowRangeQuery(field, min, max, QueryType.INTERSECTS),
// rendered by search.NewXRangeSlowRangeQuery with RangeFieldQueryTypeIntersects.

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// rfdvMust fails the test on a construction error.
func rfdvMust(t *testing.T) func(search.Query, error) search.Query {
	return func(q search.Query, err error) search.Query {
		t.Helper()
		if err != nil {
			t.Fatalf("newSlowIntersectsQuery: %v", err)
		}
		return q
	}
}

func rfdvAdd(t *testing.T, doc *document.Document, f document.IndexableField, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("new RangeDocValuesField: %v", err)
	}
	doc.Add(f)
}

func TestRangeFieldsDocValuesQueryDoubleRangeDocValuesIntersectsQuery(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	iters := atLeast(10)
	min := []float64{112.7, 296.0, 512.4}
	max := []float64{119.3, 314.8, 524.3}
	for i := 0; i < iters; i++ {
		doc := document.NewDocument()
		f, err := document.NewDoubleRangeDocValuesField("dv", min, max)
		rfdvAdd(t, doc, f, err)
		mustAddDocument(t, iw, doc)
	}
	mustCommit(t, iw)

	nonMatchingMin := []float64{256.7, 296.0, 532.4}
	nonMatchingMax := []float64{259.3, 364.8, 534.3}

	doc := document.NewDocument()
	f, err := document.NewDoubleRangeDocValuesField("dv", nonMatchingMin, nonMatchingMax)
	rfdvAdd(t, doc, f, err)
	mustAddDocument(t, iw, doc)
	mustCommit(t, iw)

	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	lowRange := []float64{111.3, 294.4, 517.4}
	highRange := []float64{116.7, 319.4, 533.0}

	query := rfdvMust(t)(search.NewDoubleRangeSlowRangeQuery("dv", lowRange, highRange, document.RangeFieldQueryTypeIntersects))
	assertIntEquals(t, mustCount(t, searcher, query), iters)

	lowRange2 := []float64{116.3, 299.3, 517.0}
	highRange2 := []float64{121.0, 317.1, 531.2}

	query = rfdvMust(t)(search.NewDoubleRangeSlowRangeQuery("dv", lowRange2, highRange2, document.RangeFieldQueryTypeIntersects))

	assertIntEquals(t, mustCount(t, searcher, query), iters)

	mustClose(t, reader, dir)
}

func TestRangeFieldsDocValuesQueryIntRangeDocValuesIntersectsQuery(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	iters := atLeast(10)
	min := []int32{3, 11, 17}
	max := []int32{27, 35, 49}
	for i := 0; i < iters; i++ {
		doc := document.NewDocument()
		f, err := document.NewIntRangeDocValuesField("dv", min, max)
		rfdvAdd(t, doc, f, err)
		mustAddDocument(t, iw, doc)
	}

	min2 := []int32{11, 19, 27}
	max2 := []int32{29, 38, 56}

	doc := document.NewDocument()
	f, err := document.NewIntRangeDocValuesField("dv", min2, max2)
	rfdvAdd(t, doc, f, err)

	mustCommit(t, iw)

	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	lowRange := []int32{6, 16, 19}
	highRange := []int32{29, 41, 42}

	query := rfdvMust(t)(search.NewIntRangeSlowRangeQuery("dv", lowRange, highRange, document.RangeFieldQueryTypeIntersects))

	assertIntEquals(t, mustCount(t, searcher, query), iters)

	lowRange2 := []int32{2, 9, 18}
	highRange2 := []int32{25, 34, 41}

	query = rfdvMust(t)(search.NewIntRangeSlowRangeQuery("dv", lowRange2, highRange2, document.RangeFieldQueryTypeIntersects))

	assertIntEquals(t, mustCount(t, searcher, query), iters)

	lowRange3 := []int32{101, 121, 153}
	highRange3 := []int32{156, 127, 176}

	query = rfdvMust(t)(search.NewIntRangeSlowRangeQuery("dv", lowRange3, highRange3, document.RangeFieldQueryTypeIntersects))

	assertIntEquals(t, mustCount(t, searcher, query), 0)

	mustClose(t, reader, dir)
}

func TestRangeFieldsDocValuesQueryLongRangeDocValuesIntersectQuery(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	iters := atLeast(10)
	min := []int64{31, 15, 2}
	max := []int64{95, 27, 4}
	for i := 0; i < iters; i++ {
		doc := document.NewDocument()
		f, err := document.NewLongRangeDocValuesField("dv", min, max)
		rfdvAdd(t, doc, f, err)
		mustAddDocument(t, iw, doc)
	}

	min2 := []int64{101, 124, 137}
	max2 := []int64{138, 145, 156}
	doc := document.NewDocument()
	f, err := document.NewLongRangeDocValuesField("dv", min2, max2)
	rfdvAdd(t, doc, f, err)

	mustCommit(t, iw)

	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	lowRange := []int64{6, 12, 1}
	highRange := []int64{34, 24, 3}

	query := rfdvMust(t)(search.NewLongRangeSlowRangeQuery("dv", lowRange, highRange, document.RangeFieldQueryTypeIntersects))

	assertIntEquals(t, mustCount(t, searcher, query), iters)

	lowRange2 := []int64{32, 18, 3}
	highRange2 := []int64{96, 29, 5}

	query = rfdvMust(t)(search.NewLongRangeSlowRangeQuery("dv", lowRange2, highRange2, document.RangeFieldQueryTypeIntersects))

	assertIntEquals(t, mustCount(t, searcher, query), iters)

	mustClose(t, reader, dir)
}

func TestRangeFieldsDocValuesQueryFloatRangeDocValuesIntersectQuery(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	iters := atLeast(10)
	min := []float32{3.7, 11.0, 33.4}
	max := []float32{8.3, 21.6, 59.8}
	for i := 0; i < iters; i++ {
		doc := document.NewDocument()
		f, err := document.NewFloatRangeDocValuesField("dv", min, max)
		rfdvAdd(t, doc, f, err)
		mustAddDocument(t, iw, doc)
	}

	nonMatchingMin := []float32{11.4, 29.7, 102.4}
	nonMatchingMax := []float32{17.6, 37.2, 160.2}
	doc := document.NewDocument()
	f, err := document.NewFloatRangeDocValuesField("dv", nonMatchingMin, nonMatchingMax)
	rfdvAdd(t, doc, f, err)
	mustAddDocument(t, iw, doc)

	mustCommit(t, iw)

	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	lowRange := []float32{1.2, 8.3, 21.4}
	highRange := []float32{6.0, 17.6, 47.1}

	query := rfdvMust(t)(search.NewFloatRangeSlowRangeQuery("dv", lowRange, highRange, document.RangeFieldQueryTypeIntersects))

	assertIntEquals(t, mustCount(t, searcher, query), iters)

	lowRange2 := []float32{6.1, 17.0, 31.3}
	highRange2 := []float32{14.2, 23.4, 61.1}

	query = rfdvMust(t)(search.NewFloatRangeSlowRangeQuery("dv", lowRange2, highRange2, document.RangeFieldQueryTypeIntersects))

	assertIntEquals(t, mustCount(t, searcher, query), iters)

	mustClose(t, reader, dir)
}

func TestRangeFieldsDocValuesQueryToString(t *testing.T) {
	doubleMin := []float64{112.7, 296.0, float64(float32(512.4))}
	doubleMax := []float64{119.3, 314.8, float64(float32(524.3))}
	q1 := rfdvMust(t)(search.NewDoubleRangeSlowRangeQuery("foo", doubleMin, doubleMax, document.RangeFieldQueryTypeIntersects))
	rfdvAssertToString(t, "foo:[[112.7, 296.0, 512.4000244140625] TO [119.3, 314.8, 524.2999877929688]]", q1)

	intMin := []int32{3, 11, 17}
	intMax := []int32{27, 35, 49}
	q2 := rfdvMust(t)(search.NewIntRangeSlowRangeQuery("foo", intMin, intMax, document.RangeFieldQueryTypeIntersects))
	rfdvAssertToString(t, "foo:[[3, 11, 17] TO [27, 35, 49]]", q2)

	floatMin := []float32{3.7, 11.0, 33.4}
	floatMax := []float32{8.3, 21.6, 59.8}
	q3 := rfdvMust(t)(search.NewFloatRangeSlowRangeQuery("foo", floatMin, floatMax, document.RangeFieldQueryTypeIntersects))
	rfdvAssertToString(t, "foo:[[3.7, 11.0, 33.4] TO [8.3, 21.6, 59.8]]", q3)

	longMin := []int64{101, 124, 137}
	longMax := []int64{138, 145, 156}
	q4 := rfdvMust(t)(search.NewLongRangeSlowRangeQuery("foo", longMin, longMax, document.RangeFieldQueryTypeIntersects))
	rfdvAssertToString(t, "foo:[[101, 124, 137] TO [138, 145, 156]]", q4)
}

// rfdvAssertToString renders assertEquals(expected, q.toString()).
func rfdvAssertToString(t *testing.T, expected string, q search.Query) {
	t.Helper()
	if got := q.(interface{ String(string) string }).String(""); got != expected {
		t.Fatalf("toString = %q, want %q", got, expected)
	}
}

func TestRangeFieldsDocValuesQueryNoData(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	doc.Add(mustStringField(t, "foo", "abc", false))
	mustAddDocument(t, iw, doc)

	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	// test on field that doesn't exist
	q1 := rfdvMust(t)(search.NewLongRangeSlowRangeQuery("bar", []int64{20}, []int64{27}, document.RangeFieldQueryTypeIntersects))
	r := mustSearch(t, searcher, q1, 10)
	if r.TotalHits.Value != 0 {
		t.Fatalf("totalHits = %d, want 0", r.TotalHits.Value)
	}

	// test on field of wrong type
	q2 := rfdvMust(t)(search.NewLongRangeSlowRangeQuery("foo", []int64{20}, []int64{27}, document.RangeFieldQueryTypeIntersects))
	// expectThrows(IllegalStateException.class, () -> searcher.search(q2, 10));
	if _, err := searcher.Search(q2, 10); err == nil {
		t.Fatal("expected IllegalStateException")
	}

	mustClose(t, reader, dir)
}
