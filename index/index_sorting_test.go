// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexSorting.java
// (Apache Lucene 10.5.0).
//
// new Sort(...) is rendered with index.NewSort, the org.apache.lucene.search.Sort
// port that IndexWriterConfig#setIndexSort consumes (search.Sort is a distinct
// type the index writer ignores).

package index_test

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Missing members the Java tests reach.
const (
	assertingNeedsIndexSortCodecMissing = "TestIndexSorting.AssertingNeedsIndexSortCodec cannot be ported: its PointsFormat " +
		"returns a PointsWriter overriding merge(MergeState), which is not part of spi.PointsWriter"
	indexWriterSourceMissing = "org.apache.lucene.index.IndexWriter#SOURCE, #SOURCE_FLUSH and #SOURCE_MERGE are not ported " +
		"(IndexWriter.setDiagnostics records no diagnostics)"
	normsSimilarityMissing = "TestIndexSorting.NormsSimilarity cannot be ported whole: Similarity#scorer(float, " +
		"CollectionStatistics, TermStatistics...) is not part of index.Similarity"
)

// sortingConfig renders new IndexWriterConfig(new MockAnalyzer(random()))
// followed by setIndexSort(indexSort).
func sortingConfig(indexSort *index.Sort) *index.IndexWriterConfig {
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetIndexSort(indexSort)
	return iwc
}

// sortedNumericDVField renders new SortedNumericDocValuesField(name, value).
func sortedNumericDVField(t testing.TB, name string, value int64) *document.SortedNumericDocValuesField {
	t.Helper()
	f, err := document.NewSortedNumericDocValuesField(name, []int64{value})
	if err != nil {
		t.Fatalf("new SortedNumericDocValuesField: %v", err)
	}
	return f
}

func doubleDVField(t testing.TB, name string, value float64) *document.DoubleDocValuesField {
	t.Helper()
	f, err := document.NewDoubleDocValuesField(name, value)
	if err != nil {
		t.Fatalf("new DoubleDocValuesField: %v", err)
	}
	return f
}

func floatDVField(t testing.TB, name string, value float32) *document.FloatDocValuesField {
	t.Helper()
	f, err := document.NewFloatDocValuesField(name, value)
	if err != nil {
		t.Fatalf("new FloatDocValuesField: %v", err)
	}
	return f
}

// sortingThreeDocs renders the shared body of the basic/missing sort tests:
// three documents in three segments (two commits so forceMerge actually
// merges, since only merging produces a sorted segment), a forceMerge(1),
// an NRT reader and its only leaf, which must hold three documents. The
// returned closer closes the reader, the writer and the directory.
func sortingThreeDocs(t *testing.T, indexSort *index.Sort, docs [3]*document.Document) (index.LeafReader, func()) {
	t.Helper()
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, sortingConfig(indexSort))
	mustAddDocument(t, w, docs[0])
	// so we get more than one segment, so that forceMerge actually does merge, since we only get a
	// sorted segment by merging:
	mustCommit(t, w)

	mustAddDocument(t, w, docs[1])
	mustCommit(t, w)

	mustAddDocument(t, w, docs[2])
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	r := openReaderFromWriter(t, w)
	leaf := getOnlyLeafReader(t, r)
	if leaf.MaxDoc() != 3 {
		t.Fatalf("leaf.maxDoc(): expected 3, got %d", leaf.MaxDoc())
	}
	return leaf, func() { mustClose(t, r, w, dir) }
}

func leafSorted(t testing.TB, leaf index.LeafReader, field string) index.SortedDocValues {
	t.Helper()
	values, err := leaf.GetSortedDocValues(field)
	if err != nil {
		t.Fatalf("getSortedDocValues(%s): %v", field, err)
	}
	if values == nil {
		t.Fatalf("getSortedDocValues(%s) is null", field)
	}
	return values
}

// assertNextSortedTerm renders assertEquals(doc, values.nextDoc()) and
// assertEquals(text, values.lookupOrd(values.ordValue()).utf8ToString()).
func assertNextSortedTerm(t testing.TB, values index.SortedDocValues, doc int, text string) {
	t.Helper()
	if got, err := values.NextDoc(); err != nil || got != doc {
		t.Fatalf("nextDoc: expected %d, got %d (%v)", doc, got, err)
	}
	ord, err := values.OrdValue()
	if err != nil {
		t.Fatalf("ordValue: %v", err)
	}
	term, err := values.LookupOrd(ord)
	if err != nil {
		t.Fatalf("lookupOrd: %v", err)
	}
	if string(term) != text {
		t.Fatalf("doc=%d: expected %q, got %q", doc, text, term)
	}
}

// assertNextDouble renders assertEquals(doc, values.nextDoc()) and
// assertEquals(value, Double.longBitsToDouble(values.longValue()), 0.0).
func assertNextDouble(t testing.TB, values index.NumericDocValues, doc int, value float64) {
	t.Helper()
	if got, err := values.NextDoc(); err != nil || got != doc {
		t.Fatalf("nextDoc: expected %d, got %d (%v)", doc, got, err)
	}
	v, err := values.LongValue()
	if err != nil {
		t.Fatalf("longValue: %v", err)
	}
	if got := math.Float64frombits(uint64(v)); got != value {
		t.Fatalf("doc=%d: expected %v, got %v", doc, value, got)
	}
}

// assertNextFloat renders assertEquals(doc, values.nextDoc()) and
// assertEquals(value, Float.intBitsToFloat((int) values.longValue()), 0.0f).
func assertNextFloat(t testing.TB, values index.NumericDocValues, doc int, value float32) {
	t.Helper()
	if got, err := values.NextDoc(); err != nil || got != doc {
		t.Fatalf("nextDoc: expected %d, got %d (%v)", doc, got, err)
	}
	v, err := values.LongValue()
	if err != nil {
		t.Fatalf("longValue: %v", err)
	}
	if got := math.Float32frombits(uint32(int32(v))); got != value {
		t.Fatalf("doc=%d: expected %v, got %v", doc, value, got)
	}
}

func assertNoMoreDocValues(t testing.TB, values interface{ NextDoc() (int, error) }) {
	t.Helper()
	if got, err := values.NextDoc(); err != nil || got != spi.NO_MORE_DOCS {
		t.Fatalf("nextDoc: expected NO_MORE_DOCS, got %d (%v)", got, err)
	}
}

// assertIDOrder renders the id NumericDocValues checks of the multi-valued
// tests: docs 0, 1, 2 carry the given ids.
func assertIDOrder(t testing.TB, leaf index.LeafReader, ids ...int64) {
	t.Helper()
	values := leafNumeric(t, leaf, "id")
	for doc, id := range ids {
		assertNextNumeric(t, values, doc, id)
	}
}

// sortedNumericSortField renders new SortedNumericSortField(field, type,
// reverse, SortedNumericSelector.Type.MIN, missingValue); a nil missingValue
// renders the constructors that leave it unset.
func sortedNumericSortField(field string, sortType index.SortType, reverse bool, missingValue any) *index.SortField {
	sf := index.NewSortedNumericSortField(field, sortType)
	sf.Reverse = reverse
	if missingValue != nil {
		sf.SetMissingValue(missingValue)
	}
	return &sf.SortField
}

// sortedSetSortField renders new SortedSetSortField(field, reverse,
// SortedSetSelector.Type.MIN, missingValue).
func sortedSetSortField(field string, reverse bool, missingValue any) *index.SortField {
	sf := index.NewSortedSetSortField(field, reverse)
	if missingValue != nil {
		sf.SetMissingValue(missingValue)
	}
	return &sf.SortField
}

// sortFieldWithMissing renders new SortField(field, type, reverse, missingValue).
func sortFieldWithMissing(field string, sortType index.SortType, reverse bool, missingValue any) *index.SortField {
	sf := index.NewSortFieldFull(field, sortType, reverse)
	sf.SetMissingValue(missingValue)
	return sf
}

func TestIndexSortingNumericAlreadySorted(t *testing.T) {
	t.Fatal(assertingNeedsIndexSortCodecMissing)
}

func TestIndexSortingStringAlreadySorted(t *testing.T) {
	t.Fatal(assertingNeedsIndexSortCodecMissing)
}

func TestIndexSortingMultiValuedNumericAlreadySorted(t *testing.T) {
	t.Fatal(assertingNeedsIndexSortCodecMissing)
}

func TestIndexSortingMultiValuedStringAlreadySorted(t *testing.T) {
	t.Fatal(assertingNeedsIndexSortCodecMissing)
}

func TestIndexSortingBasicString(t *testing.T) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(index.NewSortField("foo", index.SortTypeString)), [3]*document.Document{
		docOf(sortedDVField(t, "foo", newBytesRef("zzz"))),
		docOf(sortedDVField(t, "foo", newBytesRef("aaa"))),
		docOf(sortedDVField(t, "foo", newBytesRef("mmm"))),
	})
	defer closeAll()
	values := leafSorted(t, leaf, "foo")
	assertNextSortedTerm(t, values, 0, "aaa")
	assertNextSortedTerm(t, values, 1, "mmm")
	assertNextSortedTerm(t, values, 2, "zzz")
}

