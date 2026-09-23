// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestDocValuesQueries.java
// (Apache Lucene 10.5.0).
//
// XDocValuesField.newSlowRangeQuery / newSlowSetQuery build the package-private
// org.apache.lucene.document.SortedNumericDocValuesRangeQuery,
// SortedNumericDocValuesSetQuery and SortedSetDocValuesRangeQuery; Gocene's
// executable renderings of those classes are search.NewSortedNumericDocValuesRangeQuery,
// search.NewSortedNumericDocValuesSetQuery and search.NewSortedSetDocValuesRangeQuery
// (the document package holds data-carrier duplicates).

package search_test

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	// alwaysDocValuesFormatBlocker names TestUtil.alwaysDocValuesFormat(DocValuesFormat).
	alwaysDocValuesFormatBlocker = "requires TestUtil.alwaysDocValuesFormat(DocValuesFormat) " +
		"(org.apache.lucene.tests.codecs.asserting.AssertingCodec) (not ported)"
	// sortedIndexedFieldBlocker names the missing indexedField factories.
	sortedIndexedFieldBlocker = "requires SortedDocValuesField.indexedField(String, BytesRef) and " +
		"SortedSetDocValuesField.indexedField(String, BytesRef) (not ported)"
	// docValuesRangeIteratorBlocker names the classes testSortedSetRangeQueryUsesTwoPhaseRangeIteratorWithSkipper inspects.
	docValuesRangeIteratorBlocker = "requires org.apache.lucene.search.DocValuesRangeIterator, " +
		"ConstantScoreScorerSupplier.iterator(long) and LeafMetaData.sort() (not ported)"
)

// dvqGetCodec renders the private getCodec(): small interval size to test with
// many intervals.
func dvqGetCodec(t *testing.T) index.Codec {
	t.Helper()
	// return TestUtil.alwaysDocValuesFormat(new Lucene90DocValuesFormat(random().nextInt(4, 16)));
	t.Fatal(alwaysDocValuesFormatBlocker)
	return nil
}

// dvqGetCodecWithInterval renders the private getCodec(int skipIntervalSize).
func dvqGetCodecWithInterval(t *testing.T, skipIntervalSize int) index.Codec {
	t.Helper()
	// return TestUtil.alwaysDocValuesFormat(new Lucene90DocValuesFormat(skipIntervalSize));
	t.Fatal(alwaysDocValuesFormatBlocker)
	return nil
}

// dvqLongPointRange renders LongPoint.newRangeQuery(String, long, long).
func dvqLongPointRange(t *testing.T, field string, lower, upper int64) search.Query {
	t.Helper()
	lo := make([]byte, 8)
	hi := make([]byte, 8)
	document.EncodeDimension(lower, lo, 0)
	document.EncodeDimension(upper, hi, 0)
	q, err := search.NewPointRangeQuery(field, lo, hi)
	if err != nil {
		t.Fatalf("LongPoint.newRangeQuery: %v", err)
	}
	return q
}

// dvqSNRange renders XDocValuesField.newSlowRangeQuery(String, long, long).
func dvqSNRange(t *testing.T, field string, lower, upper int64) search.Query {
	t.Helper()
	q, err := search.NewSortedNumericDocValuesRangeQuery(field, lower, upper)
	if err != nil {
		t.Fatalf("newSlowRangeQuery: %v", err)
	}
	return q
}

// dvqSSRange renders SortedSetDocValuesField/SortedDocValuesField.newSlowRangeQuery.
func dvqSSRange(t *testing.T, field string, lower, upper *util.BytesRef, lowerInclusive, upperInclusive bool) search.Query {
	t.Helper()
	q, err := search.NewSortedSetDocValuesRangeQuery(field, lower, upper, lowerInclusive, upperInclusive)
	if err != nil {
		t.Fatalf("newSlowRangeQuery: %v", err)
	}
	return q
}

// dvqSNSet renders XDocValuesField.newSlowSetQuery(String, long...).
func dvqSNSet(t *testing.T, field string, values ...int64) search.Query {
	t.Helper()
	q, err := search.NewSortedNumericDocValuesSetQuery(field, append([]int64(nil), values...))
	if err != nil {
		t.Fatalf("newSlowSetQuery: %v", err)
	}
	return q
}

func dvqBytes(b []byte) *util.BytesRef { return util.NewBytesRef(b) }

func dvqEncode(v int64) []byte {
	encoded := make([]byte, 8)
	document.EncodeDimension(v, encoded, 0)
	return encoded
}

func dvqNextLong(lo, hi int64) int64 { return lo + random().Int63n(hi-lo+1) }

