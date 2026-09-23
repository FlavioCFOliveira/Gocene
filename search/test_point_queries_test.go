// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/TestPointQueries.java
//
// @LuceneTestCase.SuppressCodecs("SimpleText"): Gocene's default codec is
// never SimpleText, so the suppression needs no rendering.

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// pointQueryFactoriesBlocker names the production members the tests build
// their queries with: the static query factories of
// org.apache.lucene.document.IntPoint, LongPoint, FloatPoint, DoublePoint and
// BinaryPoint (newRangeQuery, newExactQuery, newSetQuery), each returning an
// anonymous PointRangeQuery / PointInSetQuery subclass with its own
// toString(...). None of them is ported.
const pointQueryFactoriesBlocker = "requires the static query factories of org.apache.lucene.document." +
	"IntPoint/LongPoint/FloatPoint/DoublePoint/BinaryPoint (newRangeQuery, newExactQuery, newSetQuery) (not ported)"

// pointNextUpDownBlocker names DoublePoint.nextUp/nextDown and
// FloatPoint.nextUp/nextDown: Gocene has unexported doubleNextUp/doubleNextDown
// only, and no FloatPoint counterpart.
const pointNextUpDownBlocker = "requires the public static org.apache.lucene.document.DoublePoint.nextUp/nextDown and " +
	"FloatPoint.nextUp/nextDown (not ported)"

// newMaybeVirusCheckingBlocker names the virus-checking file system that
// LuceneTestCase.newMaybeVirusCheckingDirectory() adds on its FSDirectory
// branch.
const newMaybeVirusCheckingBlocker = "requires LuceneTestCase.addVirusChecker(Path) and newFSDirectory(Path) (not ported)"

// errPointQueryFactories renders the blocker for the factory calls made on
// query threads, where the test cannot be stopped with t.Fatal.
var errPointQueryFactories = errors.New(pointQueryFactoriesBlocker)

// ---- static query factories of the document point classes (not ported) ----

func intPointNewRangeQuery(t *testing.T, field string, lowerValue, upperValue int32) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func intPointNewRangeQueryDims(t *testing.T, field string, lowerValue, upperValue []int32) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func intPointNewExactQuery(t *testing.T, field string, value int32) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func intPointNewSetQuery(t *testing.T, field string, values ...int32) search.Query {
	t.Helper()
	q, err := intPointNewSetQueryE(field, values...)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func intPointNewSetQueryE(field string, values ...int32) (search.Query, error) {
	return nil, errPointQueryFactories
}

func longPointNewRangeQuery(t *testing.T, field string, lowerValue, upperValue int64) search.Query {
	t.Helper()
	q, err := longPointNewRangeQueryE(field, lowerValue, upperValue)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func longPointNewRangeQueryE(field string, lowerValue, upperValue int64) (search.Query, error) {
	return nil, errPointQueryFactories
}

func longPointNewExactQuery(t *testing.T, field string, value int64) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func longPointNewSetQuery(t *testing.T, field string, values ...int64) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func floatPointNewRangeQuery(t *testing.T, field string, lowerValue, upperValue float32) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func floatPointNewExactQuery(t *testing.T, field string, value float32) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func floatPointNewSetQuery(t *testing.T, field string, values ...float32) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func doublePointNewRangeQuery(t *testing.T, field string, lowerValue, upperValue float64) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func doublePointNewRangeQueryDims(t *testing.T, field string, lowerValue, upperValue []float64) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func doublePointNewExactQuery(t *testing.T, field string, value float64) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func doublePointNewSetQuery(t *testing.T, field string, values ...float64) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func binaryPointNewRangeQuery(t *testing.T, field string, lowerValue, upperValue []byte) search.Query {
	t.Helper()
	q, err := binaryPointNewRangeQueryE(field, lowerValue, upperValue)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func binaryPointNewRangeQueryE(field string, lowerValue, upperValue []byte) (search.Query, error) {
	return nil, errPointQueryFactories
}

func binaryPointNewRangeQueryDims(t *testing.T, field string, lowerValue, upperValue [][]byte) search.Query {
	t.Helper()
	q, err := binaryPointNewRangeQueryDimsE(field, lowerValue, upperValue)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func binaryPointNewRangeQueryDimsE(field string, lowerValue, upperValue [][]byte) (search.Query, error) {
	return nil, errPointQueryFactories
}

func binaryPointNewExactQuery(t *testing.T, field string, value []byte) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

func binaryPointNewSetQuery(t *testing.T, field string, values ...[]byte) search.Query {
	t.Helper()
	t.Fatal(pointQueryFactoriesBlocker)
	return nil
}

// ---- class state ----

// Controls what range of values we randomly generate, so we sometimes test
// narrow ranges:
var (
	pointQueriesValueMid       int64
	pointQueriesValueRange     int
	pointQueriesBeforeClassRun sync.Once
)

// pointQueriesBeforeClass renders the @BeforeClass beforeClass(); it runs once
// per test binary, as JUnit runs it once per class.
func pointQueriesBeforeClass() {
	pointQueriesBeforeClassRun.Do(func() {
		if random().Intn(2) == 0 {
			pointQueriesValueMid = random().Int63() - random().Int63()
			if random().Intn(2) == 0 {
				// Wide range
				pointQueriesValueRange = nextInt(1, math.MaxInt32)
			} else {
				// Narrow range
				pointQueriesValueRange = nextInt(1, 100000)
			}
		} else {
			// All longs
			pointQueriesValueRange = 0
		}
	})
}

// randomLong renders Random.nextLong(): a uniformly distributed int64.
func randomLong() int64 {
	return int64(random().Uint64())
}

func mustNewPointIndexWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	return mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
}

func TestPointQueriesBasicInts(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	w := mustNewPointIndexWriter(t, dir)

	doc := document.NewDocument()
	doc.Add(document.NewIntPoint("point", -7))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewIntPoint("point", 0))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewIntPoint("point", 3))
	mustAddDocument(t, w, doc)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := search.NewIndexSearcher(r)
	assertCount(t, s, intPointNewRangeQuery(t, "point", -8, 1), 2)
	assertCount(t, s, intPointNewRangeQuery(t, "point", -7, 3), 3)
	assertCount(t, s, intPointNewExactQuery(t, "point", -7), 1)
	assertCount(t, s, intPointNewExactQuery(t, "point", -6), 0)
	mustClose(t, w, r, dir)
}

func TestPointQueriesBasicFloats(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	w := mustNewPointIndexWriter(t, dir)

	doc := document.NewDocument()
	doc.Add(document.NewFloatPoint("point", -7.0))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewFloatPoint("point", 0.0))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewFloatPoint("point", 3.0))
	mustAddDocument(t, w, doc)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := search.NewIndexSearcher(r)
	assertCount(t, s, floatPointNewRangeQuery(t, "point", -8.0, 1.0), 2)
	assertCount(t, s, floatPointNewRangeQuery(t, "point", -7.0, 3.0), 3)
	assertCount(t, s, floatPointNewExactQuery(t, "point", -7.0), 1)
	assertCount(t, s, floatPointNewExactQuery(t, "point", -6.0), 0)
	mustClose(t, w, r, dir)
}

func TestPointQueriesBasicLongs(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	w := mustNewPointIndexWriter(t, dir)

	doc := document.NewDocument()
	doc.Add(document.NewLongPoint("point", -7))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewLongPoint("point", 0))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewLongPoint("point", 3))
	mustAddDocument(t, w, doc)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := search.NewIndexSearcher(r)
	assertCount(t, s, longPointNewRangeQuery(t, "point", -8, 1), 2)
	assertCount(t, s, longPointNewRangeQuery(t, "point", -7, 3), 3)
	assertCount(t, s, longPointNewExactQuery(t, "point", -7), 1)
	assertCount(t, s, longPointNewExactQuery(t, "point", -6), 0)
	mustClose(t, w, r, dir)
}

func TestPointQueriesBasicDoubles(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	w := mustNewPointIndexWriter(t, dir)

	doc := document.NewDocument()
	doc.Add(document.NewDoublePoint("point", -7.0))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewDoublePoint("point", 0.0))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewDoublePoint("point", 3.0))
	mustAddDocument(t, w, doc)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := search.NewIndexSearcher(r)
	assertCount(t, s, doublePointNewRangeQuery(t, "point", -8.0, 1.0), 2)
	assertCount(t, s, doublePointNewRangeQuery(t, "point", -7.0, 3.0), 3)
	assertCount(t, s, doublePointNewExactQuery(t, "point", -7.0), 1)
	assertCount(t, s, doublePointNewExactQuery(t, "point", -6.0), 0)
	mustClose(t, w, r, dir)
}

func TestPointQueriesCrazyDoubles(t *testing.T) {
	pointQueriesBeforeClass()
	negZero := math.Copysign(0, -1)
	dir := newDirectory()
	w := mustNewPointIndexWriter(t, dir)

	for _, v := range []float64{math.Inf(-1), negZero, +0.0, math.SmallestNonzeroFloat64, math.MaxFloat64, math.Inf(1), math.NaN()} {
		doc := document.NewDocument()
		doc.Add(document.NewDoublePoint("point", v))
		mustAddDocument(t, w, doc)
	}

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := search.NewIndexSearcher(r)

	// exact queries
	assertCount(t, s, doublePointNewExactQuery(t, "point", math.Inf(-1)), 1)
	assertCount(t, s, doublePointNewExactQuery(t, "point", negZero), 1)
	assertCount(t, s, doublePointNewExactQuery(t, "point", +0.0), 1)
	assertCount(t, s, doublePointNewExactQuery(t, "point", math.SmallestNonzeroFloat64), 1)
	assertCount(t, s, doublePointNewExactQuery(t, "point", math.MaxFloat64), 1)
	assertCount(t, s, doublePointNewExactQuery(t, "point", math.Inf(1)), 1)
	assertCount(t, s, doublePointNewExactQuery(t, "point", math.NaN()), 1)

	// set query
	set := []float64{
		math.MaxFloat64,
		math.NaN(),
		+0.0,
		math.Inf(-1),
		math.SmallestNonzeroFloat64,
		negZero,
		math.Inf(1),
	}
	assertCount(t, s, doublePointNewSetQuery(t, "point", set...), 7)

	// ranges
	assertCount(t, s, doublePointNewRangeQuery(t, "point", math.Inf(-1), negZero), 2)
	assertCount(t, s, doublePointNewRangeQuery(t, "point", negZero, 0.0), 2)
	assertCount(t, s, doublePointNewRangeQuery(t, "point", 0.0, math.SmallestNonzeroFloat64), 2)
	assertCount(t, s, doublePointNewRangeQuery(t, "point", math.SmallestNonzeroFloat64, math.MaxFloat64), 2)
	assertCount(t, s, doublePointNewRangeQuery(t, "point", math.MaxFloat64, math.Inf(1)), 2)
	assertCount(t, s, doublePointNewRangeQuery(t, "point", math.Inf(1), math.NaN()), 2)

	mustClose(t, w, r, dir)
}

