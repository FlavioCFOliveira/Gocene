// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestNumericDocValuesUpdates.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const assertingCodecMissing = "org.apache.lucene.tests.codecs.asserting.AssertingCodec is not ported"

// ndvDoc renders the private doc(int): make sure we don't set the doc's value
// to 0, to not confuse with a document that's missing values.
func ndvDoc(t testing.TB, id int) *document.Document { return ndvDocVal(t, id, int64(id+1)) }

// ndvDocVal renders the private doc(int, long).
func ndvDocVal(t testing.TB, id int, val int64) *document.Document {
	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc-"+strconv.Itoa(id), false))
	doc.Add(numericDVField(t, "val", val))
	return doc
}

func mustUpdateNumericDocValue(t testing.TB, w *index.IndexWriter, term *index.Term, field string, value int64) {
	t.Helper()
	if _, err := w.UpdateNumericDocValue(term, field, value); err != nil {
		t.Fatalf("updateNumericDocValue(%v, %s, %d): %v", term, field, value, err)
	}
}

func mustUpdateDocument(t testing.TB, w *index.IndexWriter, term *index.Term, doc *document.Document) {
	t.Helper()
	if _, err := w.UpdateDocument(term, doc); err != nil {
		t.Fatalf("updateDocument: %v", err)
	}
}

// searchSortedByLong renders searcher.search(new TermQuery(term), 1, new
// Sort(new SortField(field, SortField.Type.LONG)...)).
func searchSortedByLong(t testing.TB, searcher *search.IndexSearcher, term *index.Term, fields ...string) *search.TopFieldDocs {
	t.Helper()
	sortFields := make([]*search.SortField, len(fields))
	for i, f := range fields {
		sortFields[i] = search.NewSortField(f, spi.SortFieldTypeLong)
	}
	td, err := searcher.SearchWithSortNoScores(search.NewTermQuery(term), 1, search.NewSort(sortFields...))
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	return td
}

// assertSortValue renders assertEquals(expected, ((FieldDoc) td.scoreDocs[0]).fields[i]).
func assertSortValue(t testing.TB, what string, expected int64, td *search.TopFieldDocs, i int) {
	t.Helper()
	if len(td.FieldDocs) == 0 {
		t.Fatalf("%s missing?", what)
	}
	got, ok := td.FieldDocs[0].Fields[i].(int64)
	if !ok || got != expected {
		t.Fatalf("%s value: expected %d, got %v", what, expected, td.FieldDocs[0].Fields[i])
	}
}

func TestNumericDocValuesUpdatesMultipleUpdatesSameDoc(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())

	conf.SetMaxBufferedDocs(3) // small number of docs, so use a tiny maxBufferedDocs

	writer := mustNewIndexWriter(t, dir, conf)

	mustUpdateDocument(t, writer, index.NewTerm("id", "doc-1"), ndvDocVal(t, 1, 1000000000))
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-1"), "val", 1000001111)
	mustUpdateDocument(t, writer, index.NewTerm("id", "doc-2"), ndvDocVal(t, 2, 2000000000))
	mustUpdateDocument(t, writer, index.NewTerm("id", "doc-2"), ndvDocVal(t, 2, 2222222222))
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-1"), "val", 1111111111)

	var reader *index.DirectoryReader
	if rand.Intn(2) == 0 {
		mustCommit(t, writer)
		reader = mustOpenDirectoryReader(t, dir)
	} else {
		reader = openReaderFromWriter(t, writer)
	}
	searcher := search.NewIndexSearcher(reader)

	td := searchSortedByLong(t, searcher, index.NewTerm("id", "doc-1"), "val")
	assertSortValue(t, "doc-1", 1111111111, td, 0)

	td = searchSortedByLong(t, searcher, index.NewTerm("id", "doc-2"), "val")
	assertSortValue(t, "doc-2", 2222222222, td, 0)

	mustClose(t, reader, writer, dir)
}

func TestNumericDocValuesUpdatesBiasedMixOfRandomUpdates(t *testing.T) {
	// 3 types of operations: add, updated, updateDV.
	// rather then randomizing equally, we'll pick (random) cutoffs so each test
	// run is biased, in terms of some ops happen more often then others
	addCutoff := nextInt(1, 98)
	updCutoff := nextInt(addCutoff+1, 99)

	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())

	writer := mustNewIndexWriter(t, dir, conf)

	numOperations := atLeast(1000)
	expected := make(map[int]int64, numOperations/3)

	// start with at least one doc before any chance of updates
	numSeedDocs := atLeast(1)
	for i := 0; i < numSeedDocs; i++ {
		val := javaNextLong()
		expected[i] = val
		mustAddDocument(t, writer, ndvDocVal(t, i, val))
	}

	for i := 0; i < numOperations; i++ {
		op := nextInt(1, 100)
		val := javaNextLong()
		if op <= addCutoff {
			id := len(expected)
			expected[id] = val
			mustAddDocument(t, writer, ndvDocVal(t, id, val))
		} else {
			id := nextInt(0, len(expected)-1)
			expected[id] = val
			if op <= updCutoff {
				mustUpdateDocument(t, writer, index.NewTerm("id", "doc-"+strconv.Itoa(id)), ndvDocVal(t, id, val))
			} else {
				mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-"+strconv.Itoa(id)), "val", val)
			}
		}
	}

	mustCommit(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	searcher := search.NewIndexSearcher(reader)

	// TODO: make more efficient if max numOperations is going to be increased much
	for key, value := range expected {
		id := "doc-" + strconv.Itoa(key)
		td := searchSortedByLong(t, searcher, index.NewTerm("id", id), "val")
		if td.TotalHits.Value != 1 {
			t.Fatalf("%s missing?: totalHits=%d", id, td.TotalHits.Value)
		}
		assertSortValue(t, id, value, td, 0)
	}

	mustClose(t, reader, writer, dir)
}

func TestNumericDocValuesUpdatesUpdatesAreFlushed(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newWhitespaceMockAnalyzerLower(false))
	conf.SetRAMBufferSizeMB(0.00000001)
	writer := mustNewIndexWriter(t, dir, conf)
	mustAddDocument(t, writer, ndvDoc(t, 0)) // val=1
	mustAddDocument(t, writer, ndvDoc(t, 1)) // val=2
	mustAddDocument(t, writer, ndvDoc(t, 3)) // val=2
	mustCommit(t, writer)
	defer mustClose(t, writer, dir)
	t.Fatal("org.apache.lucene.index.IndexWriter#getFlushDeletesCount() is not ported")
}

// nrtOrNot renders the recurring "if (random().nextBoolean()) { // not NRT
// writer.close(); reader = DirectoryReader.open(dir); } else { // NRT reader =
// DirectoryReader.open(writer); writer.close(); }".
func nrtOrNot(t testing.TB, writer *index.IndexWriter, dir store.Directory) *index.DirectoryReader {
	t.Helper()
	if rand.Intn(2) == 0 { // not NRT
		mustClose(t, writer)
		return mustOpenDirectoryReader(t, dir)
	}
	// NRT
	reader := openReaderFromWriter(t, writer)
	mustClose(t, writer)
	return reader
}

func assertNextNumeric(t testing.TB, ndv index.NumericDocValues, doc int, value int64) {
	t.Helper()
	if got, err := ndv.NextDoc(); err != nil || got != doc {
		t.Fatalf("nextDoc: expected %d, got %d (%v)", doc, got, err)
	}
	if v, err := ndv.LongValue(); err != nil || v != value {
		t.Fatalf("doc=%d longValue: expected %d, got %d (%v)", doc, value, v, err)
	}
}

