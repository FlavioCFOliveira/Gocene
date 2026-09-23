// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestIndexWriterMerging.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"errors"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Tests that index merging (specifically addIndexes(Directory...)) doesn't
// change the index order of documents.
func TestIndexWriterMergingLucene(t *testing.T) {
	num := 100

	indexA := newDirectory()
	indexB := newDirectory()

	indexWriterMergingFillIndex(t, indexA, 0, num)
	if indexWriterMergingVerifyIndex(t, indexA, 0) {
		t.Fatal("Index a is invalid")
	}

	indexWriterMergingFillIndex(t, indexB, num, num)
	if indexWriterMergingVerifyIndex(t, indexB, num) {
		t.Fatal("Index b is invalid")
	}

	merged := newDirectory()

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(2))
	writer := mustNewIndexWriter(t, merged, conf)
	if _, err := writer.AddIndexes(indexA, indexB); err != nil {
		t.Fatalf("addIndexes: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	if indexWriterMergingVerifyIndex(t, merged, 0) {
		t.Fatal("The merged index is invalid")
	}
	mustClose(t, indexA, indexB, merged)
}

func indexWriterMergingVerifyIndex(t *testing.T, directory store.Directory, startAt int) bool {
	t.Helper()
	fail := false
	reader := mustOpenDirectoryReader(t, directory)

	max := reader.MaxDoc()
	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	for i := 0; i < max; i++ {
		temp := storedDocument(t, storedFields, i)
		// compare the index doc number to the value that it should be
		count := docGet(temp, "count")
		if count == nil || *count != strconv.Itoa(i+startAt) {
			fail = true
			t.Logf("Document %d is returning document %v", i+startAt, count)
		}
	}
	mustClose(t, reader)
	return fail
}

func indexWriterMergingFillIndex(t *testing.T, dir store.Directory, start, numDocs int) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetOpenMode(index.Create)
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(2))
	writer := mustNewIndexWriter(t, dir, conf)

	for i := start; i < start+numDocs; i++ {
		temp := document.NewDocument()
		temp.Add(newStringField(t, "count", strconv.Itoa(i), true))

		mustAddDocument(t, writer, temp)
	}
	mustClose(t, writer)
}

func assertReaderCounts(t testing.TB, dir store.Directory, maxDoc, numDocs int) {
	t.Helper()
	ir := mustOpenDirectoryReader(t, dir)
	defer mustClose(t, ir)
	if maxDoc >= 0 && ir.MaxDoc() != maxDoc {
		t.Fatalf("maxDoc: expected %d, got %d", maxDoc, ir.MaxDoc())
	}
	if ir.NumDocs() != numDocs {
		t.Fatalf("numDocs: expected %d, got %d", numDocs, ir.NumDocs())
	}
}

func assertWriterDocStats(t testing.TB, w *index.IndexWriter, maxDoc, numDocs int) {
	t.Helper()
	stats := iwDocStats(t, w)
	if maxDoc >= 0 && stats.MaxDoc != maxDoc {
		t.Fatalf("getDocStats().maxDoc: expected %d, got %d", maxDoc, stats.MaxDoc)
	}
	if numDocs >= 0 && stats.NumDocs != numDocs {
		t.Fatalf("getDocStats().numDocs: expected %d, got %d", numDocs, stats.NumDocs)
	}
}

func mustDeleteTerm(t testing.TB, w *index.IndexWriter, field, text string) {
	t.Helper()
	if _, err := w.DeleteDocuments([]index.Term{*index.NewTerm(field, text)}); err != nil {
		t.Fatalf("deleteDocuments(%s:%s): %v", field, text, err)
	}
}

func dontMergeWriter(t testing.TB, dir store.Directory) *index.IndexWriter {
	t.Helper()
	conf := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	return mustNewIndexWriter(t, dir, conf)
}

// termVectorFieldType renders new FieldType(base) with tokenized=false and
// term vectors, positions and offsets stored.
func termVectorFieldType(base *document.FieldType) *document.FieldType {
	ft := document.NewFieldTypeFrom(base)
	ft.SetTokenized(false)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectorOffsets(true)
	return ft
}

func storedOnlyFieldType() *document.FieldType {
	ft := document.NewFieldType()
	ft.SetStored(true)
	return ft
}

