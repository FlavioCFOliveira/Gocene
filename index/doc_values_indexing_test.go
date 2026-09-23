// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestDocValuesIndexing.java
// (Apache Lucene 10.5.0): tests DocValues integration into IndexWriter.

package index_test

import (
	"errors"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// addIndexesCodecReadersMissing names the IndexWriter overload that
// RandomIndexWriter.addIndexes(CodecReader...) and TestUtil.addIndexesSlowly
// reach.
const addIndexesCodecReadersMissing = "org.apache.lucene.index.IndexWriter#addIndexes(CodecReader...) is not ported"

func numericDVField(t testing.TB, name string, value int64) *document.NumericDocValuesField {
	t.Helper()
	f, err := document.NewNumericDocValuesField(name, value)
	if err != nil {
		t.Fatalf("NumericDocValuesField: %v", err)
	}
	return f
}

func binaryDVField(t testing.TB, name string, value []byte) *document.BinaryDocValuesField {
	t.Helper()
	f, err := document.NewBinaryDocValuesField(name, value)
	if err != nil {
		t.Fatalf("BinaryDocValuesField: %v", err)
	}
	return f
}

func sortedDVField(t testing.TB, name string, value []byte) *document.SortedDocValuesField {
	t.Helper()
	f, err := document.NewSortedDocValuesField(name, value)
	if err != nil {
		t.Fatalf("SortedDocValuesField: %v", err)
	}
	return f
}

func sortedSetDVField(t testing.TB, name string, value []byte) *document.SortedSetDocValuesField {
	t.Helper()
	f, err := document.NewSortedSetDocValuesField(name, [][]byte{value})
	if err != nil {
		t.Fatalf("SortedSetDocValuesField: %v", err)
	}
	return f
}

// newBytesRef renders LuceneTestCase.newBytesRef(String): the Go doc-values
// fields take the bytes directly.
func newBytesRef(s string) []byte { return []byte(s) }

func docOf(fields ...document.IndexableField) *document.Document {
	doc := document.NewDocument()
	for _, f := range fields {
		doc.Add(f)
	}
	return doc
}

// expectAddDocumentIAE renders expectThrows(IllegalArgumentException.class,
// () -> w.addDocument(doc)).
func expectAddDocumentIAE(t testing.TB, add func(*document.Document) (int64, error), doc *document.Document) {
	t.Helper()
	if _, err := add(doc); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}
}

func assertNumDocs(t testing.TB, expected int, r index.IndexReaderInterface) {
	t.Helper()
	if r.NumDocs() != expected {
		t.Fatalf("numDocs: expected %d, got %d", expected, r.NumDocs())
	}
}

func mustGetReaderRIW(t testing.TB, w interface {
	GetReader() (*index.DirectoryReader, error)
}) *index.DirectoryReader {
	t.Helper()
	r, err := w.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	return r
}

func TestDocValuesIndexingAddIndexes(t *testing.T) {
	d1 := newDirectory()
	w := newRandomIndexWriter(t, d1)
	doc := docOf(newStringField(t, "id", "1", true), numericDVField(t, "dv", 1))
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	r1 := mustGetReaderRIW(t, w)
	mustClose(t, w)

	d2 := newDirectory()
	w = newRandomIndexWriter(t, d2)
	doc = docOf(newStringField(t, "id", "2", true), numericDVField(t, "dv", 2))
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	r2 := mustGetReaderRIW(t, w)
	mustClose(t, w)

	d3 := newDirectory()
	w = newRandomIndexWriter(t, d3)
	defer mustClose(t, w, r1, d1, r2, d2, d3)
	t.Fatal("org.apache.lucene.tests.index.RandomIndexWriter#addIndexes(CodecReader...) and " + addIndexesCodecReadersMissing)
}