func leafNumeric(t testing.TB, r index.LeafReader, field string) index.NumericDocValues {
	t.Helper()
	ndv, err := r.GetNumericDocValues(field)
	if err != nil {
		t.Fatalf("getNumericDocValues(%s): %v", field, err)
	}
	return ndv
}

func firstLeaf(t testing.TB, r index.IndexReader) index.LeafReader {
	t.Helper()
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if len(leaves) == 0 {
		t.Fatal("reader has no leaves")
	}
	return leaves[0].LeafReader()
}

func TestNumericDocValuesUpdatesSimple(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	// make sure random config doesn't flush on us
	conf.SetMaxBufferedDocs(10)
	conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
	writer := mustNewIndexWriter(t, dir, conf)
	mustAddDocument(t, writer, ndvDoc(t, 0)) // val=1
	mustAddDocument(t, writer, ndvDoc(t, 1)) // val=2
	if rand.Intn(2) == 0 {                   // randomly commit before the update is sent
		mustCommit(t, writer)
	}
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-0"), "val", 2) // doc=0, exp=2

	reader := nrtOrNot(t, writer, dir)

	assertLeafCount(t, 1, reader)
	ndv := leafNumeric(t, firstLeaf(t, reader), "val")
	assertNextNumeric(t, ndv, 0, 2)
	assertNextNumeric(t, ndv, 1, 2)
	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesUpdateFewSegments(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)                    // generate few segments
	conf.SetMergePolicy(index.NewNoMergePolicy()) // prevent merges for this test
	writer := mustNewIndexWriter(t, dir, conf)
	const numDocs = 10
	expectedValues := make([]int64, numDocs)
	for i := 0; i < numDocs; i++ {
		mustAddDocument(t, writer, ndvDoc(t, i))
		expectedValues[i] = int64(i + 1)
	}
	mustCommit(t, writer)

	// update few docs
	for i := 0; i < numDocs; i++ {
		if rand.Float64() < 0.4 {
			value := int64((i + 1) * 2)
			mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-"+strconv.Itoa(i)), "val", value)
			expectedValues[i] = value
		}
	}

	reader := nrtOrNot(t, writer, dir)

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, context := range leaves {
		r := context.LeafReader()
		ndv := leafNumeric(t, r, "val")
		if ndv == nil {
			t.Fatal("assertNotNull(ndv)")
		}
		for i := 0; i < r.MaxDoc(); i++ {
			assertNextNumeric(t, ndv, i, expectedValues[i+context.DocBase])
		}
	}

	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesReopen(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)
	mustAddDocument(t, writer, ndvDoc(t, 0))
	mustAddDocument(t, writer, ndvDoc(t, 1))

	isNRT := rand.Intn(2) == 0
	var reader1 *index.DirectoryReader
	if isNRT {
		reader1 = openReaderFromWriter(t, writer)
	} else {
		mustCommit(t, writer)
		reader1 = mustOpenDirectoryReader(t, dir)
	}

	// update doc
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-0"), "val", 10) // update doc-0's value to 10
	if !isNRT {
		mustCommit(t, writer)
	}

	// reopen reader and assert only it sees the update
	reader2 := openIfChanged(t, reader1)
	if reader2 == nil {
		t.Fatal("assertNotNull(reader2)")
	}
	if reader1 == reader2 {
		t.Fatal("assertTrue(reader1 != reader2)")
	}
	assertNextNumeric(t, leafNumeric(t, firstLeaf(t, reader1), "val"), 0, 1)
	assertNextNumeric(t, leafNumeric(t, firstLeaf(t, reader2), "val"), 0, 10)

	mustClose(t, writer, reader1, reader2, dir)
}

