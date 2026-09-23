// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterDelete.java
// (Apache Lucene 10.5.0). The @Monster and @Nightly tests live in
// index_writer_delete_monster_test.go.

package index_test

import (
	"bytes"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// Missing production members the Java tests reach.
const (
	indexWriterIsDeleterClosedMissing            = "org.apache.lucene.index.IndexWriter#isDeleterClosed() is not ported"
	indexWriterGetBufferedDeleteTermsSizeMissing = "org.apache.lucene.index.IndexWriter#getBufferedDeleteTermsSize() is not ported"
)

// newWhitespaceMockAnalyzerLower renders
// new MockAnalyzer(random(), MockTokenizer.WHITESPACE, lowerCase).
func newWhitespaceMockAnalyzerLower(lowerCase bool) analysis.Analyzer {
	return testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, lowerCase, testanalysis.DefaultMaxTokenLength, nil, true)
}

func deleteTestWriter(t testing.TB, dir store.Directory, maxBufferedDocs int) *index.IndexWriter {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newWhitespaceMockAnalyzerLower(false))
	if maxBufferedDocs > 0 {
		conf.SetMaxBufferedDocs(maxBufferedDocs)
	}
	return mustNewIndexWriter(t, dir, conf)
}

// simpleCaseDocs renders the keyword/unindexed/unstored/text document loop
// of testSimpleCase and testErrorInDocsWriterAdd.
func simpleCaseDocs(t testing.TB) []*document.Document {
	keywords := []string{"1", "2"}
	unindexed := []string{"Netherlands", "Italy"}
	unstored := []string{"Amsterdam has lots of bridges", "Venice has lots of canals"}
	text := []string{"Amsterdam", "Venice"}

	custom1 := document.NewFieldType()
	custom1.SetStored(true)
	docs := make([]*document.Document, len(keywords))
	for i := range keywords {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", keywords[i], true))
		doc.Add(newField(t, "country", unindexed[i], custom1))
		doc.Add(newTextField(t, "contents", unstored[i], false))
		doc.Add(newTextField(t, "city", text[i], true))
		docs[i] = doc
	}
	return docs
}

// test the simple case
func TestIndexWriterDeleteSimpleCase(t *testing.T) {
	dir := newDirectory()
	modifier := deleteTestWriter(t, dir, 0)

	for _, doc := range simpleCaseDocs(t) {
		mustAddDocument(t, modifier, doc)
	}
	if err := modifier.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustCommit(t, modifier)

	term := index.NewTerm("city", "Amsterdam")
	defer mustClose(t, modifier, dir)
	indexWriterDeleteGetHitCount(t, dir, term)
}

// test when delete terms only apply to disk segments
func TestIndexWriterDeleteNonRAMDelete(t *testing.T) {
	dir := newDirectory()
	modifier := deleteTestWriter(t, dir, 2)
	id := 0
	value := 100

	for i := 0; i < 7; i++ {
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
	}
	mustCommit(t, modifier)

	defer mustClose(t, modifier, dir)
	t.Fatal(indexWriterGetNumBufferedDocumentsMissing)
}

// test when delete terms only apply to ram segments
func TestIndexWriterDeleteRAMDeletes(t *testing.T) {
	for pass := 0; pass < 2; pass++ {
		dir := newDirectory()
		modifier := deleteTestWriter(t, dir, 4)
		id := 0
		value := 100

		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
		if pass == 0 {
			mustDeleteTerm(t, modifier, "value", strconv.Itoa(value))
		} else {
			mustDeleteQuery(t, modifier, search.NewTermQuery(index.NewTerm("value", strconv.Itoa(value))))
		}
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
		if pass == 0 {
			mustDeleteTerm(t, modifier, "value", strconv.Itoa(value))
			mustClose(t, modifier, dir)
			t.Fatal(indexWriterGetBufferedDeleteTermsSizeMissing)
		} else {
			mustDeleteQuery(t, modifier, search.NewTermQuery(index.NewTerm("value", strconv.Itoa(value))))
		}

		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
		assertSegmentCount(t, 0, modifier)
		mustCommit(t, modifier)

		reader := mustOpenDirectoryReader(t, dir)
		assertReaderNumDocs(t, 1, reader)

		defer mustClose(t, reader, modifier, dir)
		indexWriterDeleteGetHitCount(t, dir, index.NewTerm("id", strconv.Itoa(id)))
	}
}

