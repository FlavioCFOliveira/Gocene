// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestExitableDirectoryReader.java
// (Apache Lucene 10.5.0): tests that a default QueryTimeout exits long running
// queries that take too long to iterate over Terms. The Java class uses
// search.IndexSearcher and search.PrefixQuery; the port lives in the external
// index_test package to avoid the index -> search import cycle.

package index_test

import (
	"errors"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// exitableTestReader ports the private static TestReader (a FilterLeafReader
// whose terms enumerators sleep 100ms before each next()).
type exitableTestReader struct {
	*index.FilterLeafReader
	in index.LeafReader
}

func newExitableTestReader(reader index.LeafReader) *exitableTestReader {
	return &exitableTestReader{FilterLeafReader: index.NewFilterLeafReader(reader), in: reader}
}

func (r *exitableTestReader) Terms(field string) (index.Terms, error) {
	terms, err := r.FilterLeafReader.Terms(field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	return &exitableTestTerms{FilterTerms: index.NewFilterTerms(terms)}, nil
}

func (r *exitableTestReader) GetCoreCacheHelper() index.CacheHelper {
	return r.in.GetCoreCacheHelper()
}

func (r *exitableTestReader) GetReaderCacheHelper() index.CacheHelper {
	return r.in.GetReaderCacheHelper()
}

type exitableTestTerms struct {
	*index.FilterTerms
}

func (t *exitableTestTerms) Iterator() (index.TermsEnum, error) {
	in, err := t.FilterTerms.Iterator()
	if err != nil {
		return nil, err
	}
	return &exitableTestTermsEnum{FilterTermsEnum: index.NewFilterTermsEnum(in)}, nil
}

type exitableTestTermsEnum struct {
	*index.FilterTermsEnum
}

// Next sleeps between iterations to timeout things.
func (e *exitableTestTermsEnum) Next() (*index.Term, error) {
	// Sleep for 100ms before each .next() call.
	time.Sleep(100 * time.Millisecond)
	return e.FilterTermsEnum.Next()
}

type leavesReader interface {
	Leaves() ([]*index.LeafReaderContext, error)
}

// exitableGetOnlyLeafReader renders LuceneTestCase.getOnlyLeafReader(IndexReader).
func exitableGetOnlyLeafReader(t *testing.T, reader leavesReader) index.LeafReader {
	t.Helper()
	subReaders, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if len(subReaders) != 1 {
		t.Fatalf("reader has %d segments instead of exactly one", len(subReaders))
	}
	return subReaders[0].LeafReader()
}

type queryTimeoutFunc func() bool

func (f queryTimeoutFunc) ShouldExit() bool { return f() }

func infiniteQueryTimeout() index.QueryTimeout {
	return queryTimeoutFunc(func() bool { return false })
}

func immediateQueryTimeout() index.QueryTimeout {
	return queryTimeoutFunc(func() bool { return true })
}

// countingQueryTimeout ports the private static CountingQueryTimeout.
type countingQueryTimeout struct {
	counter int
}

func (q *countingQueryTimeout) ShouldExit() bool {
	q.counter++
	return false
}

func (q *countingQueryTimeout) getShouldExitCallCount() int {
	return q.counter
}

func exitableNewDirectory() *store.MockDirectoryWrapper {
	return store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
}

func exitableNewIndexWriterConfig() *index.IndexWriterConfig {
	return index.NewIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true))
}

func exitableAddTextDoc(t *testing.T, writer *index.IndexWriter, field, value string) {
	t.Helper()
	d := document.NewDocument()
	f, err := document.NewTextField(field, value, true)
	if err != nil {
		t.Fatalf("newTextField: %v", err)
	}
	d.Add(f)
	if _, err := writer.AddDocument(d); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
}

func expectExitingReaderException(t *testing.T, err error) {
	t.Helper()
	var ere *index.ExitingReaderError
	if !errors.As(err, &ere) {
		t.Fatalf("expected ExitingReaderException, got %v", err)
	}
}

// searchWithTimeout runs searcher.search(query, n) over a TestReader wrapping
// the only leaf of an ExitableDirectoryReader opened on directory.
func searchWithTimeout(t *testing.T, directory store.Directory, queryTimeout index.QueryTimeout, query search.Query, n int) error {
	t.Helper()
	directoryReader, err := index.OpenDirectoryReader(directory)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	exitableDirectoryReader := index.NewExitableDirectoryReader(directoryReader, queryTimeout)
	reader := newExitableTestReader(exitableGetOnlyLeafReader(t, exitableDirectoryReader))
	searcher := search.NewIndexSearcher(reader)
	_, searchErr := searcher.Search(query, n)
	if err := reader.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return searchErr
}