func TestNumericDocValuesUpdatesUpdatesAndDeletes(t *testing.T) {
	// create an index with a segment with only deletes, a segment with both
	// deletes and updates and a segment with only updates
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(10)                   // control segment flushing
	conf.SetMergePolicy(index.NewNoMergePolicy()) // prevent merges for this test
	writer := mustNewIndexWriter(t, dir, conf)

	for i := 0; i < 6; i++ {
		mustAddDocument(t, writer, ndvDoc(t, i))
		if i%2 == 1 {
			mustCommit(t, writer) // create 2-docs segments
		}
	}

	// delete doc-1 and doc-2
	if _, err := writer.DeleteDocuments([]index.Term{*index.NewTerm("id", "doc-1"), *index.NewTerm("id", "doc-2")}); err != nil { // 1st and 2nd segments
		t.Fatalf("deleteDocuments: %v", err)
	}

	// update docs 3 and 5
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-3"), "val", 17)
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-5"), "val", 17)

	reader := nrtOrNot(t, writer, dir)

	liveDocs, err := index.MultiBitsGetLiveDocs(reader)
	if err != nil {
		t.Fatalf("MultiBits.getLiveDocs: %v", err)
	}
	expectedLiveDocs := []bool{true, false, false, true, true, true}
	for i, want := range expectedLiveDocs {
		if liveDocs.Get(i) != want {
			t.Fatalf("liveDocs.get(%d): expected %v", i, want)
		}
	}

	expectedValues := []int64{1, 2, 3, 17, 5, 17}
	ndv, err := index.MultiDocValuesGetNumericValues(reader, "val")
	if err != nil {
		t.Fatalf("MultiDocValues.getNumericValues: %v", err)
	}
	for i, want := range expectedValues {
		assertNextNumeric(t, ndv, i, want)
	}

	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesUpdatesWithDeletes(t *testing.T) {
	// update and delete different documents in the same commit session
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy()) // otherwise a singleton merge could get rid of the delete
	conf.SetMaxBufferedDocs(10)                   // control segment flushing
	writer := mustNewIndexWriter(t, dir, conf)

	mustAddDocument(t, writer, ndvDoc(t, 0))
	mustAddDocument(t, writer, ndvDoc(t, 1))

	if rand.Intn(2) == 0 {
		mustCommit(t, writer)
	}

	mustDeleteTerm(t, writer, "id", "doc-0")
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-1"), "val", 17)

	reader := nrtOrNot(t, writer, dir)

	r := firstLeaf(t, reader)
	if r.GetLiveDocs().Get(0) {
		t.Fatal("assertFalse(r.getLiveDocs().get(0))")
	}
	values := leafNumeric(t, r, "val")
	if got, err := values.Advance(1); err != nil || got != 1 {
		t.Fatalf("advance(1): %d (%v)", got, err)
	}
	if v, err := values.LongValue(); err != nil || v != 17 {
		t.Fatalf("longValue: expected 17, got %d (%v)", v, err)
	}

	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesMultipleDocValuesTypes(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(10) // prevent merges
	writer := mustNewIndexWriter(t, dir, conf)

	for i := 0; i < 4; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "dvUpdateKey", "dv", false))
		doc.Add(numericDVField(t, "ndv", int64(i)))
		doc.Add(binaryDVField(t, "bdv", newBytesRef(strconv.Itoa(i))))
		doc.Add(sortedDVField(t, "sdv", newBytesRef(strconv.Itoa(i))))
		doc.Add(sortedSetDVField(t, "ssdv", newBytesRef(strconv.Itoa(i))))
		doc.Add(sortedSetDVField(t, "ssdv", newBytesRef(strconv.Itoa(i*2))))
		mustAddDocument(t, writer, doc)
	}
	mustCommit(t, writer)

	// update all docs' ndv field
	mustUpdateNumericDocValue(t, writer, index.NewTerm("dvUpdateKey", "dv"), "ndv", 17)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	r := firstLeaf(t, reader)
	ndv := leafNumeric(t, r, "ndv")
	bdv, err := r.GetBinaryDocValues("bdv")
	if err != nil {
		t.Fatalf("getBinaryDocValues: %v", err)
	}
	sdv, err := r.GetSortedDocValues("sdv")
	if err != nil {
		t.Fatalf("getSortedDocValues: %v", err)
	}
	ssdv, err := r.GetSortedSetDocValues("ssdv")
	if err != nil {
		t.Fatalf("getSortedSetDocValues: %v", err)
	}
	for i := 0; i < r.MaxDoc(); i++ {
		assertNextNumeric(t, ndv, i, 17)
		if got, err := bdv.NextDoc(); err != nil || got != i {
			t.Fatalf("bdv.nextDoc: %d (%v)", got, err)
		}
		if term, err := bdv.BinaryValue(); err != nil || string(term) != strconv.Itoa(i) {
			t.Fatalf("bdv.binaryValue: %q (%v)", term, err)
		}
		if got, err := sdv.NextDoc(); err != nil || got != i {
			t.Fatalf("sdv.nextDoc: %d (%v)", got, err)
		}
		ord, err := sdv.OrdValue()
		if err != nil {
			t.Fatalf("ordValue: %v", err)
		}
		if term, err := sdv.LookupOrd(ord); err != nil || string(term) != strconv.Itoa(i) {
			t.Fatalf("sdv.lookupOrd: %q (%v)", term, err)
		}
		if got, err := ssdv.NextDoc(); err != nil || got != i {
			t.Fatalf("ssdv.nextDoc: %d (%v)", got, err)
		}

		ssOrd, err := ssdv.NextOrd()
		if err != nil {
			t.Fatalf("nextOrd: %v", err)
		}
		term, err := ssdv.LookupOrd(ssOrd)
		if err != nil {
			t.Fatalf("lookupOrd: %v", err)
		}
		if n, err := strconv.Atoi(string(term)); err != nil || n != i {
			t.Fatalf("ssdv first value: expected %d, got %q", i, term)
		}
		if i == 0 {
			if ssdv.DocValueCount() != 1 {
				t.Fatalf("docValueCount: expected 1, got %d", ssdv.DocValueCount())
			}
		} else {
			if ssdv.DocValueCount() != 2 {
				t.Fatalf("docValueCount: expected 2, got %d", ssdv.DocValueCount())
			}
			ssOrd, err = ssdv.NextOrd()
			if err != nil {
				t.Fatalf("nextOrd: %v", err)
			}
			term, err = ssdv.LookupOrd(ssOrd)
			if err != nil {
				t.Fatalf("lookupOrd: %v", err)
			}
			if n, err := strconv.Atoi(string(term)); err != nil || n != i*2 {
				t.Fatalf("ssdv second value: expected %d, got %q", i*2, term)
			}
		}
	}

	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesMultipleNumericDocValues(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(10) // prevent merges
	writer := mustNewIndexWriter(t, dir, conf)

	for i := 0; i < 2; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "dvUpdateKey", "dv", false))
		doc.Add(numericDVField(t, "ndv1", int64(i)))
		doc.Add(numericDVField(t, "ndv2", int64(i)))
		mustAddDocument(t, writer, doc)
	}
	mustCommit(t, writer)

	// update all docs' ndv1 field
	mustUpdateNumericDocValue(t, writer, index.NewTerm("dvUpdateKey", "dv"), "ndv1", 17)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	r := firstLeaf(t, reader)
	ndv1 := leafNumeric(t, r, "ndv1")
	ndv2 := leafNumeric(t, r, "ndv2")
	for i := 0; i < r.MaxDoc(); i++ {
		assertNextNumeric(t, ndv1, i, 17)
		assertNextNumeric(t, ndv2, i, int64(i))
	}

	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesDocumentWithNoValue(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	for i := 0; i < 2; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "dvUpdateKey", "dv", false))
		if i == 0 { // index only one document with value
			doc.Add(numericDVField(t, "ndv", 5))
		}
		mustAddDocument(t, writer, doc)
	}
	mustCommit(t, writer)

	// update all docs' ndv field
	mustUpdateNumericDocValue(t, writer, index.NewTerm("dvUpdateKey", "dv"), "ndv", 17)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	r := firstLeaf(t, reader)
	ndv := leafNumeric(t, r, "ndv")
	for i := 0; i < r.MaxDoc(); i++ {
		assertNextNumeric(t, ndv, i, 17)
	}

	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesUpdateNonNumericDocValuesField(t *testing.T) {
	// we don't support adding new fields or updating existing non-numeric-dv
	// fields through numeric updates
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "key", "doc", false))
	doc.Add(newStringFieldNoRandom(t, "foo", "bar", false))
	mustAddDocument(t, writer, doc) // flushed document
	mustCommit(t, writer)
	mustAddDocument(t, writer, doc) // in-memory document

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("key", "doc"), "ndv", 17); err == nil {
		t.Fatal("expected IllegalArgumentException updating the unknown field ndv")
	}

	if _, err := writer.UpdateNumericDocValue(index.NewTerm("key", "doc"), "foo", 17); err == nil {
		t.Fatal("expected IllegalArgumentException updating the non doc-values field foo")
	}

	mustClose(t, writer, dir)
}

func TestNumericDocValuesUpdatesDifferentDVFormatPerField(t *testing.T) {
	// test relies on separate instances of the "same thing"
	t.Fatal(assertingCodecMissing)
}

// sameKeyTwoDocs renders the recurring "one flushed and one in-memory
// document with key:doc and ndv=5" prologue.
func sameKeyTwoDocs(t testing.TB, writer *index.IndexWriter, fields ...document.IndexableField) {
	t.Helper()
	doc := document.NewDocument()
	for _, f := range fields {
		doc.Add(f)
	}
	mustAddDocument(t, writer, doc) // flushed document
	mustCommit(t, writer)
	mustAddDocument(t, writer, doc) // in-memory document
}

func assertAllNumeric(t testing.TB, reader index.IndexReader, field string, value int64) {
	t.Helper()
	ndv, err := index.MultiDocValuesGetNumericValues(reader, field)
	if err != nil {
		t.Fatalf("MultiDocValues.getNumericValues: %v", err)
	}
	for i := 0; i < reader.MaxDoc(); i++ {
		assertNextNumeric(t, ndv, i, value)
	}
}