func dvqNumericField(t *testing.T, name string, value int64, indexed bool) document.IndexableField {
	t.Helper()
	var f document.IndexableField
	var err error
	if indexed {
		f, err = document.NewNumericDocValuesFieldIndexed(name, value)
	} else {
		f, err = document.NewNumericDocValuesField(name, value)
	}
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func dvqSortedNumericField(t *testing.T, name string, value int64, indexed bool) document.IndexableField {
	t.Helper()
	var f document.IndexableField
	var err error
	if indexed {
		f, err = document.NewSortedNumericDocValuesFieldIndexed(name, []int64{value})
	} else {
		f, err = document.NewSortedNumericDocValuesField(name, []int64{value})
	}
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestDocValuesQueriesDuelPointRangeSortedNumericRangeQuery(t *testing.T) {
	dvqDoTestDuelPointRangeNumericRangeQuery(t, true, 1, false)
}

func TestDocValuesQueriesDuelPointRangeSortedNumericRangeWithSlipperQuery(t *testing.T) {
	dvqDoTestDuelPointRangeNumericRangeQuery(t, true, 1, true)
}

func TestDocValuesQueriesDuelPointRangeMultivaluedSortedNumericRangeQuery(t *testing.T) {
	dvqDoTestDuelPointRangeNumericRangeQuery(t, true, 3, false)
}

func TestDocValuesQueriesDuelPointRangeMultivaluedSortedNumericRangeWithSkipperQuery(t *testing.T) {
	dvqDoTestDuelPointRangeNumericRangeQuery(t, true, 3, true)
}

func TestDocValuesQueriesDuelPointRangeNumericRangeQuery(t *testing.T) {
	dvqDoTestDuelPointRangeNumericRangeQuery(t, false, 1, false)
}

func TestDocValuesQueriesDuelPointRangeNumericRangeWithSkipperQuery(t *testing.T) {
	dvqDoTestDuelPointRangeNumericRangeQuery(t, false, 1, true)
}

func TestDocValuesQueriesDuelPointNumericSortedWithSkipperRangeQuery(t *testing.T) {
	dir := newDirectory()
	config := index.NewIndexWriterConfig()
	config.SetCodec(dvqGetCodec(t))
	config.SetIndexSort(index.NewSort(index.NewSortFieldFull("dv", index.SortTypeLong, random().Intn(2) == 0)))
	iw := dvqNewRandomIndexWriter(t, dir, config)
	numDocs := atLeast(1000)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		value := dvqNextLong(-100, 10000)
		doc.Add(dvqNumericField(t, "dv", value, true))
		doc.Add(document.NewLongPoint("idx", value))
		mustAddDocument(t, iw, doc)
	}

	reader := mustGetReader(t, iw)
	searcher := newSearcherMaybeWrap(t, reader, false)
	mustClose(t, iw)

	for i := 0; i < 100; i++ {
		min := int64(math.MinInt64)
		if random().Intn(2) != 0 {
			min = dvqNextLong(-100, 10000)
		}
		max := int64(math.MaxInt64)
		if random().Intn(2) != 0 {
			max = dvqNextLong(-100, 10000)
		}
		q1 := dvqLongPointRange(t, "idx", min, max)
		q2 := dvqSNRange(t, "dv", min, max)
		dvqAssertSameMatches(t, searcher, q1, q2, false)
	}
	mustClose(t, reader, dir)
}

// dvqNewRandomIndexWriter renders new RandomIndexWriter(random(), dir, config).
func dvqNewRandomIndexWriter(t *testing.T, dir store.Directory, config *index.IndexWriterConfig) *testindex.RandomIndexWriter {
	t.Helper()
	return newRandomIndexWriterWithConfig(t, dir, config)
}

// dvqDoTestDuelPointRangeNumericRangeQuery renders the private
// doTestDuelPointRangeNumericRangeQuery(boolean, int, boolean).
func dvqDoTestDuelPointRangeNumericRangeQuery(t *testing.T, sortedNumeric bool, maxValuesPerDoc int, skypper bool) {
	t.Helper()
	iters := atLeast(10)
	for iter := 0; iter < iters; iter++ {
		dir := newDirectory()
		var iw *testindex.RandomIndexWriter
		if sortedNumeric || random().Intn(2) == 0 {
			iw = newRandomIndexWriter(t, dir)
		} else {
			config := index.NewIndexWriterConfig()
			config.SetCodec(dvqGetCodec(t))
			config.SetIndexSort(index.NewSort(index.NewSortFieldFull("dv", index.SortTypeLong, random().Intn(2) == 0)))
			iw = newRandomIndexWriterWithConfig(t, dir, config)
		}
		numDocs := atLeast(100)
		for i := 0; i < numDocs; i++ {
			doc := document.NewDocument()
			numValues := nextInt(0, maxValuesPerDoc)
			for j := 0; j < numValues; j++ {
				value := dvqNextLong(-100, 10000)
				if sortedNumeric {
					doc.Add(dvqSortedNumericField(t, "dv", value, skypper))
				} else {
					doc.Add(dvqNumericField(t, "dv", value, skypper))
				}
				doc.Add(document.NewLongPoint("idx", value))
			}
			mustAddDocument(t, iw, doc)
		}
		if random().Intn(2) == 0 {
			if _, err := iw.DeleteDocumentsQuery(dvqLongPointRange(t, "idx", 0, 10)); err != nil {
				t.Fatal(err)
			}
		}
		reader := mustGetReader(t, iw)
		searcher := newSearcherMaybeWrap(t, reader, false)
		mustClose(t, iw)

		for i := 0; i < 100; i++ {
			min := int64(math.MinInt64)
			if random().Intn(2) != 0 {
				min = dvqNextLong(-100, 10000)
			}
			max := int64(math.MaxInt64)
			if random().Intn(2) != 0 {
				max = dvqNextLong(-100, 10000)
			}
			q1 := dvqLongPointRange(t, "idx", min, max)
			q2 := dvqSNRange(t, "dv", min, max) // both branches build SortedNumericDocValuesRangeQuery
			dvqAssertSameMatches(t, searcher, q1, q2, false)
		}

		mustClose(t, reader, dir)
	}
}

// dvqDoTestDuelPointRangeSortedRangeQuery renders the private
// doTestDuelPointRangeSortedRangeQuery(boolean, int, boolean).
func dvqDoTestDuelPointRangeSortedRangeQuery(t *testing.T, sortedSet bool, maxValuesPerDoc int, skypper bool) {
	t.Helper()
	iters := atLeast(10)
	for iter := 0; iter < iters; iter++ {
		dir := newDirectory()
		var iw *testindex.RandomIndexWriter
		if sortedSet || random().Intn(2) == 0 {
			iw = newRandomIndexWriter(t, dir)
		} else {
			config := index.NewIndexWriterConfig()
			config.SetCodec(dvqGetCodec(t))
			config.SetIndexSort(index.NewSort(index.NewSortFieldFull("dv", index.SortTypeString, random().Intn(2) == 0)))
			iw = newRandomIndexWriterWithConfig(t, dir, config)
		}
		numDocs := atLeast(100)
		for i := 0; i < numDocs; i++ {
			doc := document.NewDocument()
			numValues := nextInt(0, maxValuesPerDoc)
			for j := 0; j < numValues; j++ {
				value := dvqNextLong(-100, 10000)
				encoded := dvqEncode(value)
				if skypper {
					// SortedSetDocValuesField.indexedField / SortedDocValuesField.indexedField
					t.Fatal(sortedIndexedFieldBlocker)
				}
				if sortedSet {
					f, err := document.NewSortedSetDocValuesField("dv", [][]byte{encoded})
					if err != nil {
						t.Fatal(err)
					}
					doc.Add(f)
				} else {
					f, err := document.NewSortedDocValuesField("dv", encoded)
					if err != nil {
						t.Fatal(err)
					}
					doc.Add(f)
				}
				doc.Add(document.NewLongPoint("idx", value))
			}
			mustAddDocument(t, iw, doc)
		}
		if random().Intn(2) == 0 {
			if _, err := iw.DeleteDocumentsQuery(dvqLongPointRange(t, "idx", 0, 10)); err != nil {
				t.Fatal(err)
			}
		}
		reader := mustGetReader(t, iw)
		searcher := newSearcherMaybeWrap(t, reader, false)
		mustClose(t, iw)

		for i := 0; i < 100; i++ {
			min := int64(math.MinInt64)
			if random().Intn(2) != 0 {
				min = dvqNextLong(-100, 10000)
			}
			max := int64(math.MaxInt64)
			if random().Intn(2) != 0 {
				max = dvqNextLong(-100, 10000)
			}
			encodedMin := dvqEncode(min)
			encodedMax := dvqEncode(max)
			includeMin := true
			includeMax := true
			if random().Intn(2) == 0 {
				includeMin = false
				min++
			}
			if random().Intn(2) == 0 {
				includeMax = false
				max--
			}
			q1 := dvqLongPointRange(t, "idx", min, max)
			var lower, upper *util.BytesRef
			if !(min == math.MinInt64 && random().Intn(2) == 0) {
				lower = dvqBytes(encodedMin)
			}
			if !(max == math.MaxInt64 && random().Intn(2) == 0) {
				upper = dvqBytes(encodedMax)
			}
			q2 := dvqSSRange(t, "dv", lower, upper, includeMin, includeMax)
			dvqAssertSameMatches(t, searcher, q1, q2, false)
		}

		mustClose(t, reader, dir)
	}
}

func TestDocValuesQueriesDuelPointRangeSortedSetRangeQuery(t *testing.T) {
	dvqDoTestDuelPointRangeSortedRangeQuery(t, true, 1, false)
}

func TestDocValuesQueriesDuelPointRangeSortedSetRangeSkipperQuery(t *testing.T) {
	dvqDoTestDuelPointRangeSortedRangeQuery(t, true, 1, true)
}

func TestDocValuesQueriesDuelPointRangeMultivaluedSortedSetRangeQuery(t *testing.T) {
	dvqDoTestDuelPointRangeSortedRangeQuery(t, true, 3, false)
}

func TestDocValuesQueriesDuelPointRangeMultivaluedSortedSetRangeSkipperQuery(t *testing.T) {
	dvqDoTestDuelPointRangeSortedRangeQuery(t, true, 3, true)
}

func TestDocValuesQueriesDuelPointRangeSortedRangeQuery(t *testing.T) {
	dvqDoTestDuelPointRangeSortedRangeQuery(t, false, 1, false)
}

func TestDocValuesQueriesSortedSetRangeQueryUsesTwoPhaseRangeIteratorWithSkipper(t *testing.T) {
	dvqAssertSortedSetRangeQueryUsesTwoPhaseRangeIteratorWithSkipper(t, false)
	dvqAssertSortedSetRangeQueryUsesTwoPhaseRangeIteratorWithSkipper(t, true)
}

// dvqAssertSortedSetRangeQueryUsesTwoPhaseRangeIteratorWithSkipper renders the
// private assertSortedSetRangeQueryUsesTwoPhaseRangeIteratorWithSkipper(boolean).
func dvqAssertSortedSetRangeQueryUsesTwoPhaseRangeIteratorWithSkipper(t *testing.T, multiValued bool) {
	t.Helper()
	dir := newDirectory()
	defer mustClose(t, dir)
	config := index.NewIndexWriterConfig()
	config.SetCodec(dvqGetCodecWithInterval(t, 8))
	iw := mustNewIndexWriter(t, dir, config)
	defer mustClose(t, iw)
	for i := 0; i < 512; i++ {
		doc := document.NewDocument()
		dvqAddSortedSetValue(t, doc, int64(i%100))
		if multiValued {
			dvqAddSortedSetValue(t, doc, int64(100+(i%100)))
		}
		mustAddDocument(t, iw, doc)
	}
	if err := iw.ForceMerge(1); err != nil {
		t.Fatal(err)
	}

	reader := mustOpenDirectoryReaderFromWriter(t, iw)
	defer mustClose(t, reader)
	searcher := newSearcherMaybeWrap(t, reader, false)
	query := dvqSSRange(t, "dv", dvqBytes(dvqEncode(20)), dvqBytes(dvqEncode(40)), true, true)

	leaf := mustLeaves(t, reader)[0]
	if rewritten := mustRewrite(t, searcher, query); rewritten != query {
		t.Fatal("assertSame(query, searcher.rewrite(query))")
	}
	// assertNull(leaf.reader().getMetaData().sort());
	// assertNotNull(leaf.reader().getDocValuesSkipper("dv"));
	// assertEquals(multiValued == false, DocValues.unwrapSingleton(DocValues.getSortedSet(leaf.reader(), "dv")) != null);
	// Weight weight = query.createWeight(searcher, ScoreMode.COMPLETE_NO_SCORES, 1f);
	// ScorerSupplier scorerSupplier = weight.scorerSupplier(leaf);
	// assertNotNull(scorerSupplier);
	// assertTrue(scorerSupplier instanceof ConstantScoreScorerSupplier);
	// DocIdSetIterator iterator = ((ConstantScoreScorerSupplier) scorerSupplier).iterator(Long.MAX_VALUE);
	// The skipper route exposes a two-phase DocValuesRangeIterator whose intoBitSet
	// bulk-evaluates skip blocks, rather than a per-doc plain iterator.
	// TwoPhaseIterator twoPhase = TwoPhaseIterator.unwrap(iterator);
	// assertNotNull(twoPhase); assertTrue(twoPhase instanceof DocValuesRangeIterator);
	_ = leaf
	t.Fatal(docValuesRangeIteratorBlocker)
}

// dvqAddSortedSetValue renders the private addSortedSetValue(Document, long).
func dvqAddSortedSetValue(t *testing.T, doc *document.Document, value int64) {
	t.Helper()
	_ = dvqEncode(value)
	// doc.add(SortedSetDocValuesField.indexedField("dv", newBytesRef(encoded)));
	t.Fatal(sortedIndexedFieldBlocker)
}

func TestDocValuesQueriesDuelPointRangeSortedRangeSkipperQuery(t *testing.T) {
	dvqDoTestDuelPointRangeSortedRangeQuery(t, false, 1, true)
}

func TestDocValuesQueriesDuelPointSortedSetSortedWithSkipperRangeQuery(t *testing.T) {
	dir := newDirectory()
	config := index.NewIndexWriterConfig()
	config.SetCodec(dvqGetCodec(t))
	config.SetIndexSort(index.NewSort(index.NewSortFieldFull("dv", index.SortTypeString, random().Intn(2) == 0)))
	iw := newRandomIndexWriterWithConfig(t, dir, config)
	numDocs := atLeast(1000)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		value := dvqNextLong(-100, 10000)
		_ = dvqEncode(value)
		// doc.add(SortedDocValuesField.indexedField("dv", newBytesRef(encoded)));
		t.Fatal(sortedIndexedFieldBlocker)
		doc.Add(document.NewLongPoint("idx", value))
		mustAddDocument(t, iw, doc)
	}

	reader := mustGetReader(t, iw)
	searcher := newSearcherMaybeWrap(t, reader, false)
	mustClose(t, iw)

	for i := 0; i < 100; i++ {
		min := int64(math.MinInt64)
		if random().Intn(2) != 0 {
			min = dvqNextLong(-100, 10000)
		}
		max := int64(math.MaxInt64)
		if random().Intn(2) != 0 {
			max = dvqNextLong(-100, 10000)
		}
		encodedMin := dvqEncode(min)
		encodedMax := dvqEncode(max)
		includeMin := true
		includeMax := true
		if random().Intn(2) == 0 {
			includeMin = false
			min++
		}
		if random().Intn(2) == 0 {
			includeMax = false
			max--
		}
		q1 := dvqLongPointRange(t, "idx", min, max)
		var lower, upper *util.BytesRef
		if !(min == math.MinInt64 && random().Intn(2) == 0) {
			lower = dvqBytes(encodedMin)
		}
		if !(max == math.MaxInt64 && random().Intn(2) == 0) {
			upper = dvqBytes(encodedMax)
		}
		q2 := dvqSSRange(t, "dv", lower, upper, includeMin, includeMax)
		dvqAssertSameMatches(t, searcher, q1, q2, false)
	}
	mustClose(t, reader, dir)
}