func TestPointQueriesCrazyFloats(t *testing.T) {
	pointQueriesBeforeClass()
	negZero := float32(math.Copysign(0, -1))
	nan := float32(math.NaN())
	posInf := float32(math.Inf(1))
	negInf := float32(math.Inf(-1))
	dir := newDirectory()
	w := mustNewPointIndexWriter(t, dir)

	for _, v := range []float32{negInf, negZero, +0.0, math.SmallestNonzeroFloat32, math.MaxFloat32, posInf, nan} {
		doc := document.NewDocument()
		doc.Add(document.NewFloatPoint("point", v))
		mustAddDocument(t, w, doc)
	}

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := search.NewIndexSearcher(r)

	// exact queries
	assertCount(t, s, floatPointNewExactQuery(t, "point", negInf), 1)
	assertCount(t, s, floatPointNewExactQuery(t, "point", negZero), 1)
	assertCount(t, s, floatPointNewExactQuery(t, "point", +0.0), 1)
	assertCount(t, s, floatPointNewExactQuery(t, "point", math.SmallestNonzeroFloat32), 1)
	assertCount(t, s, floatPointNewExactQuery(t, "point", math.MaxFloat32), 1)
	assertCount(t, s, floatPointNewExactQuery(t, "point", posInf), 1)
	assertCount(t, s, floatPointNewExactQuery(t, "point", nan), 1)

	// set query
	set := []float32{
		math.MaxFloat32,
		nan,
		+0.0,
		negInf,
		math.SmallestNonzeroFloat32,
		negZero,
		posInf,
	}
	assertCount(t, s, floatPointNewSetQuery(t, "point", set...), 7)

	// ranges
	assertCount(t, s, floatPointNewRangeQuery(t, "point", negInf, negZero), 2)
	assertCount(t, s, floatPointNewRangeQuery(t, "point", negZero, 0.0), 2)
	assertCount(t, s, floatPointNewRangeQuery(t, "point", 0.0, math.SmallestNonzeroFloat32), 2)
	assertCount(t, s, floatPointNewRangeQuery(t, "point", math.SmallestNonzeroFloat32, math.MaxFloat32), 2)
	assertCount(t, s, floatPointNewRangeQuery(t, "point", math.MaxFloat32, posInf), 2)
	assertCount(t, s, floatPointNewRangeQuery(t, "point", posInf, nan), 2)

	mustClose(t, w, r, dir)
}

func TestPointQueriesAllEqual(t *testing.T) {
	pointQueriesBeforeClass()
	numValues := atLeast(1000)
	value := pointQueriesRandomValue()
	values := make([]int64, numValues)

	for i := range values {
		values[i] = value
	}

	pointQueriesVerifyLongs(t, values, nil)
}

func TestPointQueriesRandomLongsTiny(t *testing.T) {
	pointQueriesBeforeClass()
	// Make sure single-leaf-node case is OK:
	pointQueriesDoTestRandomLongs(t, 10)
}

func TestPointQueriesRandomLongsMedium(t *testing.T) {
	pointQueriesBeforeClass()
	pointQueriesDoTestRandomLongs(t, 1000)
}

func TestPointQueriesRandomLongsBig(t *testing.T) {
	pointQueriesBeforeClass()
	pointQueriesDoTestRandomLongs(t, 20_000)
}

func pointQueriesDoTestRandomLongs(t *testing.T, count int) {
	numValues := nextInt(count, count*2)

	values := make([]int64, numValues)
	ids := make([]int, numValues)

	singleValued := random().Intn(2) == 0

	sameValuePct := random().Intn(100)

	id := 0
	for ord := 0; ord < numValues; ord++ {
		if ord > 0 && random().Intn(100) < sameValuePct {
			// Identical to old value
			values[ord] = values[random().Intn(ord)]
		} else {
			values[ord] = pointQueriesRandomValue()
		}

		ids[ord] = id
		if singleValued || random().Intn(2) == 1 {
			id++
		}
	}

	pointQueriesVerifyLongs(t, values, ids)
}

func TestPointQueriesLongEncode(t *testing.T) {
	pointQueriesBeforeClass()
	for i := 0; i < 10000; i++ {
		v := randomLong()
		tmp := make([]byte, 8)
		util.LongToSortableBytes(v, tmp, 0)
		v2 := util.SortableBytesToLong(tmp, 0)
		if v != v2 {
			t.Fatalf("got bytes=%v: %d != %d", javaBytesToString(tmp), v, v2)
		}
	}
}