func TestNumericDocValuesUpdatesUpdateSameDocMultipleTimes(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	sameKeyTwoDocs(t, writer, newStringFieldNoRandom(t, "key", "doc", false), numericDVField(t, "ndv", 5))

	mustUpdateNumericDocValue(t, writer, index.NewTerm("key", "doc"), "ndv", 17) // update existing field
	mustUpdateNumericDocValue(t, writer, index.NewTerm("key", "doc"), "ndv", 3)  // update existing field 2nd time in this commit
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	assertAllNumeric(t, reader, "ndv", 3)
	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesSegmentMerges(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	docid := 0
	numRounds := atLeast(10)
	for rnd := 0; rnd < numRounds; rnd++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "key", "doc", false))
		doc.Add(numericDVField(t, "ndv", -1))
		numDocs := atLeast(30)
		for i := 0; i < numDocs; i++ {
			doc.RemoveField("id")
			doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(docid), true))
			mustAddDocument(t, writer, doc)
			docid++
		}

		value := int64(rnd + 1)
		mustUpdateNumericDocValue(t, writer, index.NewTerm("key", "doc"), "ndv", value)

		if rand.Float64() < 0.2 { // randomly delete one doc
			delID := rand.Intn(docid)
			mustDeleteTerm(t, writer, "id", strconv.Itoa(delID))
		}

		// randomly commit or reopen-IW (or nothing), before forceMerge
		if rand.Float64() < 0.4 {
			mustCommit(t, writer)
		} else if rand.Float64() < 0.1 {
			mustClose(t, writer)
			conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
			writer = mustNewIndexWriter(t, dir, conf)
		}

		// add another document with the current value, to be sure forceMerge
		// has something to merge (for instance, it could be that CMS finished
		// merging all segments down to 1 before the delete was applied, so
		// when forceMerge is called, the index will be with one segment and
		// deletes and some MPs might now merge it, thereby invalidating test's
		// assumption that the reader has no deletes).
		doc = document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(docid), true))
		doc.Add(newStringFieldNoRandom(t, "key", "doc", false))
		doc.Add(numericDVField(t, "ndv", value))
		mustAddDocument(t, writer, doc)
		docid++

		if _, err := writer.ForceMergeWithObserver(1, true); err != nil {
			t.Fatalf("forceMerge(1, true): %v", err)
		}

		var reader *index.DirectoryReader
		if rand.Intn(2) == 0 {
			mustCommit(t, writer)
			reader = mustOpenDirectoryReader(t, dir)
		} else {
			reader = openReaderFromWriter(t, writer)
		}

		assertLeafCount(t, 1, reader)
		r := firstLeaf(t, reader)
		if r.GetLiveDocs() != nil {
			t.Fatal("index should have no deletes after forceMerge")
		}
		ndv := leafNumeric(t, r, "ndv")
		if ndv == nil {
			t.Fatal("assertNotNull(ndv)")
		}
		for i := 0; i < r.MaxDoc(); i++ {
			assertNextNumeric(t, ndv, i, value)
		}
		mustClose(t, reader)
	}

	mustClose(t, writer, dir)
}

func TestNumericDocValuesUpdatesUpdateDocumentByMultipleTerms(t *testing.T) {
	// make sure the order of updates is respected, even when multiple terms
	// affect same document
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	sameKeyTwoDocs(t, writer, newStringFieldNoRandom(t, "k1", "v1", false), newStringFieldNoRandom(t, "k2", "v2", false), numericDVField(t, "ndv", 5))

	mustUpdateNumericDocValue(t, writer, index.NewTerm("k1", "v1"), "ndv", 17)
	mustUpdateNumericDocValue(t, writer, index.NewTerm("k2", "v2"), "ndv", 3)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	assertAllNumeric(t, reader, "ndv", 3)
	mustClose(t, reader, dir)
}

// numericOneSortDoc is the package-private static OneSortDoc.
type numericOneSortDoc struct {
	value     int64
	sortValue int64
	id        int
	deleted   bool
}