// TestExitableDirectoryReaderExitableFilterTermsIndexReader tests timing out of
// TermsEnum iterations.
func TestExitableDirectoryReaderExitableFilterTermsIndexReader(t *testing.T) {
	directory := exitableNewDirectory()
	writer, err := index.NewIndexWriter(directory, exitableNewIndexWriterConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	exitableAddTextDoc(t, writer, "default", "one two")
	exitableAddTextDoc(t, writer, "default", "one three")
	exitableAddTextDoc(t, writer, "default", "ones two four")
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	query := search.NewPrefixQuery(index.NewTerm("default", "o"))

	// Set a fairly high timeout value (infinite) and expect the query to
	// complete in that time frame.
	if err := searchWithTimeout(t, directory, infiniteQueryTimeout(), query, 10); err != nil {
		t.Fatalf("search: %v", err)
	}

	// Set a really low timeout value (immediate) and expect an Exception
	expectExitingReaderException(t, searchWithTimeout(t, directory, immediateQueryTimeout(), query, 10))

	// Set maximum time out and expect the query to complete.
	if err := searchWithTimeout(t, directory, infiniteQueryTimeout(), query, 10); err != nil {
		t.Fatalf("search: %v", err)
	}
	if err := directory.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

// TestExitableDirectoryReaderExitableTermsEnumSampleTimeoutCheck tests time out
// check sampling of TermsEnum iterations (@Nightly in Lucene).
func TestExitableDirectoryReaderExitableTermsEnumSampleTimeoutCheck(t *testing.T) {
	directory := exitableNewDirectory()
	defer directory.Close()
	writer, err := index.NewIndexWriter(directory, exitableNewIndexWriterConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	defer writer.Close()
	for i := 0; i < 50; i++ {
		exitableAddTextDoc(t, writer, "default", "term"+strconv.Itoa(i))
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	query := search.NewPrefixQuery(index.NewTerm("default", "term"))

	// Not checking the validity of the result, but checking the sampling
	// kicks in to reduce the number of timeout check
	queryTimeout := &countingQueryTimeout{}
	if err := searchWithTimeout(t, directory, queryTimeout, query, 300); err != nil {
		t.Fatalf("search: %v", err)
	}
	// The number of sampled query time out check here depends on two factors:
	// 1. ExitableDirectoryReader.ExitableTermsEnum.NUM_CALLS_PER_TIMEOUT_CHECK
	// 2. MultiTermQueryConstantScoreWrapper.BOOLEAN_REWRITE_TERM_COUNT_THRESHOLD
	if got := queryTimeout.getShouldExitCallCount(); got != 5 {
		t.Fatalf("expected 5 shouldExit calls, got %d", got)
	}
}

// TestExitableDirectoryReaderExitablePointValuesIndexReader ports
// testExitablePointValuesIndexReader, which searches
// IntPoint.newRangeQuery("default", 10, 20).
func TestExitableDirectoryReaderExitablePointValuesIndexReader(t *testing.T) {
	t.Fatal("org.apache.lucene.document.IntPoint#newRangeQuery(String, int, int) is not ported")
}

// minMaxExitableTerms renders the anonymous ExitableTerms subclass of
// testExitableTermsMinAndMax, whose iterator() must never be called.
type minMaxExitableTerms struct {
	*index.ExitableTerms
	t *testing.T
}

func (m *minMaxExitableTerms) Iterator() (index.TermsEnum, error) {
	m.t.Fatal("min and max should be retrieved from block tree, no need to iterate")
	return nil, nil
}

func TestExitableDirectoryReaderExitableTermsMinAndMax(t *testing.T) {
	directory := exitableNewDirectory()
	w, err := index.NewIndexWriter(directory, exitableNewIndexWriterConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	doc := document.NewDocument()
	fooField, err := document.NewStringField("foo", "bar", false)
	if err != nil {
		t.Fatalf("new StringField: %v", err)
	}
	doc.Add(fooField)
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	directoryReader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w): %v", err)
	}
	leaves, err := directoryReader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, lfc := range leaves {
		in, err := lfc.LeafReader().Terms("foo")
		if err != nil {
			t.Fatalf("terms: %v", err)
		}
		terms := &minMaxExitableTerms{ExitableTerms: index.NewExitableTerms(in, infiniteQueryTimeout()), t: t}
		minTerm, err := terms.GetMin()
		if err != nil {
			t.Fatalf("getMin: %v", err)
		}
		if minTerm == nil || string(minTerm.Bytes.ValidBytes()) != "bar" {
			t.Fatalf("getMin: expected bar, got %v", minTerm)
		}
		maxTerm, err := terms.GetMax()
		if err != nil {
			t.Fatalf("getMax: %v", err)
		}
		if maxTerm == nil || string(maxTerm.Bytes.ValidBytes()) != "bar" {
			t.Fatalf("getMax: expected bar, got %v", maxTerm)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := directoryReader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := directory.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

// dvFactory ports the DvFactory functional interface.
type dvFactory func(leaf index.LeafReader) (index.DocValuesIterator, error)

func exitableScan(leaf index.LeafReader, iter index.DocValuesIterator) error {
	if _, err := iter.NextDoc(); err != nil {
		return err
	}
	for iter.DocID() != index.NO_MORE_DOCS && iter.DocID() < leaf.MaxDoc() {
		nextDocID := iter.DocID() + 1
		if rand.Intn(2) == 0 && nextDocID < leaf.MaxDoc() {
			if rand.Intn(2) == 0 {
				if _, err := iter.Advance(nextDocID); err != nil {
					return err
				}
			} else {
				if _, err := iter.AdvanceExact(nextDocID); err != nil {
					return err
				}
			}
		} else {
			if _, err := iter.NextDoc(); err != nil {
				return err
			}
		}
	}
	return nil
}

func exitableAddDVs(t *testing.T, d1 *document.Document, i int) {
	t.Helper()
	numeric, err := document.NewNumericDocValuesField("numeric", int64(i))
	if err != nil {
		t.Fatalf("new NumericDocValuesField: %v", err)
	}
	d1.Add(numeric)
	binary, err := document.NewBinaryDocValuesField("binary", []byte(strconv.Itoa(i)))
	if err != nil {
		t.Fatalf("new BinaryDocValuesField: %v", err)
	}
	d1.Add(binary)
	sorted, err := document.NewSortedDocValuesField("sorted", []byte(strconv.Itoa(i)))
	if err != nil {
		t.Fatalf("new SortedDocValuesField: %v", err)
	}
	d1.Add(sorted)
	sortedNumeric, err := document.NewSortedNumericDocValuesField("sortednumeric", []int64{int64(i)})
	if err != nil {
		t.Fatalf("new SortedNumericDocValuesField: %v", err)
	}
	d1.Add(sortedNumeric)
	sortedSet, err := document.NewSortedSetDocValuesField("sortedset", [][]byte{[]byte(strconv.Itoa(i))})
	if err != nil {
		t.Fatalf("new SortedSetDocValuesField: %v", err)
	}
	d1.Add(sortedSet)
}

func TestExitableDirectoryReaderDocValues(t *testing.T) {
	directory := exitableNewDirectory()
	defer directory.Close()
	writer, err := index.NewIndexWriter(directory, exitableNewIndexWriterConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	for _, v := range []int{10, 100, 1000} {
		d := document.NewDocument()
		exitableAddDVs(t, d, v)
		if _, err := writer.AddDocument(d); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	factories := []dvFactory{
		func(r index.LeafReader) (index.DocValuesIterator, error) { return r.GetSortedDocValues("sorted") },
		func(r index.LeafReader) (index.DocValuesIterator, error) { return r.GetSortedSetDocValues("sortedset") },
		func(r index.LeafReader) (index.DocValuesIterator, error) {
			return r.GetSortedNumericDocValues("sortednumeric")
		},
		func(r index.LeafReader) (index.DocValuesIterator, error) { return r.GetNumericDocValues("numeric") },
		func(r index.LeafReader) (index.DocValuesIterator, error) { return r.GetBinaryDocValues("binary") },
	}
	for _, factory := range factories {
		func() {
			directoryReader, err := index.OpenDirectoryReader(directory)
			if err != nil {
				t.Fatalf("DirectoryReader.open: %v", err)
			}
			exitableDirectoryReader := index.NewExitableDirectoryReader(directoryReader, immediateQueryTimeout())
			reader := newExitableTestReader(exitableGetOnlyLeafReader(t, exitableDirectoryReader))
			defer reader.Close()
			leaf := exitableGetOnlyLeafReader(t, reader)
			iter, err := factory(leaf)
			if err == nil {
				err = exitableScan(leaf, iter)
			}
			expectExitingReaderException(t, err)
		}()

		func() {
			directoryReader, err := index.OpenDirectoryReader(directory)
			if err != nil {
				t.Fatalf("DirectoryReader.open: %v", err)
			}
			exitableDirectoryReader := index.NewExitableDirectoryReader(directoryReader, infiniteQueryTimeout())
			reader := newExitableTestReader(exitableGetOnlyLeafReader(t, exitableDirectoryReader))
			defer reader.Close()
			leaf := exitableGetOnlyLeafReader(t, reader)
			iter, err := factory(leaf)
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			if err := exitableScan(leaf, iter); err != nil {
				t.Fatalf("scan: %v", err)
			}
			if v, err := leaf.GetNumericDocValues("absent"); err != nil || v != nil {
				t.Fatalf("assertNull(getNumericDocValues(\"absent\")): %v %v", v, err)
			}
			if v, err := leaf.GetBinaryDocValues("absent"); err != nil || v != nil {
				t.Fatalf("assertNull(getBinaryDocValues(\"absent\")): %v %v", v, err)
			}
			if v, err := leaf.GetSortedDocValues("absent"); err != nil || v != nil {
				t.Fatalf("assertNull(getSortedDocValues(\"absent\")): %v %v", v, err)
			}
			if v, err := leaf.GetSortedNumericDocValues("absent"); err != nil || v != nil {
				t.Fatalf("assertNull(getSortedNumericDocValues(\"absent\")): %v %v", v, err)
			}
			if v, err := leaf.GetSortedSetDocValues("absent"); err != nil || v != nil {
				t.Fatalf("assertNull(getSortedSetDocValues(\"absent\")): %v %v", v, err)
			}
		}()
	}

	// Test that singleton docValues stay as singleton docValues after filtering
	directoryReader, err := index.OpenDirectoryReader(directory)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	reader := newExitableTestReader(exitableGetOnlyLeafReader(t, directoryReader))
	defer reader.Close()
	directoryReader2, err := index.OpenDirectoryReader(directory)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	exitableReader := newExitableTestReader(exitableGetOnlyLeafReader(t,
		index.NewExitableDirectoryReader(directoryReader2, infiniteQueryTimeout())))
	defer exitableReader.Close()
	leafReader := exitableGetOnlyLeafReader(t, reader)
	exitableLeafReader := exitableGetOnlyLeafReader(t, exitableReader)
	ssdv, err := leafReader.GetSortedSetDocValues("sortedset")
	if err != nil {
		t.Fatalf("getSortedSetDocValues: %v", err)
	}
	if index.IsSingletonSortedSetDocValues(ssdv) {
		exitableSsdv, err := exitableLeafReader.GetSortedSetDocValues("sortedset")
		if err != nil {
			t.Fatalf("getSortedSetDocValues: %v", err)
		}
		if !index.IsSingletonSortedSetDocValues(exitableSsdv) {
			t.Fatalf("expected SingletonSortedSetDocValues, got %T", exitableSsdv)
		}
	}
	sndv, err := leafReader.GetSortedNumericDocValues("sortednumeric")
	if err != nil {
		t.Fatalf("getSortedNumericDocValues: %v", err)
	}
	if index.IsSingletonSortedNumericDocValues(sndv) {
		exitableSndv, err := exitableLeafReader.GetSortedNumericDocValues("sortednumeric")
		if err != nil {
			t.Fatalf("getSortedNumericDocValues: %v", err)
		}
		if !index.IsSingletonSortedNumericDocValues(exitableSndv) {
			t.Fatalf("expected SingletonSortedNumericDocValues, got %T", exitableSndv)
		}
	}
}

const exitableVectorsMissing = "org.apache.lucene.document.KnnFloatVectorField#createFieldType(int, " +
	"VectorSimilarityFunction), org.apache.lucene.util.TestVectorUtil#randomVector(int), " +
	"TestVectorUtil#randomVectorBytes(int) and LeafReader#searchNearestVectors(String, float[]/byte[], " +
	"int, AcceptDocs, int) are not ported"

func TestExitableDirectoryReaderFloatVectorValues(t *testing.T) {
	t.Fatal(exitableVectorsMissing)
}

func TestExitableDirectoryReaderByteVectorValues(t *testing.T) {
	t.Fatal(exitableVectorsMissing)
}