// dvqAssertSameMatches renders the private assertSameMatches(IndexSearcher, Query, Query, boolean).
func dvqAssertSameMatches(t *testing.T, searcher *search.IndexSearcher, q1, q2 search.Query, scores bool) {
	t.Helper()
	maxDoc := searcher.GetIndexReader().MaxDoc()
	sort := search.INDEXORDER
	if scores {
		sort = search.RELEVANCE
	}
	td1, err := searcher.SearchWithSort(q1, maxDoc, sort, false)
	if err != nil {
		t.Fatal(err)
	}
	td2, err := searcher.SearchWithSort(q2, maxDoc, sort, false)
	if err != nil {
		t.Fatal(err)
	}
	if td1.TotalHits.Value != td2.TotalHits.Value {
		t.Fatalf("totalHits: %d vs %d", td1.TotalHits.Value, td2.TotalHits.Value)
	}
	for i := 0; i < len(td1.ScoreDocs); i++ {
		assertIntEquals(t, td1.ScoreDocs[i].Doc, td2.ScoreDocs[i].Doc)
		if scores && math.Abs(float64(td1.ScoreDocs[i].Score-td2.ScoreDocs[i].Score)) > 10e-7 {
			t.Fatalf("score %d: %v vs %v", i, td1.ScoreDocs[i].Score, td2.ScoreDocs[i].Score)
		}
	}
}