func (d *numericOneSortDoc) compareTo(other *numericOneSortDoc) int {
	cmp := compareInt64(d.sortValue, other.sortValue)
	if cmp == 0 {
		cmp = compareInt64(int64(d.id), int64(other.id))
		if util.AssertsEnabled() && !(cmp != 0) {
			panic(util.NewAssertionError("cmp != 0"))
		}
	}
	return cmp
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func TestNumericDocValuesUpdatesSortedIndex(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetIndexSort(index.NewSort(index.NewSortField("sort", index.SortTypeLong)))
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	valueRange := nextInt(1, 1000)
	sortValueRange := nextInt(1, 1000)

	refreshChance := nextInt(5, 200)
	deleteChance := nextInt(2, 100)

	deletedCount := 0

	var docs []*numericOneSortDoc
	r := mustGetReaderRIW(t, w)

	numIters := atLeast(1000)
	for iter := 0; iter < numIters; iter++ {
		value := rand.Intn(valueRange)
		if len(docs) == 0 || rand.Intn(3) == 1 {
			id := len(docs)
			// add new doc
			doc := document.NewDocument()
			doc.Add(newStringField(t, "id", strconv.Itoa(id), true))
			doc.Add(numericDVField(t, "number", int64(value)))
			sortValue := rand.Intn(sortValueRange)
			doc.Add(numericDVField(t, "sort", int64(sortValue)))
			if _, err := w.AddDocument(doc); err != nil {
				t.Fatalf("addDocument: %v", err)
			}

			docs = append(docs, &numericOneSortDoc{id: id, value: int64(value), sortValue: int64(sortValue)})
		} else {
			// update existing doc value
			idToUpdate := rand.Intn(len(docs))
			if _, err := w.UpdateNumericDocValue(index.NewTerm("id", strconv.Itoa(idToUpdate)), "number", int64(value)); err != nil {
				t.Fatalf("updateNumericDocValue: %v", err)
			}

			docs[idToUpdate].value = int64(value)
		}

		if rand.Intn(deleteChance) == 0 {
			idToDelete := rand.Intn(len(docs))
			if _, err := w.DeleteDocuments(index.NewTerm("id", strconv.Itoa(idToDelete))); err != nil {
				t.Fatalf("deleteDocuments: %v", err)
			}
			if !docs[idToDelete].deleted {
				docs[idToDelete].deleted = true
				deletedCount++
			}
		}

		if rand.Intn(refreshChance) == 0 {
			r2 := mustGetReaderRIW(t, w)
			mustClose(t, r)
			r = r2

			liveCount := 0

			leaves, err := r.Leaves()
			if err != nil {
				t.Fatalf("leaves: %v", err)
			}
			for _, ctx := range leaves {
				leafReader := ctx.LeafReader()
				values := leafNumeric(t, leafReader, "number")
				sortValues := leafNumeric(t, leafReader, "sort")
				liveDocs := leafReader.GetLiveDocs()
				storedFields, err := leafReader.StoredFields()
				if err != nil {
					t.Fatalf("storedFields: %v", err)
				}

				lastSortValue := int64(-1 << 63)
				for i := 0; i < leafReader.MaxDoc(); i++ {
					doc := storedDocument(t, storedFields, i)
					idStr := docGet(doc, "id")
					if idStr == nil {
						t.Fatalf("doc %d has no id", i)
					}
					idx, err := strconv.Atoi(*idStr)
					if err != nil {
						t.Fatalf("Integer.parseInt(%q): %v", *idStr, err)
					}
					sortDoc := docs[idx]

					if got, err := values.NextDoc(); err != nil || got != i {
						t.Fatalf("values.nextDoc: %d (%v)", got, err)
					}
					if got, err := sortValues.NextDoc(); err != nil || got != i {
						t.Fatalf("sortValues.nextDoc: %d (%v)", got, err)
					}

					if liveDocs != nil && !liveDocs.Get(i) {
						if !sortDoc.deleted {
							t.Fatalf("assertTrue(sortDoc.deleted) for id %d", idx)
						}
						continue
					}
					if sortDoc.deleted {
						t.Fatalf("assertFalse(sortDoc.deleted) for id %d", idx)
					}

					if v, err := values.LongValue(); err != nil || v != sortDoc.value {
						t.Fatalf("value: expected %d, got %d (%v)", sortDoc.value, v, err)
					}

					sortValue, err := sortValues.LongValue()
					if err != nil {
						t.Fatalf("longValue: %v", err)
					}
					if sortValue != sortDoc.sortValue {
						t.Fatalf("sortValue: expected %d, got %d", sortDoc.sortValue, sortValue)
					}

					if !(sortValue >= lastSortValue) {
						t.Fatalf("assertTrue(sortValue >= lastSortValue): %d < %d", sortValue, lastSortValue)
					}
					lastSortValue = sortValue
					liveCount++
				}
			}

			if len(docs)-deletedCount != liveCount {
				t.Fatalf("liveCount: expected %d, got %d", len(docs)-deletedCount, liveCount)
			}
		}
	}

	mustClose(t, r, w, dir)
}

func TestNumericDocValuesUpdatesManyReopensAndFields(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	lmp := newLogMergePolicy()
	lmp.SetMergeFactor(3) // merge often
	conf.SetMergePolicy(lmp)
	writer := mustNewIndexWriter(t, dir, conf)

	isNRT := rand.Intn(2) == 0
	var reader *index.DirectoryReader
	if isNRT {
		reader = openReaderFromWriter(t, writer)
	} else {
		mustCommit(t, writer)
		reader = mustOpenDirectoryReader(t, dir)
	}

	numFields := rand.Intn(4) + 3 // 3-7
	fieldValues := make([]int64, numFields)
	for i := range fieldValues {
		fieldValues[i] = 1
	}

	numRounds := atLeast(15)
	docID := 0
	for i := 0; i < numRounds; i++ {
		numDocs := atLeast(5)
		for j := 0; j < numDocs; j++ {
			doc := document.NewDocument()
			doc.Add(newStringFieldNoRandom(t, "id", "doc-"+strconv.Itoa(docID), true))
			doc.Add(newStringFieldNoRandom(t, "key", "all", false)) // update key
			// add all fields with their current value
			for f := range fieldValues {
				doc.Add(numericDVField(t, "f"+strconv.Itoa(f), fieldValues[f]))
			}
			mustAddDocument(t, writer, doc)
			docID++
		}

		fieldIdx := rand.Intn(len(fieldValues))

		updateField := "f" + strconv.Itoa(fieldIdx)
		fieldValues[fieldIdx]++
		mustUpdateNumericDocValue(t, writer, index.NewTerm("key", "all"), updateField, fieldValues[fieldIdx])

		if rand.Float64() < 0.2 {
			deleteDoc := rand.Intn(numDocs) // might also delete an already deleted document, ok!
			mustDeleteTerm(t, writer, "id", "doc-"+strconv.Itoa(deleteDoc))
		}

		// verify reader
		if !isNRT {
			mustCommit(t, writer)
		}

		newReader := openIfChanged(t, reader)
		if newReader == nil {
			t.Fatal("assertNotNull(newReader)")
		}
		mustClose(t, reader)
		reader = newReader
		if !(reader.NumDocs() > 0) { // we delete at most one document per round
			t.Fatalf("assertTrue(reader.numDocs() > 0): %d", reader.NumDocs())
		}
		leaves, err := reader.Leaves()
		if err != nil {
			t.Fatalf("leaves: %v", err)
		}
		for _, context := range leaves {
			r := context.LeafReader()
			liveDocs := r.GetLiveDocs()
			for field := range fieldValues {
				f := "f" + strconv.Itoa(field)
				ndv := leafNumeric(t, r, f)
				if ndv == nil {
					t.Fatalf("assertNotNull(ndv) for %s", f)
				}
				maxDoc := r.MaxDoc()
				for doc := 0; doc < maxDoc; doc++ {
					if liveDocs == nil || liveDocs.Get(doc) {
						if got, err := ndv.Advance(doc); err != nil || got != doc {
							t.Fatalf("advanced to wrong doc: expected %d, got %d (%v)", doc, got, err)
						}
						if v, err := ndv.LongValue(); err != nil || v != fieldValues[field] {
							t.Fatalf("invalid value for docID=%d, field=%s: expected %d, got %d (%v)", doc, f, fieldValues[field], v, err)
						}
					}
				}
			}
		}
	}

	mustClose(t, writer, reader, dir)
}

// segmentWithNoDocValuesIndex renders the shared prologue of the two
// testUpdateSegmentWithNoDocValues tests; secondSegmentFoo adds the "foo"
// doc-values field to the second segment.
func segmentWithNoDocValuesIndex(t *testing.T, dir store.Directory, secondSegmentFoo bool) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	// prevent merges, otherwise by the time updates are applied
	// (writer.close()), the segments might have merged and that update
	// becomes legit.
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir, conf)

	// first segment with NDV
	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc0", false))
	doc.Add(numericDVField(t, "ndv", 3))
	mustAddDocument(t, writer, doc)
	doc = document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc4", false)) // document without 'ndv' field
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)

	// second segment with no NDV
	doc = document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc1", false))
	if secondSegmentFoo {
		doc.Add(numericDVField(t, "foo", 3))
	}
	mustAddDocument(t, writer, doc)
	doc = document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc2", false)) // document that isn't updated
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)

	// update document in the first segment - should not affect docsWithField
	// of the document without NDV field
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc0"), "ndv", 5)

	// update document in the second segment - field should be added and we
	// should be able to handle the other document correctly (e.g. no NPE)
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc1"), "ndv", 5)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, context := range leaves {
		ndv := leafNumeric(t, context.LeafReader(), "ndv")
		assertNextNumeric(t, ndv, 0, 5)
		// docID 1 has no ndv value
		if next, err := ndv.NextDoc(); err != nil || !(next > 1) {
			t.Fatalf("assertTrue(ndv.nextDoc() > 1): %d (%v)", next, err)
		}
	}
	mustClose(t, reader)
}

func TestNumericDocValuesUpdatesUpdateSegmentWithNoDocValues(t *testing.T) {
	dir := newDirectory()
	segmentWithNoDocValuesIndex(t, dir, false)
	mustClose(t, dir)
}

func TestNumericDocValuesUpdatesUpdateSegmentWithNoDocValues2(t *testing.T) {
	dir := newDirectory()
	segmentWithNoDocValuesIndex(t, dir, true)
	defer mustClose(t, dir)
	t.Fatal(testUtilCheckIndexMissing)
}

func TestNumericDocValuesUpdatesUpdateSegmentWithPostingButNoDocValues(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	// prevent merges, otherwise by the time updates are applied
	// (writer.close()), the segments might have merged and that update
	// becomes legit.
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir, conf)

	// first segment with ndv and ndv2 fields
	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc0", false))
	doc.Add(numericDVField(t, "ndv", 5))
	doc.Add(newStringFieldNoRandom(t, "ndv2", "10", false))
	doc.Add(numericDVField(t, "ndv2", 10))
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)

	// second segment with no ndv and ndv2 fields
	doc = document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc1", false))
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)

	// update docValues of "ndv" field in the second segment
	// since global "ndv" field is docValues only field this is allowed
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc1"), "ndv", 5)

	// update docValues of "ndv2" field in the second segment
	// since global "ndv2" field is not docValues only field this NOT allowed
	_, err := writer.UpdateNumericDocValue(index.NewTerm("id", "doc1"), "ndv2", 10)
	expectedErrMsg := "Can't update [NUMERIC] doc values; the field [ndv2] must be doc values only field, but is also indexed with postings."
	if err == nil || err.Error() != expectedErrMsg {
		t.Fatalf("expected IllegalArgumentException %q, got %v", expectedErrMsg, err)
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, context := range leaves {
		r := context.LeafReader()
		ndv := leafNumeric(t, r, "ndv")
		for i := 0; i < r.MaxDoc(); i++ {
			assertNextNumeric(t, ndv, i, 5)
		}
	}
	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesUpdateNumericDVFieldWithSameNameAsPostingField(t *testing.T) {
	// this used to fail because FieldInfos.Builder neglected to update
	// globalFieldMaps.docValuesTypes map
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "f", "mock-value", false))
	doc.Add(numericDVField(t, "f", 5))
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)

	_, err := writer.UpdateNumericDocValue(index.NewTerm("f", "mock-value"), "f", 17)
	expectedErrMsg := "Can't update [NUMERIC] doc values; the field [f] must be doc values only field, but is also indexed with postings."
	if err == nil || err.Error() != expectedErrMsg {
		t.Fatalf("expected IllegalArgumentException %q, got %v", expectedErrMsg, err)
	}

	mustClose(t, writer)

	r := mustOpenDirectoryReader(t, dir)
	assertNextNumeric(t, leafNumeric(t, firstLeaf(t, r), "f"), 0, 5)
	mustClose(t, r, dir)
}