// assertOnlyNumeric17 renders the shared tail of the multi-valued and
// different-typed tests: DocValues.getNumeric on the only leaf yields doc 0
// with value 17.
func assertOnlyNumeric17(t testing.TB, r *index.DirectoryReader) {
	t.Helper()
	values, err := index.GetNumeric(getOnlyLeafReader(t, r), "field")
	if err != nil {
		t.Fatalf("DocValues.getNumeric: %v", err)
	}
	if doc, err := values.NextDoc(); err != nil || doc != 0 {
		t.Fatalf("nextDoc: expected 0, got %d (%v)", doc, err)
	}
	if v, err := values.LongValue(); err != nil || v != 17 {
		t.Fatalf("longValue: expected 17, got %d (%v)", v, err)
	}
}

func TestDocValuesIndexingMultiValuedDocValuesField(t *testing.T) {
	d := newDirectory()
	w := newRandomIndexWriter(t, d)
	doc := document.NewDocument()
	f := numericDVField(t, "field", 17)
	doc.Add(f)

	// add the doc
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}

	// Index doc values are single-valued so we should not
	// be able to add same field more than once:
	doc.Add(f)
	expectAddDocumentIAE(t, w.AddDocument, doc)

	r := mustGetReaderRIW(t, w)
	mustClose(t, w)
	assertOnlyNumeric17(t, r)
	mustClose(t, r, d)
}

func TestDocValuesIndexingDifferentTypedDocValuesField(t *testing.T) {
	d := newDirectory()
	w := newRandomIndexWriter(t, d)
	doc := docOf(numericDVField(t, "field", 17))
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}

	// Index doc values are single-valued so we should not
	// be able to add same field more than once:
	doc.Add(binaryDVField(t, "field", newBytesRef("blah")))
	expectAddDocumentIAE(t, w.AddDocument, doc)

	r := mustGetReaderRIW(t, w)
	mustClose(t, w)
	assertOnlyNumeric17(t, r)
	mustClose(t, r, d)
}

func TestDocValuesIndexingDifferentTypedDocValuesField2(t *testing.T) {
	d := newDirectory()
	w := newRandomIndexWriter(t, d)
	doc := docOf(numericDVField(t, "field", 17))
	if _, err := w.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}

	// Index doc values are single-valued so we should not
	// be able to add same field more than once:
	doc.Add(sortedDVField(t, "field", newBytesRef("hello")))
	expectAddDocumentIAE(t, w.AddDocument, doc)

	r := mustGetReaderRIW(t, w)
	assertOnlyNumeric17(t, r)
	mustClose(t, r, w, d)
}

// LUCENE-3870
func TestDocValuesIndexingLengthPrefixAcrossTwoPages(t *testing.T) {
	d := newDirectory()
	w := mustNewIndexWriter(t, d, index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	bytes := make([]byte, 32764)
	b := bytes
	doc.Add(sortedDVField(t, "field", b))
	mustAddDocument(t, w, doc)
	bytes[0] = 1
	mustAddDocument(t, w, doc)
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	r := openReaderFromWriter(t, w)
	s, err := index.GetSorted(getOnlyLeafReader(t, r), "field")
	if err != nil {
		t.Fatalf("DocValues.getSorted: %v", err)
	}
	assertSortedDocBytes := func(expectedDoc int, first byte) {
		t.Helper()
		if doc, err := s.NextDoc(); err != nil || doc != expectedDoc {
			t.Fatalf("nextDoc: expected %d, got %d (%v)", expectedDoc, doc, err)
		}
		ord, err := s.OrdValue()
		if err != nil {
			t.Fatalf("ordValue: %v", err)
		}
		bytes1, err := s.LookupOrd(ord)
		if err != nil {
			t.Fatalf("lookupOrd: %v", err)
		}
		if len(bytes) != len(bytes1) {
			t.Fatalf("length: expected %d, got %d", len(bytes), len(bytes1))
		}
		bytes[0] = first
		if string(b) != string(bytes1) {
			t.Fatalf("doc %d bytes differ", expectedDoc)
		}
	}
	assertSortedDocBytes(0, 0)
	assertSortedDocBytes(1, 1)
	mustClose(t, r, w, d)
}

func TestDocValuesIndexingDocValuesUnstored(t *testing.T) {
	dir := newDirectory()
	iwconfig := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwconfig.SetMergePolicy(newLogMergePolicy())
	writer := mustNewIndexWriter(t, dir, iwconfig)
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		doc.Add(numericDVField(t, "dv", int64(i)))
		doc.Add(newTextField(t, "docId", strconv.Itoa(i), true))
		mustAddDocument(t, writer, doc)
	}
	r := openReaderFromWriter(t, writer)
	fi, err := spi.GetMergedFieldInfos(r)
	if err != nil {
		t.Fatalf("FieldInfos.getMergedFieldInfos: %v", err)
	}
	dvInfo := fi.FieldInfo("dv")
	if dvInfo == nil || dvInfo.DocValuesType() == index.DocValuesTypeNone {
		t.Fatalf("assertTrue(dvInfo.getDocValuesType() != DocValuesType.NONE): %v", dvInfo)
	}
	dv, err := index.MultiDocValuesGetNumericValues(r, "dv")
	if err != nil {
		t.Fatalf("MultiDocValues.getNumericValues: %v", err)
	}
	storedFields, err := r.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	for i := 0; i < 50; i++ {
		if doc, err := dv.NextDoc(); err != nil || doc != i {
			t.Fatalf("nextDoc: expected %d, got %d (%v)", i, doc, err)
		}
		if v, err := dv.LongValue(); err != nil || v != int64(i) {
			t.Fatalf("longValue: expected %d, got %d (%v)", i, v, err)
		}
		d := storedDocument(t, storedFields, i)
		// cannot use d.get("dv") due to another bug!
		if d.Get("dv") != nil {
			t.Fatalf("assertNull(d.getField(\"dv\")) at %d", i)
		}
		if got := docGet(d, "docId"); got == nil || *got != strconv.Itoa(i) {
			t.Fatalf("docId at %d: got %v", i, got)
		}
	}
	mustClose(t, r, writer, dir)
}