// LUCENE-325: test forceMergeDeletes, when 2 singular merges
// are required
func TestIndexWriterMergingForceMergeDeletes(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
	writer := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()

	customType := storedOnlyFieldType()
	customType1 := termVectorFieldType(document.TextFieldTypeStored)

	idField := newStringField(t, "id", "", false)
	doc.Add(idField)
	doc.Add(newField(t, "stored", "stored", customType))
	doc.Add(newField(t, "termVector", "termVector", customType1))
	for i := 0; i < 10; i++ {
		idField.SetStringValue(strconv.Itoa(i))
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)

	assertReaderCounts(t, dir, 10, 10)

	writer = dontMergeWriter(t, dir)
	mustDeleteTerm(t, writer, "id", "0")
	mustDeleteTerm(t, writer, "id", "7")
	mustClose(t, writer)

	assertReaderCounts(t, dir, -1, 8)

	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicy())
	writer = mustNewIndexWriter(t, dir, conf)
	assertWriterDocStats(t, writer, 10, 8)
	if err := writer.ForceMergeDeletes(); err != nil {
		t.Fatalf("forceMergeDeletes: %v", err)
	}
	assertWriterDocStats(t, writer, -1, 8)
	mustClose(t, writer)
	assertReaderCounts(t, dir, 8, 8)
	mustClose(t, dir)
}

// forceMergeDeletes98 renders the shared body of testForceMergeDeletes2 and
// testForceMergeDeletes3: 98 documents, every even id deleted.
func forceMergeDeletes98(t *testing.T, dir store.Directory) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(50))
	writer := mustNewIndexWriter(t, dir, conf)

	customType := storedOnlyFieldType()
	customType1 := termVectorFieldType(document.TextFieldTypeNotStored)

	doc := document.NewDocument()
	doc.Add(newField(t, "stored", "stored", customType))
	doc.Add(newField(t, "termVector", "termVector", customType1))
	idField := newStringField(t, "id", "", false)
	doc.Add(idField)
	for i := 0; i < 98; i++ {
		idField.SetStringValue(strconv.Itoa(i))
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)

	assertReaderCounts(t, dir, 98, 98)

	writer = dontMergeWriter(t, dir)
	for i := 0; i < 98; i += 2 {
		mustDeleteTerm(t, writer, "id", strconv.Itoa(i))
	}
	mustClose(t, writer)

	assertReaderCounts(t, dir, -1, 49)
}

// LUCENE-325: test forceMergeDeletes, when many adjacent merges are required
func TestIndexWriterMergingForceMergeDeletes2(t *testing.T) {
	dir := newDirectory()
	forceMergeDeletes98(t, dir)

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(3))
	writer := mustNewIndexWriter(t, dir, conf)
	assertWriterDocStats(t, writer, -1, 49)
	if err := writer.ForceMergeDeletes(); err != nil {
		t.Fatalf("forceMergeDeletes: %v", err)
	}
	mustClose(t, writer)
	assertReaderCounts(t, dir, 49, 49)
	mustClose(t, dir)
}

// LUCENE-325: test forceMergeDeletes without waiting, when
// many adjacent merges are required
func TestIndexWriterMergingForceMergeDeletes3(t *testing.T) {
	dir := newDirectory()
	forceMergeDeletes98(t, dir)

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicyWithMergeFactor(3))
	writer := mustNewIndexWriter(t, dir, conf)
	if _, err := writer.ForceMergeDeletesWithObserver(false); err != nil {
		t.Fatalf("forceMergeDeletes(false): %v", err)
	}
	mustClose(t, writer)
	assertReaderCounts(t, dir, 49, 49)
	mustClose(t, dir)
}

// myMergeScheduler is the private MyMergeScheduler: it intercepts all merges
// and verifies that we are never merging a segment with >= 20 (maxMergeDocs)
// docs.
type myMergeScheduler struct {
	*index.BaseMergeScheduler
	t  testing.TB
	mu sync.Mutex
}

func (s *myMergeScheduler) Merge(mergeSource index.MergeSource, _ index.MergeTrigger) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for {
		merge := mergeSource.GetNextMerge()
		if merge == nil {
			break
		}
		numDocs := 0
		for _, sci := range merge.Segments {
			maxDoc := sci.SegmentInfo().MaxDoc()
			numDocs += maxDoc
			if !(maxDoc < 20) {
				s.t.Errorf("assertTrue(maxDoc < 20): maxDoc=%d", maxDoc)
			}
		}
		if err := mergeSource.Merge(merge); err != nil {
			return err
		}
		if got := merge.GetMergeInfo().SegmentInfo().MaxDoc(); got != numDocs {
			s.t.Errorf("merged maxDoc: expected %d, got %d", numDocs, got)
		}
	}
	return nil
}

func (s *myMergeScheduler) Close() error { return nil }