func TestDocValuesQueriesEquals(t *testing.T) {
	q1 := dvqSNRange(t, "foo", 3, 5)
	queryUtilsCheckEqual(t, q1, dvqSNRange(t, "foo", 3, 5))
	queryUtilsCheckUnequal(t, q1, dvqSNRange(t, "foo", 3, 6))
	queryUtilsCheckUnequal(t, q1, dvqSNRange(t, "foo", 4, 5))
	queryUtilsCheckUnequal(t, q1, dvqSNRange(t, "bar", 3, 5))

	b := func(s string) *util.BytesRef { return dvqBytes([]byte(s)) }
	q2 := dvqSSRange(t, "foo", b("bar"), b("baz"), true, true)
	queryUtilsCheckEqual(t, q2, dvqSSRange(t, "foo", b("bar"), b("baz"), true, true))
	queryUtilsCheckUnequal(t, q2, dvqSSRange(t, "foo", b("baz"), b("baz"), true, true))
	queryUtilsCheckUnequal(t, q2, dvqSSRange(t, "foo", b("bar"), b("bar"), true, true))
	queryUtilsCheckUnequal(t, q2, dvqSSRange(t, "quux", b("bar"), b("baz"), true, true))
}

// dvqToString renders Query.toString(String field); "" renders toString().
func dvqToString(q search.Query, field string) string {
	return q.(interface{ ToString(string) string }).ToString(field)
}

func dvqAssertToString(t *testing.T, want, got string) {
	t.Helper()
	if want != got {
		t.Fatalf("toString = %q, want %q", got, want)
	}
}

