// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterMaxDocs.java
// (Apache Lucene 10.5.0). The @Monster testExactlyAtTrueLimit and the @Nightly
// testAddTooManyIndexesDir live in index_writer_max_docs_monster_test.go.

package index_test

import (
	"errors"
	"math"
	"strconv"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// byteBuffersDirectoryLockFactoryMissing names what
// LuceneTestCase.newDirectory(Random, LockFactory) builds.
const byteBuffersDirectoryLockFactoryMissing = "org.apache.lucene.store.ByteBuffersDirectory(LockFactory) " +
	"(reached by LuceneTestCase.newDirectory(Random, LockFactory)) is not ported"

func nullAnalyzerWriter(t testing.TB, dir store.Directory) *index.IndexWriter {
	t.Helper()
	return mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(nil))
}

// expectAddDocumentIAEEmpty renders
// expectThrows(IllegalArgumentException.class, () -> w.addDocument(new Document())).
func expectAddDocumentIAEEmpty(t testing.TB, w *index.IndexWriter) {
	t.Helper()
	expectAddDocumentIAE(t, w.AddDocument, document.NewDocument())
}

// maxDocsTen renders the shared prologue of the setIndexWriterMaxDocs(10)
// tests that fill an index with ten empty documents.
func maxDocsTen(t *testing.T) (store.Directory, *index.IndexWriter) {
	t.Helper()
	setIndexWriterMaxDocs(t, 10)
	t.Cleanup(func() { restoreIndexWriterMaxDocs(t) })
	dir := newDirectory()
	w := nullAnalyzerWriter(t, dir)
	for i := 0; i < 10; i++ {
		mustAddDocument(t, w, document.NewDocument())
	}
	return dir, w
}

func TestIndexWriterMaxDocsAddDocument(t *testing.T) {
	dir, w := maxDocsTen(t)

	// 11th document should fail:
	expectAddDocumentIAEEmpty(t, w)

	mustClose(t, w, dir)
}

func TestIndexWriterMaxDocsAddDocuments(t *testing.T) {
	dir, w := maxDocsTen(t)

	// 11th document should fail:
	if _, err := w.AddDocuments([]*document.Document{document.NewDocument()}); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocuments")
	}

	mustClose(t, w, dir)
}

func TestIndexWriterMaxDocsUpdateDocument(t *testing.T) {
	dir, w := maxDocsTen(t)

	// 11th document should fail:
	if _, err := w.UpdateDocument(index.NewTerm("field", "foo"), document.NewDocument()); err == nil {
		t.Fatal("expected IllegalArgumentException from updateDocument")
	}

	mustClose(t, w, dir)
}

func TestIndexWriterMaxDocsUpdateDocuments(t *testing.T) {
	dir, w := maxDocsTen(t)

	// 11th document should fail:
	if _, err := w.UpdateDocuments(index.NewTerm("field", "foo"), []*document.Document{document.NewDocument()}); err == nil {
		t.Fatal("expected IllegalArgumentException from updateDocuments")
	}

	mustClose(t, w, dir)
}

// reclaimedDeletes renders the shared body of the two reclaimed-deletes
// tests; newSegmentEvery2 makes a new segment every 2 docs under
// NoMergePolicy.
func reclaimedDeletes(t *testing.T, newSegmentEvery2 bool) {
	t.Helper()
	setIndexWriterMaxDocs(t, 10)
	defer restoreIndexWriterMaxDocs(t)
	dir := newDirectory()
	iwc := index.NewIndexWriterConfigWithAnalyzer(nil)
	if newSegmentEvery2 {
		iwc.SetMergePolicy(index.NewNoMergePolicy())
	}
	w := mustNewIndexWriter(t, dir, iwc)
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		mustAddDocument(t, w, doc)
		if newSegmentEvery2 && i%2 == 0 {
			// Make a new segment every 2 docs:
			mustCommit(t, w)
		}
	}

	// Delete 5 of them:
	for i := 0; i < 5; i++ {
		mustDeleteTerm(t, w, "id", strconv.Itoa(i))
	}

	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	assertWriterDocStats(t, w, 5, -1)

	// Add 5 more docs
	for i := 0; i < 5; i++ {
		mustAddDocument(t, w, document.NewDocument())
	}

	// 11th document should fail:
	expectAddDocumentIAEEmpty(t, w)

	mustClose(t, w, dir)
}

func TestIndexWriterMaxDocsReclaimedDeletes(t *testing.T) {
	reclaimedDeletes(t, false)
}

// Tests that 100% deleted segments (which IW "specializes" by dropping
// entirely) are not mis-counted
func TestIndexWriterMaxDocsReclaimedDeletesWholeSegments(t *testing.T) {
	reclaimedDeletes(t, true)
}