// deleteEvenIDs renders the shared prologue of the forceMergeDeletes observer
// tests: ten single-id documents, every even id deleted under NoMergePolicy.
func deleteEvenIDs(t *testing.T, dir store.Directory, checkReaders bool) {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
	indexer := mustNewIndexWriter(t, dir, conf)

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		mustAddDocument(t, indexer, doc)
	}
	mustClose(t, indexer)

	if checkReaders {
		assertReaderCounts(t, dir, 10, 10)
	}

	conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	deleter := mustNewIndexWriter(t, dir, conf)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			mustDeleteTerm(t, deleter, "id", strconv.Itoa(i))
		}
	}
	mustClose(t, deleter)

	if checkReaders {
		assertReaderCounts(t, dir, 10, 5)
	}
}

func TestIndexWriterMergingForceMergeDeletesWithObserver(t *testing.T) {
	dir := newDirectory()
	deleteEvenIDs(t, dir, true)

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicy())
	iw := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, iw, dir)
	assertWriterDocStats(t, iw, 10, 5)
	observer, err := iw.ForceMergeDeletesWithObserver(false)
	if err != nil {
		t.Fatalf("forceMergeDeletes(false): %v", err)
	}

	if !(observer.NumMerges() > 0) {
		t.Fatal("Should have scheduled merges")
	}

	if !observer.AwaitWithTimeout(30_000 * time.Millisecond) {
		t.Fatal("Merges should complete within 30 seconds")
	}

	if observer.NumMerges() != observer.NumCompletedMerges() {
		t.Fatalf("All merges should be completed after await() returns true: %d != %d",
			observer.NumMerges(), observer.NumCompletedMerges())
	}

	assertWriterDocStats(t, iw, 5, 5)

	t.Fatal(indexWriterWaitForMergesMissing)
}

func TestIndexWriterMergingMergeObserverNoMerges(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "1", false))
	mustAddDocument(t, writer, doc)
	mustCommit(t, writer)

	observer, err := writer.ForceMergeDeletesWithObserver(false)
	if err != nil {
		t.Fatalf("forceMergeDeletes(false): %v", err)
	}

	if observer.NumMerges() != 0 {
		t.Fatalf("Should have zero merges: %d", observer.NumMerges())
	}

	mustClose(t, writer, dir)
}

func TestIndexWriterMergingMergeObserverAwaitWithTimeout(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicy())
	iw := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, iw, dir)

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		mustAddDocument(t, iw, doc)
	}
	mustCommit(t, iw)

	mustDeleteTerm(t, iw, "id", "0")
	mustDeleteTerm(t, iw, "id", "1")
	mustDeleteTerm(t, iw, "id", "2")
	mustCommit(t, iw)

	observer, err := iw.ForceMergeDeletesWithObserver(false)
	if err != nil {
		t.Fatalf("forceMergeDeletes(false): %v", err)
	}

	if !observer.AwaitWithTimeout(30_000 * time.Millisecond) {
		t.Fatal("Merges should complete within 30 seconds")
	}

	if observer.NumMerges() != observer.NumCompletedMerges() {
		t.Fatalf("All merges should be completed after await() returns true: %d != %d",
			observer.NumMerges(), observer.NumCompletedMerges())
	}

	t.Fatal(indexWriterWaitForMergesMissing)
}

func TestIndexWriterMergingMergeObserverAwaitTimeout(t *testing.T) {
	dir := newDirectory()
	defer mustClose(t, dir)
	t.Fatal("overriding the protected org.apache.lucene.index.ConcurrentMergeScheduler#doMerge(MergeSource, OneMerge) " +
		"in a ConcurrentMergeScheduler subclass is not ported")
}

func TestIndexWriterMergingForceMergeDeletesBlockingWithObserver(t *testing.T) {
	dir := newDirectory()
	deleteEvenIDs(t, dir, false)

	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(newLogMergePolicy())
	iw := mustNewIndexWriter(t, dir, conf)
	assertWriterDocStats(t, iw, 10, 5)

	observer, err := iw.ForceMergeDeletesWithObserver(true)
	if err != nil {
		t.Fatalf("forceMergeDeletes(true): %v", err)
	}
	if !(observer.NumMerges() > 0) {
		t.Fatal("Should have completed merges")
	}
	if !observer.Await() {
		t.Fatal("await should return true immediately")
	}

	assertWriterDocStats(t, iw, 5, 5)

	mustClose(t, iw, dir)
}