func newMockAnalyzerWriter(t testing.TB, dir store.Directory) *index.IndexWriter {
	t.Helper()
	return mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
}

// Same field in one document as different types:
func TestDocValuesIndexingMixedTypesSameDocument(t *testing.T) {
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, w, document.NewDocument())

	doc := docOf(numericDVField(t, "foo", 0), sortedDVField(t, "foo", newBytesRef("hello")))
	expectAddDocumentIAE(t, w.AddDocument, doc)

	ir := openReaderFromWriter(t, w)
	assertNumDocs(t, 1, ir)
	mustClose(t, ir, w, dir)
}

// Two documents with same field as different types:
func TestDocValuesIndexingMixedTypesDifferentDocuments(t *testing.T) {
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, w, docOf(numericDVField(t, "foo", 0)))

	doc2 := docOf(sortedDVField(t, "foo", newBytesRef("hello")))
	expectAddDocumentIAE(t, w.AddDocument, doc2)

	ir := openReaderFromWriter(t, w)
	assertNumDocs(t, 1, ir)
	mustClose(t, ir, w, dir)
}

// addTwiceTest renders the shared body of testAddSortedTwice,
// testAddBinaryTwice and testAddNumericTwice. We don't use RandomIndexWriter
// because it might add more docvalues than we expect !!!!1
func addTwiceTest(t *testing.T, first, second document.IndexableField) {
	t.Helper()
	analyzer := newMockAnalyzer()

	directory := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(analyzer)
	iwc.SetMergePolicy(newLogMergePolicy())
	iwriter := mustNewIndexWriter(t, directory, iwc)
	doc := docOf(first)
	mustAddDocument(t, iwriter, doc)

	doc.Add(second)
	expectAddDocumentIAE(t, iwriter.AddDocument, doc)

	ir := openReaderFromWriter(t, iwriter)
	assertNumDocs(t, 1, ir)
	mustClose(t, ir, iwriter, directory)
}

func TestDocValuesIndexingAddSortedTwice(t *testing.T) {
	addTwiceTest(t, sortedDVField(t, "dv", newBytesRef("foo!")), sortedDVField(t, "dv", newBytesRef("bar!")))
}

func TestDocValuesIndexingAddBinaryTwice(t *testing.T) {
	addTwiceTest(t, binaryDVField(t, "dv", newBytesRef("foo!")), binaryDVField(t, "dv", newBytesRef("bar!")))
}