// javaBytesToString renders java.util.Arrays.toString(byte[]).
func javaBytesToString(b []byte) string {
	parts := make([]string, len(b))
	for i, v := range b {
		parts[i] = strconv.Itoa(int(int8(v)))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// newMaybeVirusCheckingDirectory renders
// LuceneTestCase.newMaybeVirusCheckingDirectory().
func newMaybeVirusCheckingDirectory(t *testing.T) store.Directory {
	t.Helper()
	if random().Intn(5) == 4 {
		// Path path = addVirusChecker(createTempDir()); return newFSDirectory(path);
		t.Fatal(newMaybeVirusCheckingBlocker)
		return nil
	}
	return newDirectory()
}

// pointQueriesVerifyLongs verifies for long values.
func pointQueriesVerifyLongs(t *testing.T, values []int64, ids []int) {
	iwc := newIndexWriterConfig()

	// Else we can get O(N^2) merging:
	mbd := iwc.GetMaxBufferedDocs()
	if mbd != -1 && mbd < len(values)/100 {
		iwc.SetMaxBufferedDocs(len(values) / 100)
	}
	iwc.SetCodec(pointQueriesGetCodec(t))
	var dir store.Directory
	if len(values) > 100000 {
		// dir = newMaybeVirusCheckingFSDirectory(createTempDir("TestRangeTree"));
		t.Fatal(newFSDirectoryBlocker)
	} else {
		dir = newMaybeVirusCheckingDirectory(t)
	}

	// The point range query chooses only considers using an inverse BKD
	// visitor if there is exactly one value per document. If any document
	// misses a value that code is not exercised. Using a nextBoolean() here
	// increases the likelihood that there is no missing values, making the
	// test more likely to test that code.
	missingPct := 0
	if random().Intn(2) != 0 {
		missingPct = random().Intn(100)
	}
	deletedPct := random().Intn(100)

	missing := make(map[int]bool)
	deleted := make(map[int]bool)

	var doc *document.Document
	lastID := -1

	w := mustNewIndexWriter(t, dir, iwc)
	for ord := range values {
		var id int
		if ids == nil {
			id = ord
		} else {
			id = ids[ord]
		}
		if id != lastID {
			if random().Intn(100) < missingPct {
				missing[id] = true
			}

			if doc != nil {
				mustAddDocument(t, w, doc)
				if random().Intn(100) < deletedPct {
					idToDelete := random().Intn(id)
					if _, err := w.DeleteDocuments([]index.Term{*index.NewTerm("id", strconv.Itoa(idToDelete))}); err != nil {
						t.Fatalf("deleteDocuments: %v", err)
					}
					deleted[idToDelete] = true
				}
			}

			doc = document.NewDocument()
			doc.Add(newStringField(t, "id", strconv.Itoa(id), false))
			doc.Add(mustNumericDocValuesField(t, "id", int64(id)))
			lastID = id
		}

		if !missing[id] {
			doc.Add(document.NewLongPoint("sn_value", values[id]))
			b := make([]byte, 8)
			util.LongToSortableBytes(values[id], b, 0)
			doc.Add(document.NewBinaryPoint("ss_value", b))
		}
	}

	mustAddDocument(t, w, doc)

	if random().Intn(2) == 0 {
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}
	r := mustOpenDirectoryReaderFromWriter(t, w)
	mustClose(t, w)

	s := newSearcherMaybeWrap(t, r, false)

	numThreads := nextInt(2, 5)

	iters := atLeast(100)

	startingGun := make(chan struct{})
	var failed atomic.Bool
	var wg sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		name := "T" + strconv.Itoa(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := func() error {
				<-startingGun

				for iter := 0; iter < iters && !failed.Load(); iter++ {
					lower := pointQueriesRandomValue()
					upper := pointQueriesRandomValue()

					if upper < lower {
						x := lower
						lower = upper
						upper = x
					}

					var query search.Query
					var err error

					if random().Intn(2) == 0 {
						query, err = longPointNewRangeQueryE("sn_value", lower, upper)
					} else {
						lowerBytes := make([]byte, 8)
						util.LongToSortableBytes(lower, lowerBytes, 0)
						upperBytes := make([]byte, 8)
						util.LongToSortableBytes(upper, upperBytes, 0)
						query, err = binaryPointNewRangeQueryE("ss_value", lowerBytes, upperBytes)
					}
					if err != nil {
						return err
					}

					hits, err := search.SearchWithCollectorManager(s, query, testsearch.FixedBitSetCollectorCreateManager(r.MaxDoc()))
					if err != nil {
						return err
					}

					docIDToID, err := index.MultiDocValuesGetNumericValues(r, "id")
					if err != nil {
						return err
					}

					for docID := 0; docID < r.MaxDoc(); docID++ {
						next, err := docIDToID.NextDoc()
						if err != nil {
							return err
						}
						if next != docID {
							return fmt.Errorf("nextDoc = %d, want %d", next, docID)
						}
						idValue, err := docIDToID.LongValue()
						if err != nil {
							return err
						}
						id := int(idValue)
						expected := !missing[id] && !deleted[id] && values[id] >= lower && values[id] <= upper
						if hits.Get(docID) != expected {
							// We do exact quantized comparison so the bbox query should
							// never disagree:
							return fmt.Errorf("%s: iter=%d id=%d docID=%d value=%d (range: %d TO %d) expected %v but got: %v deleted?=%v query=%v",
								name, iter, id, docID, values[id], lower, upper, expected, hits.Get(docID), deleted[id], query)
						}
					}
				}
				return nil
			}(); err != nil {
				failed.Store(true)
				t.Error(err)
			}
		}()
	}
	close(startingGun)
	wg.Wait()
	mustClose(t, r, dir)
}

func TestPointQueriesRandomBinaryTiny(t *testing.T) {
	pointQueriesBeforeClass()
	pointQueriesDoTestRandomBinary(t, 10)
}

func TestPointQueriesRandomBinaryMedium(t *testing.T) {
	pointQueriesBeforeClass()
	pointQueriesDoTestRandomBinary(t, 1000)
}

func pointQueriesDoTestRandomBinary(t *testing.T, count int) {
	numValues := nextInt(count, count*2)
	numBytesPerDim := nextInt(2, index.PointValuesMaxNumBytes)
	numDims := nextInt(1, index.PointValuesMaxIndexDimensions)

	sameValuePct := random().Intn(100)

	docValues := make([][][]byte, numValues)

	singleValued := random().Intn(2) == 0
	ids := make([]int, numValues)

	id := 0
	for ord := 0; ord < numValues; ord++ {
		if ord > 0 && random().Intn(100) < sameValuePct {
			// Identical to old value
			docValues[ord] = docValues[random().Intn(ord)]
		} else {
			// Make a new random value
			values := make([][]byte, numDims)
			for dim := 0; dim < numDims; dim++ {
				values[dim] = make([]byte, numBytesPerDim)
				random().Read(values[dim])
			}
			docValues[ord] = values
		}
		ids[ord] = id
		if singleValued || random().Intn(2) == 1 {
			id++
		}
	}

	pointQueriesVerifyBinary(t, docValues, ids, numBytesPerDim)
}

// pointQueriesVerifyBinary verifies for byte[][] values.
func pointQueriesVerifyBinary(t *testing.T, docValues [][][]byte, ids []int, numBytesPerDim int) {
	iwc := newIndexWriterConfig()

	numDims := len(docValues[0])
	bytesPerDim := len(docValues[0][0])

	// Else we can get O(N^2) merging:
	mbd := iwc.GetMaxBufferedDocs()
	if mbd != -1 && mbd < len(docValues)/100 {
		iwc.SetMaxBufferedDocs(len(docValues) / 100)
	}
	iwc.SetCodec(pointQueriesGetCodec(t))

	var dir store.Directory
	if len(docValues) > 100000 {
		// dir = newFSDirectory(createTempDir("TestPointQueries"));
		t.Fatal(newFSDirectoryBlocker)
	} else {
		dir = newDirectory()
	}

	w := mustNewIndexWriter(t, dir, iwc)

	numValues := len(docValues)

	missingPct := random().Intn(100)
	deletedPct := random().Intn(100)

	missing := make(map[int]bool)
	deleted := make(map[int]bool)

	var doc *document.Document
	lastID := -1

	for ord := 0; ord < numValues; ord++ {
		id := ids[ord]
		if id != lastID {
			if random().Intn(100) < missingPct {
				missing[id] = true
			}

			if doc != nil {
				mustAddDocument(t, w, doc)
				if random().Intn(100) < deletedPct {
					idToDelete := random().Intn(id)
					if _, err := w.DeleteDocuments([]index.Term{*index.NewTerm("id", strconv.Itoa(idToDelete))}); err != nil {
						t.Fatalf("deleteDocuments: %v", err)
					}
					deleted[idToDelete] = true
				}
			}

			doc = document.NewDocument()
			doc.Add(newStringField(t, "id", strconv.Itoa(id), false))
			doc.Add(mustNumericDocValuesField(t, "id", int64(id)))
			lastID = id
		}

		if !missing[id] {
			doc.Add(document.NewBinaryPoint("value", docValues[ord]...))
			doc.Add(mustSortedNumericDocValuesField(t, "value", 1))
		}
	}

	mustAddDocument(t, w, doc)

	if random().Intn(2) == 0 {
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}
	r := mustOpenDirectoryReaderFromWriter(t, w)
	mustClose(t, w)

	s := newSearcherMaybeWrap(t, r, false)

	numThreads := nextInt(2, 5)

	iters := atLeast(100)

	startingGun := make(chan struct{})
	var failed atomic.Bool
	var wg sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := func() error {
				<-startingGun

				for iter := 0; iter < iters && !failed.Load(); iter++ {

					lower := make([][]byte, numDims)
					upper := make([][]byte, numDims)
					for dim := 0; dim < numDims; dim++ {
						lower[dim] = make([]byte, bytesPerDim)
						random().Read(lower[dim])

						upper[dim] = make([]byte, bytesPerDim)
						random().Read(upper[dim])

						if bytes.Compare(lower[dim][:bytesPerDim], upper[dim][:bytesPerDim]) > 0 {
							x := lower[dim]
							lower[dim] = upper[dim]
							upper[dim] = x
						}
					}

					query, err := binaryPointNewRangeQueryDimsE("value", lower, upper)
					if err != nil {
						return err
					}

					hits, err := search.SearchWithCollectorManager(s, query, testsearch.FixedBitSetCollectorCreateManager(r.MaxDoc()))
					if err != nil {
						return err
					}

					expected := make(map[int]bool)
					for ord := 0; ord < numValues; ord++ {
						id := ids[ord]
						if !missing[id] && !deleted[id] && pointQueriesMatches(bytesPerDim, lower, upper, docValues[ord]) {
							expected[id] = true
						}
					}

					docIDToID, err := index.MultiDocValuesGetNumericValues(r, "id")
					if err != nil {
						return err
					}

					failCount := 0
					for docID := 0; docID < r.MaxDoc(); docID++ {
						next, err := docIDToID.NextDoc()
						if err != nil {
							return err
						}
						if next != docID {
							return fmt.Errorf("nextDoc = %d, want %d", next, docID)
						}
						idValue, err := docIDToID.LongValue()
						if err != nil {
							return err
						}
						id := int(idValue)
						if hits.Get(docID) != expected[id] {
							fmt.Printf("FAIL: iter=%d id=%d docID=%d expected=%v but got %v deleted?=%v missing?=%v\n",
								iter, id, docID, expected[id], hits.Get(docID), deleted[id], missing[id])
							for dim := 0; dim < numDims; dim++ {
								fmt.Printf("  dim=%d range: %s TO %s\n", dim, pointQueriesBytesToString(lower[dim]), pointQueriesBytesToString(upper[dim]))
								failCount++
							}
						}
					}
					if failCount != 0 {
						return fmt.Errorf("%d hits were wrong", failCount)
					}
				}
				return nil
			}(); err != nil {
				failed.Store(true)
				t.Error(err)
			}
		}()
	}

	close(startingGun)
	wg.Wait()

	mustClose(t, r, dir)
}

// pointQueriesBytesToString renders TestPointQueries.bytesToString(byte[]).
func pointQueriesBytesToString(b []byte) string {
	if b == nil {
		return "null"
	}
	return newBytesRef(b).String()
}

// pointQueriesMatches renders TestPointQueries.matches(int, byte[][], byte[][], byte[][]).
func pointQueriesMatches(bytesPerDim int, lower, upper, value [][]byte) bool {
	numDims := len(lower)
	for dim := 0; dim < numDims; dim++ {

		if bytes.Compare(value[dim][:bytesPerDim], lower[dim][:bytesPerDim]) < 0 {
			// Value is below the lower bound, on this dim
			return false
		}

		if bytes.Compare(value[dim][:bytesPerDim], upper[dim][:bytesPerDim]) > 0 {
			// Value is above the upper bound, on this dim
			return false
		}
	}

	return true
}

// pointQueriesRandomValue renders TestPointQueries.randomValue().
func pointQueriesRandomValue() int64 {
	if pointQueriesValueRange == 0 {
		return randomLong()
	}
	return pointQueriesValueMid + int64(nextInt(-pointQueriesValueRange, pointQueriesValueRange))
}

func TestPointQueriesMinMaxLong(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(document.NewLongPoint("value", math.MinInt64))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewLongPoint("value", math.MaxInt64))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)

	s := newSearcherMaybeWrap(t, r, false)

	assertCount(t, s, longPointNewRangeQuery(t, "value", math.MinInt64, 0), 1)
	assertCount(t, s, longPointNewRangeQuery(t, "value", 0, math.MaxInt64), 1)
	assertCount(t, s, longPointNewRangeQuery(t, "value", math.MinInt64, math.MaxInt64), 2)

	mustClose(t, r, w, dir)
}

// toUTF8 renders TestPointQueries.toUTF8(String).
func toUTF8(s string) []byte {
	return []byte(s)
}

// toUTF8Padded renders TestPointQueries.toUTF8(String, int): right zero pads.
func toUTF8Padded(s string, length int) []byte {
	b := []byte(s)
	if length < len(b) {
		panic(fmt.Sprintf("length=%d but string's UTF8 bytes has length=%d", length, len(b)))
	}
	result := make([]byte, length)
	copy(result, b)
	return result
}

func TestPointQueriesBasicSortedSet(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(document.NewBinaryPoint("value", toUTF8("abc")))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(document.NewBinaryPoint("value", toUTF8("def")))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)

	s := newSearcherMaybeWrap(t, r, false)

	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8("aaa"), toUTF8("bbb")), 1)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8Padded("c", 3), toUTF8Padded("e", 3)), 1)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8Padded("a", 3), toUTF8Padded("z", 3)), 2)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8Padded("", 3), toUTF8("abc")), 1)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8Padded("a", 3), toUTF8("abc")), 1)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8Padded("a", 3), toUTF8("abb")), 0)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8("def"), toUTF8("zzz")), 1)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8("def"), toUTF8Padded("z", 3)), 1)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8("deg"), toUTF8Padded("z", 3)), 0)

	mustClose(t, r, w, dir)
}

func TestPointQueriesLongMinMaxNumeric(t *testing.T) {
	pointQueriesBeforeClass()
	pointQueriesLongMinMax(t)
}

func TestPointQueriesLongMinMaxSortedSet(t *testing.T) {
	pointQueriesBeforeClass()
	pointQueriesLongMinMax(t)
}