func mustDeleteQuery(t testing.TB, w *index.IndexWriter, q index.Query) {
	t.Helper()
	if _, err := w.DeleteDocumentsQuery([]index.Query{q}); err != nil {
		t.Fatalf("deleteDocuments(Query): %v", err)
	}
}

// test when delete terms apply to both disk and ram segments
func TestIndexWriterDeleteBothDeletes(t *testing.T) {
	dir := newDirectory()
	modifier := deleteTestWriter(t, dir, 100)

	id := 0
	value := 100

	for i := 0; i < 5; i++ {
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
	}

	value = 200
	for i := 0; i < 5; i++ {
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
	}
	mustCommit(t, modifier)

	for i := 0; i < 5; i++ {
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
	}
	mustDeleteTerm(t, modifier, "value", strconv.Itoa(value))

	mustCommit(t, modifier)

	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, 5, reader)
	mustClose(t, modifier, reader, dir)
}

// test that batched delete terms are flushed together
func TestIndexWriterDeleteBatchDeletes(t *testing.T) {
	dir := newDirectory()
	modifier := deleteTestWriter(t, dir, 2)

	id := 0
	value := 100

	for i := 0; i < 7; i++ {
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
	}
	mustCommit(t, modifier)

	assertDirNumDocs(t, dir, 7)

	id = 0
	id++
	mustDeleteTerm(t, modifier, "id", strconv.Itoa(id))
	id++
	mustDeleteTerm(t, modifier, "id", strconv.Itoa(id))

	mustCommit(t, modifier)

	assertDirNumDocs(t, dir, 5)

	terms := make([]index.Term, 3)
	for i := range terms {
		id++
		terms[i] = *index.NewTerm("id", strconv.Itoa(id))
	}
	if _, err := modifier.DeleteDocuments(terms); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}
	mustCommit(t, modifier)
	assertDirNumDocs(t, dir, 2)

	mustClose(t, modifier, dir)
}

func assertDirNumDocs(t testing.TB, dir store.Directory, expected int) {
	t.Helper()
	reader := mustOpenDirectoryReader(t, dir)
	assertReaderNumDocs(t, expected, reader)
	mustClose(t, reader)
}

// test deleteAll()
func TestIndexWriterDeleteDeleteAllSimple(t *testing.T) {
	dir := newDirectory()
	modifier := deleteTestWriter(t, dir, 2)

	id := 0
	value := 100

	for i := 0; i < 7; i++ {
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
	}
	mustCommit(t, modifier)

	assertDirNumDocs(t, dir, 7)

	// Add 1 doc (so we will have something buffered)
	indexWriterDeleteAddDoc(t, modifier, 99, value)

	// Delete all
	mustDeleteAll(t, modifier)

	// Delete all shouldn't be on disk yet
	assertDirNumDocs(t, dir, 7)

	// Add a doc and update a doc (after the deleteAll, before the commit)
	indexWriterDeleteAddDoc(t, modifier, 101, value)
	indexWriterDeleteUpdateDoc(t, modifier, 102, value)

	// commit the delete all
	mustCommit(t, modifier)

	// Validate there are no docs left
	assertDirNumDocs(t, dir, 2)

	mustClose(t, modifier, dir)
}

func TestIndexWriterDeleteDeleteAllNoDeadLock(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal("org.apache.lucene.tests.index.MockRandomMergePolicy is not ported")
}

