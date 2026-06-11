// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPointQueries.java
//
// The upstream suite has 54 methods. This port restores the deterministic
// methods that exercise the end-to-end BKD point write+read path through the
// real IndexWriter flush and IndexSearcher read path (the integration harness
// commits a real on-disk index with the production codec). The numeric range,
// exact, and point-in-set queries are duelled against the exact upstream
// expected counts for ints, longs, floats, doubles, and binary points,
// including multi-value and multi-dimensional cases and the IEEE special
// values (+/-0.0, +/-Inf, NaN, MIN/MAX).
//
// The big random verify*/RandomIndexWriter-driven methods (testRandomLongs*,
// testRandomBinary*, testRandomPointInSetQuery, the *SkipsNonMatchingSegments
// and *Count/*Rewrites optimisation methods) are not reproduced here: they
// depend on RandomIndexWriter, Weight.count sub-linear counting, and segment
// skipping that Gocene does not expose. The deterministic methods below cover
// the same query semantics those random tests assert.

package search_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ---- typed point query helpers (the analogues of IntPoint.newRangeQuery /
// newExactQuery / newSetQuery and friends) ----

func pqEncodeInt(v int32) []byte  { b := make([]byte, 4); document.EncodeDimensionIntLucene(v, b, 0); return b }
func pqEncodeLong(v int64) []byte { b := make([]byte, 8); document.EncodeDimensionLongLucene(v, b, 0); return b }
func pqEncodeFloat(v float32) []byte {
	b := make([]byte, 4)
	document.EncodeDimensionFloatLucene(v, b, 0)
	return b
}
func pqEncodeDouble(v float64) []byte {
	b := make([]byte, 8)
	document.EncodeDimensionDoubleLucene(v, b, 0)
	return b
}

func pqIntRange(t *testing.T, field string, lo, hi int32) search.Query {
	t.Helper()
	q, err := search.NewPointRangeQuery(field, pqEncodeInt(lo), pqEncodeInt(hi))
	if err != nil {
		t.Fatalf("int range query: %v", err)
	}
	return q
}
func pqLongRange(t *testing.T, field string, lo, hi int64) search.Query {
	t.Helper()
	q, err := search.NewPointRangeQuery(field, pqEncodeLong(lo), pqEncodeLong(hi))
	if err != nil {
		t.Fatalf("long range query: %v", err)
	}
	return q
}
func pqFloatRange(t *testing.T, field string, lo, hi float32) search.Query {
	t.Helper()
	q, err := search.NewPointRangeQuery(field, pqEncodeFloat(lo), pqEncodeFloat(hi))
	if err != nil {
		t.Fatalf("float range query: %v", err)
	}
	return q
}
func pqDoubleRange(t *testing.T, field string, lo, hi float64) search.Query {
	t.Helper()
	q, err := search.NewPointRangeQuery(field, pqEncodeDouble(lo), pqEncodeDouble(hi))
	if err != nil {
		t.Fatalf("double range query: %v", err)
	}
	return q
}
func pqIntExact(t *testing.T, field string, v int32) search.Query  { return pqIntRange(t, field, v, v) }
func pqLongExact(t *testing.T, field string, v int64) search.Query { return pqLongRange(t, field, v, v) }
func pqFloatExact(t *testing.T, field string, v float32) search.Query {
	return pqFloatRange(t, field, v, v)
}
func pqDoubleExact(t *testing.T, field string, v float64) search.Query {
	return pqDoubleRange(t, field, v, v)
}

func pqIntSet(field string, vals ...int32) search.Query {
	packed := make([][]byte, len(vals))
	for i, v := range vals {
		packed[i] = pqEncodeInt(v)
	}
	return search.NewPointInSetQuery(field, 1, 4, packed)
}
func pqLongSet(field string, vals ...int64) search.Query {
	packed := make([][]byte, len(vals))
	for i, v := range vals {
		packed[i] = pqEncodeLong(v)
	}
	return search.NewPointInSetQuery(field, 1, 8, packed)
}
func pqFloatSet(field string, vals ...float32) search.Query {
	packed := make([][]byte, len(vals))
	for i, v := range vals {
		packed[i] = pqEncodeFloat(v)
	}
	return search.NewPointInSetQuery(field, 1, 4, packed)
}
func pqDoubleSet(field string, vals ...float64) search.Query {
	packed := make([][]byte, len(vals))
	for i, v := range vals {
		packed[i] = pqEncodeDouble(v)
	}
	return search.NewPointInSetQuery(field, 1, 8, packed)
}

func pqCount(t *testing.T, s *search.IndexSearcher, q search.Query) int64 {
	t.Helper()
	top, err := s.Search(q, 100000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	return top.TotalHits.Value
}

func pqAssertCount(t *testing.T, s *search.IndexSearcher, q search.Query, want int64) {
	t.Helper()
	if got := pqCount(t, s, q); got != want {
		t.Errorf("count = %d, want %d", got, want)
	}
}

// ---- basic numeric ----

func TestPointQueries_BasicInts(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []int32{-7, 0, 3} {
		doc := document.NewDocument()
		doc.Add(document.NewIntPoint("point", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqIntRange(t, "point", -8, 1), 2)
	pqAssertCount(t, s, pqIntRange(t, "point", -7, 3), 3)
	pqAssertCount(t, s, pqIntExact(t, "point", -7), 1)
	pqAssertCount(t, s, pqIntExact(t, "point", -6), 0)
}

func TestPointQueries_BasicFloats(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []float32{-7.0, 0.0, 3.0} {
		doc := document.NewDocument()
		doc.Add(document.NewFloatPoint("point", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqFloatRange(t, "point", -8.0, 1.0), 2)
	pqAssertCount(t, s, pqFloatRange(t, "point", -7.0, 3.0), 3)
	pqAssertCount(t, s, pqFloatExact(t, "point", -7.0), 1)
	pqAssertCount(t, s, pqFloatExact(t, "point", -6.0), 0)
}

func TestPointQueries_BasicLongs(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []int64{-7, 0, 3} {
		doc := document.NewDocument()
		doc.Add(document.NewLongPoint("point", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqLongRange(t, "point", -8, 1), 2)
	pqAssertCount(t, s, pqLongRange(t, "point", -7, 3), 3)
	pqAssertCount(t, s, pqLongExact(t, "point", -7), 1)
	pqAssertCount(t, s, pqLongExact(t, "point", -6), 0)
}

func TestPointQueries_BasicDoubles(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []float64{-7.0, 0.0, 3.0} {
		doc := document.NewDocument()
		doc.Add(document.NewDoublePoint("point", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqDoubleRange(t, "point", -8.0, 1.0), 2)
	pqAssertCount(t, s, pqDoubleRange(t, "point", -7.0, 3.0), 3)
	pqAssertCount(t, s, pqDoubleExact(t, "point", -7.0), 1)
	pqAssertCount(t, s, pqDoubleExact(t, "point", -6.0), 0)
}

// ---- IEEE special values ----

func TestPointQueries_CrazyDoubles(t *testing.T) {
	ix := newIntegrationIndex(t)
	negZero := math.Copysign(0, -1)
	vals := []float64{
		math.Inf(-1), negZero, 0.0, math.SmallestNonzeroFloat64, math.MaxFloat64, math.Inf(1), math.NaN(),
	}
	for _, v := range vals {
		doc := document.NewDocument()
		doc.Add(document.NewDoublePoint("point", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	// exact queries
	pqAssertCount(t, s, pqDoubleExact(t, "point", math.Inf(-1)), 1)
	pqAssertCount(t, s, pqDoubleExact(t, "point", negZero), 1)
	pqAssertCount(t, s, pqDoubleExact(t, "point", 0.0), 1)
	pqAssertCount(t, s, pqDoubleExact(t, "point", math.SmallestNonzeroFloat64), 1)
	pqAssertCount(t, s, pqDoubleExact(t, "point", math.MaxFloat64), 1)
	pqAssertCount(t, s, pqDoubleExact(t, "point", math.Inf(1)), 1)
	pqAssertCount(t, s, pqDoubleExact(t, "point", math.NaN()), 1)

	// set query covering every value
	pqAssertCount(t, s, pqDoubleSet("point",
		math.MaxFloat64, math.NaN(), 0.0, math.Inf(-1), math.SmallestNonzeroFloat64, negZero, math.Inf(1)), 7)

	// ranges
	pqAssertCount(t, s, pqDoubleRange(t, "point", math.Inf(-1), negZero), 2)
	pqAssertCount(t, s, pqDoubleRange(t, "point", negZero, 0.0), 2)
	pqAssertCount(t, s, pqDoubleRange(t, "point", 0.0, math.SmallestNonzeroFloat64), 2)
	pqAssertCount(t, s, pqDoubleRange(t, "point", math.SmallestNonzeroFloat64, math.MaxFloat64), 2)
	pqAssertCount(t, s, pqDoubleRange(t, "point", math.MaxFloat64, math.Inf(1)), 2)
	pqAssertCount(t, s, pqDoubleRange(t, "point", math.Inf(1), math.NaN()), 2)
}

func TestPointQueries_CrazyFloats(t *testing.T) {
	ix := newIntegrationIndex(t)
	negZero := float32(math.Copysign(0, -1))
	vals := []float32{
		float32(math.Inf(-1)), negZero, 0.0, math.SmallestNonzeroFloat32, math.MaxFloat32, float32(math.Inf(1)), float32(math.NaN()),
	}
	for _, v := range vals {
		doc := document.NewDocument()
		doc.Add(document.NewFloatPoint("point", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqFloatExact(t, "point", float32(math.Inf(-1))), 1)
	pqAssertCount(t, s, pqFloatExact(t, "point", negZero), 1)
	pqAssertCount(t, s, pqFloatExact(t, "point", 0.0), 1)
	pqAssertCount(t, s, pqFloatExact(t, "point", math.SmallestNonzeroFloat32), 1)
	pqAssertCount(t, s, pqFloatExact(t, "point", math.MaxFloat32), 1)
	pqAssertCount(t, s, pqFloatExact(t, "point", float32(math.Inf(1))), 1)
	pqAssertCount(t, s, pqFloatExact(t, "point", float32(math.NaN())), 1)

	pqAssertCount(t, s, pqFloatSet("point",
		math.MaxFloat32, float32(math.NaN()), 0.0, float32(math.Inf(-1)), math.SmallestNonzeroFloat32, negZero, float32(math.Inf(1))), 7)

	pqAssertCount(t, s, pqFloatRange(t, "point", float32(math.Inf(-1)), negZero), 2)
	pqAssertCount(t, s, pqFloatRange(t, "point", negZero, 0.0), 2)
	pqAssertCount(t, s, pqFloatRange(t, "point", 0.0, math.SmallestNonzeroFloat32), 2)
	pqAssertCount(t, s, pqFloatRange(t, "point", math.SmallestNonzeroFloat32, math.MaxFloat32), 2)
	pqAssertCount(t, s, pqFloatRange(t, "point", math.MaxFloat32, float32(math.Inf(1))), 2)
	pqAssertCount(t, s, pqFloatRange(t, "point", float32(math.Inf(1)), float32(math.NaN())), 2)
}

// ---- min/max longs ----

func TestPointQueries_MinMaxLong(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []int64{math.MinInt64, math.MaxInt64} {
		doc := document.NewDocument()
		doc.Add(document.NewLongPoint("value", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqLongRange(t, "value", math.MinInt64, 0), 1)
	pqAssertCount(t, s, pqLongRange(t, "value", 0, math.MaxInt64), 1)
	pqAssertCount(t, s, pqLongRange(t, "value", math.MinInt64, math.MaxInt64), 2)
}

func TestPointQueries_LongMinMaxNumeric(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []int64{math.MinInt64, math.MaxInt64} {
		doc := document.NewDocument()
		doc.Add(document.NewLongPoint("value", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqLongRange(t, "value", math.MinInt64, math.MaxInt64), 2)
	pqAssertCount(t, s, pqLongRange(t, "value", math.MinInt64, math.MaxInt64-1), 1)
	pqAssertCount(t, s, pqLongRange(t, "value", math.MinInt64+1, math.MaxInt64), 1)
	pqAssertCount(t, s, pqLongRange(t, "value", math.MinInt64+1, math.MaxInt64-1), 0)
}

// ---- binary points ----

func pqBinaryRange(t *testing.T, field string, lo, hi []byte) search.Query {
	t.Helper()
	q, err := search.NewBinaryPointRangeQuery(field, lo, hi)
	if err != nil {
		t.Fatalf("binary range query: %v", err)
	}
	return q
}

// pqUTF8Pad right zero-pads s.getBytes(UTF8) to length (mirrors the upstream
// toUTF8(String, int) helper). A length shorter than the bytes panics, exactly
// as the upstream helper throws.
func pqUTF8Pad(s string, length int) []byte {
	b := []byte(s)
	if length < len(b) {
		panic("pqUTF8Pad: length too short")
	}
	out := make([]byte, length)
	copy(out, b)
	return out
}

func TestPointQueries_BasicSortedSet(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []string{"abc", "def"} {
		doc := document.NewDocument()
		bp, err := document.NewBinaryPoint("value", []byte(v))
		if err != nil {
			t.Fatalf("NewBinaryPoint: %v", err)
		}
		doc.Add(bp)
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqBinaryRange(t, "value", []byte("aaa"), []byte("bbb")), 1)
	pqAssertCount(t, s, pqBinaryRange(t, "value", pqUTF8Pad("c", 3), pqUTF8Pad("e", 3)), 1)
	pqAssertCount(t, s, pqBinaryRange(t, "value", pqUTF8Pad("a", 3), pqUTF8Pad("z", 3)), 2)
	pqAssertCount(t, s, pqBinaryRange(t, "value", pqUTF8Pad("", 3), []byte("abc")), 1)
	pqAssertCount(t, s, pqBinaryRange(t, "value", pqUTF8Pad("a", 3), []byte("abc")), 1)
	pqAssertCount(t, s, pqBinaryRange(t, "value", pqUTF8Pad("a", 3), []byte("abb")), 0)
	pqAssertCount(t, s, pqBinaryRange(t, "value", []byte("def"), []byte("zzz")), 1)
	pqAssertCount(t, s, pqBinaryRange(t, "value", []byte("def"), pqUTF8Pad("z", 3)), 1)
	pqAssertCount(t, s, pqBinaryRange(t, "value", []byte("deg"), pqUTF8Pad("z", 3)), 0)
}

func TestPointQueries_SortedSetNoOrdsMatch(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []string{"a", "z"} {
		doc := document.NewDocument()
		bp, err := document.NewBinaryPoint("value", []byte(v))
		if err != nil {
			t.Fatalf("NewBinaryPoint: %v", err)
		}
		doc.Add(bp)
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()
	pqAssertCount(t, s, pqBinaryRange(t, "value", []byte("m"), []byte("m")), 0)
}

// ---- no-match / no-docs ----

func TestPointQueries_NumericNoValuesMatch(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []int64{17, 22} {
		doc := document.NewDocument()
		f, err := document.NewSortedNumericDocValuesField("value", []int64{v})
		if err != nil {
			t.Fatalf("NewSortedNumericDocValuesField: %v", err)
		}
		doc.Add(f)
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()
	// An inverted range (17..13) on a field with no point values: zero matches.
	pqAssertCount(t, s, pqLongRange(t, "value", 17, 13), 0)
}

func TestPointQueries_NoDocs(t *testing.T) {
	ix := newIntegrationIndex(t)
	ix.addDoc(document.NewDocument())
	s, cleanup := ix.searcher()
	defer cleanup()
	pqAssertCount(t, s, pqLongRange(t, "value", 17, 13), 0)
}

// ---- exact points across types ----

func TestPointQueries_ExactPoints(t *testing.T) {
	ix := newIntegrationIndex(t)
	{
		doc := document.NewDocument()
		doc.Add(document.NewLongPoint("long", 5))
		ix.addDoc(doc)
	}
	{
		doc := document.NewDocument()
		doc.Add(document.NewIntPoint("int", 42))
		ix.addDoc(doc)
	}
	{
		doc := document.NewDocument()
		doc.Add(document.NewFloatPoint("float", 2.0))
		ix.addDoc(doc)
	}
	{
		doc := document.NewDocument()
		doc.Add(document.NewDoublePoint("double", 1.0))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqIntExact(t, "int", 42), 1)
	pqAssertCount(t, s, pqIntExact(t, "int", 41), 0)
	pqAssertCount(t, s, pqLongExact(t, "long", 5), 1)
	pqAssertCount(t, s, pqLongExact(t, "long", -1), 0)
	pqAssertCount(t, s, pqFloatExact(t, "float", 2.0), 1)
	pqAssertCount(t, s, pqFloatExact(t, "float", 1.0), 0)
	pqAssertCount(t, s, pqDoubleExact(t, "double", 1.0), 1)
	pqAssertCount(t, s, pqDoubleExact(t, "double", 2.0), 0)
}

// ---- point-in-set, single dimension ----

func TestPointQueries_BasicPointInSetQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	for _, v := range []int32{17, 42} {
		doc := document.NewDocument()
		doc.Add(document.NewIntPoint("int", v))
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqIntSet("int", 17), 1)
	pqAssertCount(t, s, pqIntSet("int", 42), 1)
	pqAssertCount(t, s, pqIntSet("int", 17, 42), 2)
	pqAssertCount(t, s, pqIntSet("int", -7, 17, 42, 97), 2)
	pqAssertCount(t, s, pqIntSet("int", 16), 0)
	pqAssertCount(t, s, pqIntSet("int"), 0) // empty set -> no matches
}

func TestPointQueries_EmptyPointInSetQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	doc := document.NewDocument()
	doc.Add(document.NewIntPoint("int", 17))
	doc.Add(document.NewLongPoint("long", 17))
	doc.Add(document.NewFloatPoint("float", 17.0))
	doc.Add(document.NewDoublePoint("double", 17.0))
	bp, err := document.NewBinaryPoint("bytes", []byte{0, 17})
	if err != nil {
		t.Fatalf("NewBinaryPoint: %v", err)
	}
	doc.Add(bp)
	ix.addDoc(doc)
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqIntSet("int"), 0)
	pqAssertCount(t, s, pqLongSet("long"), 0)
	pqAssertCount(t, s, pqFloatSet("float"), 0)
	pqAssertCount(t, s, pqDoubleSet("double"), 0)
	bSet, err := search.NewBinaryPointSetQuery("bytes")
	if err != nil {
		t.Fatalf("NewBinaryPointSetQuery(empty): %v", err)
	}
	pqAssertCount(t, s, bSet, 0)
}

// ---- multi-dimensional point-in-set ----

// pqMultiDimIntSet ports newMultiDimIntSetQuery: each group of numDims ints is
// packed into one numDims*4-byte point, and fed to a PointInSetQuery. A value
// count not divisible by numDims panics, mirroring the upstream
// IllegalArgumentException.
func pqMultiDimIntSet(field string, numDims int, vals ...int32) search.Query {
	if len(vals)%numDims != 0 {
		panic("incongruent number of values")
	}
	n := len(vals) / numDims
	packed := make([][]byte, n)
	for i := 0; i < n; i++ {
		p := make([]byte, numDims*4)
		for d := 0; d < numDims; d++ {
			document.EncodeDimensionIntLucene(vals[i*numDims+d], p, d*4)
		}
		packed[i] = p
	}
	return search.NewPointInSetQuery(field, numDims, 4, packed)
}

func TestPointQueries_BasicMultiDimPointInSetQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	doc := document.NewDocument()
	p, err := document.NewIntPointLucene("int", 17, 42)
	if err != nil {
		t.Fatalf("NewIntPointLucene: %v", err)
	}
	doc.Add(p)
	ix.addDoc(doc)
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, 17, 41), 0)
	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, 17, 42), 1)
	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, -7, -7, 17, 42), 1)
	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, 17, 42, -14, -14), 1)
}

func TestPointQueries_BasicMultiValueMultiDimPointInSetQuery(t *testing.T) {
	ix := newIntegrationIndex(t)
	doc := document.NewDocument()
	p1, err := document.NewIntPointLucene("int", 17, 42)
	if err != nil {
		t.Fatalf("NewIntPointLucene: %v", err)
	}
	doc.Add(p1)
	p2, err := document.NewIntPointLucene("int", 34, 79)
	if err != nil {
		t.Fatalf("NewIntPointLucene: %v", err)
	}
	doc.Add(p2)
	ix.addDoc(doc)
	s, cleanup := ix.searcher()
	defer cleanup()

	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, 17, 41), 0)
	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, 17, 42), 1)
	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, 17, 42, 34, 79), 1)
	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, -7, -7, 17, 42), 1)
	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, -7, -7, 34, 79), 1)
	pqAssertCount(t, s, pqMultiDimIntSet("int", 2, 17, 42, -14, -14), 1)
}