func TestIndexWriterMergingBlockingModeWithNoMerges(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	iw := mustNewIndexWriter(t, dir, conf)

	doc := document.NewDocument()
	doc.Add(newStringField(t, "id", "1", false))
	mustAddDocument(t, iw, doc)
	mustCommit(t, iw)

	observer, err := iw.ForceMergeDeletesWithObserver(true)
	if err != nil {
		t.Fatalf("forceMergeDeletes(true): %v", err)
	}
	if observer.NumMerges() != 0 {
		t.Fatalf("Should have zero merges: %d", observer.NumMerges())
	}
	if !observer.AwaitWithTimeout(time.Second) {
		t.Fatal("await with timeout should return true")
	}
	if !observer.Await() {
		t.Fatal("await should return true")
	}

	future := observer.AwaitAsync()
	select {
	case <-future:
	default:
		t.Fatal("Future should be done")
	}
	// A CompletableFuture<Void> from awaitAsync never completes exceptionally
	// in Go's rendering: the channel carries no error.

	mustClose(t, iw, dir)
}

// LUCENE-1013
func TestIndexWriterMergingSetMaxMergeDocs(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergeScheduler(&myMergeScheduler{BaseMergeScheduler: index.NewBaseMergeScheduler(), t: t})
	conf.SetMaxBufferedDocs(2)
	conf.SetMergePolicy(newLogMergePolicy())
	lmp := conf.GetMergePolicy().(logMergePolicy)
	lmp.SetMaxMergeDocs(20)
	lmp.SetMergeFactor(2)
	iw := mustNewIndexWriter(t, dir, conf)
	doc := document.NewDocument()

	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)

	doc.Add(newField(t, "tvtest", "a b c", customType))
	for i := 0; i < 177; i++ {
		mustAddDocument(t, iw, doc)
	}
	mustClose(t, iw, dir)
}

func TestIndexWriterMergingNoWaitClose(t *testing.T) {
	directory := newDirectory()

	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetTokenized(false)

	idField := newField(t, "id", "", customType)
	doc.Add(idField)

	for pass := 0; pass < 2; pass++ {
		conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
		conf.SetOpenMode(index.Create)
		conf.SetMaxBufferedDocs(2)
		conf.SetMergePolicy(newLogMergePolicy())
		conf.SetCommitOnClose(false)
		if pass == 2 {
			conf.SetMergeScheduler(index.NewSerialMergeScheduler())
		}

		writer := mustNewIndexWriter(t, directory, conf)
		writer.GetConfig().GetMergePolicy().(logMergePolicy).SetMergeFactor(100)

		for iter := 0; iter < atLeast(3); iter++ {
			for j := 0; j < 199; j++ {
				idField.SetStringValue(strconv.Itoa(iter*201 + j))
				mustAddDocument(t, writer, doc)
			}

			delID := iter * 199
			for j := 0; j < 20; j++ {
				mustDeleteTerm(t, writer, "id", strconv.Itoa(delID))
				delID += 5
			}

			mustCommit(t, writer)

			// Force a bunch of merge threads to kick off so we
			// stress out aborting them on close:
			writer.GetConfig().GetMergePolicy().(logMergePolicy).SetMergeFactor(2)

			finalWriter := writer
			var failure error
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				done := false
				for !done {
					for i := 0; i < 100; i++ {
						if _, err := finalWriter.AddDocument(doc); err != nil {
							var ace *store.AlreadyClosedException
							if !errors.As(err, &ace) {
								failure = err
							}
							done = true
							break
						}
					}
					runtime.Gosched()
				}
			}()

			mustClose(t, writer)
			wg.Wait()

			if failure != nil {
				t.Fatalf("addDocument during close: %v", failure)
			}

			// Make sure reader can read
			mustClose(t, mustOpenDirectoryReader(t, directory))

			// Reopen
			conf = newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
			conf.SetOpenMode(index.Append)
			conf.SetMergePolicy(newLogMergePolicy())
			conf.SetCommitOnClose(false)
			writer = mustNewIndexWriter(t, directory, conf)
		}
		mustClose(t, writer)
	}

	mustClose(t, directory)
}

func TestIndexWriterMergingAddEstimatedBytesToMerge(t *testing.T) {
	dir := newDirectory()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	writer := mustNewIndexWriter(t, dir, conf)
	defer mustClose(t, writer, dir)

	doc := document.NewDocument()
	doc.Add(newTextField(t, "field", "content", true))

	for i := 0; i < 10; i++ {
		mustAddDocument(t, writer, doc)
	}
	mustFlush(t, writer)

	t.Fatal("org.apache.lucene.index.IndexWriter#cloneSegmentInfos() and " +
		"IndexWriter#addEstimatedBytesToMerge(MergePolicy.OneMerge) are not ported")
}