// test rollback of deleteAll()
func TestIndexWriterDeleteDeleteAllRollback(t *testing.T) {
	dir := newDirectory()
	modifier := deleteTestWriter(t, dir, 2)

	id := 0
	value := 100

	for i := 0; i < 7; i++ {
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
	}
	mustCommit(t, modifier)

	id++
	indexWriterDeleteAddDoc(t, modifier, id, value)

	assertDirNumDocs(t, dir, 7)

	// Delete all
	mustDeleteAll(t, modifier)

	// Roll it back
	if err := modifier.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Validate that the docs are still there
	assertDirNumDocs(t, dir, 7)

	mustClose(t, dir)
}

// test deleteAll() w/ near real-time reader
func TestIndexWriterDeleteDeleteAllNRT(t *testing.T) {
	dir := newDirectory()
	modifier := deleteTestWriter(t, dir, 2)

	id := 0
	value := 100

	for i := 0; i < 7; i++ {
		id++
		indexWriterDeleteAddDoc(t, modifier, id, value)
	}
	mustCommit(t, modifier)

	reader := openReaderFromWriter(t, modifier)
	assertReaderNumDocs(t, 7, reader)
	mustClose(t, reader)

	id++
	indexWriterDeleteAddDoc(t, modifier, id, value)
	id++
	indexWriterDeleteAddDoc(t, modifier, id, value)

	// Delete all
	mustDeleteAll(t, modifier)

	reader = openReaderFromWriter(t, modifier)
	assertReaderNumDocs(t, 0, reader)
	mustClose(t, reader)

	// Roll it back
	if err := modifier.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	// Validate that the docs are still there
	assertDirNumDocs(t, dir, 7)

	mustClose(t, dir)
}

func indexWriterDeleteDoc(t testing.TB, id, value int) *document.Document {
	doc := document.NewDocument()
	doc.Add(newTextField(t, "content", "aaa", false))
	doc.Add(newStringField(t, "id", strconv.Itoa(id), true))
	doc.Add(newStringField(t, "value", strconv.Itoa(value), false))
	doc.Add(numericDVField(t, "dv", int64(value)))
	return doc
}

// indexWriterDeleteUpdateDoc renders the private updateDoc(IndexWriter, int, int).
func indexWriterDeleteUpdateDoc(t testing.TB, modifier *index.IndexWriter, id, value int) {
	t.Helper()
	if _, err := modifier.UpdateDocument(index.NewTerm("id", strconv.Itoa(id)), indexWriterDeleteDoc(t, id, value)); err != nil {
		t.Fatalf("updateDocument: %v", err)
	}
}

// indexWriterDeleteAddDoc renders the private addDoc(IndexWriter, int, int).
func indexWriterDeleteAddDoc(t testing.TB, modifier *index.IndexWriter, id, value int) {
	t.Helper()
	mustAddDocument(t, modifier, indexWriterDeleteDoc(t, id, value))
}

// indexWriterDeleteGetHitCount renders the private getHitCount(Directory,
// Term), whose searcher comes from newSearcher.
func indexWriterDeleteGetHitCount(t testing.TB, dir store.Directory, term *index.Term) int64 {
	t.Helper()
	reader := mustOpenDirectoryReader(t, dir)
	searcher := newSearcher(t, reader)
	topDocs, err := searcher.Search(search.NewTermQuery(term), 1000)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	mustClose(t, reader)
	return topDocs.TotalHits.Value
}

// testIndexWriterDeleteErrorAfterApplyDeletes renders the @Ignore'd
// testErrorAfterApplyDeletes: JUnit never runs it, so it has no Test entry
// point. Its MockDirectoryWrapper.Failure relies on
// callStackContainsAnyOf(String...), which is not ported.
func testIndexWriterDeleteErrorAfterApplyDeletes(t *testing.T) {
	t.Fatal("MockDirectoryWrapper.Failure#callStackContainsAnyOf(String...) is not ported")
}