func TestDocValuesIndexingAddNumericTwice(t *testing.T) {
	addTwiceTest(t, numericDVField(t, "dv", 1), numericDVField(t, "dv", 2))
}

// tooLargeTest renders the shared body of the two too-large tests.
func tooLargeTest(t *testing.T, newField func(value []byte) document.IndexableField) {
	t.Helper()
	analyzer := newMockAnalyzer()

	directory := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(analyzer)
	iwc.SetMergePolicy(newLogMergePolicy())
	iwriter := mustNewIndexWriter(t, directory, iwc)
	mustAddDocument(t, iwriter, docOf(newField(newBytesRef("just fine"))))

	bytes := make([]byte, 100000)
	b := bytes
	rand.Read(bytes)
	hugeDoc := docOf(newField(b))
	expectAddDocumentIAE(t, iwriter.AddDocument, hugeDoc)

	ir := openReaderFromWriter(t, iwriter)
	assertNumDocs(t, 1, ir)
	mustClose(t, ir, iwriter, directory)
}

func TestDocValuesIndexingTooLargeSortedBytes(t *testing.T) {
	tooLargeTest(t, func(v []byte) document.IndexableField { return sortedDVField(t, "dv", v) })
}

func TestDocValuesIndexingTooLargeTermSortedSetBytes(t *testing.T) {
	tooLargeTest(t, func(v []byte) document.IndexableField { return sortedSetDVField(t, "dv", v) })
}

// Two documents across segments
func TestDocValuesIndexingMixedTypesDifferentSegments(t *testing.T) {
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, w, docOf(numericDVField(t, "foo", 0)))
	mustCommit(t, w)

	doc2 := docOf(sortedDVField(t, "foo", newBytesRef("hello")))
	expectAddDocumentIAE(t, w.AddDocument, doc2)

	mustClose(t, w, dir)
}

// Add inconsistent document after deleteAll
func TestDocValuesIndexingMixedTypesAfterDeleteAll(t *testing.T) {
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, w, docOf(numericDVField(t, "foo", 0)))
	if _, err := w.DeleteAll(); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}

	mustAddDocument(t, w, docOf(sortedDVField(t, "foo", newBytesRef("hello"))))
	mustClose(t, w, dir)
}

// Add inconsistent document after reopening IW w/ create
func TestDocValuesIndexingMixedTypesAfterReopenCreate(t *testing.T) {
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, w, docOf(numericDVField(t, "foo", 0)))
	mustClose(t, w)

	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetOpenMode(index.Create)
	w = mustNewIndexWriter(t, dir, iwc)
	mustAddDocument(t, w, document.NewDocument())
	mustClose(t, w, dir)
}

func TestDocValuesIndexingMixedTypesAfterReopenAppend1(t *testing.T) {
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, w, docOf(numericDVField(t, "foo", 0)))
	mustClose(t, w)

	w2 := newMockAnalyzerWriter(t, dir)
	doc2 := docOf(sortedDVField(t, "foo", newBytesRef("hello")))
	expectAddDocumentIAE(t, w2.AddDocument, doc2)

	mustClose(t, w2, dir)
}

// mixedTypesAfterReopenAppend renders the shared body of
// testMixedTypesAfterReopenAppend2 and testMixedTypesAfterReopenAppend3.
func mixedTypesAfterReopenAppend(t *testing.T, addAnother bool) {
	t.Helper()
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, w, docOf(sortedSetDVField(t, "foo", newBytesRef("foo"))))
	mustClose(t, w)

	w2 := newMockAnalyzerWriter(t, dir)
	doc2 := docOf(newStringField(t, "foo", "bar", false), binaryDVField(t, "foo", newBytesRef("foo")))
	// NOTE: this case follows a different code path inside
	// DefaultIndexingChain/FieldInfos, because the field (foo)
	// is first added without DocValues:
	expectAddDocumentIAE(t, w2.AddDocument, doc2)

	if addAnother {
		// Also add another document so there is a segment to write here:
		mustAddDocument(t, w2, document.NewDocument())
	}
	if err := w2.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w2, dir)
}

func TestDocValuesIndexingMixedTypesAfterReopenAppend2(t *testing.T) {
	mixedTypesAfterReopenAppend(t, false)
}