// pointQueriesLongMinMax is the body testLongMinMaxNumeric and
// testLongMinMaxSortedSet share verbatim.
func pointQueriesLongMinMax(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(document.NewLongPoint("value", math.MinInt64))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(document.NewLongPoint("value", math.MaxInt64))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)

	s := newSearcherMaybeWrap(t, r, false)

	assertCount(t, s, longPointNewRangeQuery(t, "value", math.MinInt64, math.MaxInt64), 2)
	assertCount(t, s, longPointNewRangeQuery(t, "value", math.MinInt64, math.MaxInt64-1), 1)
	assertCount(t, s, longPointNewRangeQuery(t, "value", math.MinInt64+1, math.MaxInt64), 1)
	assertCount(t, s, longPointNewRangeQuery(t, "value", math.MinInt64+1, math.MaxInt64-1), 0)

	mustClose(t, r, w, dir)
}

func TestPointQueriesSortedSetNoOrdsMatch(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(document.NewBinaryPoint("value", toUTF8("a")))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(document.NewBinaryPoint("value", toUTF8("z")))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)

	s := newSearcherMaybeWrap(t, r, false)
	assertCount(t, s, binaryPointNewRangeQuery(t, "value", toUTF8("m"), toUTF8("m")), 0)

	mustClose(t, r, w, dir)
}

func TestPointQueriesNumericNoValuesMatch(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(mustSortedNumericDocValuesField(t, "value", 17))
	mustAddDocument(t, w, doc)
	doc = document.NewDocument()
	doc.Add(mustSortedNumericDocValuesField(t, "value", 22))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)

	s := search.NewIndexSearcher(r)
	assertCount(t, s, longPointNewRangeQuery(t, "value", 17, 13), 0)

	mustClose(t, r, w, dir)
}

func TestPointQueriesNoDocs(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	mustAddDocument(t, w, document.NewDocument())

	r := mustGetReader(t, w)

	s := newSearcherMaybeWrap(t, r, false)
	assertCount(t, s, longPointNewRangeQuery(t, "value", 17, 13), 0)

	mustClose(t, r, w, dir)
}

func TestPointQueriesWrongNumDims(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(document.NewLongPoint("value", math.MinInt64))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)

	// no wrapping, else the exc might happen in executor thread:
	s := search.NewIndexSearcher(r)
	point := make([][]byte, 2)
	point[0] = make([]byte, 8)
	point[1] = make([]byte, 8)
	_, err := s.Count(binaryPointNewRangeQueryDims(t, "value", point, point))
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if got, want := err.Error(), `field="value" was indexed with numIndexDimensions=1 but this query has numDims=2`; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	mustClose(t, r, w, dir)
}

func TestPointQueriesWrongNumBytes(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(document.NewLongPoint("value", math.MinInt64))
	mustAddDocument(t, w, doc)

	r := mustGetReader(t, w)

	// no wrapping, else the exc might happen in executor thread:
	s := search.NewIndexSearcher(r)
	point := make([][]byte, 1)
	point[0] = make([]byte, 10)
	_, err := s.Count(binaryPointNewRangeQueryDims(t, "value", point, point))
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if got, want := err.Error(), `field="value" was indexed with bytesPerDim=8 but this query has bytesPerDim=10`; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}

	mustClose(t, r, w, dir)
}

func TestPointQueriesAllPointDocsWereDeletedAndThenMergedAgain(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "0", false))
	doc.Add(document.NewLongPoint("value", 0))
	mustAddDocument(t, w, doc)

	// Add document that won't be deleted to avoid IW dropping segment below
	// since it's 100% deleted:
	mustAddDocument(t, w, document.NewDocument())
	mustCommit(t, w)

	// Need another segment so we invoke BKDWriter.merge
	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "0", false))
	doc.Add(document.NewLongPoint("value", 0))
	mustAddDocument(t, w, doc)
	mustAddDocument(t, w, document.NewDocument())

	if _, err := w.DeleteDocuments([]index.Term{*index.NewTerm("id", "0")}); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	doc = document.NewDocument()
	doc.Add(newStringField(t, "id", "0", false))
	doc.Add(document.NewLongPoint("value", 0))
	mustAddDocument(t, w, doc)
	mustAddDocument(t, w, document.NewDocument())

	if _, err := w.DeleteDocuments([]index.Term{*index.NewTerm("id", "0")}); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	mustClose(t, w, dir)
}

// pointQueriesGetCodec renders TestPointQueries.getCodec().
func pointQueriesGetCodec(t *testing.T) spi.Codec {
	t.Helper()
	if index.GetDefaultCodec().Name() == "Lucene84" {
		// return new FilterCodec("Lucene84", Codec.getDefault()) { pointsFormat()
		// -> Lucene90PointsWriter(writeState, maxPointsInLeafNode, maxMBSortInHeap) };
		t.Fatal("requires org.apache.lucene.codecs.FilterCodec with an anonymous PointsFormat over " +
			"Lucene90PointsWriter(SegmentWriteState, int, double) (not ported)")
		return nil
	}
	return index.GetDefaultCodec()
}

func TestPointQueriesExactPoints(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	doc := document.NewDocument()
	doc.Add(document.NewLongPoint("long", 5))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewIntPoint("int", 42))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewFloatPoint("float", 2.0))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(document.NewDoublePoint("double", 1.0))
	mustAddDocument(t, w, doc)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	assertCount(t, s, intPointNewExactQuery(t, "int", 42), 1)
	assertCount(t, s, intPointNewExactQuery(t, "int", 41), 0)

	assertCount(t, s, longPointNewExactQuery(t, "long", 5), 1)
	assertCount(t, s, longPointNewExactQuery(t, "long", -1), 0)

	assertCount(t, s, floatPointNewExactQuery(t, "float", 2.0), 1)
	assertCount(t, s, floatPointNewExactQuery(t, "float", 1.0), 0)

	assertCount(t, s, doublePointNewExactQuery(t, "double", 1.0), 1)
	assertCount(t, s, doublePointNewExactQuery(t, "double", 2.0), 0)
	mustClose(t, w, r, dir)
}

// assertQueryString renders assertEquals(expected, query.toString()).
func assertQueryString(t *testing.T, expected string, q search.Query) {
	t.Helper()
	var got string
	switch v := q.(type) {
	case interface{ ToString(string) string }:
		got = v.ToString("")
	case fmt.Stringer:
		got = v.String()
	default:
		t.Fatalf("%T has no toString()", q)
	}
	if got != expected {
		t.Fatalf("toString = %q, want %q", got, expected)
	}
}

func TestPointQueriesToString(t *testing.T) {
	pointQueriesBeforeClass()

	// ints
	assertQueryString(t, "field:[1 TO 2]", intPointNewRangeQuery(t, "field", 1, 2))
	assertQueryString(t, "field:[-2 TO 1]", intPointNewRangeQuery(t, "field", -2, 1))

	// longs
	assertQueryString(t, "field:[1099511627776 TO 2199023255552]", longPointNewRangeQuery(t, "field", 1<<40, 1<<41))
	assertQueryString(t, "field:[-5 TO 6]", longPointNewRangeQuery(t, "field", -5, 6))

	// floats
	assertQueryString(t, "field:[1.3 TO 2.5]", floatPointNewRangeQuery(t, "field", 1.3, 2.5))
	assertQueryString(t, "field:[-2.9 TO 1.0]", floatPointNewRangeQuery(t, "field", -2.9, 1.0))

	// doubles
	assertQueryString(t, "field:[1.3 TO 2.5]", doublePointNewRangeQuery(t, "field", 1.3, 2.5))
	assertQueryString(t, "field:[-2.9 TO 1.0]", doublePointNewRangeQuery(t, "field", -2.9, 1.0))

	// n-dimensional double
	assertQueryString(t, "field:[1.3 TO 2.5],[-2.9 TO 1.0]",
		doublePointNewRangeQueryDims(t, "field", []float64{1.3, -2.9}, []float64{2.5, 1.0}))
}

// pointQueriesToArray renders TestPointQueries.toArray(Set<Integer>).
func pointQueriesToArray(valuesSet map[int32]struct{}) []int32 {
	values := make([]int32, 0, len(valuesSet))
	for value := range valuesSet {
		values = append(values, value)
	}
	return values
}

// pointQueriesRandomIntValue renders TestPointQueries.randomIntValue(Integer,
// Integer); a nil min selects random().nextInt().
func pointQueriesRandomIntValue(min, max *int32) int32 {
	if min == nil {
		return int32(random().Uint32())
	}
	return int32(nextInt(int(*min), int(*max)))
}

func TestPointQueriesRandomPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()

	useNarrowRange := random().Intn(2) == 0
	var valueMin, valueMax *int32
	var numValues int
	if useNarrowRange {
		gap := int32(random().Intn(100))
		minV := int32(random().Intn(int(math.MaxInt32 - gap)))
		maxV := minV + gap
		valueMin, valueMax = &minV, &maxV
		numValues = nextInt(1, int(gap)+1)
	} else {
		numValues = nextInt(1, 100)
	}
	valuesSet := make(map[int32]struct{})
	for len(valuesSet) < numValues {
		valuesSet[pointQueriesRandomIntValue(valueMin, valueMax)] = struct{}{}
	}
	values := pointQueriesToArray(valuesSet)
	numDocs := nextInt(1, 10000)

	var dir store.Directory
	if numDocs > 100000 {
		// dir = newFSDirectory(createTempDir("TestPointQueries"));
		t.Fatal(newFSDirectoryBlocker)
	} else {
		dir = newDirectory()
	}

	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	docValues := make([]int32, numDocs)
	for i := 0; i < numDocs; i++ {
		x := values[random().Intn(len(values))]
		doc := document.NewDocument()
		doc.Add(document.NewIntPoint("int", x))
		docValues[i] = x
		mustAddDocument(t, w, doc)
	}

	if random().Intn(2) == 0 {
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}
	r := mustGetReader(t, w)
	mustClose(t, w)

	s := newSearcherMaybeWrap(t, r, false)

	numThreads := nextInt(2, 5)

	iters := atLeast(100)

	startingGun := make(chan struct{})
	var failed atomic.Bool
	var wg sync.WaitGroup

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := func() error {
				<-startingGun

				for iter := 0; iter < iters && !failed.Load(); iter++ {

					numValidValuesToQuery := random().Intn(len(values))

					valuesToQuery := make(map[int32]struct{})
					for len(valuesToQuery) < numValidValuesToQuery {
						valuesToQuery[values[random().Intn(len(values))]] = struct{}{}
					}

					numExtraValuesToQuery := random().Intn(20)
					for len(valuesToQuery) < numValidValuesToQuery+numExtraValuesToQuery {
						valuesToQuery[int32(random().Uint32())] = struct{}{}
					}

					expectedCount := 0
					for _, value := range docValues {
						if _, ok := valuesToQuery[value]; ok {
							expectedCount++
						}
					}

					query, err := intPointNewSetQueryE("int", pointQueriesToArray(valuesToQuery)...)
					if err != nil {
						return err
					}
					count, err := s.Count(query)
					if err != nil {
						return err
					}
					if count != expectedCount {
						return fmt.Errorf("count = %d, want %d", count, expectedCount)
					}
				}
				return nil
			}(); err != nil {
				failed.Store(true)
				t.Error(err)
			}
		}()
	}
	close(startingGun)
	wg.Wait()
	mustClose(t, r, dir)
}