// This test tests that the files created by the docs writer before
// a segment is written are cleaned up if there's an i/o error
func TestIndexWriterDeleteErrorInDocsWriterAdd(t *testing.T) {
	failed := false
	failure := &store.Failure{}
	failure.SetEval(func(*store.MockDirectoryWrapper) error {
		if !failed {
			failed = true
			return errFailInAddDoc
		}
		return nil
	})

	// create a couple of files
	dir := newDirectory()
	modifier := deleteTestWriter(t, dir, 0)
	mustCommit(t, modifier)
	failed = false // failure.reset()
	dir.FailOn(failure)

	for _, doc := range simpleCaseDocs(t) {
		if _, err := modifier.AddDocument(doc); err != nil {
			break
		}
	}
	defer mustClose(t, dir)
	t.Fatal(indexWriterIsDeleterClosedMissing)
}

var errFailInAddDoc = &failInAddDocError{}

type failInAddDocError struct{}

func (*failInAddDocError) Error() string { return "fail in add doc" }

func TestIndexWriterDeleteDeleteNullQuery(t *testing.T) {
	dir := newDirectory()
	modifier := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(newWhitespaceMockAnalyzerLower(false)))

	for i := 0; i < 5; i++ {
		indexWriterDeleteAddDoc(t, modifier, i, 2*i)
	}

	mustDeleteQuery(t, modifier, search.NewTermQuery(index.NewTerm("nada", "nada")))
	mustCommit(t, modifier)
	assertWriterDocStats(t, modifier, -1, 5)
	mustClose(t, modifier, dir)
}

func TestIndexWriterDeleteDeleteAllSlowly(t *testing.T) {
	dir := newDirectory()
	w := newRandomIndexWriter(t, dir)
	numDocs := atLeast(1000)
	ids := make([]int, numDocs)
	for id := range ids {
		ids[id] = id
	}
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	for _, id := range ids {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(id), false))
		if _, err := w.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	upto := 0
	for upto < len(ids) {
		left := len(ids) - upto
		inc := min(left, nextInt(1, 20))
		limit := upto + inc
		for upto < limit {
			if _, err := w.DeleteDocuments(index.NewTerm("id", strconv.Itoa(ids[upto]))); err != nil {
				t.Fatalf("deleteDocuments: %v", err)
			}
			upto++
		}
		r := mustGetReaderRIW(t, w)
		assertReaderNumDocs(t, numDocs-upto, r)
		mustClose(t, r)
	}

	mustClose(t, w, dir)
}

// LUCENE-3340: make sure deletes that we don't apply
// during flush (ie are just pushed into the stream) are
// in fact later flushed due to their RAM usage:
func TestIndexWriterDeleteFlushPushedDeletesByRAM(t *testing.T) {
	dir := newDirectory()
	// Cannot use RandomIndexWriter because we don't want to
	// ever call commit() for this test:
	// note: tiny RAM buffer used, as with a 1MB buffer the test is too slow (flush @ 128,999)
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetRAMBufferSizeMB(0.5)
	conf.SetMaxBufferedDocs(1000)
	conf.SetMergePolicy(index.NewNoMergePolicy())
	conf.SetReaderPooling(false)
	w := mustNewIndexWriter(t, dir, conf)
	count := 0
	for {
		doc := document.NewDocument()
		doc.Add(mustNewStringField(t, "id", strconv.Itoa(count)))
		var delTerm *index.Term
		if count == 1010 {
			// This is the only delete that applies
			delTerm = index.NewTerm("id", "0")
		} else {
			// These get buffered, taking up RAM, but delete
			// nothing when applied:
			delTerm = index.NewTerm("id", "x"+strconv.Itoa(count))
		}
		if _, err := w.UpdateDocument(delTerm, doc); err != nil {
			t.Fatalf("updateDocument: %v", err)
		}
		// Eventually segment 0 should get a del docs:
		// TODO: fix this test
		if slowFileExists(t, dir, "_0_1.del") || slowFileExists(t, dir, "_0_1.liv") {
			break
		}
		count++

		// Today we applyDeletes @ count=21553; even if we make
		// sizable improvements to RAM efficiency of buffered
		// del term we're unlikely to go over 100K:
		if count > 100000 {
			mustClose(t, w, dir)
			t.Fatal("delete's were not applied")
		}
	}
	mustClose(t, w, dir)
}