func TestDocValuesQueriesToString(t *testing.T) {
	q1 := dvqSNRange(t, "foo", 3, 5)
	dvqAssertToString(t, "foo:[3 TO 5]", dvqToString(q1, ""))
	dvqAssertToString(t, "[3 TO 5]", dvqToString(q1, "foo"))
	dvqAssertToString(t, "foo:[3 TO 5]", dvqToString(q1, "bar"))

	b := func(s string) *util.BytesRef { return dvqBytes([]byte(s)) }
	q2 := dvqSSRange(t, "foo", b("bar"), b("baz"), true, true)
	dvqAssertToString(t, "foo:[[62 61 72] TO [62 61 7a]]", dvqToString(q2, ""))
	q2 = dvqSSRange(t, "foo", b("bar"), b("baz"), false, true)
	dvqAssertToString(t, "foo:{[62 61 72] TO [62 61 7a]]", dvqToString(q2, ""))
	q2 = dvqSSRange(t, "foo", b("bar"), b("baz"), false, false)
	dvqAssertToString(t, "foo:{[62 61 72] TO [62 61 7a]}", dvqToString(q2, ""))
	q2 = dvqSSRange(t, "foo", b("bar"), nil, true, true)
	dvqAssertToString(t, "foo:[[62 61 72] TO *}", dvqToString(q2, ""))
	q2 = dvqSSRange(t, "foo", nil, b("baz"), true, true)
	dvqAssertToString(t, "foo:{* TO [62 61 7a]]", dvqToString(q2, ""))
	dvqAssertToString(t, "{* TO [62 61 7a]]", dvqToString(q2, "foo"))
	dvqAssertToString(t, "foo:{* TO [62 61 7a]]", dvqToString(q2, "bar"))
}

func TestDocValuesQueriesMissingField(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	mustAddDocument(t, iw, document.NewDocument())
	reader := mustGetReader(t, iw)
	mustClose(t, iw)
	searcher := newSearcher(t, reader)
	b := func(s string) *util.BytesRef { return dvqBytes([]byte(s)) }
	for _, query := range []search.Query{
		dvqSNRange(t, "foo", 2, 4),
		dvqSNRange(t, "foo", 2, 4),
		dvqSSRange(t, "foo", b("abc"), b("bcd"), random().Intn(2) == 0, random().Intn(2) == 0),
		dvqSSRange(t, "foo", b("abc"), b("bcd"), random().Intn(2) == 0, random().Intn(2) == 0),
	} {
		w := mustCreateWeight(t, searcher, mustRewrite(t, searcher, query), search.COMPLETE, 1)
		s, err := w.Scorer(mustLeaves(t, searcher.GetIndexReader())[0])
		if err != nil {
			t.Fatal(err)
		}
		if s != nil {
			t.Fatalf("assertNull(w.scorer(...)) for %v", query)
		}
	}
	mustClose(t, reader, dir)
}

func TestDocValuesQueriesSlowRangeQueryRewrite(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	reader := mustGetReader(t, iw)
	mustClose(t, iw)
	searcher := newSearcher(t, reader)

	r1, err := dvqSNRange(t, "foo", 10, 1).Rewrite(searcher)
	if err != nil {
		t.Fatal(err)
	}
	queryUtilsCheckEqual(t, r1, search.NewMatchNoDocsQuery(""))
	r2, err := dvqSNRange(t, "foo", math.MinInt64, math.MaxInt64).Rewrite(searcher)
	if err != nil {
		t.Fatal(err)
	}
	queryUtilsCheckEqual(t, r2, search.NewFieldExistsQuery("foo"))
	mustClose(t, reader, dir)
}

func TestDocValuesQueriesSortedNumericNPE(t *testing.T) {
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)
	nums := []float64{
		-1.7147449030215377e-208,
		-1.6887024655302576e-11,
		1.534911516604164e113,
		0.0,
		2.6947996404505155e-166,
		-2.649722021970773e306,
		6.138239235731689e-198,
		2.3967090122610808e111,
	}
	for i := 0; i < len(nums); i++ {
		doc := document.NewDocument()
		doc.Add(dvqSortedNumericField(t, "dv", util.DoubleToSortableLong(nums[i]), false))
		mustAddDocument(t, iw, doc)
	}
	mustCommit(t, iw)
	reader := mustGetReader(t, iw)
	searcher := newSearcher(t, reader)
	mustClose(t, iw)

	lo := util.DoubleToSortableLong(8.701032080293731e-226)
	hi := util.DoubleToSortableLong(2.0801416404385346e-41)

	query := dvqSNRange(t, "dv", lo, hi)
	// TODO: assert expected matches
	if _, err := searcher.SearchWithSort(query, reader.MaxDoc(), search.INDEXORDER, false); err != nil {
		t.Fatal(err)
	}

	// swap order, should still work
	query = dvqSNRange(t, "dv", hi, lo)
	// TODO: assert expected matches
	if _, err := searcher.SearchWithSort(query, reader.MaxDoc(), search.INDEXORDER, false); err != nil {
		t.Fatal(err)
	}

	mustClose(t, reader, dir)
}

func TestDocValuesQueriesSetEquals(t *testing.T) {
	if !dvqSNSet(t, "field", 17, 42).Equals(dvqSNSet(t, "field", 17, 42)) {
		t.Fatal("assertEquals(newSlowSetQuery(field, 17, 42), ...)")
	}
	if !dvqSNSet(t, "field", 17, 42, 32416190071).Equals(dvqSNSet(t, "field", 17, 32416190071, 42)) {
		t.Fatal("assertEquals(newSlowSetQuery(field, 17, 42, 32416190071), ...)")
	}
	if dvqSNSet(t, "field", 42).Equals(dvqSNSet(t, "field2", 42)) {
		t.Fatal("assertFalse(field vs field2)")
	}
	if dvqSNSet(t, "field", 17, 42).Equals(dvqSNSet(t, "field", 17, 32416190071)) {
		t.Fatal("assertFalse(17,42 vs 17,32416190071)")
	}
}