func TestIndexSortingBasicMultiValuedString(t *testing.T) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortedSetSortField("foo", false, nil)), [3]*document.Document{
		docOf(numericDVField(t, "id", 3), sortedSetDVField(t, "foo", newBytesRef("zzz"))),
		docOf(numericDVField(t, "id", 1), sortedSetDVField(t, "foo", newBytesRef("aaa")),
			sortedSetDVField(t, "foo", newBytesRef("zzz")), sortedSetDVField(t, "foo", newBytesRef("bcg"))),
		docOf(numericDVField(t, "id", 2), sortedSetDVField(t, "foo", newBytesRef("mmm")),
			sortedSetDVField(t, "foo", newBytesRef("pppp"))),
	})
	defer closeAll()
	assertIDOrder(t, leaf, 1, 2, 3)
}

func TestIndexSortingMissingStringFirst(t *testing.T) {
	for _, reverse := range []bool{true, false} {
		sortField := sortFieldWithMissing("foo", index.SortTypeString, reverse, spi.STRING_FIRST)
		leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortField), [3]*document.Document{
			docOf(sortedDVField(t, "foo", newBytesRef("zzz"))),
			// missing
			document.NewDocument(),
			docOf(sortedDVField(t, "foo", newBytesRef("mmm"))),
		})
		values := leafSorted(t, leaf, "foo")
		if reverse {
			assertNextSortedTerm(t, values, 0, "zzz")
			assertNextSortedTerm(t, values, 1, "mmm")
		} else {
			// docID 0 is missing:
			assertNextSortedTerm(t, values, 1, "mmm")
			assertNextSortedTerm(t, values, 2, "zzz")
		}
		closeAll()
	}
}

func TestIndexSortingMissingMultiValuedStringFirst(t *testing.T) {
	for _, reverse := range []bool{true, false} {
		sortField := sortedSetSortField("foo", reverse, spi.STRING_FIRST)
		leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortField), [3]*document.Document{
			docOf(numericDVField(t, "id", 3), sortedSetDVField(t, "foo", newBytesRef("zzz")),
				sortedSetDVField(t, "foo", newBytesRef("zzza")), sortedSetDVField(t, "foo", newBytesRef("zzzd"))),
			// missing
			docOf(numericDVField(t, "id", 1)),
			docOf(numericDVField(t, "id", 2), sortedSetDVField(t, "foo", newBytesRef("mmm")),
				sortedSetDVField(t, "foo", newBytesRef("nnnn"))),
		})
		if reverse {
			assertIDOrder(t, leaf, 3, 2, 1)
		} else {
			assertIDOrder(t, leaf, 1, 2, 3)
		}
		closeAll()
	}
}

func TestIndexSortingMissingStringLast(t *testing.T) {
	for _, reverse := range []bool{true, false} {
		sortField := sortFieldWithMissing("foo", index.SortTypeString, reverse, spi.STRING_LAST)
		leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortField), [3]*document.Document{
			docOf(sortedDVField(t, "foo", newBytesRef("zzz"))),
			// missing
			document.NewDocument(),
			docOf(sortedDVField(t, "foo", newBytesRef("mmm"))),
		})
		values := leafSorted(t, leaf, "foo")
		if reverse {
			assertNextSortedTerm(t, values, 1, "zzz")
			assertNextSortedTerm(t, values, 2, "mmm")
		} else {
			assertNextSortedTerm(t, values, 0, "mmm")
			assertNextSortedTerm(t, values, 1, "zzz")
		}
		assertNoMoreDocValues(t, values)
		closeAll()
	}
}

func TestIndexSortingMissingMultiValuedStringLast(t *testing.T) {
	for _, reverse := range []bool{true, false} {
		sortField := sortedSetSortField("foo", reverse, spi.STRING_LAST)
		leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortField), [3]*document.Document{
			docOf(numericDVField(t, "id", 2), sortedSetDVField(t, "foo", newBytesRef("zzz")),
				sortedSetDVField(t, "foo", newBytesRef("zzzd"))),
			// missing
			docOf(numericDVField(t, "id", 3)),
			docOf(numericDVField(t, "id", 1), sortedSetDVField(t, "foo", newBytesRef("mmm")),
				sortedSetDVField(t, "foo", newBytesRef("ppp"))),
		})
		if reverse {
			assertIDOrder(t, leaf, 3, 2, 1)
		} else {
			assertIDOrder(t, leaf, 1, 2, 3)
		}
		closeAll()
	}
}

// basicNumericSort renders testBasicLong/testBasicInt: values 18, -1, 7
// sort to -1, 7, 18.
func basicNumericSort(t *testing.T, sortType index.SortType) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(index.NewSortField("foo", sortType)), [3]*document.Document{
		docOf(numericDVField(t, "foo", 18)),
		docOf(numericDVField(t, "foo", -1)),
		docOf(numericDVField(t, "foo", 7)),
	})
	defer closeAll()
	values := leafNumeric(t, leaf, "foo")
	assertNextNumeric(t, values, 0, -1)
	assertNextNumeric(t, values, 1, 7)
	assertNextNumeric(t, values, 2, 18)
}

func TestIndexSortingBasicLong(t *testing.T) {
	basicNumericSort(t, index.SortTypeLong)
}

// multiValuedDocs renders the three documents of a multi-valued sort test:
// each has an "id" NumericDocValuesField and its "foo" SortedNumericDocValuesField
// values.
func multiValuedDocs(t testing.TB, ids [3]int64, values [3][]int64) [3]*document.Document {
	var docs [3]*document.Document
	for i := range docs {
		doc := docOf(numericDVField(t, "id", ids[i]))
		for _, v := range values[i] {
			doc.Add(sortedNumericDVField(t, "foo", v))
		}
		docs[i] = doc
	}
	return docs
}

func TestIndexSortingBasicMultiValuedLong(t *testing.T) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortedNumericSortField("foo", index.SortTypeLong, false, nil)),
		multiValuedDocs(t, [3]int64{3, 1, 2}, [3][]int64{{18, 35}, {-1}, {7, 22}}))
	defer closeAll()
	assertIDOrder(t, leaf, 1, 2, 3)
}

// missingNumericFirstOrLast renders testMissingLong/IntFirst/Last: values
// 18, missing, 7 under the given missing value.
func missingNumericFirstOrLast(t *testing.T, sortType index.SortType, missingValue any, last bool) {
	for _, reverse := range []bool{true, false} {
		sortField := sortFieldWithMissing("foo", sortType, reverse, missingValue)
		leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortField), [3]*document.Document{
			docOf(numericDVField(t, "foo", 18)),
			// missing
			document.NewDocument(),
			docOf(numericDVField(t, "foo", 7)),
		})
		values := leafNumeric(t, leaf, "foo")
		switch {
		case !last && reverse:
			assertNextNumeric(t, values, 0, 18)
			assertNextNumeric(t, values, 1, 7)
		case !last:
			// docID 0 has no value
			assertNextNumeric(t, values, 1, 7)
			assertNextNumeric(t, values, 2, 18)
		case reverse:
			// docID 0 is missing
			assertNextNumeric(t, values, 1, 18)
			assertNextNumeric(t, values, 2, 7)
		default:
			assertNextNumeric(t, values, 0, 7)
			assertNextNumeric(t, values, 1, 18)
		}
		if last {
			assertNoMoreDocValues(t, values)
		}
		closeAll()
	}
}

func TestIndexSortingMissingLongFirst(t *testing.T) {
	missingNumericFirstOrLast(t, index.SortTypeLong, int64(math.MinInt64), false)
}

// missingMultiValuedFirstOrLast renders the testMissingMultiValued*First/Last
// tests: the id order is 3, 2, 1 when reversed and 1, 2, 3 otherwise.
func missingMultiValuedFirstOrLast(t *testing.T, sortType index.SortType, missingValue any, ids [3]int64, values [3][]int64) {
	for _, reverse := range []bool{true, false} {
		sortField := sortedNumericSortField("foo", sortType, reverse, missingValue)
		leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortField), multiValuedDocs(t, ids, values))
		if reverse {
			assertIDOrder(t, leaf, 3, 2, 1)
		} else {
			assertIDOrder(t, leaf, 1, 2, 3)
		}
		closeAll()
	}
}

func TestIndexSortingMissingMultiValuedLongFirst(t *testing.T) {
	missingMultiValuedFirstOrLast(t, index.SortTypeLong, int64(math.MinInt64),
		[3]int64{3, 1, 2}, [3][]int64{{18, 27}, nil, {7, 24}})
}

func TestIndexSortingMissingLongLast(t *testing.T) {
	missingNumericFirstOrLast(t, index.SortTypeLong, int64(math.MaxInt64), true)
}

func TestIndexSortingMissingMultiValuedLongLast(t *testing.T) {
	missingMultiValuedFirstOrLast(t, index.SortTypeLong, int64(math.MaxInt64),
		[3]int64{2, 3, 1}, [3][]int64{{18, 65}, nil, {7, 34, 74}})
}