// mustNewStringField renders new StringField(name, value, Field.Store.NO).
func mustNewStringField(t testing.TB, name, value string) *document.StringField {
	t.Helper()
	f, err := document.NewStringField(name, value, false)
	if err != nil {
		t.Fatalf("StringField: %v", err)
	}
	return f
}

// LUCENE-4455
func TestIndexWriterDeleteDeletesCheckIndexOutput(t *testing.T) {
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	iwc.SetMaxBufferedDocs(2)
	w := mustNewIndexWriter(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(newField(t, "field", "0", document.StringFieldTypeNotStored))
	mustAddDocument(t, w, doc)

	doc = document.NewDocument()
	doc.Add(newField(t, "field", "1", document.StringFieldTypeNotStored))
	mustAddDocument(t, w, doc)
	mustCommit(t, w)
	assertSegmentCount(t, 1, w)

	mustDeleteTerm(t, w, "field", "0")
	mustCommit(t, w)
	assertSegmentCount(t, 1, w)
	mustClose(t, w)

	s := checkIndexOutput(t, dir)

	// Segment should have deletions:
	if !strings.Contains(s, "has deletions") {
		t.Fatalf("assertTrue(s.contains(\"has deletions\")): %s", s)
	}
	iwc = index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w = mustNewIndexWriter(t, dir, iwc)
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, w)

	s = checkIndexOutput(t, dir)
	if strings.Contains(s, "has deletions") {
		t.Fatalf("assertFalse(s.contains(\"has deletions\")): %s", s)
	}
	mustClose(t, dir)
}

// checkIndexOutput runs CheckIndex over dir with a non-verbose info stream,
// asserts the index is clean and returns the captured output.
func checkIndexOutput(t testing.TB, dir store.Directory) string {
	t.Helper()
	var bos bytes.Buffer
	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("new CheckIndex: %v", err)
	}
	checker.SetInfoStream(&bos, false)
	indexStatus, err := checker.CheckIndex(nil)
	if err != nil {
		t.Fatalf("checkIndex: %v", err)
	}
	if !indexStatus.Clean {
		t.Fatalf("assertTrue(indexStatus.clean): %s", bos.String())
	}
	if err := checker.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return bos.String()
}

func TestIndexWriterDeleteTryDeleteDocument(t *testing.T) {
	d := newDirectory()

	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, d, iwc)
	doc := document.NewDocument()
	mustAddDocument(t, w, doc)
	mustAddDocument(t, w, doc)
	mustAddDocument(t, w, doc)
	mustClose(t, w)

	iwc = index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(index.NewNoMergePolicy())
	iwc.SetOpenMode(index.Append)
	w = mustNewIndexWriter(t, d, iwc)
	r, err := index.OpenDirectoryReaderFromWriterWithOptions(w, false, false)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w, false, false): %v", err)
	}
	defer mustClose(t, r, w, d)
	t.Fatal(tryDeleteDocumentMissing)
}

func TestIndexWriterDeleteNRTIsCurrentAfterDelete(t *testing.T) {
	d := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	w := mustNewIndexWriter(t, d, iwc)
	doc := document.NewDocument()
	for i := 0; i < 5; i++ {
		mustAddDocument(t, w, doc)
	}
	doc.Add(newStringFieldNoRandom(t, "id", "1", true))
	mustAddDocument(t, w, doc)
	mustClose(t, w)
	iwc = index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetOpenMode(index.Append)
	w = mustNewIndexWriter(t, d, iwc)
	r, err := index.OpenDirectoryReaderFromWriterWithOptions(w, false, false)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w, false, false): %v", err)
	}
	mustDeleteTerm(t, w, "id", "1")
	r2, err := index.OpenDirectoryReaderFromWriterWithOptions(w, true, true)
	if err != nil {
		t.Fatalf("DirectoryReader.open(w, true, true): %v", err)
	}
	if current, err := r.IsCurrent(); err != nil || current {
		t.Fatalf("assertFalse(r.isCurrent()): %v (%v)", current, err)
	}
	if current, err := r2.IsCurrent(); err != nil || !current {
		t.Fatalf("assertTrue(r2.isCurrent()): %v (%v)", current, err)
	}
	mustClose(t, r, r2, w, d)
}