func TestDocValuesQueriesDuelSetVsTermsQuery(t *testing.T) {
	iters := atLeast(2)
	for iter := 0; iter < iters; iter++ {
		var allNumbers []int64
		numNumbers := nextInt(1, 1<<nextInt(1, 10))
		for i := 0; i < numNumbers; i++ {
			allNumbers = append(allNumbers, int64(random().Uint64()))
		}
		dir := newDirectory()
		iw := newRandomIndexWriter(t, dir)
		numDocs := atLeast(100)
		for i := 0; i < numDocs; i++ {
			doc := document.NewDocument()
			number := allNumbers[random().Intn(len(allNumbers))]
			doc.Add(mustStringField(t, "text", strconv.FormatInt(number, 10), false))
			doc.Add(dvqNumericField(t, "long", number, false))
			doc.Add(dvqSortedNumericField(t, "twolongs", number, false))
			doc.Add(dvqSortedNumericField(t, "twolongs", number*2, false))
			mustAddDocument(t, iw, doc)
		}
		if numNumbers > 1 && random().Intn(2) == 0 {
			if _, err := iw.DeleteDocumentsQuery(search.NewTermQuery(index.NewTerm("text", strconv.FormatInt(allNumbers[0], 10)))); err != nil {
				t.Fatal(err)
			}
		}
		mustCommit(t, iw)
		reader := mustGetReader(t, iw)
		searcher := newSearcher(t, reader)
		mustClose(t, iw)

		if reader.NumDocs() == 0 {
			// may occasionally happen if all documents got the same term
			mustClose(t, reader, dir)
			continue
		}

		for i := 0; i < 100; i++ {
			boost := random().Float32() * 10
			numQueryNumbers := nextInt(1, 1<<nextInt(1, 8))
			queryNumbers := map[int64]struct{}{}
			queryNumbersX2 := map[int64]struct{}{}
			var queryNumbersArray, queryNumbersX2Array []int64
			for j := 0; j < numQueryNumbers; j++ {
				number := allNumbers[random().Intn(len(allNumbers))]
				if _, ok := queryNumbers[number]; !ok {
					queryNumbers[number] = struct{}{}
					queryNumbersArray = append(queryNumbersArray, number)
				}
				if _, ok := queryNumbersX2[2*number]; !ok {
					queryNumbersX2[2*number] = struct{}{}
					queryNumbersX2Array = append(queryNumbersX2Array, 2*number)
				}
			}
			bq := search.NewBooleanQueryBuilder()
			for _, number := range queryNumbersArray {
				bq.Add(search.NewTermQuery(index.NewTerm("text", strconv.FormatInt(number, 10))), search.SHOULD)
			}
			q1 := search.NewBoostQuery(search.NewConstantScoreQuery(bq.Build()), boost)

			q2 := search.NewBoostQuery(dvqSNSet(t, "long", queryNumbersArray...), boost)
			dvqAssertSameMatches(t, searcher, q1, q2, true)

			q3 := search.NewBoostQuery(dvqSNSet(t, "twolongs", queryNumbersArray...), boost)
			dvqAssertSameMatches(t, searcher, q1, q3, true)

			q4 := search.NewBoostQuery(dvqSNSet(t, "twolongs", queryNumbersX2Array...), boost)
			dvqAssertSameMatches(t, searcher, q1, q4, true)
		}

		mustClose(t, reader, dir)
	}
}

func TestDocValuesQueriesSortedNumericDocValuesRangeQueryCount(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	iw := newRandomIndexWriter(t, dir)
	defer mustClose(t, iw)
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.Add(dvqSortedNumericField(t, "with_index", int64(100+i), true))
		doc.Add(dvqSortedNumericField(t, "without_index", int64(100+i), false))
		if i != 55 {
			doc.Add(dvqSortedNumericField(t, "sparse", int64(100+i), true))
		}
		mustAddDocument(t, iw, doc)
	}
	mustCommit(t, iw)
	if err := iw.ForceMerge(1); err != nil {
		t.Fatal(err)
	}

	func() {
		reader := mustGetReader(t, iw)
		defer mustClose(t, reader)
		searcher := search.NewIndexSearcher(reader)

		dvqAssertCount(t, searcher, dvqSNRange(t, "with_index", 0, 50), 0)
		dvqAssertCount(t, searcher, dvqSNRange(t, "without_index", 0, 50), -1)
		dvqAssertCount(t, searcher, dvqSNRange(t, "sparse", 0, 50), 0)

		dvqAssertCount(t, searcher, dvqSNRange(t, "with_index", 50, 250), 100)
		dvqAssertCount(t, searcher, dvqSNRange(t, "without_index", 50, 250), -1)
		dvqAssertCount(t, searcher, dvqSNRange(t, "sparse", 50, 250), -1)

		dvqAssertCount(t, searcher, dvqSNRange(t, "with_index", 150, 250), -1)
		dvqAssertCount(t, searcher, dvqSNRange(t, "without_index", 150, 250), -1)
		dvqAssertCount(t, searcher, dvqSNRange(t, "sparse", 150, 250), -1)

		dvqAssertCount(t, searcher, dvqSNRange(t, "with_index", 250, 350), 0)
		dvqAssertCount(t, searcher, dvqSNRange(t, "without_index", 250, 350), -1)
		dvqAssertCount(t, searcher, dvqSNRange(t, "sparse", 250, 350), 0)
	}()

	if _, err := iw.DeleteDocumentsQuery(dvqSNRange(t, "with_index", 102, 103)); err != nil {
		t.Fatal(err)
	}
	mustCommit(t, iw)

	reader := mustGetReader(t, iw)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)

	dvqAssertCount(t, searcher, dvqSNRange(t, "with_index", 0, 50), 0)
	dvqAssertCount(t, searcher, dvqSNRange(t, "with_index", 50, 250), 98)
	dvqAssertCount(t, searcher, dvqSNRange(t, "with_index", 150, 250), -1)
	dvqAssertCount(t, searcher, dvqSNRange(t, "with_index", 250, 350), 0)
}