func TestDocValuesIndexingMixedTypesAfterReopenAppend3(t *testing.T) {
	mixedTypesAfterReopenAppend(t, true)
}

// Two documents with same field as different types, added
// from separate threads:
func TestDocValuesIndexingMixedTypesDifferentThreads(t *testing.T) {
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)

	startingGun := make(chan struct{})
	var hitExc atomic.Bool
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		var field document.IndexableField
		if i == 0 {
			field = sortedDVField(t, "foo", newBytesRef("hello"))
		} else if i == 1 {
			field = numericDVField(t, "foo", 0)
		} else {
			field = binaryDVField(t, "foo", newBytesRef("bazz"))
		}
		doc := docOf(field)

		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startingGun
			if _, err := w.AddDocument(doc); err != nil {
				// expected: IllegalArgumentException
				hitExc.Store(true)
			}
		}()
	}

	close(startingGun)
	wg.Wait()
	if !hitExc.Load() {
		t.Fatal("assertTrue(hitExc.get())")
	}
	mustClose(t, w, dir)
}

// Adding documents via addIndexes
func TestDocValuesIndexingMixedTypesViaAddIndexes(t *testing.T) {
	dir := newDirectory()
	w := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, w, docOf(numericDVField(t, "foo", 0)))

	// Make 2nd index w/ inconsistent field
	dir2 := newDirectory()
	w2 := newMockAnalyzerWriter(t, dir2)
	mustAddDocument(t, w2, docOf(sortedDVField(t, "foo", newBytesRef("hello"))))
	mustClose(t, w2)

	if _, err := w.AddIndexes(dir2); err == nil {
		t.Fatal("expected IllegalArgumentException from addIndexes(Directory...)")
	}

	r := mustOpenDirectoryReader(t, dir2)
	defer mustClose(t, r, dir2, w, dir)
	t.Fatal("TestUtil.addIndexesSlowly(IndexWriter, DirectoryReader...) needs " + addIndexesCodecReadersMissing)
}

func TestDocValuesIndexingIllegalTypeChange(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	doc2 := docOf(sortedDVField(t, "dv", newBytesRef("foo")))
	expectAddDocumentIAE(t, writer.AddDocument, doc2)

	ir := openReaderFromWriter(t, writer)
	assertNumDocs(t, 1, ir)
	mustClose(t, ir, writer, dir)
}

func TestDocValuesIndexingIllegalTypeChangeAcrossSegments(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	mustClose(t, writer)

	writer2 := newMockAnalyzerWriter(t, dir)
	doc2 := docOf(sortedDVField(t, "dv", newBytesRef("foo")))
	expectAddDocumentIAE(t, writer2.AddDocument, doc2)

	mustClose(t, writer2, dir)
}

func TestDocValuesIndexingTypeChangeAfterCloseAndDeleteAll(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	mustClose(t, writer)

	writer = newMockAnalyzerWriter(t, dir)
	if _, err := writer.DeleteAll(); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	mustAddDocument(t, writer, docOf(sortedDVField(t, "dv", newBytesRef("foo"))))
	mustClose(t, writer, dir)
}

func TestDocValuesIndexingTypeChangeAfterDeleteAll(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	if _, err := writer.DeleteAll(); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	mustAddDocument(t, writer, docOf(sortedDVField(t, "dv", newBytesRef("foo"))))
	mustClose(t, writer, dir)
}

func TestDocValuesIndexingTypeChangeAfterCommitAndDeleteAll(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	mustCommit(t, writer)
	if _, err := writer.DeleteAll(); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
	mustAddDocument(t, writer, docOf(sortedDVField(t, "dv", newBytesRef("foo"))))
	mustClose(t, writer, dir)
}

func TestDocValuesIndexingTypeChangeAfterOpenCreate(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	mustClose(t, writer)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(index.Create)
	writer = mustNewIndexWriter(t, dir, conf)
	mustAddDocument(t, writer, docOf(sortedDVField(t, "dv", newBytesRef("foo"))))
	mustClose(t, writer, dir)
}