// newMultiDimIntSetQuery renders the test-only query factory
// TestPointQueries.newMultiDimIntSetQuery(String, int, int...).
func newMultiDimIntSetQuery(field string, numDims int, valuesIn ...int32) (search.Query, error) {
	if len(valuesIn)%numDims != 0 {
		return nil, fmt.Errorf("incongruent number of values: valuesIn.length=%d but numDims=%d", len(valuesIn), numDims)
	}

	// Pack all values:
	packedValues := make([][]byte, len(valuesIn)/numDims)
	for i := range packedValues {
		packedValue := make([]byte, numDims*4)
		packedValues[i] = packedValue
		for dim := 0; dim < numDims; dim++ {
			document.EncodeDimensionIntLucene(valuesIn[i*numDims+dim], packedValue, dim*4)
		}
	}

	// Sort:
	slices.SortFunc(packedValues, func(a, b []byte) int {
		return bytes.Compare(a[:len(a)], b[:len(a)])
	})

	value := &util.BytesRef{}
	value.Length = numDims * 4

	upto := 0
	return search.NewPointInSetQuery(field, numDims, 4, util.BytesRefIteratorFunc(func() (*util.BytesRef, error) {
		if upto >= len(packedValues) {
			return nil, nil
		}
		value.Bytes = packedValues[upto]
		upto++
		return value, nil
	}), func(value []byte) string {
		if util.AssertsEnabled() && len(value) != numDims*4 {
			panic(util.NewAssertionError("value.length == numDims * Integer.BYTES"))
		}
		var sb strings.Builder
		for dim := 0; dim < numDims; dim++ {
			if dim > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(strconv.Itoa(int(document.DecodeDimensionIntLucene(value, dim*4))))
		}

		return sb.String()
	})
}

func mustNewMultiDimIntSetQuery(t *testing.T, field string, numDims int, valuesIn ...int32) search.Query {
	t.Helper()
	q, err := newMultiDimIntSetQuery(field, numDims, valuesIn...)
	if err != nil {
		t.Fatalf("newMultiDimIntSetQuery: %v", err)
	}
	return q
}

func TestPointQueriesBasicMultiDimPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	doc := document.NewDocument()
	doc.Add(document.NewIntPoint("int", 17, 42))
	mustAddDocument(t, w, doc)
	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 17, 41), 0)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 17, 42), 1)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, -7, -7, 17, 42), 1)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 17, 42, -14, -14), 1)

	mustClose(t, w, r, dir)
}

func TestPointQueriesBasicMultiValueMultiDimPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	doc := document.NewDocument()
	doc.Add(document.NewIntPoint("int", 17, 42))
	doc.Add(document.NewIntPoint("int", 34, 79))
	mustAddDocument(t, w, doc)
	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 17, 41), 0)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 17, 42), 1)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 17, 42, 34, 79), 1)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, -7, -7, 17, 42), 1)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, -7, -7, 34, 79), 1)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 17, 42, -14, -14), 1)

	assertQueryString(t, "int:{-14,-14 17,42}", mustNewMultiDimIntSetQuery(t, "int", 2, 17, 42, -14, -14))

	mustClose(t, w, r, dir)
}

func TestPointQueriesManyEqualValuesMultiDimPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	zeroCount := 0
	for i := 0; i < 10000; i++ {
		x := int32(random().Intn(2))
		if x == 0 {
			zeroCount++
		}
		doc := document.NewDocument()
		doc.Add(document.NewIntPoint("int", x, x))
		mustAddDocument(t, w, doc)
	}
	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 0, 0), zeroCount)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 1, 1), 10000-zeroCount)
	assertCount(t, s, mustNewMultiDimIntSetQuery(t, "int", 2, 2, 2), 0)

	mustClose(t, w, r, dir)
}

func TestPointQueriesInvalidMultiDimPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	_, err := newMultiDimIntSetQuery("int", 2, 3, 4, 5)
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if got, want := err.Error(), "incongruent number of values: valuesIn.length=3 but numDims=2"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

// addPointsOfEveryType adds the int/long/float/double/bytes points of value x,
// the fixture shared by the point-in-set tests.
func addPointsOfEveryType(doc *document.Document, x int32) {
	doc.Add(document.NewIntPoint("int", x))
	doc.Add(document.NewLongPoint("long", int64(x)))
	doc.Add(document.NewFloatPoint("float", float32(x)))
	doc.Add(document.NewDoublePoint("double", float64(x)))
	doc.Add(document.NewBinaryPoint("bytes", []byte{0, byte(x)}))
}

func TestPointQueriesBasicPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	for _, x := range []int32{17, 42, 97} {
		doc := document.NewDocument()
		addPointsOfEveryType(doc, x)
		mustAddDocument(t, w, doc)
	}

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	assertCount(t, s, intPointNewSetQuery(t, "int", 16), 0)
	assertCount(t, s, intPointNewSetQuery(t, "int", 17), 1)
	assertCount(t, s, intPointNewSetQuery(t, "int", 17, 97, 42), 3)
	assertCount(t, s, intPointNewSetQuery(t, "int", -7, 17, 42, 97), 3)
	assertCount(t, s, intPointNewSetQuery(t, "int", 17, 20, 42, 97), 3)
	assertCount(t, s, intPointNewSetQuery(t, "int", 17, 105, 42, 97), 3)

	assertCount(t, s, longPointNewSetQuery(t, "long", 16), 0)
	assertCount(t, s, longPointNewSetQuery(t, "long", 17), 1)
	assertCount(t, s, longPointNewSetQuery(t, "long", 17, 97, 42), 3)
	assertCount(t, s, longPointNewSetQuery(t, "long", -7, 17, 42, 97), 3)
	assertCount(t, s, longPointNewSetQuery(t, "long", 17, 20, 42, 97), 3)
	assertCount(t, s, longPointNewSetQuery(t, "long", 17, 105, 42, 97), 3)

	assertCount(t, s, floatPointNewSetQuery(t, "float", 16), 0)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 17), 1)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 17, 97, 42), 3)
	assertCount(t, s, floatPointNewSetQuery(t, "float", -7, 17, 42, 97), 3)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 17, 20, 42, 97), 3)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 17, 105, 42, 97), 3)

	assertCount(t, s, doublePointNewSetQuery(t, "double", 16), 0)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 17), 1)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 17, 97, 42), 3)
	assertCount(t, s, doublePointNewSetQuery(t, "double", -7, 17, 42, 97), 3)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 17, 20, 42, 97), 3)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 17, 105, 42, 97), 3)

	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 16}), 0)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 17}), 1)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 17}, []byte{0, 97}, []byte{0, 42}), 3)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 0xf9}, []byte{0, 17}, []byte{0, 42}, []byte{0, 97}), 3)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 17}, []byte{0, 20}, []byte{0, 42}, []byte{0, 97}), 3)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 17}, []byte{0, 105}, []byte{0, 42}, []byte{0, 97}), 3)

	mustClose(t, w, r, dir)
}

// TestPointQueriesPointIntSetBoxed: boxed methods for primitive types should
// behave the same as unboxed: just sugar.
func TestPointQueriesPointIntSetBoxed(t *testing.T) {
	pointQueriesBeforeClass()
	// assertEquals(IntPoint.newSetQuery("foo", 1, 2, 3),
	//     IntPoint.newSetQuery("foo", Arrays.asList(1, 2, 3))); and likewise
	// for FloatPoint, LongPoint and DoublePoint.
	for _, pair := range [][2]search.Query{
		{intPointNewSetQuery(t, "foo", 1, 2, 3), intPointNewSetQuery(t, "foo", []int32{1, 2, 3}...)},
		{floatPointNewSetQuery(t, "foo", 1, 2, 3), floatPointNewSetQuery(t, "foo", []float32{1, 2, 3}...)},
		{longPointNewSetQuery(t, "foo", 1, 2, 3), longPointNewSetQuery(t, "foo", []int64{1, 2, 3}...)},
		{doublePointNewSetQuery(t, "foo", 1, 2, 3), doublePointNewSetQuery(t, "foo", []float64{1, 2, 3}...)},
	} {
		if !pair[0].Equals(pair[1]) {
			t.Fatalf("%v != %v", pair[0], pair[1])
		}
	}
}

func TestPointQueriesBasicMultiValuedPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	doc := document.NewDocument()
	doc.Add(document.NewIntPoint("int", 17))
	doc.Add(document.NewIntPoint("int", 42))
	doc.Add(document.NewLongPoint("long", 17))
	doc.Add(document.NewLongPoint("long", 42))
	doc.Add(document.NewFloatPoint("float", 17.0))
	doc.Add(document.NewFloatPoint("float", 42.0))
	doc.Add(document.NewDoublePoint("double", 17.0))
	doc.Add(document.NewDoublePoint("double", 42.0))
	doc.Add(document.NewBinaryPoint("bytes", []byte{0, 17}))
	doc.Add(document.NewBinaryPoint("bytes", []byte{0, 42}))
	mustAddDocument(t, w, doc)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	assertCount(t, s, intPointNewSetQuery(t, "int", 16), 0)
	assertCount(t, s, intPointNewSetQuery(t, "int", 17), 1)
	assertCount(t, s, intPointNewSetQuery(t, "int", 17, 97, 42), 1)
	assertCount(t, s, intPointNewSetQuery(t, "int", -7, 17, 42, 97), 1)
	assertCount(t, s, intPointNewSetQuery(t, "int", 16, 20, 41, 97), 0)

	assertCount(t, s, longPointNewSetQuery(t, "long", 16), 0)
	assertCount(t, s, longPointNewSetQuery(t, "long", 17), 1)
	assertCount(t, s, longPointNewSetQuery(t, "long", 17, 97, 42), 1)
	assertCount(t, s, longPointNewSetQuery(t, "long", -7, 17, 42, 97), 1)
	assertCount(t, s, longPointNewSetQuery(t, "long", 16, 20, 41, 97), 0)

	assertCount(t, s, floatPointNewSetQuery(t, "float", 16), 0)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 17), 1)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 17, 97, 42), 1)
	assertCount(t, s, floatPointNewSetQuery(t, "float", -7, 17, 42, 97), 1)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 16, 20, 41, 97), 0)

	assertCount(t, s, doublePointNewSetQuery(t, "double", 16), 0)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 17), 1)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 17, 97, 42), 1)
	assertCount(t, s, doublePointNewSetQuery(t, "double", -7, 17, 42, 97), 1)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 16, 20, 41, 97), 0)

	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 16}), 0)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 17}), 1)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 17}, []byte{0, 97}, []byte{0, 42}), 1)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 0xf9}, []byte{0, 17}, []byte{0, 42}, []byte{0, 97}), 1)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0, 16}, []byte{0, 20}, []byte{0, 41}, []byte{0, 97}), 0)

	mustClose(t, w, r, dir)
}

func TestPointQueriesEmptyPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	doc := document.NewDocument()
	addPointsOfEveryType(doc, 17)
	mustAddDocument(t, w, doc)

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	assertCount(t, s, intPointNewSetQuery(t, "int"), 0)
	assertCount(t, s, longPointNewSetQuery(t, "long"), 0)
	assertCount(t, s, floatPointNewSetQuery(t, "float"), 0)
	assertCount(t, s, doublePointNewSetQuery(t, "double"), 0)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes"), 0)

	mustClose(t, w, r, dir)
}

// addOneBytePointsOfEveryType adds the int/long/float/double points of x and
// the one-byte BinaryPoint (byte) x, the fixture of the many-equal-values
// tests.
func addOneBytePointsOfEveryType(doc *document.Document, x int32) {
	doc.Add(document.NewIntPoint("int", x))
	doc.Add(document.NewLongPoint("long", int64(x)))
	doc.Add(document.NewFloatPoint("float", float32(x)))
	doc.Add(document.NewDoublePoint("double", float64(x)))
	doc.Add(document.NewBinaryPoint("bytes", []byte{byte(x)}))
}

func TestPointQueriesPointInSetQueryManyEqualValues(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	zeroCount := 0
	for i := 0; i < 10000; i++ {
		x := int32(random().Intn(2))
		if x == 0 {
			zeroCount++
		}
		doc := document.NewDocument()
		addOneBytePointsOfEveryType(doc, x)
		mustAddDocument(t, w, doc)
	}

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	assertCount(t, s, intPointNewSetQuery(t, "int", 0), zeroCount)
	assertCount(t, s, intPointNewSetQuery(t, "int", 0, -7), zeroCount)
	assertCount(t, s, intPointNewSetQuery(t, "int", 7, 0), zeroCount)
	assertCount(t, s, intPointNewSetQuery(t, "int", 1), 10000-zeroCount)
	assertCount(t, s, intPointNewSetQuery(t, "int", 2), 0)

	assertCount(t, s, longPointNewSetQuery(t, "long", 0), zeroCount)
	assertCount(t, s, longPointNewSetQuery(t, "long", 0, -7), zeroCount)
	assertCount(t, s, longPointNewSetQuery(t, "long", 7, 0), zeroCount)
	assertCount(t, s, longPointNewSetQuery(t, "long", 1), 10000-zeroCount)
	assertCount(t, s, longPointNewSetQuery(t, "long", 2), 0)

	assertCount(t, s, floatPointNewSetQuery(t, "float", 0), zeroCount)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 0, -7), zeroCount)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 7, 0), zeroCount)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 1), 10000-zeroCount)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 2), 0)

	assertCount(t, s, doublePointNewSetQuery(t, "double", 0), zeroCount)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 0, -7), zeroCount)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 7, 0), zeroCount)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 1), 10000-zeroCount)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 2), 0)

	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0}), zeroCount)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0}, []byte{0xf9}), zeroCount)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{7}, []byte{0}), zeroCount)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{1}), 10000-zeroCount)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{2}), 0)

	mustClose(t, w, r, dir)
}

func TestPointQueriesPointRangeQueryManyEqualValues(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	cardinality := nextInt(2, 20)

	zeroCount := 0
	oneCount := 0
	for i := 0; i < 10000; i++ {
		x := int32(random().Intn(cardinality))
		if x == 0 {
			zeroCount++
		} else if x == 1 {
			oneCount++
		}
		doc := document.NewDocument()
		addOneBytePointsOfEveryType(doc, x)
		mustAddDocument(t, w, doc)
	}

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)

	c := int32(cardinality)
	assertCount(t, s, intPointNewRangeQuery(t, "int", 0, 0), zeroCount)
	assertCount(t, s, intPointNewRangeQuery(t, "int", 1, 1), oneCount)
	assertCount(t, s, intPointNewRangeQuery(t, "int", 0, 1), zeroCount+oneCount)
	assertCount(t, s, intPointNewRangeQuery(t, "int", 2, c), 10000-zeroCount-oneCount)

	assertCount(t, s, longPointNewRangeQuery(t, "long", 0, 0), zeroCount)
	assertCount(t, s, longPointNewRangeQuery(t, "long", 1, 1), oneCount)
	assertCount(t, s, longPointNewRangeQuery(t, "long", 0, 1), zeroCount+oneCount)
	assertCount(t, s, longPointNewRangeQuery(t, "long", 2, int64(c)), 10000-zeroCount-oneCount)

	assertCount(t, s, floatPointNewRangeQuery(t, "float", 0, 0), zeroCount)
	assertCount(t, s, floatPointNewRangeQuery(t, "float", 1, 1), oneCount)
	assertCount(t, s, floatPointNewRangeQuery(t, "float", 0, 1), zeroCount+oneCount)
	assertCount(t, s, floatPointNewRangeQuery(t, "float", 2, float32(c)), 10000-zeroCount-oneCount)

	assertCount(t, s, doublePointNewRangeQuery(t, "double", 0, 0), zeroCount)
	assertCount(t, s, doublePointNewRangeQuery(t, "double", 1, 1), oneCount)
	assertCount(t, s, doublePointNewRangeQuery(t, "double", 0, 1), zeroCount+oneCount)
	assertCount(t, s, doublePointNewRangeQuery(t, "double", 2, float64(c)), 10000-zeroCount-oneCount)

	assertCount(t, s, binaryPointNewRangeQuery(t, "bytes", []byte{0}, []byte{0}), zeroCount)
	assertCount(t, s, binaryPointNewRangeQuery(t, "bytes", []byte{1}, []byte{1}), oneCount)
	assertCount(t, s, binaryPointNewRangeQuery(t, "bytes", []byte{0}, []byte{1}), zeroCount+oneCount)
	assertCount(t, s, binaryPointNewRangeQuery(t, "bytes", []byte{2}, []byte{byte(cardinality)}), 10000-zeroCount-oneCount)

	mustClose(t, w, r, dir)
}

func TestPointQueriesPointInSetQueryManyEqualValuesWithBigGap(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetCodec(pointQueriesGetCodec(t))
	w := mustNewIndexWriter(t, dir, iwc)

	zeroCount := 0
	for i := 0; i < 10000; i++ {
		x := int32(200 * random().Intn(2))
		if x == 0 {
			zeroCount++
		}
		doc := document.NewDocument()
		addOneBytePointsOfEveryType(doc, x)
		mustAddDocument(t, w, doc)
	}

	r := mustOpenDirectoryReaderFromWriter(t, w)
	s := newSearcherMaybeWrap(t, r, false)
	assertCount(t, s, intPointNewSetQuery(t, "int", 0), zeroCount)
	assertCount(t, s, intPointNewSetQuery(t, "int", 0, -7), zeroCount)
	assertCount(t, s, intPointNewSetQuery(t, "int", 7, 0), zeroCount)
	assertCount(t, s, intPointNewSetQuery(t, "int", 200), 10000-zeroCount)
	assertCount(t, s, intPointNewSetQuery(t, "int", 2), 0)

	assertCount(t, s, longPointNewSetQuery(t, "long", 0), zeroCount)
	assertCount(t, s, longPointNewSetQuery(t, "long", 0, -7), zeroCount)
	assertCount(t, s, longPointNewSetQuery(t, "long", 7, 0), zeroCount)
	assertCount(t, s, longPointNewSetQuery(t, "long", 200), 10000-zeroCount)
	assertCount(t, s, longPointNewSetQuery(t, "long", 2), 0)

	assertCount(t, s, floatPointNewSetQuery(t, "float", 0), zeroCount)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 0, -7), zeroCount)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 7, 0), zeroCount)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 200), 10000-zeroCount)
	assertCount(t, s, floatPointNewSetQuery(t, "float", 2), 0)

	assertCount(t, s, doublePointNewSetQuery(t, "double", 0), zeroCount)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 0, -7), zeroCount)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 7, 0), zeroCount)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 200), 10000-zeroCount)
	assertCount(t, s, doublePointNewSetQuery(t, "double", 2), 0)

	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0}), zeroCount)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{0}, []byte{0xf9}), zeroCount)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{7}, []byte{0}), zeroCount)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{200}), 10000-zeroCount)
	assertCount(t, s, binaryPointNewSetQuery(t, "bytes", []byte{2}), 0)

	mustClose(t, w, r, dir)
}

func TestPointQueriesInvalidPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	_, err := search.NewPointInSetQuery("foo", 3, 4, util.BytesRefIteratorFunc(func() (*util.BytesRef, error) {
		return newBytesRef(make([]byte, 3)), nil
	}), func(point []byte) string {
		return javaBytesToString(point)
	})
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if got, want := err.Error(), `packed point length should be 12 but got 3; field="foo" numDims=3 bytesPerDim=4`; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestPointQueriesInvalidPointInSetBinaryQuery(t *testing.T) {
	pointQueriesBeforeClass()
	// expectThrows(IllegalArgumentException.class,
	//     () -> BinaryPoint.newSetQuery("bytes", new byte[] {2}, new byte[0]));
	// "all byte[] must be the same length, but saw 1 and 0"
	binaryPointNewSetQuery(t, "bytes", []byte{2}, []byte{})
}

func TestPointQueriesPointInSetQueryToString(t *testing.T) {
	pointQueriesBeforeClass()
	// int
	assertQueryString(t, "int:{-42 18}", intPointNewSetQuery(t, "int", -42, 18))

	// long
	assertQueryString(t, "long:{-42 18}", longPointNewSetQuery(t, "long", -42, 18))

	// float
	assertQueryString(t, "float:{-42.0 18.0}", floatPointNewSetQuery(t, "float", -42.0, 18.0))

	// double
	assertQueryString(t, "double:{-42.0 18.0}", doublePointNewSetQuery(t, "double", -42.0, 18.0))

	// binary
	assertQueryString(t, "bytes:{[12] [2a]}", binaryPointNewSetQuery(t, "bytes", []byte{42}, []byte{18}))
}

func TestPointQueriesPointInSetQueryGetPackedPoints(t *testing.T) {
	pointQueriesBeforeClass()
	one, thirtyTwo := int32(1), int32(32)
	numValues := int(pointQueriesRandomIntValue(&one, &thirtyTwo))
	values := make([][]byte, 0, numValues)
	for i := 0; i < numValues; i++ {
		values = append(values, []byte{byte(i)})
	}

	query := binaryPointNewSetQuery(t, "field", values...).(*search.PointInSetQuery)
	packedPoints := query.GetPackedPoints()
	if len(packedPoints) != numValues {
		t.Fatalf("packedPoints.size() = %d, want %d", len(packedPoints), numValues)
	}
	for i, expectedValue := range values {
		if !bytes.Equal(expectedValue, packedPoints[i]) {
			t.Fatalf("packedPoints[%d] = %v, want %v", i, packedPoints[i], expectedValue)
		}
	}
}

func TestPointQueriesRangeOptimizesIfAllPointsMatch(t *testing.T) {
	pointQueriesBeforeClass()
	numDims := nextInt(1, 3)
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	doc := document.NewDocument()
	value := make([]int32, numDims)
	for i := 0; i < numDims; i++ {
		value[i] = int32(nextInt(1, 10))
	}
	doc.Add(document.NewIntPoint("point", value...))
	mustAddDocument(t, w, doc)
	reader := mustGetReader(t, w)
	searcher := search.NewIndexSearcher(reader)
	searcher.SetQueryCache(nil)
	lowerBound := make([]int32, numDims)
	upperBound := make([]int32, numDims)
	for i := 0; i < numDims; i++ {
		lowerBound[i] = value[i] - int32(random().Intn(1))
		upperBound[i] = value[i] + int32(random().Intn(1))
	}
	query := intPointNewRangeQueryDims(t, "point", lowerBound, upperBound)
	weight := mustCreateWeight(t, searcher, query, search.COMPLETE_NO_SCORES, 1)
	scorer := mustScorer(t, weight, mustLeaves(t, searcher.GetIndexReader())[0])
	if fmt.Sprintf("%T", search.All(1)) != fmt.Sprintf("%T", scorer.Iterator()) {
		t.Fatalf("iterator = %T, want %T", scorer.Iterator(), search.All(1))
	}

	// When not all documents in the query have a value, the optimization is
	// not applicable
	mustClose(t, reader)
	mustAddDocument(t, w, document.NewDocument())
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	reader = mustGetReader(t, w)
	searcher = search.NewIndexSearcher(reader)
	searcher.SetQueryCache(nil)
	weight = mustCreateWeight(t, searcher, query, search.COMPLETE_NO_SCORES, 1)
	scorer = mustScorer(t, weight, mustLeaves(t, searcher.GetIndexReader())[0])
	if fmt.Sprintf("%T", search.All(1)) == fmt.Sprintf("%T", scorer.Iterator()) {
		t.Fatalf("iterator = %T, want a different class", scorer.Iterator())
	}

	mustClose(t, reader, w, dir)
}

// randomIntRange renders Random.nextInt(int origin, int bound): a value in
// [origin, bound).
func randomIntRange(origin, bound int) int {
	return origin + random().Intn(bound-origin)
}

func TestPointQueriesPointRangeWeightCount(t *testing.T) {
	pointQueriesBeforeClass()
	// the optimization for Weight#count kicks in only when the number of
	// dimensions is 1
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)

	numPoints := randomIntRange(1, 10)
	points := make([]int32, numPoints)

	numQueries := randomIntRange(1, 10)
	lowerBound := make([]int32, numQueries)
	upperBound := make([]int32, numQueries)
	expectedCount := make([]int, numQueries)

	for i := 0; i < numQueries; i++ {
		// generate random queries
		lowerBound[i] = int32(randomIntRange(1, 10))
		// allow malformed ranges where upperBound could be less than lowerBound
		upperBound[i] = int32(randomIntRange(1, 10))
	}

	for i := 0; i < numPoints; i++ {
		// generate random 1D points
		points[i] = int32(randomIntRange(1, 10))
		if random().Intn(2) == 0 {
			// the doc may have at-most 1 point
			doc := document.NewDocument()
			doc.Add(document.NewIntPoint("point", points[i]))
			mustAddDocument(t, w, doc)
			for j := 0; j < numQueries; j++ {
				// calculate the number of points that lie within the query range
				if lowerBound[j] <= points[i] && points[i] <= upperBound[j] {
					expectedCount[j]++
				}
			}
		}
	}
	mustCommit(t, w)
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	reader := mustGetReader(t, w)
	searcher := search.NewIndexSearcher(reader)
	if len(searcher.GetLeafContexts()) != 0 { // we need at least 1 leaf in the segment
		for i := 0; i < numQueries; i++ {
			query := intPointNewRangeQuery(t, "point", lowerBound[i], upperBound[i])
			weight := mustCreateWeight(t, searcher, query, search.COMPLETE_NO_SCORES, 1)
			count, err := weight.Count(searcher.GetLeafContexts()[0])
			if err != nil {
				t.Fatalf("count: %v", err)
			}
			if count != expectedCount[i] {
				t.Fatalf("count = %d, want %d", count, expectedCount[i])
			}
		}
	}

	mustClose(t, reader, w, dir)
}

// assertQueriesEqual renders assertEquals(q1, q2) followed by
// assertEquals(q1.hashCode(), q2.hashCode()).
func assertQueriesEqual(t *testing.T, q1, q2 search.Query) {
	t.Helper()
	if !q1.Equals(q2) {
		t.Fatalf("%v != %v", q1, q2)
	}
	if q1.HashCode() != q2.HashCode() {
		t.Fatalf("hashCode %d != %d", q1.HashCode(), q2.HashCode())
	}
}

func assertQueriesNotEqual(t *testing.T, q1, q2 search.Query) {
	t.Helper()
	if q1.Equals(q2) {
		t.Fatalf("%v equals %v", q1, q2)
	}
}

func TestPointQueriesPointRangeEquals(t *testing.T) {
	pointQueriesBeforeClass()
	var q1, q2 search.Query

	q1 = intPointNewRangeQuery(t, "a", 0, 1000)
	q2 = intPointNewRangeQuery(t, "a", 0, 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, intPointNewRangeQuery(t, "a", 1, 1000))
	assertQueriesNotEqual(t, q1, intPointNewRangeQuery(t, "b", 0, 1000))

	q1 = longPointNewRangeQuery(t, "a", 0, 1000)
	q2 = longPointNewRangeQuery(t, "a", 0, 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, longPointNewRangeQuery(t, "a", 1, 1000))

	q1 = floatPointNewRangeQuery(t, "a", 0, 1000)
	q2 = floatPointNewRangeQuery(t, "a", 0, 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, floatPointNewRangeQuery(t, "a", 1, 1000))

	q1 = doublePointNewRangeQuery(t, "a", 0, 1000)
	q2 = doublePointNewRangeQuery(t, "a", 0, 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, doublePointNewRangeQuery(t, "a", 1, 1000))

	zeros := make([]byte, 5)
	ones := bytes.Repeat([]byte{0xff}, 5)
	q1 = binaryPointNewRangeQueryDims(t, "a", [][]byte{zeros}, [][]byte{ones})
	q2 = binaryPointNewRangeQueryDims(t, "a", [][]byte{zeros}, [][]byte{ones})
	assertQueriesEqual(t, q1, q2)
	other := slices.Clone(ones)
	other[2] = 5
	assertQueriesNotEqual(t, q1, binaryPointNewRangeQueryDims(t, "a", [][]byte{zeros}, [][]byte{other}))
}

// assertSamePointRangeBounds renders the instanceof PointRangeQuery checks and
// the getLowerPoint()/getUpperPoint() comparisons of testPointExactEquals.
func assertSamePointRangeBounds(t *testing.T, q1, q2 search.Query) {
	t.Helper()
	pq1, ok1 := q1.(*search.PointRangeQuery)
	pq2, ok2 := q2.(*search.PointRangeQuery)
	if !ok1 || !ok2 {
		t.Fatalf("%T, %T are not PointRangeQuery", q1, q2)
	}
	if !bytes.Equal(pq1.LowerValue(), pq2.LowerValue()) {
		t.Fatal("lower points differ")
	}
	if !bytes.Equal(pq1.UpperValue(), pq2.UpperValue()) {
		t.Fatal("upper points differ")
	}
}