// assertControlDoubled renders the final verification of the stress and
// generations tests: control == f * 2 for every live doc.
func assertControlDoubled(t testing.TB, r index.LeafReader, f, cf string, advance bool) {
	t.Helper()
	ndv := leafNumeric(t, r, f)
	control := leafNumeric(t, r, cf)
	liveDocs := r.GetLiveDocs()
	for j := 0; j < r.MaxDoc(); j++ {
		if !advance || liveDocs == nil || liveDocs.Get(j) {
			var gotN, gotC int
			var err error
			if advance {
				if gotN, err = ndv.Advance(j); err == nil {
					gotC, err = control.Advance(j)
				}
			} else {
				if gotN, err = ndv.NextDoc(); err == nil {
					gotC, err = control.NextDoc()
				}
			}
			if err != nil || gotN != j || gotC != j {
				t.Fatalf("positioning at %d: %d/%d (%v)", j, gotN, gotC, err)
			}
			cv, err := control.LongValue()
			if err != nil {
				t.Fatalf("control.longValue: %v", err)
			}
			nv, err := ndv.LongValue()
			if err != nil {
				t.Fatalf("longValue: %v", err)
			}
			if cv != nv*2 {
				t.Fatalf("doc %d: control %d != 2*%d", j, cv, nv)
			}
		}
	}
}

func TestNumericDocValuesUpdatesStressMultiThreading(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	// create index
	numFields := nextInt(1, 4)
	numDocs := atLeast(200)
	if testNightly {
		numDocs = atLeast(2000)
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", "doc"+strconv.Itoa(i), false))
		doc.Add(newStringFieldNoRandom(t, "updKey", updKeyTerm(rand.Float64()).Text(), false))
		for j := 0; j < numFields; j++ {
			value := javaNextInt()
			doc.Add(numericDVField(t, "f"+strconv.Itoa(j), value))
			doc.Add(numericDVField(t, "cf"+strconv.Itoa(j), value*2)) // control, always updated to f * 2
		}
		mustAddDocument(t, writer, doc)
	}

	numThreads := 2
	if testNightly {
		numThreads = nextInt(3, 6)
	}
	var numUpdates atomic.Int64
	numUpdates.Store(int64(atLeast(100)))

	// same thread updates a field as well as reopens
	var wg sync.WaitGroup
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var reader *index.DirectoryReader
			defer func() {
				if reader != nil {
					if err := reader.Close(); err != nil {
						t.Errorf("close: %v", err)
					}
				}
			}()
			for numUpdates.Add(-1) >= 0 {
				term := updKeyTerm(rand.Float64())
				field := rand.Intn(numFields)
				f := "f" + strconv.Itoa(field)
				cf := "cf" + strconv.Itoa(field)
				updValue := javaNextInt()
				nf, err1 := document.NewNumericDocValuesField(f, updValue)
				ncf, err2 := document.NewNumericDocValuesField(cf, updValue*2)
				if err1 != nil || err2 != nil {
					t.Errorf("NumericDocValuesField: %v %v", err1, err2)
					return
				}
				if _, err := writer.UpdateDocValues(term, []*document.Field{nf.Field, ncf.Field}); err != nil {
					t.Errorf("updateDocValues: %v", err)
					return
				}

				if rand.Float64() < 0.2 {
					// delete a random document
					doc := rand.Intn(numDocs)
					if _, err := writer.DeleteDocuments([]index.Term{*index.NewTerm("id", "doc"+strconv.Itoa(doc))}); err != nil {
						t.Errorf("deleteDocuments: %v", err)
						return
					}
				}

				if rand.Float64() < 0.05 { // commit every 20 updates on average
					if _, err := writer.Commit(); err != nil {
						t.Errorf("commit: %v", err)
						return
					}
				}

				if rand.Float64() < 0.1 { // reopen NRT reader (apply updates), on average once every 10 updates
					if reader == nil {
						r, err := index.OpenDirectoryReaderFromWriter(writer)
						if err != nil {
							t.Errorf("DirectoryReader.open(writer): %v", err)
							return
						}
						reader = r
					} else {
						r2, err := index.OpenIfChangedFromWriter(reader, writer)
						if err != nil {
							t.Errorf("openIfChanged: %v", err)
							return
						}
						if r2 != nil {
							if err := reader.Close(); err != nil {
								t.Errorf("close: %v", err)
								return
							}
							reader = r2
						}
					}
				}
			}
		}()
	}

	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, context := range leaves {
		for i := 0; i < numFields; i++ {
			assertControlDoubled(t, context.LeafReader(), "f"+strconv.Itoa(i), "cf"+strconv.Itoa(i), true)
		}
	}
	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesUpdateDifferentDocsInDifferentGens(t *testing.T) {
	// update same document multiple times across generations
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(4)
	writer := mustNewIndexWriter(t, dir, conf)
	numDocs := atLeast(10)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", "doc"+strconv.Itoa(i), false))
		value := javaNextInt()
		doc.Add(numericDVField(t, "f", value))
		doc.Add(numericDVField(t, "cf", value*2))
		mustAddDocument(t, writer, doc)
	}

	numGens := atLeast(5)
	for i := 0; i < numGens; i++ {
		doc := rand.Intn(numDocs)
		term := index.NewTerm("id", "doc"+strconv.Itoa(doc))
		value := javaNextLong()
		mustUpdateDocValues(t, writer, term, numericDVField(t, "f", value).Field, numericDVField(t, "cf", value*2).Field)
		reader := openReaderFromWriter(t, writer)
		leaves, err := reader.Leaves()
		if err != nil {
			t.Fatalf("leaves: %v", err)
		}
		for _, context := range leaves {
			assertControlDoubled(t, context.LeafReader(), "f", "cf", false)
		}
		mustClose(t, reader)
	}
	mustClose(t, writer, dir)
}

func TestNumericDocValuesUpdatesChangeCodec(t *testing.T) {
	t.Fatal(assertingCodecMissing)
}

func randomSimpleStrings(n int) []string {
	r := rand.New(rand.NewSource(rand.Int63()))
	set := map[string]bool{}
	for len(set) < n {
		set[util.RandomSimpleString(r, 0, 10)] = true
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func TestNumericDocValuesUpdatesAddIndexes(t *testing.T) {
	dir1 := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir1, conf)

	numDocs := atLeast(50)
	numTerms := nextInt(1, numDocs/5)
	randomTerms := randomSimpleStrings(numTerms)
	randomFrom := func() string { return randomTerms[rand.Intn(len(randomTerms))] }

	// create first index
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", randomFrom(), false))
		doc.Add(numericDVField(t, "ndv", 4))
		doc.Add(numericDVField(t, "control", 8))
		mustAddDocument(t, writer, doc)
	}

	if rand.Intn(2) == 0 {
		mustCommit(t, writer)
	}

	// update some docs to a random value
	value := javaNextInt()
	term := index.NewTerm("id", randomFrom())
	mustUpdateDocValues(t, writer, term, numericDVField(t, "ndv", value).Field, numericDVField(t, "control", value*2).Field)
	mustClose(t, writer)

	dir2 := newDirectory()
	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer = mustNewIndexWriter(t, dir2, conf)
	if rand.Intn(2) == 0 {
		if _, err := writer.AddIndexes(dir1); err != nil {
			t.Fatalf("addIndexes: %v", err)
		}
	} else {
		mustClose(t, writer, dir1, dir2)
		t.Fatal("TestUtil.addIndexesSlowly(IndexWriter, DirectoryReader...) needs " + addIndexesCodecReadersMissing)
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir2)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, context := range leaves {
		r := context.LeafReader()
		ndv := leafNumeric(t, r, "ndv")
		control := leafNumeric(t, r, "control")
		for i := 0; i < r.MaxDoc(); i++ {
			if got, err := ndv.NextDoc(); err != nil || got != i {
				t.Fatalf("ndv.nextDoc: %d (%v)", got, err)
			}
			if got, err := control.NextDoc(); err != nil || got != i {
				t.Fatalf("control.nextDoc: %d (%v)", got, err)
			}
			nv, _ := ndv.LongValue()
			cv, _ := control.LongValue()
			if nv*2 != cv {
				t.Fatalf("doc %d: control %d != 2*%d", i, cv, nv)
			}
		}
	}
	mustClose(t, reader, dir1, dir2)
}