func TestIndexSortingBasicInt(t *testing.T) {
	basicNumericSort(t, index.SortTypeInt)
}

func TestIndexSortingBasicMultiValuedInt(t *testing.T) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortedNumericSortField("foo", index.SortTypeInt, false, nil)),
		multiValuedDocs(t, [3]int64{3, 1, 2}, [3][]int64{{18, 34}, {-1, 34}, {7, 22, 27}}))
	defer closeAll()
	assertIDOrder(t, leaf, 1, 2, 3)
}

func TestIndexSortingMissingIntFirst(t *testing.T) {
	missingNumericFirstOrLast(t, index.SortTypeInt, int32(math.MinInt32), false)
}

func TestIndexSortingMissingMultiValuedIntFirst(t *testing.T) {
	missingMultiValuedFirstOrLast(t, index.SortTypeInt, int32(math.MinInt32),
		[3]int64{3, 1, 2}, [3][]int64{{18, 187667}, nil, {7, 34}})
}

func TestIndexSortingMissingIntLast(t *testing.T) {
	missingNumericFirstOrLast(t, index.SortTypeInt, int32(math.MaxInt32), true)
}

func TestIndexSortingMissingMultiValuedIntLast(t *testing.T) {
	missingMultiValuedFirstOrLast(t, index.SortTypeInt, int32(math.MaxInt32),
		[3]int64{2, 3, 1}, [3][]int64{{18, 6372}, nil, {7, 8}})
}

func TestIndexSortingBasicDouble(t *testing.T) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(index.NewSortField("foo", index.SortTypeDouble)), [3]*document.Document{
		docOf(doubleDVField(t, "foo", 18.0)),
		docOf(doubleDVField(t, "foo", -1.0)),
		docOf(doubleDVField(t, "foo", 7.0)),
	})
	defer closeAll()
	values := leafNumeric(t, leaf, "foo")
	assertNextDouble(t, values, 0, -1.0)
	assertNextDouble(t, values, 1, 7.0)
	assertNextDouble(t, values, 2, 18.0)
}

func sortableDoubles(values ...float64) []int64 {
	out := make([]int64, len(values))
	for i, v := range values {
		out[i] = util.DoubleToSortableLong(v)
	}
	return out
}

func sortableFloats(values ...float32) []int64 {
	out := make([]int64, len(values))
	for i, v := range values {
		out[i] = int64(util.FloatToSortableInt(v))
	}
	return out
}

func TestIndexSortingBasicMultiValuedDouble(t *testing.T) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortedNumericSortField("foo", index.SortTypeDouble, false, nil)),
		multiValuedDocs(t, [3]int64{3, 1, 2}, [3][]int64{sortableDoubles(7.54, 27.0), sortableDoubles(-1.0, 0.0), sortableDoubles(7.0, 7.67)}))
	defer closeAll()
	assertIDOrder(t, leaf, 1, 2, 3)
}

// missingFloatingFirstOrLast renders testMissingDouble/FloatFirst/Last:
// values 18, missing, 7 under the given missing value.
func missingFloatingFirstOrLast(t *testing.T, sortType index.SortType, missingValue any, last bool) {
	for _, reverse := range []bool{true, false} {
		sortField := sortFieldWithMissing("foo", sortType, reverse, missingValue)
		var docs [3]*document.Document
		if sortType == index.SortTypeDouble {
			docs = [3]*document.Document{docOf(doubleDVField(t, "foo", 18.0)), document.NewDocument(), docOf(doubleDVField(t, "foo", 7.0))}
		} else {
			docs = [3]*document.Document{docOf(floatDVField(t, "foo", 18.0)), document.NewDocument(), docOf(floatDVField(t, "foo", 7.0))}
		}
		leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortField), docs)
		values := leafNumeric(t, leaf, "foo")
		assertNext := func(doc int, value float64) {
			if sortType == index.SortTypeDouble {
				assertNextDouble(t, values, doc, value)
			} else {
				assertNextFloat(t, values, doc, float32(value))
			}
		}
		switch {
		case !last && reverse:
			assertNext(0, 18.0)
			assertNext(1, 7.0)
		case !last:
			assertNext(1, 7.0)
			assertNext(2, 18.0)
		case reverse:
			assertNext(1, 18.0)
			assertNext(2, 7.0)
		default:
			assertNext(0, 7.0)
			assertNext(1, 18.0)
		}
		if last {
			assertNoMoreDocValues(t, values)
		}
		closeAll()
	}
}

func TestIndexSortingMissingDoubleFirst(t *testing.T) {
	missingFloatingFirstOrLast(t, index.SortTypeDouble, math.Inf(-1), false)
}

func TestIndexSortingMissingMultiValuedDoubleFirst(t *testing.T) {
	missingMultiValuedFirstOrLast(t, index.SortTypeDouble, math.Inf(-1),
		[3]int64{3, 1, 2}, [3][]int64{sortableDoubles(18.0, 18.76), nil, sortableDoubles(7.0, 70.0)})
}

func TestIndexSortingMissingDoubleLast(t *testing.T) {
	missingFloatingFirstOrLast(t, index.SortTypeDouble, math.Inf(1), true)
}

func TestIndexSortingMissingMultiValuedDoubleLast(t *testing.T) {
	missingMultiValuedFirstOrLast(t, index.SortTypeDouble, math.Inf(1),
		[3]int64{2, 3, 1}, [3][]int64{sortableDoubles(18.0, 8262.0), nil, sortableDoubles(7.0, 7.87)})
}

func TestIndexSortingBasicFloat(t *testing.T) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(index.NewSortField("foo", index.SortTypeFloat)), [3]*document.Document{
		docOf(floatDVField(t, "foo", 18.0)),
		docOf(floatDVField(t, "foo", -1.0)),
		docOf(floatDVField(t, "foo", 7.0)),
	})
	defer closeAll()
	values := leafNumeric(t, leaf, "foo")
	assertNextFloat(t, values, 0, -1.0)
	assertNextFloat(t, values, 1, 7.0)
	assertNextFloat(t, values, 2, 18.0)
}

func TestIndexSortingBasicMultiValuedFloat(t *testing.T) {
	leaf, closeAll := sortingThreeDocs(t, index.NewSort(sortedNumericSortField("foo", index.SortTypeFloat, false, nil)),
		multiValuedDocs(t, [3]int64{3, 1, 2}, [3][]int64{sortableFloats(18.0, 29.0), sortableFloats(-1.0, 34.0), sortableFloats(7.0)}))
	defer closeAll()
	assertIDOrder(t, leaf, 1, 2, 3)
}

func TestIndexSortingMissingFloatFirst(t *testing.T) {
	missingFloatingFirstOrLast(t, index.SortTypeFloat, float32(math.Inf(-1)), false)
}

func TestIndexSortingMissingMultiValuedFloatFirst(t *testing.T) {
	missingMultiValuedFirstOrLast(t, index.SortTypeFloat, float32(math.Inf(-1)),
		[3]int64{3, 1, 2}, [3][]int64{sortableFloats(18.0, 726.0), nil, sortableFloats(7.0, 18.0)})
}

func TestIndexSortingMissingFloatLast(t *testing.T) {
	missingFloatingFirstOrLast(t, index.SortTypeFloat, float32(math.Inf(1)), true)
}

func TestIndexSortingMissingMultiValuedFloatLast(t *testing.T) {
	missingMultiValuedFirstOrLast(t, index.SortTypeFloat, float32(math.Inf(1)),
		[3]int64{2, 3, 1}, [3][]int64{sortableFloats(726.0, 18.0), nil, sortableFloats(12.67, 7.0)})
}

// sortingRandomIndex renders the indexing loop of testRandom1 and
// testMultiValuedRandom1.
func sortingRandomIndex(t *testing.T, w *index.IndexWriter, numDocs int, addFoo func(*document.Document)) *util.FixedBitSet {
	t.Helper()
	deleted, err := util.NewFixedBitSet(numDocs)
	if err != nil {
		t.Fatalf("new FixedBitSet: %v", err)
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		addFoo(doc)
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(i), true))
		doc.Add(numericDVField(t, "id", int64(i)))
		mustAddDocument(t, w, doc)
		if rand.Intn(5) == 0 {
			mustClose(t, openReaderFromWriter(t, w))
		} else if rand.Intn(30) == 0 {
			if err := w.ForceMerge(2); err != nil {
				t.Fatalf("forceMerge(2): %v", err)
			}
		} else if rand.Intn(4) == 0 {
			id := nextInt(0, i)
			deleted.Set(id)
			mustDeleteTerm(t, w, "id", strconv.Itoa(id))
		}
	}
	return deleted
}