func TestPointQueriesPointExactEquals(t *testing.T) {
	pointQueriesBeforeClass()
	var q1, q2 search.Query

	q1 = intPointNewExactQuery(t, "a", 1000)
	q2 = intPointNewExactQuery(t, "a", 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, intPointNewExactQuery(t, "a", 1))
	assertQueriesNotEqual(t, q1, intPointNewExactQuery(t, "b", 1000))
	assertSamePointRangeBounds(t, q1, q2)

	q1 = longPointNewExactQuery(t, "a", 1000)
	q2 = longPointNewExactQuery(t, "a", 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, longPointNewExactQuery(t, "a", 1))
	assertSamePointRangeBounds(t, q1, q2)

	q1 = floatPointNewExactQuery(t, "a", 1000)
	q2 = floatPointNewExactQuery(t, "a", 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, floatPointNewExactQuery(t, "a", 1))
	assertSamePointRangeBounds(t, q1, q2)

	q1 = doublePointNewExactQuery(t, "a", 1000)
	q2 = doublePointNewExactQuery(t, "a", 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, doublePointNewExactQuery(t, "a", 1))
	assertSamePointRangeBounds(t, q1, q2)

	ones := bytes.Repeat([]byte{0xff}, 5)
	q1 = binaryPointNewExactQuery(t, "a", ones)
	q2 = binaryPointNewExactQuery(t, "a", ones)
	assertQueriesEqual(t, q1, q2)
	other := slices.Clone(ones)
	other[2] = 5
	assertQueriesNotEqual(t, q1, binaryPointNewExactQuery(t, "a", other))
	assertSamePointRangeBounds(t, q1, q2)
}

func TestPointQueriesPointInSetEquals(t *testing.T) {
	pointQueriesBeforeClass()
	var q1, q2 search.Query
	q1 = intPointNewSetQuery(t, "a", 0, 1000, 17)
	q2 = intPointNewSetQuery(t, "a", 17, 0, 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, intPointNewSetQuery(t, "a", 1, 17, 1000))
	assertQueriesNotEqual(t, q1, intPointNewSetQuery(t, "b", 0, 1000, 17))

	q1 = longPointNewSetQuery(t, "a", 0, 1000, 17)
	q2 = longPointNewSetQuery(t, "a", 17, 0, 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, longPointNewSetQuery(t, "a", 1, 17, 1000))

	q1 = floatPointNewSetQuery(t, "a", 0, 1000, 17)
	q2 = floatPointNewSetQuery(t, "a", 17, 0, 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, floatPointNewSetQuery(t, "a", 1, 17, 1000))

	q1 = doublePointNewSetQuery(t, "a", 0, 1000, 17)
	q2 = doublePointNewSetQuery(t, "a", 17, 0, 1000)
	assertQueriesEqual(t, q1, q2)
	assertQueriesNotEqual(t, q1, doublePointNewSetQuery(t, "a", 1, 17, 1000))

	zeros := make([]byte, 5)
	ones := bytes.Repeat([]byte{0xff}, 5)
	q1 = binaryPointNewSetQuery(t, "a", zeros, ones)
	q2 = binaryPointNewSetQuery(t, "a", zeros, ones)
	assertQueriesEqual(t, q1, q2)
	other := slices.Clone(ones)
	other[2] = 5
	assertQueriesNotEqual(t, q1, binaryPointNewSetQuery(t, "a", zeros, other))
}

func TestPointQueriesInvalidPointLength(t *testing.T) {
	pointQueriesBeforeClass()
	_, err := search.NewPointRangeQueryMultiDim("field", make([]byte, 4), make([]byte, 8), 1)
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if got, want := err.Error(), "lowerPoint has length=4 but upperPoint has different length=8"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func TestPointQueriesNextUp(t *testing.T) {
	pointQueriesBeforeClass()
	// assertTrue(Double.compare(0d, DoublePoint.nextUp(-0d)) == 0); ...
	t.Fatal(pointNextUpDownBlocker)
}

func TestPointQueriesNextDown(t *testing.T) {
	pointQueriesBeforeClass()
	// assertTrue(Double.compare(-0d, DoublePoint.nextDown(0d)) == 0); ...
	t.Fatal(pointNextUpDownBlocker)
}

func TestPointQueriesRangeQueryOptimizesRewrites(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	w := newRandomIndexWriterWithConfig(t, dir, newIndexWriterConfig())

	one, ten := int32(1), int32(10)
	maxDims := int32(index.PointValuesMaxIndexDimensions)
	numDims := int(pointQueriesRandomIntValue(&one, &maxDims))
	point := make([]int32, numDims)
	lower := make([]int32, numDims)
	upper := make([]int32, numDims)

	zero, five := int32(0), int32(5)
	for i := 0; i < numDims; i++ {
		point[i] = pointQueriesRandomIntValue(&one, &ten)
		lower[i] = point[i] - pointQueriesRandomIntValue(&zero, &five)
		upper[i] = point[i] + pointQueriesRandomIntValue(&zero, &five)
	}

	// Should rewrite to match all docs query if fully contained
	mustAddDocument(t, w, newTestDocument(document.NewIntPoint("field", point...), mustSortedNumericDocValuesField(t, "field", 1)))

	reader := mustGetReader(t, w)
	searcher := search.NewIndexSearcher(reader)

	query := intPointNewRangeQueryDims(t, "field", lower, upper)
	rewritten := mustRewrite(t, searcher, query)
	if _, ok := rewritten.(*search.MatchAllDocsQuery); !ok {
		t.Fatalf("Expected MatchAllDocsQuery, but got [%T]", rewritten)
	}
	assertCount(t, searcher, query, 1)

	// Should rewrite to FieldExistsQuery if fully contained but not all docs
	// have values
	mustAddDocument(t, w, document.NewDocument())
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	mustClose(t, reader)
	reader = mustGetReader(t, w)
	searcher = search.NewIndexSearcher(reader)
	rewritten = mustRewrite(t, searcher, query)
	if _, ok := rewritten.(*search.FieldExistsQuery); !ok {
		t.Fatalf("Expected FieldExistsQuery, but got [%T]", rewritten)
	}
	assertCount(t, searcher, query, 1)

	// Should fallback to MatchNoDocsQuery if no docs have values
	for i := 0; i < numDims; i++ {
		lower[i] = point[i] - 3
		upper[i] = point[i] - 2
	}

	query = intPointNewRangeQueryDims(t, "field", lower, upper)
	rewritten = mustRewrite(t, searcher, query)
	if _, ok := rewritten.(*search.MatchNoDocsQuery); !ok {
		t.Fatalf("Expected MatchNoDocsQuery, but got [%T]", rewritten)
	}
	assertCount(t, searcher, query, 0)

	mustClose(t, reader, w, dir)
}

// assertScorerSupplierNull renders assertNull / assertNotNull on
// weight.scorerSupplier(reader.leaves().get(0)).
func assertScorerSupplierNull(t *testing.T, searcher *search.IndexSearcher, query search.Query, ctx *index.LeafReaderContext, wantNull bool) {
	t.Helper()
	weight := mustCreateWeight(t, searcher, mustRewrite(t, searcher, query), search.COMPLETE_NO_SCORES, 1)
	ss, err := weight.ScorerSupplier(ctx)
	if err != nil {
		t.Fatalf("scorerSupplier: %v", err)
	}
	if (ss == nil) != wantNull {
		t.Fatalf("scorerSupplier = %v, want null=%v", ss, wantNull)
	}
}

func TestPointQueriesRangeQuerySkipsNonMatchingSegments(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	doc := document.NewDocument()
	doc.Add(document.NewIntPoint("field", 2))
	doc.Add(document.NewIntPoint("field2d", 1, 3))
	mustAddDocument(t, w, doc)

	reader := mustOpenDirectoryReaderFromWriter(t, w)
	searcher := newSearcher(t, reader)
	leaf := mustLeaves(t, reader)[0]

	assertScorerSupplierNull(t, searcher, intPointNewRangeQuery(t, "field", 0, 1), leaf, true)
	assertScorerSupplierNull(t, searcher, intPointNewRangeQuery(t, "field", 3, 4), leaf, true)
	assertScorerSupplierNull(t, searcher, intPointNewRangeQueryDims(t, "field2d", []int32{0, 0}, []int32{2, 2}), leaf, true)
	assertScorerSupplierNull(t, searcher, intPointNewRangeQueryDims(t, "field2d", []int32{2, 2}, []int32{4, 4}), leaf, true)

	mustClose(t, reader, w, dir)
}

func TestPointQueriesPointInSetQuerySkipsNonMatchingSegments(t *testing.T) {
	pointQueriesBeforeClass()
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfig())
	doc := document.NewDocument()
	doc.Add(document.NewIntPoint("field", 10))
	doc.Add(document.NewIntPoint("field2d", 10, 10))
	mustAddDocument(t, w, doc)

	reader := mustOpenDirectoryReaderFromWriter(t, w)
	searcher := newSearcher(t, reader)
	leaf := mustLeaves(t, reader)[0]

	assertScorerSupplierNull(t, searcher, intPointNewSetQuery(t, "field", 1, 3, 5), leaf, true)
	assertScorerSupplierNull(t, searcher, intPointNewSetQuery(t, "field", 11, 13, 15), leaf, true)
	assertScorerSupplierNull(t, searcher, intPointNewSetQuery(t, "field", 5, 10, 15), leaf, false)
	assertScorerSupplierNull(t, searcher, mustNewMultiDimIntSetQuery(t, "field2d", 2, 5, 5), leaf, true)
	assertScorerSupplierNull(t, searcher, mustNewMultiDimIntSetQuery(t, "field2d", 2, 15, 15), leaf, true)
	assertScorerSupplierNull(t, searcher, mustNewMultiDimIntSetQuery(t, "field2d", 2, 10, 10), leaf, false)

	mustClose(t, reader, w, dir)
}

func TestPointQueriesOutOfOrderValuesInPointInSetQuery(t *testing.T) {
	pointQueriesBeforeClass()
	values := []*util.BytesRef{
		newBytesRef([]byte{2}), newBytesRef([]byte{1}), // out of order
	}
	idx := 0
	_, err := search.NewPointInSetQuery("foo", 1, 1, util.BytesRefIteratorFunc(func() (*util.BytesRef, error) {
		if idx < len(values) {
			v := values[idx]
			idx++
			return v, nil
		}
		return nil, nil
	}), func(point []byte) string {
		return javaBytesToString(point)
	})
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if got, want := err.Error(), "values are out of order: saw [2] before [1]"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}

func mustSortedNumericDocValuesField(t *testing.T, name string, value int64) *document.SortedNumericDocValuesField {
	t.Helper()
	f, err := document.NewSortedNumericDocValuesField(name, []int64{value})
	if err != nil {
		t.Fatalf("SortedNumericDocValuesField: %v", err)
	}
	return f
}