// leafFieldInfos collects the FieldInfos of every leaf of an NRT reader.
func leafFieldInfos(t testing.TB, writer *index.IndexWriter) []*index.FieldInfos {
	t.Helper()
	reader := openReaderFromWriter(t, writer)
	defer mustClose(t, reader)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	var out []*index.FieldInfos
	for _, leaf := range leaves {
		out = append(out, leaf.LeafReader().GetFieldInfos())
	}
	return out
}

func noMergeIndex(t *testing.T, dir store.Directory, from, to int, add func(doc *document.Document, i int)) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir, conf)
	for i := from; i < to; i++ {
		doc := document.NewDocument()
		doc.Add(newStringFieldNoRandom(t, "id", strconv.Itoa(i), false))
		add(doc, i)
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)
}

func TestNumericDocValuesUpdatesAddNewFieldAfterAddIndexes(t *testing.T) {
	dir1 := newDirectory()
	numDocs := atLeast(50)
	// create first index
	noMergeIndex(t, dir1, 0, numDocs, func(doc *document.Document, _ int) {
		doc.Add(numericDVField(t, "a1", 0))
		doc.Add(numericDVField(t, "a2", 1))
	})

	dir2 := newDirectory()
	// create second index
	noMergeIndex(t, dir2, 0, numDocs, func(doc *document.Document, _ int) {
		doc.Add(numericDVField(t, "i1", 0))
		doc.Add(numericDVField(t, "i2", 1))
		doc.Add(numericDVField(t, "i3", 2))
	})

	mainDir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, mainDir, conf)
	if _, err := writer.AddIndexes(dir1, dir2); err != nil {
		t.Fatalf("addIndexes: %v", err)
	}

	originalFieldInfos := leafFieldInfos(t, writer)
	if !(len(originalFieldInfos) > 0) {
		t.Fatal("assertTrue(originalFieldInfos.size() > 0)")
	}

	// update all doc values
	value := javaNextInt()
	for i := 0; i < numDocs; i++ {
		term := index.NewTerm("id", strconv.Itoa(i))
		mustUpdateDocValues(t, writer, term, numericDVField(t, "ndv", value).Field)
	}

	reader := openReaderFromWriter(t, writer)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for i, leaf := range leaves {
		leafReader := leaf.LeafReader()
		newFieldInfos := leafReader.GetFieldInfos()
		ensureConsistentFieldInfos(t, originalFieldInfos[i], newFieldInfos)
		if fi := newFieldInfos.FieldInfo("ndv"); fi == nil || fi.DocValuesType() != index.DocValuesTypeNumeric {
			t.Fatalf("ndv docValuesType: %v", fi)
		}
		ndv := leafNumeric(t, leafReader, "ndv")
		for docID := 0; docID < leafReader.MaxDoc(); docID++ {
			assertNextNumeric(t, ndv, docID, value)
		}
	}
	mustClose(t, reader, writer, dir1, dir2, mainDir)
}

func TestNumericDocValuesUpdatesUpdatesAfterAddIndexes(t *testing.T) {
	dir1 := newDirectory()
	numDocs := atLeast(50)
	// create first index
	noMergeIndex(t, dir1, 0, numDocs, func(doc *document.Document, _ int) {
		doc.Add(numericDVField(t, "ndv", 4))
		doc.Add(numericDVField(t, "control", 8))
		doc.Add(document.NewLongPoint("i1", 4))
	})

	dir2 := newDirectory()
	// create second index
	noMergeIndex(t, dir2, numDocs, numDocs*2, func(doc *document.Document, _ int) {
		doc.Add(numericDVField(t, "ndv", 2))
		doc.Add(numericDVField(t, "control", 4))
		doc.Add(document.NewLongPoint("i2", 16))
		doc.Add(document.NewLongPoint("i2", 24))
	})

	mainDir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, mainDir, conf)
	if _, err := writer.AddIndexes(dir1, dir2); err != nil {
		t.Fatalf("addIndexes: %v", err)
	}

	originalFieldInfos := leafFieldInfos(t, writer)
	if !(len(originalFieldInfos) > 0) {
		t.Fatal("assertTrue(originalFieldInfos.size() > 0)")
	}

	// update some docs to a random value
	value := javaNextInt()
	term := index.NewTerm("id", strconv.Itoa(rand.Intn(numDocs)*2))
	mustUpdateDocValues(t, writer, term, numericDVField(t, "ndv", value).Field, numericDVField(t, "control", value*2).Field)

	reader := openReaderFromWriter(t, writer)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for i, leaf := range leaves {
		leafReader := leaf.LeafReader()
		newFieldInfos := leafReader.GetFieldInfos()
		ensureConsistentFieldInfos(t, originalFieldInfos[i], newFieldInfos)
		if fi := newFieldInfos.FieldInfo("ndv"); fi == nil || fi.DocValuesType() != index.DocValuesTypeNumeric {
			t.Fatalf("ndv docValuesType: %v", fi)
		}
		if fi := newFieldInfos.FieldInfo("control"); fi == nil || fi.DocValuesType() != index.DocValuesTypeNumeric {
			t.Fatalf("control docValuesType: %v", fi)
		}
		ndv := leafNumeric(t, leafReader, "ndv")
		control := leafNumeric(t, leafReader, "control")
		for docID := 0; docID < leafReader.MaxDoc(); docID++ {
			if got, err := ndv.NextDoc(); err != nil || got != docID {
				t.Fatalf("ndv.nextDoc: %d (%v)", got, err)
			}
			if got, err := control.NextDoc(); err != nil || got != docID {
				t.Fatalf("control.nextDoc: %d (%v)", got, err)
			}
			nv, _ := ndv.LongValue()
			cv, _ := control.LongValue()
			if nv*2 != cv {
				t.Fatalf("doc %d: control %d != 2*%d", docID, cv, nv)
			}
		}
	}
	defer mustClose(t, reader, writer, dir1, dir2, mainDir)
	t.Fatal("org.apache.lucene.document.LongPoint#newExactQuery(String, long) is not ported")
}