// newStringFieldNoRandom renders new StringField(name, value, store).
func newStringFieldNoRandom(t testing.TB, name, value string, stored bool) *document.StringField {
	t.Helper()
	f, err := document.NewStringField(name, value, stored)
	if err != nil {
		t.Fatalf("StringField: %v", err)
	}
	return f
}

// onlyDeletesWriter renders the writer of the testOnlyDeletes* and
// testMergingAfterDeleteAll tests.
func onlyDeletesWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxBufferedDocs(2)
	mp := index.NewLogDocMergePolicy()
	mp.SetMinMergeDocs(1)
	iwc.SetMergePolicy(mp)
	iwc.SetMergeScheduler(index.NewSerialMergeScheduler())
	return mustNewIndexWriter(t, dir, iwc)
}

func addIDDocs(t testing.TB, w *index.IndexWriter, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		mustAddDocument(t, w, doc)
	}
}

func deleteIDs(t testing.TB, w *index.IndexWriter, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		mustDeleteTerm(t, w, "id", strconv.Itoa(i))
	}
}

func TestIndexWriterDeleteOnlyDeletesTriggersMergeOnClose(t *testing.T) {
	dir := newDirectory()
	w := onlyDeletesWriter(t, dir)
	addIDDocs(t, w, 38)
	mustCommit(t, w)

	deleteIDs(t, w, 18)

	mustClose(t, w)
	r := mustOpenDirectoryReader(t, dir)
	assertLeafCount(t, 1, r)
	mustClose(t, r, dir)
}

func TestIndexWriterDeleteOnlyDeletesTriggersMergeOnGetReader(t *testing.T) {
	dir := newDirectory()
	w := onlyDeletesWriter(t, dir)
	addIDDocs(t, w, 38)
	mustCommit(t, w)

	deleteIDs(t, w, 18)

	// First one triggers, but does not reflect, the merge:
	mustClose(t, openReaderFromWriter(t, w))
	r := openReaderFromWriter(t, w)
	assertLeafCount(t, 1, r)
	mustClose(t, r, w, dir)
}

func TestIndexWriterDeleteOnlyDeletesTriggersMergeOnFlush(t *testing.T) {
	dir := newDirectory()
	w := onlyDeletesWriter(t, dir)
	addIDDocs(t, w, 38)
	mustCommit(t, w)

	// Deleting 18 out of the 20 docs in the first segment make it the same
	// "level" as the other 9 which should cause a merge to kick off:
	deleteIDs(t, w, 18)
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	assertLeafCount(t, 1, r)
	mustClose(t, r, dir)
}

func TestIndexWriterDeleteOnlyDeletesDeleteAllDocs(t *testing.T) {
	dir := newDirectory()
	w := onlyDeletesWriter(t, dir)
	addIDDocs(t, w, 38)
	mustCommit(t, w)

	deleteIDs(t, w, 38)

	r := openReaderFromWriter(t, w)
	assertLeafCount(t, 0, r)
	if r.MaxDoc() != 0 {
		t.Fatalf("maxDoc: expected 0, got %d", r.MaxDoc())
	}
	mustClose(t, r, w, dir)
}

// Make sure merges still kick off after IW.deleteAll!
func TestIndexWriterDeleteMergingAfterDeleteAll(t *testing.T) {
	dir := newDirectory()
	w := onlyDeletesWriter(t, dir)
	addIDDocs(t, w, 10)
	mustCommit(t, w)
	mustDeleteAll(t, w)

	addIDDocs(t, w, 100)

	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	r := openReaderFromWriter(t, w)
	assertLeafCount(t, 1, r)
	mustClose(t, r, w, dir)
}