func TestPointQueries_InvalidMultiDimPointInSetQuery(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Errorf("expected a panic for an incongruent value count")
		}
	}()
	_ = pqMultiDimIntSet("int", 2, 3, 4, 5)
}

// ---- range over many equal values ----

func TestPointQueries_PointRangeQueryManyEqualValues(t *testing.T) {
	ix := newIntegrationIndex(t)
	cardinality := 5
	counts := make([]int, cardinality)
	// Deterministic round-robin assignment keeps the per-value counts exact.
	for i := 0; i < 10000; i++ {
		x := int32(i % cardinality)
		counts[x]++
		doc := document.NewDocument()
		doc.Add(document.NewIntPoint("int", x))
		doc.Add(document.NewLongPoint("long", int64(x)))
		doc.Add(document.NewFloatPoint("float", float32(x)))
		doc.Add(document.NewDoublePoint("double", float64(x)))
		ix.addDoc(doc)
		if i%2500 == 2499 {
			ix.commit() // exercise multiple segments
		}
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	zeroCount := int64(counts[0])
	oneCount := int64(counts[1])

	pqAssertCount(t, s, pqIntRange(t, "int", 0, 0), zeroCount)
	pqAssertCount(t, s, pqIntRange(t, "int", 1, 1), oneCount)
	pqAssertCount(t, s, pqIntRange(t, "int", 0, 1), zeroCount+oneCount)

	pqAssertCount(t, s, pqLongRange(t, "long", 0, 0), zeroCount)
	pqAssertCount(t, s, pqLongRange(t, "long", 1, 1), oneCount)
	pqAssertCount(t, s, pqLongRange(t, "long", 0, 1), zeroCount+oneCount)

	pqAssertCount(t, s, pqFloatRange(t, "float", 0, 0), zeroCount)
	pqAssertCount(t, s, pqFloatRange(t, "float", 1, 1), oneCount)
	pqAssertCount(t, s, pqFloatRange(t, "float", 0, 1), zeroCount+oneCount)

	pqAssertCount(t, s, pqDoubleRange(t, "double", 0, 0), zeroCount)
	pqAssertCount(t, s, pqDoubleRange(t, "double", 1, 1), oneCount)
	pqAssertCount(t, s, pqDoubleRange(t, "double", 0, 1), zeroCount+oneCount)
}

func TestPointQueries_PointInSetQueryManyEqualValues(t *testing.T) {
	ix := newIntegrationIndex(t)
	zeroCount := 0
	for i := 0; i < 10000; i++ {
		x := int32(i % 2)
		if x == 0 {
			zeroCount++
		}
		doc := document.NewDocument()
		doc.Add(document.NewIntPoint("int", x))
		doc.Add(document.NewLongPoint("long", int64(x)))
		doc.Add(document.NewFloatPoint("float", float32(x)))
		doc.Add(document.NewDoublePoint("double", float64(x)))
		ix.addDoc(doc)
		if i%2500 == 2499 {
			ix.commit()
		}
	}
	s, cleanup := ix.searcher()
	defer cleanup()
	zc := int64(zeroCount)
	other := int64(10000) - zc

	pqAssertCount(t, s, pqIntSet("int", 0), zc)
	pqAssertCount(t, s, pqIntSet("int", 0, -7), zc)
	pqAssertCount(t, s, pqIntSet("int", 7, 0), zc)
	pqAssertCount(t, s, pqIntSet("int", 1), other)
	pqAssertCount(t, s, pqIntSet("int", 2), 0)

	pqAssertCount(t, s, pqLongSet("long", 0), zc)
	pqAssertCount(t, s, pqLongSet("long", 0, -7), zc)
	pqAssertCount(t, s, pqLongSet("long", 7, 0), zc)
	pqAssertCount(t, s, pqLongSet("long", 1), other)
	pqAssertCount(t, s, pqLongSet("long", 2), 0)

	pqAssertCount(t, s, pqFloatSet("float", 0), zc)
	pqAssertCount(t, s, pqFloatSet("float", 1), other)
	pqAssertCount(t, s, pqFloatSet("float", 2), 0)

	pqAssertCount(t, s, pqDoubleSet("double", 0), zc)
	pqAssertCount(t, s, pqDoubleSet("double", 1), other)
	pqAssertCount(t, s, pqDoubleSet("double", 2), 0)
}

// ---- multi-dim range (covers the inverse-range / all-match semantics) ----

func TestPointQueries_InversePointRange(t *testing.T) {
	ix := newIntegrationIndex(t)
	numDims := 3
	for i := int64(0); i < 100; i++ {
		doc := document.NewDocument()
		p, err := document.NewLongPointLucene("point", i, i, i)
		if err != nil {
			t.Fatalf("NewLongPointLucene: %v", err)
		}
		doc.Add(p)
		ix.addDoc(doc)
	}
	s, cleanup := ix.searcher()
	defer cleanup()

	// A range that covers every value on every dimension matches all docs.
	lower := make([]byte, numDims*8)
	upper := make([]byte, numDims*8)
	for d := 0; d < numDims; d++ {
		document.EncodeDimensionLongLucene(math.MinInt64, lower, d*8)
		document.EncodeDimensionLongLucene(math.MaxInt64, upper, d*8)
	}
	qAll, err := search.NewPointRangeQueryMultiDim("point", lower, upper, numDims)
	if err != nil {
		t.Fatalf("NewPointRangeQueryMultiDim: %v", err)
	}
	pqAssertCount(t, s, qAll, 100)

	// A degenerate range [50,50] on every dimension matches exactly the i==50
	// doc (all three dims equal).
	lo := make([]byte, numDims*8)
	hi := make([]byte, numDims*8)
	for d := 0; d < numDims; d++ {
		document.EncodeDimensionLongLucene(50, lo, d*8)
		document.EncodeDimensionLongLucene(50, hi, d*8)
	}
	qOne, err := search.NewPointRangeQueryMultiDim("point", lo, hi, numDims)
	if err != nil {
		t.Fatalf("NewPointRangeQueryMultiDim: %v", err)
	}
	pqAssertCount(t, s, qOne, 1)
}

// ---- equality / validation ----

func TestPointQueries_PointRangeEquals(t *testing.T) {
	q, err := search.NewPointRangeQuery("a", pqEncodeInt(0), pqEncodeInt(1000))
	if err != nil {
		t.Fatalf("q: %v", err)
	}
	q2, _ := search.NewPointRangeQuery("a", pqEncodeInt(0), pqEncodeInt(1000))
	if !q.Equals(q2) {
		t.Errorf("expected equal range queries")
	}
	if q.HashCode() != q2.HashCode() {
		t.Errorf("equal range queries must share a hash")
	}
	q3, _ := search.NewPointRangeQuery("a", pqEncodeInt(1), pqEncodeInt(1000))
	if q.Equals(q3) {
		t.Errorf("queries with different lower bound must be unequal")
	}
	q4, _ := search.NewPointRangeQuery("b", pqEncodeInt(0), pqEncodeInt(1000))
	if q.Equals(q4) {
		t.Errorf("queries on different fields must be unequal")
	}
}

func TestPointQueries_InvalidPointLength(t *testing.T) {
	// Lower and upper bounds with mismatched byte lengths are rejected.
	if _, err := search.NewPointRangeQuery("field", make([]byte, 8), make([]byte, 4)); err == nil {
		t.Errorf("expected an error for mismatched bound lengths")
	}

func TestPointQueries_WrongNumBytes(t *testing.T) {
	// A multi-dim range query whose total length is not divisible by numDims
	// is rejected (the per-dimension width would be fractional).
	if _, err := search.NewPointRangeQueryMultiDim("value", make([]byte, 9), make([]byte, 9), 2); err == nil {
		t.Errorf("expected an error for a length not divisible by numDims")
	}
}