func TestIndexSortingRandom1(t *testing.T) {
	dir := newDirectory()
	indexSort := index.NewSort(index.NewSortField("foo", index.SortTypeLong))
	w := mustNewIndexWriter(t, dir, sortingConfig(indexSort))
	numDocs := atLeast(200)
	sortingRandomIndex(t, w, numDocs, func(doc *document.Document) {
		doc.Add(numericDVField(t, "foo", int64(rand.Intn(20))))
	})

	// Check that segments are sorted
	reader := openReaderFromWriter(t, w)
	defer mustClose(t, reader, w, dir)
	if len(mustLeaves(t, reader)) > 0 {
		// switch (info.getDiagnostics().get(IndexWriter.SOURCE))
		t.Fatal(indexWriterSourceMissing)
	}
	newSearcher(t, reader)
}

func TestIndexSortingMultiValuedRandom1(t *testing.T) {
	dir := newDirectory()
	indexSort := index.NewSort(sortedNumericSortField("foo", index.SortTypeLong, false, nil))
	w := mustNewIndexWriter(t, dir, sortingConfig(indexSort))
	numDocs := atLeast(200)
	sortingRandomIndex(t, w, numDocs, func(doc *document.Document) {
		num := rand.Intn(10)
		for j := 0; j < num; j++ {
			doc.Add(sortedNumericDVField(t, "foo", int64(rand.Intn(2000))))
		}
	})

	reader := openReaderFromWriter(t, w)
	defer mustClose(t, reader, w, dir)
	// Now check that the index is consistent
	newSearcher(t, reader)
}

// updateRunnable renders the static UpdateRunnable (and, with dvUpdate,
// DVUpdateRunnable): threads repeatedly update a random id, recording the
// value under the values lock, and sometimes reopen or force-merge.
func updateRunnable(t *testing.T, w *index.IndexWriter, numDocs int, r *rand.Rand, latch *countDownLatch,
	updateCount *atomic.Int64, valuesMu *sync.Mutex, values map[int]int64, dvUpdate bool) {
	if !latch.awaitFromGoroutine(t, "latch") {
		return
	}
	for updateCount.Add(-1) >= 0 {
		id := r.Intn(numDocs)
		value := int64(r.Intn(20))
		valuesMu.Lock()
		var err error
		if dvUpdate {
			f, ferr := document.NewNumericDocValuesField("bar", value)
			if ferr != nil {
				valuesMu.Unlock()
				t.Errorf("new NumericDocValuesField: %v", ferr)
				return
			}
			_, err = w.UpdateDocValues(index.NewTerm("id", strconv.Itoa(id)), []*document.Field{f.Field})
		} else {
			doc := document.NewDocument()
			sf, serr := document.NewStringField("id", strconv.Itoa(id), false)
			if serr != nil {
				valuesMu.Unlock()
				t.Errorf("new StringField: %v", serr)
				return
			}
			doc.Add(sf)
			f, ferr := document.NewNumericDocValuesField("foo", value)
			if ferr != nil {
				valuesMu.Unlock()
				t.Errorf("new NumericDocValuesField: %v", ferr)
				return
			}
			doc.Add(f)
			_, err = w.UpdateDocument(index.NewTerm("id", strconv.Itoa(id)), doc)
		}
		if err == nil {
			values[id] = value
		}
		valuesMu.Unlock()
		if err != nil {
			t.Errorf("update: %v", err)
			return
		}

		switch r.Intn(10) {
		case 0, 1:
			// reopen
			dr, err := index.OpenDirectoryReaderFromWriter(w)
			if err != nil {
				t.Errorf("DirectoryReader.open(w): %v", err)
				return
			}
			if err := dr.Close(); err != nil {
				t.Errorf("close: %v", err)
				return
			}
		case 2:
			if err := w.ForceMerge(3); err != nil {
				t.Errorf("forceMerge(3): %v", err)
				return
			}
		}
	}
}

// There is tricky logic to resolve deletes that happened while merging
func TestIndexSortingConcurrentUpdates(t *testing.T) {
	dir := newDirectory()
	indexSort := index.NewSort(index.NewSortField("foo", index.SortTypeLong))
	w := mustNewIndexWriter(t, dir, sortingConfig(indexSort))
	var valuesMu sync.Mutex
	values := make(map[int]int64)

	numDocs := atLeast(100)
	const numThreads = 2

	var updateCount atomic.Int64
	updateCount.Store(int64(atLeast(1000)))
	latch := newCountDownLatch()
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		r := rand.New(rand.NewSource(javaNextLong()))
		wg.Add(1)
		go func() {
			defer wg.Done()
			updateRunnable(t, w, numDocs, r, latch, &updateCount, &valuesMu, values, false)
		}()
	}
	latch.countDown()
	wg.Wait()
	if t.Failed() {
		mustClose(t, w, dir)
		t.FailNow()
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	reader := openReaderFromWriter(t, w)
	defer mustClose(t, reader, w, dir)
	newSearcher(t, reader)
}

// docvalues fields involved in the index sort cannot be updated
func TestIndexSortingBadDVUpdate(t *testing.T) {
	dir := newDirectory()
	indexSort := index.NewSort(index.NewSortField("foo", index.SortTypeLong))
	w := mustNewIndexWriter(t, dir, sortingConfig(indexSort))
	doc := document.NewDocument()
	id, err := document.NewStringFieldFromBytesRef("id", newBytesRef("0"), false)
	if err != nil {
		t.Fatalf("new StringField: %v", err)
	}
	doc.Add(id)
	doc.Add(numericDVField(t, "foo", javaNextInt()))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	const message = `cannot update docvalues field involved in the index sort, field=foo, sort=<long: "foo">`
	_, err = w.UpdateDocValues(index.NewTerm("id", "0"), []*document.Field{numericDVField(t, "foo", -1).Field})
	expectIAEMessage(t, err, message)
	_, err = w.UpdateNumericDocValue(index.NewTerm("id", "0"), "foo", -1)
	expectIAEMessage(t, err, message)
	mustClose(t, w, dir)
}

// There is tricky logic to resolve dv updates that happened while merging
func TestIndexSortingConcurrentDVUpdates(t *testing.T) {
	dir := newDirectory()
	indexSort := index.NewSort(index.NewSortField("foo", index.SortTypeLong))
	w := mustNewIndexWriter(t, dir, sortingConfig(indexSort))
	var valuesMu sync.Mutex
	values := make(map[int]int64)

	numDocs := atLeast(100)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(i), false))
		doc.Add(numericDVField(t, "foo", javaNextInt()))
		doc.Add(numericDVField(t, "bar", -1))
		mustAddDocument(t, w, doc)
		values[i] = -1
	}
	const numThreads = 2
	var updateCount atomic.Int64
	updateCount.Store(int64(atLeast(1000)))
	latch := newCountDownLatch()
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		r := rand.New(rand.NewSource(javaNextLong()))
		wg.Add(1)
		go func() {
			defer wg.Done()
			updateRunnable(t, w, numDocs, r, latch, &updateCount, &valuesMu, values, true)
		}()
	}
	latch.countDown()
	wg.Wait()
	if t.Failed() {
		mustClose(t, w, dir)
		t.FailNow()
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	reader := openReaderFromWriter(t, w)
	defer mustClose(t, reader, w, dir)
	newSearcher(t, reader)
}

func expectIAEContaining(t testing.TB, err error, fragment string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected IllegalArgumentException containing %q", fragment)
	}
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("getMessage() %q does not contain %q", err.Error(), fragment)
	}
}

func TestIndexSortingBadAddIndexes(t *testing.T) {
	dir := newDirectory()
	indexSort := index.NewSort(index.NewSortField("foo", index.SortTypeLong))
	iwc1 := newIndexWriterConfig()
	iwc1.SetIndexSort(indexSort)
	w := mustNewIndexWriter(t, dir, iwc1)
	mustAddDocument(t, w, document.NewDocument())
	indexSorts := []*index.Sort{nil, index.NewSort(index.NewSortField("bar", index.SortTypeLong))}
	// The first iteration, sort == null, reaches w.addIndexes(codecReaders).
	sort := indexSorts[0]
	dir2 := newDirectory()
	iwc2 := newIndexWriterConfig()
	if sort != nil {
		iwc2.SetIndexSort(sort)
	}
	w2 := mustNewIndexWriter(t, dir2, iwc2)
	mustAddDocument(t, w2, document.NewDocument())
	reader := openReaderFromWriter(t, w2)
	mustClose(t, w2)
	_, err := w.AddIndexes(dir2)
	expectIAEContaining(t, err, "cannot change index sort")
	defer mustClose(t, reader, dir2, w, dir)
	// w.addIndexes(CodecReader[] codecReaders)
	t.Fatal(addIndexesCodecReadersMissing)
}