func TestIndexWriterMaxDocsAddIndexes(t *testing.T) {
	setIndexWriterMaxDocs(t, 10)
	defer restoreIndexWriterMaxDocs(t)
	dir := newDirectory()
	w := nullAnalyzerWriter(t, dir)
	for i := 0; i < 10; i++ {
		mustAddDocument(t, w, document.NewDocument())
	}
	mustClose(t, w)

	dir2 := newDirectory()
	w2 := nullAnalyzerWriter(t, dir2)
	mustAddDocument(t, w2, document.NewDocument())
	if _, err := w2.AddIndexes(dir); err == nil {
		t.Fatal("expected IllegalArgumentException from addIndexes(Directory...)")
	}

	assertWriterDocStats(t, w2, 1, -1)
	ir := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, w2, ir, dir, dir2)
	t.Fatal("TestUtil.addIndexesSlowly(IndexWriter, DirectoryReader...) needs " + addIndexesCodecReadersMissing)
}

// multiReaderLimitSetup renders the shared prologue of the two MultiReader
// limit tests: 100000 docs in one index, remainder (+extra) in another.
func multiReaderLimitSetup(t *testing.T, extra int) (store.Directory, store.Directory, *index.DirectoryReader, *index.DirectoryReader, []spi.IndexReaderInterface) {
	t.Helper()
	dir := newDirectory()
	doc := document.NewDocument()
	w := nullAnalyzerWriter(t, dir)
	for i := 0; i < 100000; i++ {
		mustAddDocument(t, w, doc)
	}
	mustClose(t, w)

	remainder := index.MaxDocs%100000 + extra
	dir2 := newDirectory()
	w = nullAnalyzerWriter(t, dir2)
	for i := 0; i < remainder; i++ {
		mustAddDocument(t, w, doc)
	}
	mustClose(t, w)

	copies := index.MaxDocs / 100000

	ir := mustOpenDirectoryReader(t, dir)
	ir2 := mustOpenDirectoryReader(t, dir2)
	subReaders := make([]spi.IndexReaderInterface, copies+1)
	for i := range subReaders {
		subReaders[i] = ir
	}
	subReaders[len(subReaders)-1] = ir2
	return dir, dir2, ir, ir2, subReaders
}

// Make sure MultiReader lets you search exactly the limit number of docs:
func TestIndexWriterMaxDocsMultiReaderExactLimit(t *testing.T) {
	dir, dir2, ir, ir2, subReaders := multiReaderLimitSetup(t, 0)

	mr, err := index.NewMultiReader(subReaders)
	if err != nil {
		t.Fatalf("new MultiReader: %v", err)
	}
	if mr.MaxDoc() != index.MaxDocs {
		t.Fatalf("maxDoc: expected %d, got %d", index.MaxDocs, mr.MaxDoc())
	}
	if mr.NumDocs() != index.MaxDocs {
		t.Fatalf("numDocs: expected %d, got %d", index.MaxDocs, mr.NumDocs())
	}
	mustClose(t, ir, ir2, dir, dir2)
}

// Make sure MultiReader is upset if you exceed the limit
func TestIndexWriterMaxDocsMultiReaderBeyondLimit(t *testing.T) {
	// One too many:
	dir, dir2, ir, ir2, subReaders := multiReaderLimitSetup(t, 1)

	if _, err := index.NewMultiReader(subReaders); err == nil {
		t.Fatal("expected IllegalArgumentException from new MultiReader")
	}

	mustClose(t, ir, ir2, dir, dir2)
}

// LUCENE-6299: Test if addindexes(CodecReader[]) prevents exceeding max docs.
func TestIndexWriterMaxDocsAddTooManyIndexesCodecReader(t *testing.T) {
	// we cheat and add the same one over again... IW wants a write lock on each
	t.Fatal(byteBuffersDirectoryLockFactoryMissing)
}

func TestIndexWriterMaxDocsTooLargeMaxDocs(t *testing.T) {
	if err := index.SetMaxDocs(math.MaxInt32); err == nil {
		restoreIndexWriterMaxDocs(t)
		t.Fatal("expected IllegalArgumentException from IndexWriter.setMaxDocs(Integer.MAX_VALUE)")
	}
}

// LUCENE-6299
func TestIndexWriterMaxDocsDeleteAll(t *testing.T) {
	setIndexWriterMaxDocs(t, 1)
	defer restoreIndexWriterMaxDocs(t)
	dir := newDirectory()
	w := nullAnalyzerWriter(t, dir)
	mustAddDocument(t, w, document.NewDocument())
	expectAddDocumentIAEEmpty(t, w)

	mustDeleteAll(t, w)
	mustAddDocument(t, w, document.NewDocument())
	expectAddDocumentIAEEmpty(t, w)

	mustClose(t, w, dir)
}

func mustDeleteAll(t testing.TB, w *index.IndexWriter) {
	t.Helper()
	if _, err := w.DeleteAll(); err != nil {
		t.Fatalf("deleteAll: %v", err)
	}
}