// dvqAssertCount renders the private assertCount(IndexSearcher, Query, int).
func dvqAssertCount(t *testing.T, searcher *search.IndexSearcher, query search.Query, expectedCount int) {
	t.Helper()
	w := mustCreateWeight(t, searcher, query, search.COMPLETE, 1.0)
	got, err := w.Count(mustLeaves(t, searcher.GetIndexReader())[0])
	if err != nil {
		t.Fatal(err)
	}
	if got != expectedCount {
		t.Fatalf("count(%v) = %d, want %d", query, got, expectedCount)
	}
}

func TestDocValuesQueriesSortedSetDocValuesRangeQueryCount(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	iw := newRandomIndexWriter(t, dir)
	defer mustClose(t, iw)
	for i := 0; i < 100; i++ {
		val := fmt.Sprintf("%03d", i)
		doc := document.NewDocument()
		// doc.add(SortedSetDocValuesField.indexedField("with_index", newBytesRef(val)));
		t.Fatal(sortedIndexedFieldBlocker)
		f, err := document.NewSortedSetDocValuesField("without_index", [][]byte{[]byte(val)})
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f)
		if i != 55 {
			// doc.add(SortedSetDocValuesField.indexedField("sparse", newBytesRef(val)));
			t.Fatal(sortedIndexedFieldBlocker)
		}
		mustAddDocument(t, iw, doc)
	}
	mustCommit(t, iw)
	if err := iw.ForceMerge(1); err != nil {
		t.Fatal(err)
	}

	b := func(s string) *util.BytesRef { return dvqBytes([]byte(s)) }
	func() {
		reader := mustGetReader(t, iw)
		defer mustClose(t, reader)
		searcher := search.NewIndexSearcher(reader)

		// Nonexistent field
		dvqAssertCount(t, searcher, dvqSSRange(t, "nonexistent", b("000"), b("099"), true, true), 0)

		// Below all values: range is entirely before "000"
		dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("!"), b("/"), true, true), 0)
		dvqAssertCount(t, searcher, dvqSSRange(t, "without_index", b("!"), b("/"), true, true), 0)
		dvqAssertCount(t, searcher, dvqSSRange(t, "sparse", b("!"), b("/"), true, true), 0)

		// Match all values: "000" through "099"
		dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("000"), b("099"), true, true), 100)
		dvqAssertCount(t, searcher, dvqSSRange(t, "without_index", b("000"), b("099"), true, true), -1)
		dvqAssertCount(t, searcher, dvqSSRange(t, "sparse", b("000"), b("099"), true, true), -1)

		// Partial match: "050" through "060" = 11 values
		dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("050"), b("060"), true, true), -1)
		dvqAssertCount(t, searcher, dvqSSRange(t, "without_index", b("050"), b("060"), true, true), -1)
		dvqAssertCount(t, searcher, dvqSSRange(t, "sparse", b("050"), b("060"), true, true), -1)

		// Partial match with bounds not in the index: "0501" falls between "050" and "051"
		dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("0501"), b("100"), true, true), -1)
		dvqAssertCount(t, searcher, dvqSSRange(t, "without_index", b("0501"), b("100"), true, true), -1)
		dvqAssertCount(t, searcher, dvqSSRange(t, "sparse", b("0501"), b("100"), true, true), -1)

		// Non-indexed bounds that cover all values: "//" < "000" and "1000" > "099"
		dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("//"), b("1000"), true, true), 100)
		dvqAssertCount(t, searcher, dvqSSRange(t, "without_index", b("//"), b("1000"), true, true), -1)
		dvqAssertCount(t, searcher, dvqSSRange(t, "sparse", b("//"), b("1000"), true, true), -1)

		// Non-indexed bounds that match nothing: "0991" through "0999" are above all values
		dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("0991"), b("0999"), true, true), 0)
		dvqAssertCount(t, searcher, dvqSSRange(t, "without_index", b("0991"), b("0999"), true, true), 0)
		dvqAssertCount(t, searcher, dvqSSRange(t, "sparse", b("0991"), b("0999"), true, true), 0)

		// Above all values: range is entirely after "099"
		dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("100"), b("199"), true, true), 0)
		dvqAssertCount(t, searcher, dvqSSRange(t, "without_index", b("100"), b("199"), true, true), 0)
		dvqAssertCount(t, searcher, dvqSSRange(t, "sparse", b("100"), b("199"), true, true), 0)
	}()

	// Delete docs with values "020" through "030" (11 docs)
	if _, err := iw.DeleteDocumentsQuery(dvqSSRange(t, "with_index", b("020"), b("030"), true, true)); err != nil {
		t.Fatal(err)
	}
	mustCommit(t, iw)

	reader := mustGetReader(t, iw)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)

	// Below all values still matches nothing
	dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("!"), b("/"), true, true), 0)
	// All values minus 11 deleted = 89
	dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("000"), b("099"), true, true), 89)
	// Partial match still can't be counted cheaply
	dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("050"), b("060"), true, true), -1)
	// Above all values still matches nothing
	dvqAssertCount(t, searcher, dvqSSRange(t, "with_index", b("100"), b("199"), true, true), 0)
}

// testSortedNumericDocValuesRangeQueryRewrites(Random) takes a parameter, so
// JUnit never runs it as a test method; it is ported as a plain method.
func dvqSortedNumericDocValuesRangeQueryRewrites(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	iw := newRandomIndexWriter(t, dir)
	defer mustClose(t, iw)
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		doc.Add(dvqSortedNumericField(t, "with_index", int64(100+i), true))
		doc.Add(dvqSortedNumericField(t, "without_index", int64(100+i), false))
		if i%17 == 0 {
			mustCommit(t, iw)
		}
		if i != 55 {
			doc.Add(dvqSortedNumericField(t, "sparse", int64(100+i), true))
		}
		if i == 74 {
			doc.Add(dvqSortedNumericField(t, "super_sparse", 174, true))
		}
		mustAddDocument(t, iw, doc)
	}
	mustCommit(t, iw)

	reader := mustGetReader(t, iw)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "with_index", 0, 50), "MatchNoDocsQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "with_index", 0, 250), "MatchAllDocsQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "sparse", 0, 50), "MatchNoDocsQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "super_sparse", 0, 50), "MatchNoDocsQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "super_sparse", 250, 350), "MatchNoDocsQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "super_sparse", 174, 174), "SortedNumericDocValuesRangeQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "with_index", 0, 150), "SortedNumericDocValuesRangeQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "with_index", 150, 250), "SortedNumericDocValuesRangeQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "with_index", 120, 150), "SortedNumericDocValuesRangeQuery")
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "sparse", 0, 250), "SortedNumericDocValuesRangeQuery")
}