// testIndexSortingAddIndexes renders the public testAddIndexes(boolean,
// boolean).
func testIndexSortingAddIndexes(t *testing.T, withDeletes, useReaders bool) {
	dir := newDirectory()
	iwc1 := newIndexWriterConfig()
	useParent := rarely()
	if useParent {
		iwc1.SetParentField("___parent")
	}
	indexSort := index.NewSort(index.NewSortField("foo", index.SortTypeLong), index.NewSortField("bar", index.SortTypeLong))
	iwc1.SetIndexSort(indexSort)
	w := newRandomIndexWriterWithConfig(t, dir, iwc1)
	numDocs := atLeast(100)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(i), false))
		doc.Add(numericDVField(t, "foo", int64(rand.Intn(20))))
		doc.Add(numericDVField(t, "bar", int64(rand.Intn(20))))
		if _, err := w.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if withDeletes {
		for i := rand.Intn(5); i < numDocs; i += nextInt(1, 5) {
			if _, err := w.DeleteDocuments(index.NewTerm("id", strconv.Itoa(i))); err != nil {
				t.Fatalf("deleteDocuments: %v", err)
			}
		}
	}
	if rand.Intn(2) == 0 {
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}
	reader, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	mustClose(t, w)

	dir2 := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	if rand.Intn(2) == 0 {
		// test congruent index sort
		iwc.SetIndexSort(index.NewSort(index.NewSortField("foo", index.SortTypeLong)))
	} else {
		iwc.SetIndexSort(indexSort)
	}
	if useParent {
		iwc.SetParentField("___parent")
	}
	w2 := mustNewIndexWriter(t, dir2, iwc)

	if useReaders {
		mustClose(t, reader, w2, dir, dir2)
		// w2.addIndexes(CodecReader[] codecReaders)
		t.Fatal(addIndexesCodecReadersMissing)
	}
	mustAddIndexes(t, w2, dir)
	reader2 := openReaderFromWriter(t, w2)
	defer mustClose(t, reader, reader2, w2, dir, dir2)
	newSearcher(t, reader)
}

func TestIndexSortingAddIndexes(t *testing.T) {
	testIndexSortingAddIndexes(t, false, true)
}

func TestIndexSortingAddIndexesWithDeletions(t *testing.T) {
	testIndexSortingAddIndexes(t, true, true)
}

func TestIndexSortingAddIndexesWithDirectory(t *testing.T) {
	testIndexSortingAddIndexes(t, false, false)
}

func TestIndexSortingAddIndexesWithDeletionsAndDirectory(t *testing.T) {
	testIndexSortingAddIndexes(t, true, false)
}

func TestIndexSortingBadSort(t *testing.T) {
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	const message = "Cannot sort index with sort field <score>"
	var got any
	func() {
		defer func() { got = recover() }()
		iwc.SetIndexSort(index.SortRELEVANCE)
	}()
	if got == nil {
		t.Fatal("expected IllegalArgumentException from setIndexSort(Sort.RELEVANCE)")
	}
	if msg, _ := got.(string); msg != message {
		if e, ok := got.(error); !ok || e.Error() != message {
			t.Fatalf("getMessage(): expected %q, got %v", message, got)
		}
	}
}

// you can't change the index sort on an existing index:
func TestIndexSortingIllegalChangeSort(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, sortingConfig(index.NewSort(index.NewSortField("foo", index.SortTypeLong))))
	mustAddDocument(t, w, document.NewDocument())
	mustClose(t, openReaderFromWriter(t, w))
	mustAddDocument(t, w, document.NewDocument())
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w)

	iwc2 := sortingConfig(index.NewSort(index.NewSortField("bar", index.SortTypeLong)))
	w2, err := index.NewIndexWriter(dir, iwc2)
	if err == nil {
		mustClose(t, w2)
		t.Fatal("expected IllegalArgumentException from new IndexWriter(dir, iwc2)")
	}
	message := err.Error()
	if !strings.Contains(message, `cannot change previous indexSort=<long: "foo">`) {
		t.Fatalf("message %q lacks the previous index sort", message)
	}
	if !strings.Contains(message, `to new indexSort=<long: "bar">`) {
		t.Fatalf("message %q lacks the new index sort", message)
	}
	mustClose(t, dir)
}

// positionsTokenStream renders the static PositionsTokenStream.
type positionsTokenStream struct {
	*analysis.BaseTokenStream
	term     analysis.CharTermAttribute
	payload  analysis.PayloadAttribute
	offset   tokenattributes.OffsetAttribute
	pos, off int
}

func newPositionsTokenStream() *positionsTokenStream {
	s := &positionsTokenStream{BaseTokenStream: analysis.NewBaseTokenStream()}
	s.term = s.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	s.payload = s.AddAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
	s.offset = s.AddAttribute(tokenattributes.OffsetAttributeType).(tokenattributes.OffsetAttribute)
	return s
}

func (s *positionsTokenStream) IncrementToken() (bool, error) {
	if s.pos == 0 {
		return false, nil
	}

	s.ClearAttributes()
	s.term.AppendString("#all#")
	s.payload.SetPayload(newBytesRef(strconv.Itoa(s.pos)))
	s.offset.SetOffset(s.off, s.off)
	s.pos--
	s.off++
	return true, nil
}

func (s *positionsTokenStream) setID(id int) {
	s.pos = id/10 + 1
	s.off = 0
}

func TestIndexSortingRandom2(t *testing.T) {
	numDocs := atLeast(100)

	positionsType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	positionsType.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	positionsType.Freeze()

	termVectorsType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	termVectorsType.SetStoreTermVectors(true)
	termVectorsType.Freeze()

	docs := make([]*document.Document, 0, numDocs)
	for i := 0; i < numDocs; i++ {
		id := i * 10
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(id), true))
		doc.Add(newStringFieldNoRandom(t, "docs", "#all#", false))
		positions := newPositionsTokenStream()
		positions.setID(id)
		pf, err := document.NewField("positions", positions, positionsType)
		if err != nil {
			t.Fatalf("new Field(positions): %v", err)
		}
		doc.Add(pf)
		doc.Add(numericDVField(t, "numeric", int64(id)))
		value := strings.TrimSuffix(strings.Repeat(strconv.Itoa(id)+" ", id), " ")
		doc.Add(mustNewTextFieldNoRandom(t, "norms", value, false))
		doc.Add(binaryDVField(t, "binary", newBytesRef(strconv.Itoa(id))))
		doc.Add(sortedDVField(t, "sorted", newBytesRef(strconv.Itoa(id))))
		doc.Add(sortedSetDVField(t, "multi_valued_string", newBytesRef(strconv.Itoa(id))))
		doc.Add(sortedSetDVField(t, "multi_valued_string", newBytesRef(strconv.Itoa(id+1))))
		doc.Add(sortedNumericDVField(t, "multi_valued_numeric", int64(id)))
		doc.Add(sortedNumericDVField(t, "multi_valued_numeric", int64(id+1)))
		doc.Add(mustNewFieldNoRandom(t, "term_vectors", strconv.Itoa(id), termVectorsType))
		bytes := make([]byte, 4)
		util.IntToSortableBytes(int32(id), bytes, 0)
		doc.Add(document.NewBinaryPoint("points", bytes))
		docs = append(docs, doc)
	}

	// We add document already in ID order for the first writer:
	dir1 := newFSDirectory(t)
	defer mustClose(t, dir1)
	// iwc1.setSimilarity(new NormsSimilarity(iwc1.getSimilarity()))
	t.Fatal(normsSimilarityMissing)
}

// randomDoc renders the private static RandomDoc.
type randomDoc struct {
	intValue     int32
	intValues    []int32
	longValue    int64
	longValues   []int64
	floatValue   float32
	floatValues  []float32
	doubleValue  float64
	doubleValues []float64
	bytesValue   []byte
	bytesValues  [][]byte
}

func newRandomDoc() *randomDoc {
	d := &randomDoc{
		intValue:    int32(javaNextInt()),
		longValue:   javaNextLong(),
		floatValue:  rand.Float32(),
		doubleValue: rand.Float64(),
	}
	d.bytesValue = make([]byte, nextInt(1, 50))
	fillRandomBytes(d.bytesValue)

	numValues := rand.Intn(10)
	d.intValues = make([]int32, numValues)
	d.longValues = make([]int64, numValues)
	d.floatValues = make([]float32, numValues)
	d.doubleValues = make([]float64, numValues)
	d.bytesValues = make([][]byte, numValues)
	for i := 0; i < numValues; i++ {
		d.intValues[i] = int32(javaNextInt())
		d.longValues[i] = javaNextLong()
		d.floatValues[i] = rand.Float32()
		d.doubleValues[i] = rand.Float64()
		d.bytesValues[i] = make([]byte, nextInt(1, 50))
		// Java fills bytesValue here, not bytesValues[i]:
		fillRandomBytes(d.bytesValue)
	}
	return d
}

// fillRandomBytes renders Random.nextBytes(byte[]).
func fillRandomBytes(b []byte) {
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
}

