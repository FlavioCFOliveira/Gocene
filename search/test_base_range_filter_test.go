// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestBaseRangeFilter.java
// (Apache Lucene 10.5.0). The static @BeforeClass state is built by
// brfBeforeClass for every test that needs it; subclasses
// (TestMultiTermConstantScore) reuse it.

package search_test

import (
	"math"
	"math/rand"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

const (
	brfF = false
	brfT = true
)

// brfTestIndex renders the static class TestIndex: collation interacts badly
// with hyphens -- collation produces different ordering than Unicode
// code-point ordering -- so two indexes are created: one which can't have
// negative random integers, for testing collated ranges, and the other which
// can have negative random integers, for all other tests.
type brfTestIndex struct {
	maxR                    int32
	minR                    int32
	allowNegativeRandomInts bool
	index                   store.Directory
}

func newBrfTestIndex(minR, maxR int32, allowNegativeRandomInts bool) *brfTestIndex {
	return &brfTestIndex{minR: minR, maxR: maxR, allowNegativeRandomInts: allowNegativeRandomInts, index: newDirectory()}
}

// brfClass renders the static fields of TestBaseRangeFilter.
type brfClass struct {
	signedIndexReader   *index.DirectoryReader
	unsignedIndexReader *index.DirectoryReader
	signedIndexDir      *brfTestIndex
	unsignedIndexDir    *brfTestIndex
	maxID               int
}

// brfMinID renders `static final int minId = 0`.
const brfMinID = 0

// brfBeforeClass renders beforeClassBaseTestRangeFilter(); afterClass is
// registered with t.Cleanup.
func brfBeforeClass(t *testing.T) *brfClass {
	t.Helper()
	c := &brfClass{}
	c.maxID = atLeast(500)
	c.signedIndexDir = newBrfTestIndex(math.MaxInt32, math.MinInt32, true)
	c.unsignedIndexDir = newBrfTestIndex(math.MaxInt32, 0, false)
	c.signedIndexReader = brfBuild(t, random(), c.signedIndexDir, c.maxID)
	c.unsignedIndexReader = brfBuild(t, random(), c.unsignedIndexDir, c.maxID)
	t.Cleanup(func() {
		mustClose(t, c.signedIndexReader, c.unsignedIndexReader, c.signedIndexDir.index, c.unsignedIndexDir.index)
	})
	return c
}

// brfBuild renders the private static build(Random, TestIndex).
func brfBuild(t *testing.T, random *rand.Rand, idx *brfTestIndex, maxID int) *index.DirectoryReader {
	t.Helper()
	/* build an index */

	doc := document.NewDocument()
	idField := newStringField(t, "id", "", true)
	idDVField, err := document.NewSortedDocValuesField("id", []byte{})
	if err != nil {
		t.Fatal(err)
	}
	intIDField := document.NewIntPoint("id_int", 0)
	intDVField, _ := document.NewNumericDocValuesField("id_int", 0)
	floatIDField := document.NewFloatPoint("id_float", 0)
	floatDVField, _ := document.NewNumericDocValuesField("id_float", 0)
	longIDField := document.NewLongPoint("id_long", 0)
	longDVField, _ := document.NewNumericDocValuesField("id_long", 0)
	doubleIDField := document.NewDoublePoint("id_double", 0)
	doubleDVField, _ := document.NewNumericDocValuesField("id_double", 0)
	randField := newStringField(t, "rand", "", true)
	randDVField, _ := document.NewSortedDocValuesField("rand", []byte{})
	bodyField := newStringField(t, "body", "", false)
	bodyDVField, _ := document.NewSortedDocValuesField("body", []byte{})
	doc.Add(idField)
	doc.Add(idDVField)
	doc.Add(intIDField)
	doc.Add(intDVField)
	doc.Add(floatIDField)
	doc.Add(floatDVField)
	doc.Add(longIDField)
	doc.Add(longDVField)
	doc.Add(doubleIDField)
	doc.Add(doubleDVField)
	doc.Add(randField)
	doc.Add(randDVField)
	doc.Add(bodyField)
	doc.Add(bodyDVField)

	iwc := newIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzerRandom(random))
	iwc.SetOpenMode(index.Create)
	iwc.SetMaxBufferedDocs(nextInt(50, 1000))
	iwc.SetMergePolicy(newLogMergePolicy())
	writer := newRandomIndexWriterWithConfig(t, idx.index, iwc)
	// TestUtil.reduceOpenFiles(writer.w);
	brfReduceOpenFiles(t, writer.W)

	for {
		minCount := 0
		maxCount := 0

		for d := brfMinID; d <= maxID; d++ {
			idField.SetStringValue(padRangeFilter(int32(d)))
			idDVField.SetBytesValue([]byte(padRangeFilter(int32(d))))
			intIDField.SetIntValue(int32(d))
			intDVField.SetLongValue(int64(d))
			floatIDField.SetFloatValue(float32(d))
			floatDVField.SetLongValue(int64(math.Float32bits(float32(d))))
			longIDField.SetLongValue(int64(d))
			longDVField.SetLongValue(int64(d))
			doubleIDField.SetDoubleValue(float64(d))
			doubleDVField.SetLongValue(int64(math.Float64bits(float64(d))))
			var r int32
			if idx.allowNegativeRandomInts {
				r = int32(random.Uint32())
			} else {
				r = random.Int31n(math.MaxInt32)
			}
			if idx.maxR < r {
				idx.maxR = r
				maxCount = 1
			} else if idx.maxR == r {
				maxCount++
			}

			if r < idx.minR {
				idx.minR = r
				minCount = 1
			} else if r == idx.minR {
				minCount++
			}
			randField.SetStringValue(padRangeFilter(r))
			randDVField.SetBytesValue([]byte(padRangeFilter(r)))
			bodyField.SetStringValue("body")
			bodyDVField.SetBytesValue([]byte("body"))
			mustAddDocument(t, writer, doc)
		}

		if minCount == 1 && maxCount == 1 {
			// our subclasses rely on only 1 doc having the min or
			// max, so, we loop until we satisfy that.  it should be
			// exceedingly rare (Yonik calculates 1 in ~429,000)
			// times) that this loop requires more than one try:
			ir := mustGetReader(t, writer)
			mustClose(t, writer)
			return ir
		}

		// try again
		if _, err := writer.DeleteAll(); err != nil {
			t.Fatalf("deleteAll: %v", err)
		}
	}
}