var _ = dvqSortedNumericDocValuesRangeQueryRewrites

// dvqAssertRewrittenType renders assertThat(searcher.rewrite(q), instanceOf(...))
// and assertThat(rewrite(q).getClass().toString(), containsString(...)).
func dvqAssertRewrittenType(t *testing.T, searcher *search.IndexSearcher, q search.Query, className string) {
	t.Helper()
	rewritten := mustRewrite(t, searcher, q)
	if got := fmt.Sprintf("%T", rewritten); !strings.Contains(strings.ToLower(got), strings.ToLower(className)) {
		t.Fatalf("rewrite = %s, want %s", got, className)
	}
}

func TestDocValuesQueriesRewriteWorksWithPointsButNoSkipIndex(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	iw := newRandomIndexWriter(t, dir)
	defer mustClose(t, iw)
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		f, err := document.NewLongField("field", int64(100+i), false)
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f)
		mustAddDocument(t, iw, doc)
	}
	mustCommit(t, iw)
	reader := mustGetReader(t, iw)
	defer mustClose(t, reader)
	searcher := search.NewIndexSearcher(reader)
	// Query range [0, 50] is entirely below field range [100, 199]
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "field", 0, 50), "MatchNoDocsQuery")

	// Query range [0, 250] covers entire field range [100, 199]
	// and all docs have a value
	dvqAssertRewrittenType(t, searcher, dvqSNRange(t, "field", 0, 250), "MatchAllDocsQuery")
}

func TestDocValuesQueriesPrimarySortDenseSortedDocValuesExactMatch(t *testing.T) {
	dvqDoTestPrimarySortDenseExactMatch(t, index.SortTypeString,
		func(i int) document.IndexableField {
			// i -> SortedDocValuesField.indexedField("dv", newBytesRef(i + ""))
			t.Fatal(sortedIndexedFieldBlocker)
			return nil
		},
		func(i int) search.Query {
			v := dvqBytes([]byte(strconv.Itoa(i)))
			return dvqSSRange(t, "dv", v, v, true, true)
		})
}

func TestDocValuesQueriesPrimarySortDenseNumericDocValuesExactMatch(t *testing.T) {
	dvqDoTestPrimarySortDenseExactMatch(t, index.SortTypeLong,
		func(i int) document.IndexableField { return dvqNumericField(t, "dv", int64(i), true) },
		func(i int) search.Query { return dvqSNRange(t, "dv", int64(i), int64(i)) })
}

// dvqDoTestPrimarySortDenseExactMatch renders doTestPrimarySortDenseExactMatch.
func dvqDoTestPrimarySortDenseExactMatch(t *testing.T, sortType index.SortType, fields func(int) document.IndexableField, queries func(int) search.Query) {
	t.Helper()
	deletes := random().Intn(2) == 0
	dir := newDirectory()
	skipIntervalSize := 4 + random().Intn(4096-4)
	config := index.NewIndexWriterConfig()
	config.SetCodec(dvqGetCodecWithInterval(t, skipIntervalSize))
	config.SetIndexSort(index.NewSort(index.NewSortFieldFull("dv", sortType, random().Intn(2) == 0)))
	numBlocks := 4 + random().Intn(12)
	sizes := make([]int, numBlocks)
	for i := 0; i < numBlocks; i++ {
		sizes[i] = 1 + random().Intn(249)
	}
	totalSizes := make([]int, numBlocks)
	iw := newRandomIndexWriterWithConfig(t, dir, config)
	for i := 0; i < numBlocks; i++ {
		totalSizes[i] = sizes[i]
		for j := 0; j < sizes[i]; j++ {
			doc := document.NewDocument()
			dv := fields(i)
			doc.Add(dv)
			mustAddDocument(t, iw, doc)
			if deletes && random().Intn(10) == 0 {
				doc = document.NewDocument()
				doc.Add(dv)
				doc.Add(mustStringField(t, "id", "to_delete", false))
				mustAddDocument(t, iw, doc)
				totalSizes[i]++
			}
		}
	}
	mustCommit(t, iw)
	if err := iw.ForceMerge(1); err != nil {
		t.Fatal(err)
	}

	if deletes {
		if _, err := iw.DeleteDocumentsQuery(search.NewTermQuery(index.NewTerm("id", "to_delete"))); err != nil {
			t.Fatal(err)
		}
	}

	reader := mustGetReader(t, iw)
	searcher := newSearcherMaybeWrap(t, reader, false)
	mustClose(t, iw)

	for i := 0; i < numBlocks; i++ {
		q := queries(i)
		assertIntEquals(t, sizes[i], mustCount(t, searcher, q))
		if got := mustSearch(t, searcher, q, 1000).TotalHits.Value; got != int64(sizes[i]) {
			t.Fatalf("totalHits = %d, want %d", got, sizes[i])
		}
		// check cost
		leaves := mustLeaves(t, reader)
		assertIntEquals(t, 1, len(leaves))
		ctx := leaves[0]
		rewritten := mustRewrite(t, searcher, q)
		weight, err := rewritten.CreateWeight(searcher, search.COMPLETE_NO_SCORES, 1.0)
		if err != nil {
			t.Fatal(err)
		}
		supplier, err := weight.ScorerSupplier(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !(supplier.Cost() >= int64(sizes[i])) {
			t.Fatalf("cost %d < %d", supplier.Cost(), sizes[i])
		}
		if !(supplier.Cost() <= int64(totalSizes[i])+2*int64(skipIntervalSize)) {
			t.Fatalf("cost %d > %d", supplier.Cost(), int64(totalSizes[i])+2*int64(skipIntervalSize))
		}
	}
	mustClose(t, reader, dir)
}