// randomIndexSortField renders the private static randomIndexSortField().
func randomIndexSortField() *index.SortField {
	reversed := rand.Intn(2) == 0
	maybe := func(v func() any) any {
		if rand.Intn(2) == 0 {
			return v()
		}
		return nil
	}
	switch rand.Intn(10) {
	case 0:
		return sortFieldMaybeMissing("int", index.SortTypeInt, reversed, maybe(func() any { return int32(javaNextInt()) }))
	case 1:
		return sortedNumericSortField("multi_valued_int", index.SortTypeInt, reversed, maybe(func() any { return int32(javaNextInt()) }))
	case 2:
		return sortFieldMaybeMissing("long", index.SortTypeLong, reversed, maybe(func() any { return javaNextLong() }))
	case 3:
		return sortedNumericSortField("multi_valued_long", index.SortTypeLong, reversed, maybe(func() any { return javaNextLong() }))
	case 4:
		return sortFieldMaybeMissing("float", index.SortTypeFloat, reversed, maybe(func() any { return rand.Float32() }))
	case 5:
		return sortedNumericSortField("multi_valued_float", index.SortTypeFloat, reversed, maybe(func() any { return rand.Float32() }))
	case 6:
		return sortFieldMaybeMissing("double", index.SortTypeDouble, reversed, maybe(func() any { return rand.Float64() }))
	case 7:
		return sortedNumericSortField("multi_valued_double", index.SortTypeDouble, reversed, maybe(func() any { return rand.Float64() }))
	case 8:
		return sortFieldMaybeMissing("bytes", index.SortTypeString, reversed, maybe(func() any { return spi.STRING_LAST }))
	default:
		return sortedSetSortField("multi_valued_bytes", reversed, maybe(func() any { return spi.STRING_LAST }))
	}
}

// sortFieldMaybeMissing renders new SortField(field, type, reverse,
// missingValue) where missingValue may be null.
func sortFieldMaybeMissing(field string, sortType index.SortType, reverse bool, missingValue any) *index.SortField {
	sf := index.NewSortFieldFull(field, sortType, reverse)
	if missingValue != nil {
		sf.SetMissingValue(missingValue)
	}
	return sf
}

// randomSort renders the private static randomSort().
func randomSort() *index.Sort {
	// at least 2
	numFields := nextInt(2, 4)
	sortFields := make([]*index.SortField, numFields)
	for i := 0; i < numFields-1; i++ {
		sortFields[i] = randomIndexSortField()
	}

	// tie-break by id:
	sortFields[numFields-1] = index.NewSortField("id", index.SortTypeInt)

	return index.NewSort(sortFields...)
}

// pits index time sorting against query time sorting
func TestIndexSortingRandom3(t *testing.T) {
	numDocs := atLeast(1000)

	sort := randomSort()

	// no index sorting, all search-time sorting:
	dir1 := newFSDirectory(t)
	iwc1 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w1 := mustNewIndexWriter(t, dir1, iwc1)

	// use index sorting:
	dir2 := newFSDirectory(t)
	iwc2 := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc2.SetIndexSort(sort)
	w2 := mustNewIndexWriter(t, dir2, iwc2)

	toDelete := make(map[int]struct{})

	deleteChance := rand.Float64()

	for id := 0; id < numDocs; id++ {
		docValues := newRandomDoc()

		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(id), true))
		doc.Add(numericDVField(t, "id", int64(id)))
		doc.Add(numericDVField(t, "int", int64(docValues.intValue)))
		doc.Add(numericDVField(t, "long", docValues.longValue))
		doc.Add(doubleDVField(t, "double", docValues.doubleValue))
		doc.Add(floatDVField(t, "float", docValues.floatValue))
		doc.Add(sortedDVField(t, "bytes", docValues.bytesValue))

		for _, value := range docValues.intValues {
			doc.Add(sortedNumericDVField(t, "multi_valued_int", int64(value)))
		}

		for _, value := range docValues.longValues {
			doc.Add(sortedNumericDVField(t, "multi_valued_long", value))
		}

		for _, value := range docValues.floatValues {
			doc.Add(sortedNumericDVField(t, "multi_valued_float", int64(util.FloatToSortableInt(value))))
		}

		for _, value := range docValues.doubleValues {
			doc.Add(sortedNumericDVField(t, "multi_valued_double", util.DoubleToSortableLong(value)))
		}

		for _, value := range docValues.bytesValues {
			doc.Add(sortedSetDVField(t, "multi_valued_bytes", value))
		}

		mustAddDocument(t, w1, doc)
		mustAddDocument(t, w2, doc)
		if rand.Float64() < deleteChance {
			toDelete[id] = struct{}{}
		}
	}
	for id := range toDelete {
		mustDeleteTerm(t, w1, "id", strconv.Itoa(id))
		mustDeleteTerm(t, w2, "id", strconv.Itoa(id))
	}
	r1 := openReaderFromWriter(t, w1)
	defer mustClose(t, r1, w1, w2, dir1, dir2)
	newSearcher(t, r1)
}

func TestIndexSortingTieBreak(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetIndexSort(index.NewSort(index.NewSortField("foo", index.SortTypeString)))
	iwc.SetMergePolicy(newLogMergePolicy())
	w := mustNewIndexWriter(t, dir, iwc)
	for id := 0; id < 1000; id++ {
		doc := document.NewDocument()
		sf, err := document.NewStoredFieldFromInt("id", id)
		if err != nil {
			t.Fatalf("new StoredField: %v", err)
		}
		doc.Add(sf)
		value := "bar1"
		if id < 500 {
			value = "bar2"
		}
		doc.Add(sortedDVField(t, "foo", newBytesRef(value)))
		mustAddDocument(t, w, doc)
		if id == 500 {
			mustCommit(t, w)
		}
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r := openReaderFromWriter(t, w)
	storedFields, err := r.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	for docID := 0; docID < 1000; docID++ {
		expectedID := docID - 500
		if docID < 500 {
			expectedID = 500 + docID
		}
		f := storedDocument(t, storedFields, docID).Get("id")
		if f == nil {
			t.Fatalf("doc %d has no id", docID)
		}
		if got := javaIntValue(f.NumericValue()); got != expectedID {
			t.Fatalf("doc %d: expected id %d, got %d", docID, expectedID, got)
		}
	}
	mustClose(t, r, w, dir)
}

// javaIntValue renders Number.intValue() over a stored numeric value.
func javaIntValue(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(int32(n))
	case float32:
		return int(n)
	case float64:
		return int(n)
	}
	return math.MinInt
}

func assertAdvanceExact(t testing.TB, values interface{ AdvanceExact(int) (bool, error) }, docID int, expected bool, what string) {
	t.Helper()
	got, err := values.AdvanceExact(docID)
	if err != nil {
		t.Fatalf("%s.advanceExact(%d): %v", what, docID, err)
	}
	if got != expected {
		t.Fatalf("%s.advanceExact(%d): expected %v, got %v", what, docID, expected, got)
	}
}

func mustLongValue(t testing.TB, values index.NumericDocValues) int64 {
	t.Helper()
	v, err := values.LongValue()
	if err != nil {
		t.Fatalf("longValue: %v", err)
	}
	return v
}

func TestIndexSortingIndexSortWithSparseField(t *testing.T) {
	dir := newDirectory()
	sortField := index.NewSortFieldFull("dense_int", index.SortTypeInt, true)
	w := mustNewIndexWriter(t, dir, sortingConfig(index.NewSort(sortField)))
	textField := newTextField(t, "sparse_text", "", false)
	for i := 0; i < 128; i++ {
		doc := document.NewDocument()
		doc.Add(numericDVField(t, "dense_int", int64(i)))
		if i < 64 {
			doc.Add(numericDVField(t, "sparse_int", int64(i)))
			doc.Add(binaryDVField(t, "sparse_binary", newBytesRef(strconv.Itoa(i))))
			textField.SetStringValue("foo")
			doc.Add(textField)
		}
		mustAddDocument(t, w, doc)
	}
	mustCommit(t, w)
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r := openReaderFromWriter(t, w)
	defer mustClose(t, r, w, dir)
	leaves := mustLeaves(t, r)
	if len(leaves) != 1 {
		t.Fatalf("r.leaves().size(): expected 1, got %d", len(leaves))
	}
	leafReader := leaves[0].LeafReader()

	denseValues := leafNumeric(t, leafReader, "dense_int")
	sparseValues := leafNumeric(t, leafReader, "sparse_int")
	sparseBinaryValues, err := leafReader.GetBinaryDocValues("sparse_binary")
	if err != nil || sparseBinaryValues == nil {
		t.Fatalf("getBinaryDocValues(sparse_binary): %v (%v)", sparseBinaryValues, err)
	}
	normsValues, err := leafReader.GetNormValues("sparse_text")
	if err != nil || normsValues == nil {
		t.Fatalf("getNormValues(sparse_text): %v (%v)", normsValues, err)
	}
	for docID := 0; docID < 128; docID++ {
		assertAdvanceExact(t, denseValues, docID, true, "denseValues")
		if got := int(int32(mustLongValue(t, denseValues))); got != 127-docID {
			t.Fatalf("dense_int[%d]: expected %d, got %d", docID, 127-docID, got)
		}
		if docID >= 64 {
			assertAdvanceExact(t, denseValues, docID, true, "denseValues")
			assertAdvanceExact(t, sparseValues, docID, true, "sparseValues")
			assertAdvanceExact(t, sparseBinaryValues, docID, true, "sparseBinaryValues")
			assertAdvanceExact(t, normsValues, docID, true, "normsValues")
			if v := mustLongValue(t, normsValues); v != 1 {
				t.Fatalf("norms[%d]: expected 1, got %d", docID, v)
			}
			if got := int(int32(mustLongValue(t, sparseValues))); got != 127-docID {
				t.Fatalf("sparse_int[%d]: expected %d, got %d", docID, 127-docID, got)
			}
			bv, err := sparseBinaryValues.BinaryValue()
			if err != nil {
				t.Fatalf("binaryValue: %v", err)
			}
			if string(bv) != strconv.Itoa(127-docID) {
				t.Fatalf("sparse_binary[%d]: expected %q, got %q", docID, strconv.Itoa(127-docID), bv)
			}
		} else {
			assertAdvanceExact(t, sparseBinaryValues, docID, false, "sparseBinaryValues")
			assertAdvanceExact(t, sparseValues, docID, false, "sparseValues")
			assertAdvanceExact(t, normsValues, docID, false, "normsValues")
		}
	}
}