// deleteAllAfter renders the shared body of testDeleteAllAfterFlush and
// testDeleteAllAfterCommit.
func deleteAllAfter(t *testing.T, between func(*index.IndexWriter)) {
	t.Helper()
	setIndexWriterMaxDocs(t, 2)
	defer restoreIndexWriterMaxDocs(t)
	dir := newDirectory()
	w := nullAnalyzerWriter(t, dir)
	mustAddDocument(t, w, document.NewDocument())
	between(w)
	mustAddDocument(t, w, document.NewDocument())
	expectAddDocumentIAEEmpty(t, w)

	mustDeleteAll(t, w)
	mustAddDocument(t, w, document.NewDocument())
	mustAddDocument(t, w, document.NewDocument())
	expectAddDocumentIAEEmpty(t, w)

	mustClose(t, w, dir)
}

// LUCENE-6299
func TestIndexWriterMaxDocsDeleteAllAfterFlush(t *testing.T) {
	deleteAllAfter(t, func(w *index.IndexWriter) { mustClose(t, openReaderFromWriter(t, w)) })
}

// LUCENE-6299
func TestIndexWriterMaxDocsDeleteAllAfterCommit(t *testing.T) {
	deleteAllAfter(t, func(w *index.IndexWriter) { mustCommit(t, w) })
}

// LUCENE-6299
func TestIndexWriterMaxDocsDeleteAllMultipleThreads(t *testing.T) {
	limit := nextInt(2, 10)
	setIndexWriterMaxDocs(t, limit)
	defer restoreIndexWriterMaxDocs(t)
	dir := newDirectory()
	w := nullAnalyzerWriter(t, dir)

	startingGun := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startingGun
			if _, err := w.AddDocument(document.NewDocument()); err != nil {
				t.Errorf("addDocument: %v", err)
			}
		}()
	}

	close(startingGun)
	wg.Wait()
	if t.Failed() {
		t.FailNow()
	}

	expectAddDocumentIAEEmpty(t, w)

	mustDeleteAll(t, w)
	for i := 0; i < limit; i++ {
		mustAddDocument(t, w, document.NewDocument())
	}
	expectAddDocumentIAEEmpty(t, w)

	mustClose(t, w, dir)
}

// LUCENE-6299
func TestIndexWriterMaxDocsDeleteAllAfterClose(t *testing.T) {
	setIndexWriterMaxDocs(t, 2)
	defer restoreIndexWriterMaxDocs(t)
	dir := newDirectory()
	w := nullAnalyzerWriter(t, dir)
	mustAddDocument(t, w, document.NewDocument())
	mustClose(t, w)

	w2 := nullAnalyzerWriter(t, dir)
	mustAddDocument(t, w2, document.NewDocument())
	expectAddDocumentIAEEmpty(t, w2)

	mustDeleteAll(t, w2)
	mustAddDocument(t, w2, document.NewDocument())
	mustAddDocument(t, w2, document.NewDocument())
	expectAddDocumentIAEEmpty(t, w2)

	mustClose(t, w2, dir)
}

// LUCENE-6299
func TestIndexWriterMaxDocsAcrossTwoIndexWriters(t *testing.T) {
	setIndexWriterMaxDocs(t, 1)
	defer restoreIndexWriterMaxDocs(t)
	dir := newDirectory()
	w := nullAnalyzerWriter(t, dir)
	mustAddDocument(t, w, document.NewDocument())
	mustClose(t, w)
	w2 := nullAnalyzerWriter(t, dir)
	expectAddDocumentIAEEmpty(t, w2)

	mustClose(t, w2, dir)
}

// isCorruptIndex reports whether err is a CorruptIndexException.
func isCorruptIndex(err error) bool {
	var cie *index.CorruptIndexException
	return errors.As(err, &cie)
}

// twoDocIndex renders the shared prologue of the two CorruptIndexException
// tests.
func twoDocIndex(t *testing.T) store.Directory {
	t.Helper()
	dir := newDirectory()
	w := nullAnalyzerWriter(t, dir)
	mustAddDocument(t, w, document.NewDocument())
	mustAddDocument(t, w, document.NewDocument())
	mustClose(t, w)
	return dir
}

// LUCENE-6299
func TestIndexWriterMaxDocsCorruptIndexExceptionTooLarge(t *testing.T) {
	dir := twoDocIndex(t)

	setIndexWriterMaxDocs(t, 1)
	r, err := index.OpenDirectoryReader(dir)
	restoreIndexWriterMaxDocs(t)
	if err == nil {
		mustClose(t, r)
		t.Fatal("expected CorruptIndexException from DirectoryReader.open")
	}
	if !isCorruptIndex(err) {
		t.Fatalf("expected CorruptIndexException, got %T: %v", err, err)
	}

	mustClose(t, dir)
}

// LUCENE-6299
func TestIndexWriterMaxDocsCorruptIndexExceptionTooLargeWriter(t *testing.T) {
	dir := twoDocIndex(t)

	setIndexWriterMaxDocs(t, 1)
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfigWithAnalyzer(nil))
	restoreIndexWriterMaxDocs(t)
	if err == nil {
		mustClose(t, w)
		t.Fatal("expected CorruptIndexException from new IndexWriter")
	}
	if !isCorruptIndex(err) {
		t.Fatalf("expected CorruptIndexException, got %T: %v", err, err)
	}

	mustClose(t, dir)
}