// ensureConsistentFieldInfos renders the private
// ensureConsistentFieldInfos(FieldInfos, FieldInfos).
func ensureConsistentFieldInfos(t testing.TB, old, after *index.FieldInfos) {
	t.Helper()
	for it := old.Iterator(); it.HasNext(); {
		fi := it.Next()
		if after.FieldInfoByNumber(fi.Number()) == nil {
			t.Fatalf("assertNotNull(after.fieldInfo(%d))", fi.Number())
		}
		afterFi := after.FieldInfo(fi.Name())
		if afterFi == nil {
			t.Fatalf("assertNotNull(after.fieldInfo(%q))", fi.Name())
		}
		if fi.Number() != afterFi.Number() {
			t.Fatalf("field %s number: %d vs %d", fi.Name(), fi.Number(), afterFi.Number())
		}
		if !(fi.DocValuesGen() <= afterFi.DocValuesGen()) {
			t.Fatalf("field %s docValuesGen: %d > %d", fi.Name(), fi.DocValuesGen(), afterFi.DocValuesGen())
		}
	}
}

func TestNumericDocValuesUpdatesDeleteUnusedUpdatesFiles(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "d0", false))
	doc.Add(numericDVField(t, "f1", 1))
	doc.Add(numericDVField(t, "f2", 1))
	mustAddDocument(t, writer, doc)

	// update each field twice to make sure all unneeded files are deleted
	for _, f := range []string{"f1", "f2"} {
		mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "d0"), f, 2)
		mustCommit(t, writer)
		numFiles := listAllCount(t, dir)

		// update again, number of files shouldn't change (old field's gen is
		// removed)
		mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "d0"), f, 3)
		mustCommit(t, writer)

		if got := listAllCount(t, dir); got != numFiles {
			t.Fatalf("listAll().length: expected %d, got %d", numFiles, got)
		}
	}

	mustClose(t, writer, dir)
}

func TestNumericDocValuesUpdatesTonsOfUpdates(t *testing.T) {
	// LUCENE-5248: make sure that when there are many updates, we don't use
	// too much RAM
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetRAMBufferSizeMB(index.DefaultRAMBufferSizeMB)
	conf.SetMaxBufferedDocs(index.DisableAutoFlush) // don't flush by doc
	writer := mustNewIndexWriter(t, dir, conf)

	// test data: lots of documents (few 10Ks) and lots of update terms (few hundreds)
	numDocs := atLeast(200)
	if testNightly {
		numDocs = atLeast(20000)
	}
	numNumericFields := atLeast(5)
	numTerms := nextInt(10, 100) // terms should affect many docs
	updateTerms := randomSimpleStrings(numTerms)
	randomFrom := func() string { return updateTerms[rand.Intn(len(updateTerms))] }

	// build a large index with many NDV fields and update terms
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		numUpdateTerms := nextInt(1, numTerms/10)
		for j := 0; j < numUpdateTerms; j++ {
			doc.Add(newStringFieldNoRandom(t, "upd", randomFrom(), false))
		}
		for j := 0; j < numNumericFields; j++ {
			val := javaNextInt()
			doc.Add(numericDVField(t, "f"+strconv.Itoa(j), val))
			doc.Add(numericDVField(t, "cf"+strconv.Itoa(j), val*2))
		}
		mustAddDocument(t, writer, doc)
	}

	mustCommit(t, writer) // commit so there's something to apply to

	// set to flush every 2048 bytes (approximately every 12 updates), so we
	// get many flushes during numeric updates
	writer.GetConfig().SetRAMBufferSizeMB(2048.0 / 1024 / 1024)
	numUpdates := atLeast(100)
	for i := 0; i < numUpdates; i++ {
		field := rand.Intn(numNumericFields)
		updateTerm := index.NewTerm("upd", randomFrom())
		value := javaNextInt()
		mustUpdateDocValues(t, writer, updateTerm,
			numericDVField(t, "f"+strconv.Itoa(field), value).Field,
			numericDVField(t, "cf"+strconv.Itoa(field), value*2).Field)
	}

	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	for _, context := range leaves {
		for i := 0; i < numNumericFields; i++ {
			assertControlDoubled(t, context.LeafReader(), "f"+strconv.Itoa(i), "cf"+strconv.Itoa(i), false)
		}
	}
	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesUpdatesOrder(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "upd", "t1", false))
	doc.Add(newStringFieldNoRandom(t, "upd", "t2", false))
	doc.Add(numericDVField(t, "f1", 1))
	doc.Add(numericDVField(t, "f2", 1))
	mustAddDocument(t, writer, doc)
	mustUpdateNumericDocValue(t, writer, index.NewTerm("upd", "t1"), "f1", 2) // update f1 to 2
	mustUpdateNumericDocValue(t, writer, index.NewTerm("upd", "t1"), "f2", 2) // update f2 to 2
	mustUpdateNumericDocValue(t, writer, index.NewTerm("upd", "t2"), "f1", 3) // update f1 to 3
	mustUpdateNumericDocValue(t, writer, index.NewTerm("upd", "t2"), "f2", 3) // update f2 to 3
	mustUpdateNumericDocValue(t, writer, index.NewTerm("upd", "t1"), "f1", 4) // update f1 to 4 (but not f2)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	leaf := firstLeaf(t, reader)
	assertNextNumeric(t, leafNumeric(t, leaf, "f1"), 0, 4)
	assertNextNumeric(t, leafNumeric(t, leaf, "f2"), 0, 3)
	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesUpdateAllDeletedSegment(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc", false))
	doc.Add(numericDVField(t, "f1", 1))
	mustAddDocument(t, writer, doc)
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)
	mustDeleteTerm(t, writer, "id", "doc") // delete all docs in the first segment
	mustAddDocument(t, writer, doc)
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc"), "f1", 2)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	assertLeafCount(t, 1, reader)
	assertNextNumeric(t, leafNumeric(t, firstLeaf(t, reader), "f1"), 0, 2)
	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesUpdateTwoNonexistingTerms(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	writer := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newStringFieldNoRandom(t, "id", "doc", false))
	doc.Add(numericDVField(t, "f1", 1))
	mustAddDocument(t, writer, doc)
	// update w/ multiple nonexisting terms in same field
	mustUpdateNumericDocValue(t, writer, index.NewTerm("c", "foo"), "f1", 2)
	mustUpdateNumericDocValue(t, writer, index.NewTerm("c", "bar"), "f1", 2)
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	assertLeafCount(t, 1, reader)
	assertNextNumeric(t, leafNumeric(t, firstLeaf(t, reader), "f1"), 0, 1)
	mustClose(t, reader, dir)
}

func TestNumericDocValuesUpdatesIOContext(t *testing.T) {
	// LUCENE-5591: make sure we pass an IOContext with an approximate
	// segmentSize in FlushInfo
	dir := newDirectory()
	manualFlushConf := func() *index.IndexWriterConfig {
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		// we want a single large enough segment so that a doc-values update
		// writes a large file
		conf.SetMergePolicy(index.NewNoMergePolicy())
		conf.SetMaxBufferedDocs(int(^uint32(0) >> 1)) // manually flush
		conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
		return conf
	}
	writer := mustNewIndexWriter(t, dir, manualFlushConf())
	for i := 0; i < 100; i++ {
		mustAddDocument(t, writer, ndvDoc(t, i))
	}
	mustCommit(t, writer)
	mustClose(t, writer)

	cachingDir := store.NewNRTCachingDirectory(dir, 100, 1/(1024.*1024.))
	writer = mustNewIndexWriter(t, cachingDir, manualFlushConf())
	mustUpdateNumericDocValue(t, writer, index.NewTerm("id", "doc-0"), "val", 100)
	reader := openReaderFromWriter(t, writer) // flush
	cached, err := cachingDir.ListCachedFiles()
	if err != nil {
		t.Fatalf("listCachedFiles: %v", err)
	}
	if len(cached) != 0 {
		t.Fatalf("listCachedFiles().length: expected 0, got %d (%v)", len(cached), cached)
	}

	mustClose(t, reader, writer, cachingDir)
}