func TestIndexSortingIndexSortOnSparseField(t *testing.T) {
	dir := newDirectory()
	sortField := sortFieldWithMissing("sparse", index.SortTypeInt, false, int32(math.MinInt32))
	w := mustNewIndexWriter(t, dir, sortingConfig(index.NewSort(sortField)))
	for i := 0; i < 128; i++ {
		doc := document.NewDocument()
		if i < 64 {
			doc.Add(numericDVField(t, "sparse", int64(i)))
		}
		mustAddDocument(t, w, doc)
	}
	mustCommit(t, w)
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r := openReaderFromWriter(t, w)
	defer mustClose(t, r, w, dir)
	leaves := mustLeaves(t, r)
	if len(leaves) != 1 {
		t.Fatalf("r.leaves().size(): expected 1, got %d", len(leaves))
	}
	sparseValues := leafNumeric(t, leaves[0].LeafReader(), "sparse")
	for docID := 0; docID < 128; docID++ {
		if docID >= 64 {
			assertAdvanceExact(t, sparseValues, docID, true, "sparseValues")
			if got := int(int32(mustLongValue(t, sparseValues))); got != docID-64 {
				t.Fatalf("sparse[%d]: expected %d, got %d", docID, docID-64, got)
			}
		} else {
			assertAdvanceExact(t, sparseValues, docID, false, "sparseValues")
		}
	}
}

func TestIndexSortingWrongSortFieldType(t *testing.T) {
	dir := newDirectory()
	dvs := []*document.Field{
		sortedDVField(t, "field", newBytesRef("")).Field,
		sortedSetDVField(t, "field", newBytesRef("")).Field,
		numericDVField(t, "field", 42).Field,
		sortedNumericDVField(t, "field", 42).Field,
	}

	sortFields := []*index.SortField{
		index.NewSortField("field", index.SortTypeString),
		sortedSetSortField("field", false, nil),
		index.NewSortField("field", index.SortTypeInt),
		sortedNumericSortField("field", index.SortTypeInt, false, nil),
	}

	for i := range sortFields {
		for j := range dvs {
			if i == j {
				continue
			}
			indexSort := index.NewSort(sortFields[i])
			w := mustNewIndexWriter(t, dir, sortingConfig(indexSort))
			doc := document.NewDocument()
			doc.Add(dvs[j])
			_, err := w.AddDocument(doc)
			expectIAEContaining(t, err, "expected field [field] to be ")
			doc.Clear()
			doc.Add(dvs[i])
			mustAddDocument(t, w, doc)
			doc.Add(dvs[j])
			_, err = w.AddDocument(doc)
			expectIAEMessage(t, err,
				"Inconsistency of field data structures across documents for field [field] of doc [2]. doc values type: expected '"+
					dvs[i].FieldType().DocValuesType().String()+"', but it has '"+dvs[j].FieldType().DocValuesType().String()+"'.")
			if err := w.Rollback(); err != nil {
				t.Fatalf("rollback: %v", err)
			}
			mustClose(t, w)
		}
	}
	mustClose(t, dir)
}

func TestIndexSortingDeleteByTermOrQuery(t *testing.T) {
	dir := newDirectory()
	config := newIndexWriterConfig()
	config.SetIndexSort(index.NewSort(index.NewSortField("numeric", index.SortTypeLong)))
	w := mustNewIndexWriter(t, dir, config)
	doc := document.NewDocument()
	numDocs := rand.Intn(2000) + 5
	expectedValues := make([]int64, numDocs)

	for i := 0; i < numDocs; i++ {
		expectedValues[i] = int64(rand.Intn(math.MaxInt32))
		doc.Clear()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(i), true))
		doc.Add(numericDVField(t, "numeric", expectedValues[i]))
		mustAddDocument(t, w, doc)
	}
	numDeleted := rand.Intn(numDocs) + 1
	for i := 0; i < numDeleted; i++ {
		idToDelete := rand.Intn(numDocs)
		if rand.Intn(2) == 0 {
			if _, err := w.DeleteDocumentsQuery([]index.Query{search.NewTermQuery(index.NewTerm("id", strconv.Itoa(idToDelete)))}); err != nil {
				t.Fatalf("deleteDocuments(query): %v", err)
			}
		} else {
			mustDeleteTerm(t, w, "id", strconv.Itoa(idToDelete))
		}

		expectedValues[idToDelete] = -int64(rand.Intn(math.MaxInt32)) // force a reordering
		doc.Clear()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(idToDelete), true))
		doc.Add(numericDVField(t, "numeric", expectedValues[idToDelete]))
		mustAddDocument(t, w, doc)
	}

	docCount := 0
	reader := openReaderFromWriter(t, w)
	for _, leafCtx := range mustLeaves(t, reader) {
		leaf := leafCtx.LeafReader()
		liveDocs := leaf.GetLiveDocs()
		values, err := leaf.GetNumericDocValues("numeric")
		if err != nil {
			t.Fatalf("getNumericDocValues: %v", err)
		}
		if values == nil {
			continue
		}
		storedFields, err := leaf.StoredFields()
		if err != nil {
			t.Fatalf("storedFields: %v", err)
		}
		for id := 0; id < leaf.MaxDoc(); id++ {
			if liveDocs != nil && !liveDocs.Get(id) {
				continue
			}
			if ok, err := values.AdvanceExact(id); err != nil {
				t.Fatalf("advanceExact: %v", err)
			} else if !ok {
				continue
			}
			idValue := docGet(storedDocument(t, storedFields, id), "id")
			if idValue == nil {
				t.Fatalf("doc %d has no id", id)
			}
			globalID, err := strconv.Atoi(*idValue)
			if err != nil {
				t.Fatalf("Integer.parseInt(%q): %v", *idValue, err)
			}
			assertAdvanceExact(t, values, id, true, "values")
			if got := mustLongValue(t, values); got != expectedValues[globalID] {
				t.Fatalf("id %d: expected %d, got %d", globalID, expectedValues[globalID], got)
			}
			docCount++
		}
	}
	if docCount != numDocs {
		t.Fatalf("docCount: expected %d, got %d", numDocs, docCount)
	}
	mustClose(t, reader, w, dir)
}

// sortedFieldPostings renders getOnlyLeafReader(reader).terms("field").iterator().
func sortedFieldPostings(t testing.TB, reader *index.DirectoryReader) index.TermsEnum {
	t.Helper()
	return onlyLeafTermsEnum(t, reader, "field")
}

func assertNextPostingsDoc(t testing.TB, postings index.PostingsEnum, doc int) {
	t.Helper()
	if got, err := postings.NextDoc(); err != nil || got != doc {
		t.Fatalf("postings.nextDoc(): expected %d, got %d (%v)", doc, got, err)
	}
}

func assertPostingsFreq(t testing.TB, postings index.PostingsEnum, freq int) {
	t.Helper()
	if got, err := postings.Freq(); err != nil || got != freq {
		t.Fatalf("postings.freq(): expected %d, got %d (%v)", freq, got, err)
	}
}

func assertNextPosition(t testing.TB, postings index.PostingsEnum, position int) {
	t.Helper()
	if got, err := postings.NextPosition(); err != nil || got != position {
		t.Fatalf("postings.nextPosition(): expected %d, got %d (%v)", position, got, err)
	}
}

func assertOffsets(t testing.TB, postings index.PostingsEnum, start, end int) {
	t.Helper()
	if got, err := postings.StartOffset(); err != nil || got != start {
		t.Fatalf("postings.startOffset(): expected %d, got %d (%v)", start, got, err)
	}
	if got, err := postings.EndOffset(); err != nil || got != end {
		t.Fatalf("postings.endOffset(): expected %d, got %d (%v)", end, got, err)
	}
}