func TestDocValuesIndexingTypeChangeViaAddIndexes(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	mustClose(t, writer)

	dir2 := newDirectory()
	writer2 := newMockAnalyzerWriter(t, dir2)
	mustAddDocument(t, writer2, docOf(sortedDVField(t, "dv", newBytesRef("foo"))))
	if _, err := writer2.AddIndexes(dir); err == nil {
		t.Fatal("expected IllegalArgumentException from addIndexes(Directory...)")
	}
	mustClose(t, writer2, dir, dir2)
}

func TestDocValuesIndexingTypeChangeViaAddIndexesIR(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	mustClose(t, writer)

	dir2 := newDirectory()
	writer2 := newMockAnalyzerWriter(t, dir2)
	mustAddDocument(t, writer2, docOf(sortedDVField(t, "dv", newBytesRef("foo"))))
	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader, writer2, dir, dir2)
	t.Fatal("TestUtil.addIndexesSlowly(IndexWriter, DirectoryReader...) needs " + addIndexesCodecReadersMissing)
}

func TestDocValuesIndexingTypeChangeViaAddIndexes2(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	mustClose(t, writer)

	dir2 := newDirectory()
	writer2 := newMockAnalyzerWriter(t, dir2)
	if _, err := writer2.AddIndexes(dir); err != nil {
		t.Fatalf("addIndexes: %v", err)
	}
	doc2 := docOf(sortedDVField(t, "dv", newBytesRef("foo")))
	expectAddDocumentIAE(t, writer2.AddDocument, doc2)

	mustClose(t, writer2, dir2, dir)
}

func TestDocValuesIndexingTypeChangeViaAddIndexesIR2(t *testing.T) {
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)
	mustAddDocument(t, writer, docOf(numericDVField(t, "dv", 0)))
	mustClose(t, writer)

	dir2 := newDirectory()
	writer2 := newMockAnalyzerWriter(t, dir2)
	reader := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, reader, writer2, dir2, dir)
	t.Fatal("TestUtil.addIndexesSlowly(IndexWriter, DirectoryReader...) needs " + addIndexesCodecReadersMissing)
}

func TestDocValuesIndexingSameFieldNameForPostingAndDocValue(t *testing.T) {
	// LUCENE-5192: FieldInfos.Builder neglected to update
	// globalFieldNumbers.docValuesType map if the field existed, resulting in
	// potentially adding the same field with different DV types.
	dir := newDirectory()
	writer := newMockAnalyzerWriter(t, dir)

	mustAddDocument(t, writer, docOf(newStringField(t, "f", "mock-value", false), numericDVField(t, "f", 5)))
	mustCommit(t, writer)

	doc2 := docOf(binaryDVField(t, "f", newBytesRef("mock")))
	expectAddDocumentIAE(t, writer.AddDocument, doc2)
	if err := writer.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	mustClose(t, dir)
}

// excField renders the anonymous Field subclass of
// testExcIndexingDocBeforeDocValues whose tokenStream throws from
// incrementToken.
type excField struct {
	*document.Field
}

func (f *excField) TokenStream(analysis.Analyzer, analysis.TokenStream) analysis.TokenStream {
	return &excTokenStream{BaseTokenStream: analysis.NewBaseTokenStream()}
}

// excTokenStream renders the anonymous TokenStream: incrementToken throws a
// RuntimeException.
type excTokenStream struct {
	*analysis.BaseTokenStream
}

func (s *excTokenStream) IncrementToken() (bool, error) {
	return false, errors.New("RuntimeException")
}

// LUCENE-6049
func TestDocValuesIndexingExcIndexingDocBeforeDocValues(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, dir, iwc)
	doc := document.NewDocument()
	ft := document.NewFieldTypeFrom(document.StringFieldTypeNotStored)
	ft.SetDocValuesType(index.DocValuesTypeSorted)
	ft.Freeze()
	base, err := document.NewField("test", []byte("value"), ft)
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	doc.Add(&excField{Field: base})
	if _, err := w.AddDocument(doc); err == nil {
		t.Fatal("expected RuntimeException from addDocument")
	}

	mustAddDocument(t, w, document.NewDocument())
	mustClose(t, w, dir)
}