// brfReduceOpenFiles renders TestUtil.reduceOpenFiles(IndexWriter): keep the
// number of open files lowish.
func brfReduceOpenFiles(t *testing.T, w *index.IndexWriter) {
	t.Helper()
	mp := w.GetConfig().GetMergePolicy()
	mp.(interface{ SetNoCFSRatio(float64) }).SetNoCFSRatio(1.0)
	switch p := mp.(type) {
	case interface {
		GetMergeFactor() int
		SetMergeFactor(int)
	}:
		p.SetMergeFactor(min(5, p.GetMergeFactor()))
	case *index.TieredMergePolicy:
		p.SetMaxMergeAtOnce(min(5, p.GetMaxMergeAtOnce()))
		p.SetSegmentsPerTier(min(5, p.GetSegmentsPerTier()))
	}
	if cms, ok := w.GetConfig().GetMergeScheduler().(*index.ConcurrentMergeScheduler); ok {
		// wtf... shouldn't it be even lower since it's 1 by default?!?!
		if err := cms.SetMaxMergesAndThreads(3, 2); err != nil {
			t.Fatal(err)
		}
	}
}

// intLength renders `static final int intLength = Integer.toString(Integer.MAX_VALUE).length()`.
const intLength = 10

// padRangeFilter renders pad(int): a simple padding function that
// maps any int32 to a fixed-width, sign-prefixed decimal string whose natural
// (lexicographic) order matches the signed integer order.
//
// For negative n, the magnitude is folded into the unsigned range via
// MAX_INT + n + 1 (matching Java's int arithmetic) and prefixed with '-' so
// that more-negative values produce lexicographically smaller strings.
func padRangeFilter(n int32) string {
	var b []byte
	p := "0"
	v := int64(n)
	if n < 0 {
		p = "-"
		// Java: n = Integer.MAX_VALUE + n + 1 (computed in 32-bit int space).
		v = int64(int32(int64(2147483647) + int64(n) + 1))
	}
	b = append(b, p...)
	s := strconv.FormatInt(v, 10)
	for i := len(s); i <= intLength; i++ {
		b = append(b, '0')
	}
	b = append(b, s...)
	return string(b)
}

func TestBaseRangeFilterPad(t *testing.T) {
	tests := []int32{-9999999, -99560, -100, -3, -1, 0, 3, 9, 10, 1000, 999999999}
	for i := 0; i < len(tests)-1; i++ {
		a := tests[i]
		b := tests[i+1]
		aa := padRangeFilter(a)
		bb := padRangeFilter(b)
		if len(aa) != len(bb) {
			t.Fatalf("length of %d:%s vs %d:%s differ: %d vs %d", a, aa, b, bb, len(aa), len(bb))
		}
		if !(aa < bb) {
			t.Fatalf("compare less than failed: %d:%s vs %d:%s", a, aa, b, bb)
		}
	}
}