// sortFiveDocs renders the indexing of the testSortDocs* tests: five documents
// whose "sort" values are 0, 1, -1, 2, 3, force-merged into one segment.
func sortFiveDocs(t *testing.T, config *index.IndexWriterConfig, fields [5]func() []document.IndexableField) *index.DirectoryReader {
	t.Helper()
	config.SetIndexSort(index.NewSort(index.NewSortField("sort", index.SortTypeLong)))
	dir := newDirectory()
	t.Cleanup(func() { mustClose(t, dir) })
	w := mustNewIndexWriter(t, dir, config)
	for i, sortValue := range []int64{0, 1, -1, 2, 3} {
		doc := docOf(numericDVField(t, "sort", sortValue))
		for _, f := range fields[i]() {
			doc.Add(f)
		}
		mustAddDocument(t, w, doc)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	reader := openReaderFromWriter(t, w)
	mustClose(t, w)
	return reader
}

func TestIndexSortingSortDocs(t *testing.T) {
	dir := newDirectory()
	config := newIndexWriterConfig()
	config.SetIndexSort(index.NewSort(index.NewSortField("sort", index.SortTypeLong)))
	w := mustNewIndexWriter(t, dir, config)
	doc := document.NewDocument()
	sort := numericDVField(t, "sort", 0)
	doc.Add(sort)
	field := newStringFieldNoRandom(t, "field", "a", false)
	doc.Add(field)
	mustAddDocument(t, w, doc)
	sort.SetLongValue(1)
	field.SetStringValue("b")
	mustAddDocument(t, w, doc)
	sort.SetLongValue(-1)
	field.SetStringValue("a")
	mustAddDocument(t, w, doc)
	sort.SetLongValue(2)
	field.SetStringValue("a")
	mustAddDocument(t, w, doc)
	sort.SetLongValue(3)
	field.SetStringValue("b")
	mustAddDocument(t, w, doc)
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	reader := openReaderFromWriter(t, w)
	mustClose(t, w)
	fieldTerms := sortedFieldPostings(t, reader)
	assertNextTermBytes(t, fieldTerms, "a")
	postings := mustPostingsAll(t, fieldTerms)
	assertNextPostingsDoc(t, postings, 0)
	assertNextPostingsDoc(t, postings, 1)
	assertNextPostingsDoc(t, postings, 3)
	assertNextPostingsDoc(t, postings, spi.NO_MORE_DOCS)
	assertNextTermBytes(t, fieldTerms, "b")
	postings = mustPostingsAll(t, fieldTerms)
	assertNextPostingsDoc(t, postings, 2)
	assertNextPostingsDoc(t, postings, 4)
	assertNextPostingsDoc(t, postings, spi.NO_MORE_DOCS)
	assertNoNextTerm(t, fieldTerms)
	mustClose(t, reader, dir)
}

func frozenFieldTypeWith(opts index.IndexOptions, tokenized bool) *document.FieldType {
	ft := document.NewFieldType()
	ft.SetIndexOptions(opts)
	ft.SetTokenized(tokenized)
	ft.Freeze()
	return ft
}

func repeatedFields(t testing.TB, ft *document.FieldType, values ...string) func() []document.IndexableField {
	return func() []document.IndexableField {
		out := make([]document.IndexableField, len(values))
		for i, v := range values {
			out[i] = mustNewFieldNoRandom(t, "field", v, ft)
		}
		return out
	}
}

func TestIndexSortingSortDocsAndFreqs(t *testing.T) {
	ft := frozenFieldTypeWith(index.IndexOptionsDocsAndFreqs, false)
	reader := sortFiveDocs(t, newIndexWriterConfig(), [5]func() []document.IndexableField{
		repeatedFields(t, ft, "a", "a"),
		repeatedFields(t, ft, "b"),
		repeatedFields(t, ft, "a", "a", "a"),
		repeatedFields(t, ft, "a"),
		repeatedFields(t, ft, "b", "b", "b"),
	})
	defer mustClose(t, reader)
	fieldTerms := sortedFieldPostings(t, reader)
	assertNextTermBytes(t, fieldTerms, "a")
	postings := mustPostingsAll(t, fieldTerms)
	for _, df := range [][2]int{{0, 3}, {1, 2}, {3, 1}} {
		assertNextPostingsDoc(t, postings, df[0])
		assertPostingsFreq(t, postings, df[1])
	}
	assertNextPostingsDoc(t, postings, spi.NO_MORE_DOCS)
	assertNextTermBytes(t, fieldTerms, "b")
	postings = mustPostingsAll(t, fieldTerms)
	for _, df := range [][2]int{{2, 1}, {4, 3}} {
		assertNextPostingsDoc(t, postings, df[0])
		assertPostingsFreq(t, postings, df[1])
	}
	assertNextPostingsDoc(t, postings, spi.NO_MORE_DOCS)
	assertNoNextTerm(t, fieldTerms)
}

// positionsExpectation is one document of a testSortDocsAndFreqsAndPositions*
// expectation: its docID and, per position, the position and offsets.
type positionsExpectation struct {
	doc       int
	positions [][3]int // position, startOffset, endOffset
}

func assertPositionsPostings(t testing.TB, postings index.PostingsEnum, withOffsets bool, expected []positionsExpectation) {
	t.Helper()
	for _, e := range expected {
		assertNextPostingsDoc(t, postings, e.doc)
		assertPostingsFreq(t, postings, len(e.positions))
		for _, p := range e.positions {
			assertNextPosition(t, postings, p[0])
			if withOffsets {
				assertOffsets(t, postings, p[1], p[2])
			}
		}
	}
	assertNextPostingsDoc(t, postings, spi.NO_MORE_DOCS)
}

func sortDocsAndPositions(t *testing.T, opts index.IndexOptions, withOffsets bool) {
	ft := frozenFieldTypeWith(opts, true)
	reader := sortFiveDocs(t, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()), [5]func() []document.IndexableField{
		repeatedFields(t, ft, "a a b"),
		repeatedFields(t, ft, "b"),
		repeatedFields(t, ft, "b a b b"),
		repeatedFields(t, ft, "a"),
		repeatedFields(t, ft, "b b"),
	})
	defer mustClose(t, reader)
	fieldTerms := sortedFieldPostings(t, reader)
	assertNextTermBytes(t, fieldTerms, "a")
	postings := mustPostingsAll(t, fieldTerms)
	assertPositionsPostings(t, postings, withOffsets, []positionsExpectation{
		{doc: 0, positions: [][3]int{{1, 2, 3}}},
		{doc: 1, positions: [][3]int{{0, 0, 1}, {1, 2, 3}}},
		{doc: 3, positions: [][3]int{{0, 0, 1}}},
	})
	assertNextTermBytes(t, fieldTerms, "b")
	postings = mustPostingsAll(t, fieldTerms)
	assertPositionsPostings(t, postings, withOffsets, []positionsExpectation{
		{doc: 0, positions: [][3]int{{0, 0, 1}, {2, 4, 5}, {3, 6, 7}}},
		{doc: 1, positions: [][3]int{{2, 4, 5}}},
		{doc: 2, positions: [][3]int{{0, 0, 1}}},
		{doc: 4, positions: [][3]int{{0, 0, 1}, {1, 2, 3}}},
	})
	assertNoNextTerm(t, fieldTerms)
}

func TestIndexSortingSortDocsAndFreqsAndPositions(t *testing.T) {
	sortDocsAndPositions(t, index.IndexOptionsDocsAndFreqsAndPositions, false)
}

func TestIndexSortingSortDocsAndFreqsAndPositionsAndOffsets(t *testing.T) {
	sortDocsAndPositions(t, index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, true)
}

func TestIndexSortingParentFieldNotConfigured(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetIndexSort(index.NewSort(index.NewSortField("foo", index.SortTypeInt)))
	writer := mustNewIndexWriter(t, dir, iwc)
	_, err := writer.AddDocuments([]*document.Document{document.NewDocument(), document.NewDocument()})
	expectIAEMessage(t, err,
		"a parent field must be set in order to use document blocks with index sorting; see IndexWriterConfig#setParentField")
	mustClose(t, writer, dir)
}

func TestIndexSortingBlockContainsParentField(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	parentField := "parent"
	iwc.SetParentField(parentField)
	iwc.SetIndexSort(index.NewSort(index.NewSortField("foo", index.SortTypeInt)))
	writer := mustNewIndexWriter(t, dir, iwc)
	const message = `"parent" is a reserved field and should not be added to any document`
	runnables := []func(){
		func() {
			doc := docOf(numericDVField(t, "parent", 0))
			_, err := writer.AddDocuments([]*document.Document{doc, document.NewDocument()})
			expectIAEMessage(t, err, message)
		},
		func() {
			doc := docOf(numericDVField(t, "parent", 0))
			_, err := writer.AddDocuments([]*document.Document{document.NewDocument(), doc})
			expectIAEMessage(t, err, message)
		},
	}
	rand.Shuffle(len(runnables), func(i, j int) { runnables[i], runnables[j] = runnables[j], runnables[i] })
	for _, runnable := range runnables {
		runnable()
	}
	mustClose(t, writer, dir)
}

func TestIndexSortingIndexSortWithBlocks(t *testing.T) {
	t.Fatal(assertingNeedsIndexSortCodecMissing)
}

func TestIndexSortingMixRandomDocumentsWithBlocks(t *testing.T) {
	t.Fatal(assertingNeedsIndexSortCodecMissing)
}
