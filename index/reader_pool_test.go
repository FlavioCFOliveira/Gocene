// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestReaderPool.java
// (Apache Lucene 10.5.0).

package index

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func readerPoolZeroDelGen() int64 { return 0 }

// readerPoolFixture opens DirectoryReader.open(directory) and clones its
// segmentInfos, as every Java test method does after buildIndex.
func readerPoolFixture(t *testing.T, directory store.Directory) (*DirectoryReader, *SegmentInfos) {
	t.Helper()
	reader, err := OpenDirectoryReader(directory)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	return reader, reader.GetSegmentInfos().Clone()
}

func newTestReaderPool(t *testing.T, directory store.Directory, segmentInfos *SegmentInfos, fieldNumbers *FieldNumbers, infoStream util.InfoStream) *ReaderPool {
	t.Helper()
	pool, err := NewReaderPool(directory, directory, segmentInfos, fieldNumbers, readerPoolZeroDelGen, infoStream, "", nil)
	if err != nil {
		t.Fatalf("new ReaderPool: %v", err)
	}
	return pool
}

// readerPoolRandomFrom renders RandomPicks.randomFrom(random(), segmentInfos.asList()).
func readerPoolRandomFrom(segmentInfos *SegmentInfos) *SegmentCommitInfo {
	return segmentInfos.Get(rand.Intn(segmentInfos.Size()))
}

func mustPoolGet(t *testing.T, pool *ReaderPool, info *SegmentCommitInfo, create bool) *ReadersAndUpdates {
	t.Helper()
	rld, err := pool.Get(info, create)
	if err != nil {
		t.Fatalf("pool.get: %v", err)
	}
	return rld
}

func mustPoolRelease(t *testing.T, pool *ReaderPool, rld *ReadersAndUpdates, assertInfoLive bool) bool {
	t.Helper()
	changed, err := pool.Release(rld, assertInfoLive)
	if err != nil {
		t.Fatalf("pool.release: %v", err)
	}
	return changed
}

func mustPoolDrop(t *testing.T, pool *ReaderPool, info *SegmentCommitInfo) bool {
	t.Helper()
	dropped, err := pool.Drop(info)
	if err != nil {
		t.Fatalf("pool.drop: %v", err)
	}
	return dropped
}

func readerPoolCloseAll(t *testing.T, closers ...interface{ Close() error }) {
	t.Helper()
	for _, c := range closers {
		if err := c.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

func TestReaderPoolDrop(t *testing.T) {
	directory := newDirectory()
	fieldNumbers := readerPoolBuildIndex(t, directory)
	reader, segmentInfos := readerPoolFixture(t, directory)

	pool := newTestReaderPool(t, directory, segmentInfos, fieldNumbers, nil)
	commitInfo := readerPoolRandomFrom(segmentInfos)
	readersAndUpdates := mustPoolGet(t, pool, commitInfo, true)
	if readersAndUpdates != mustPoolGet(t, pool, commitInfo, false) {
		t.Fatal("assertSame(readersAndUpdates, pool.get(commitInfo, false))")
	}
	if !mustPoolDrop(t, pool, commitInfo) {
		t.Fatal("assertTrue(pool.drop(commitInfo))")
	}
	if rand.Intn(2) == 0 {
		if mustPoolDrop(t, pool, commitInfo) {
			t.Fatal("assertFalse(pool.drop(commitInfo))")
		}
	}
	if mustPoolGet(t, pool, commitInfo, false) != nil {
		t.Fatal("assertNull(pool.get(commitInfo, false))")
	}
	mustPoolRelease(t, pool, readersAndUpdates, rand.Intn(2) == 0)
	readerPoolCloseAll(t, pool, reader, directory)
}

func TestReaderPoolPoolReaders(t *testing.T) {
	directory := newDirectory()
	fieldNumbers := readerPoolBuildIndex(t, directory)
	reader, segmentInfos := readerPoolFixture(t, directory)

	pool := newTestReaderPool(t, directory, segmentInfos, fieldNumbers, nil)
	commitInfo := readerPoolRandomFrom(segmentInfos)
	if pool.IsReaderPoolingEnabled() {
		t.Fatal("assertFalse(pool.isReaderPoolingEnabled())")
	}
	mustPoolRelease(t, pool, mustPoolGet(t, pool, commitInfo, true), rand.Intn(2) == 0)
	if mustPoolGet(t, pool, commitInfo, false) != nil {
		t.Fatal("assertNull(pool.get(commitInfo, false))")
	}
	// now start pooling
	pool.EnableReaderPooling()
	if !pool.IsReaderPoolingEnabled() {
		t.Fatal("assertTrue(pool.isReaderPoolingEnabled())")
	}
	mustPoolRelease(t, pool, mustPoolGet(t, pool, commitInfo, true), rand.Intn(2) == 0)
	if mustPoolGet(t, pool, commitInfo, false) == nil {
		t.Fatal("assertNotNull(pool.get(commitInfo, false))")
	}
	if mustPoolGet(t, pool, commitInfo, false) != mustPoolGet(t, pool, commitInfo, false) {
		t.Fatal("assertSame(pool.get(commitInfo, false), pool.get(commitInfo, false))")
	}
	mustPoolDrop(t, pool, commitInfo)
	var ramBytesUsed int64
	if pool.RamBytesUsed() != 0 {
		t.Fatalf("expected 0, got %d", pool.RamBytesUsed())
	}
	for info := range segmentInfos.Iterator() {
		mustPoolRelease(t, pool, mustPoolGet(t, pool, info, true), rand.Intn(2) == 0)
		if pool.RamBytesUsed() != 0 {
			t.Fatalf(" used: %d actual: %d", ramBytesUsed, pool.RamBytesUsed())
		}
		ramBytesUsed = pool.RamBytesUsed()
		if mustPoolGet(t, pool, info, false) != mustPoolGet(t, pool, info, false) {
			t.Fatal("assertSame(pool.get(info, false), pool.get(info, false))")
		}
	}
	// assertNotSame(0, pool.ramBytesUsed()): Java compares a boxed Integer with a
	// boxed Long, which are never the same object, so the assertion always holds.
	if err := pool.DropAll(); err != nil {
		t.Fatalf("dropAll: %v", err)
	}
	for info := range segmentInfos.Iterator() {
		if mustPoolGet(t, pool, info, false) != nil {
			t.Fatal("assertNull(pool.get(info, false))")
		}
	}
	if pool.RamBytesUsed() != 0 {
		t.Fatalf("expected 0, got %d", pool.RamBytesUsed())
	}
	readerPoolCloseAll(t, pool, reader, directory)
}

// readerPoolFirstPosting renders
// `postings = readOnlyClone.postings(new Term("id", "" + id))` followed by
// `postings != null && postings.nextDoc() != NO_MORE_DOCS`.
func readerPoolFirstPosting(t *testing.T, sr *SegmentReader, id int) (PostingsEnum, bool) {
	t.Helper()
	postings, err := sr.Postings(*NewTerm("id", strconv.Itoa(id)), PostingsFlagFreqs)
	if err != nil {
		t.Fatalf("postings: %v", err)
	}
	if postings == nil {
		return nil, false
	}
	doc, err := postings.NextDoc()
	if err != nil {
		t.Fatalf("nextDoc: %v", err)
	}
	return postings, doc != NO_MORE_DOCS
}

func readerPoolAssertExhausted(t *testing.T, postings PostingsEnum) {
	t.Helper()
	doc, err := postings.NextDoc()
	if err != nil {
		t.Fatalf("nextDoc: %v", err)
	}
	if doc != NO_MORE_DOCS {
		t.Fatalf("expected NO_MORE_DOCS, got %d", doc)
	}
}

func mustReadOnlyClone(t *testing.T, rld *ReadersAndUpdates) *SegmentReader {
	t.Helper()
	sr, err := rld.GetReadOnlyClone()
	if err != nil {
		t.Fatalf("getReadOnlyClone: %v", err)
	}
	return sr
}

func TestReaderPoolUpdate(t *testing.T) {
	directory := newDirectory()
	fieldNumbers := readerPoolBuildIndex(t, directory)
	reader, segmentInfos := readerPoolFixture(t, directory)
	pool := newTestReaderPool(t, directory, segmentInfos, fieldNumbers, util.NoOpInfoStream)
	id := rand.Intn(10)
	if rand.Intn(2) == 0 {
		pool.EnableReaderPooling()
	}
	for commitInfo := range segmentInfos.Iterator() {
		readersAndUpdates := mustPoolGet(t, pool, commitInfo, true)
		readOnlyClone := mustReadOnlyClone(t, readersAndUpdates)
		postings, found := readerPoolFirstPosting(t, readOnlyClone, id)
		expectUpdate := false
		doc := -1
		if found {
			number, err := NewNumericDocValuesFieldUpdates(0, "number", commitInfo.SegmentInfo().MaxDoc())
			if err != nil {
				t.Fatalf("new NumericDocValuesFieldUpdates: %v", err)
			}
			doc = postings.DocID()
			if err := number.AddLong(doc, 1000); err != nil {
				t.Fatalf("add: %v", err)
			}
			if err := number.Finish(); err != nil {
				t.Fatalf("finish: %v", err)
			}
			if err := readersAndUpdates.AddDVUpdate(number); err != nil {
				t.Fatalf("addDVUpdate: %v", err)
			}
			expectUpdate = true
			readerPoolAssertExhausted(t, postings)
			if !pool.AnyDocValuesChanges() {
				t.Fatal("assertTrue(pool.anyDocValuesChanges())")
			}
		} else if pool.AnyDocValuesChanges() {
			t.Fatal("assertFalse(pool.anyDocValuesChanges())")
		}
		readerPoolCloseAll(t, readOnlyClone)
		var writtenToDisk bool
		var err error
		if pool.IsReaderPoolingEnabled() {
			if rand.Intn(2) == 0 {
				writtenToDisk, err = pool.WriteAllDocValuesUpdates()
				if readersAndUpdates.IsMerging() {
					t.Fatal("assertFalse(readersAndUpdates.isMerging())")
				}
			} else if rand.Intn(2) == 0 {
				writtenToDisk, err = pool.Commit(segmentInfos)
				if readersAndUpdates.IsMerging() {
					t.Fatal("assertFalse(readersAndUpdates.isMerging())")
				}
			} else {
				writtenToDisk, err = pool.WriteDocValuesUpdatesForMerge([]*SegmentCommitInfo{commitInfo})
				if !readersAndUpdates.IsMerging() {
					t.Fatal("assertTrue(readersAndUpdates.isMerging())")
				}
			}
			if err != nil {
				t.Fatalf("write: %v", err)
			}
			if mustPoolRelease(t, pool, readersAndUpdates, rand.Intn(2) == 0) {
				t.Fatal("assertFalse(pool.release(readersAndUpdates, random().nextBoolean()))")
			}
		} else {
			if rand.Intn(2) == 0 {
				writtenToDisk = mustPoolRelease(t, pool, readersAndUpdates, rand.Intn(2) == 0)
				if readersAndUpdates.IsMerging() {
					t.Fatal("assertFalse(readersAndUpdates.isMerging())")
				}
			} else {
				writtenToDisk, err = pool.WriteDocValuesUpdatesForMerge([]*SegmentCommitInfo{commitInfo})
				if err != nil {
					t.Fatalf("writeDocValuesUpdatesForMerge: %v", err)
				}
				if !readersAndUpdates.IsMerging() {
					t.Fatal("assertTrue(readersAndUpdates.isMerging())")
				}
				if mustPoolRelease(t, pool, readersAndUpdates, rand.Intn(2) == 0) {
					t.Fatal("assertFalse(pool.release(readersAndUpdates, random().nextBoolean()))")
				}
			}
		}
		if pool.AnyDocValuesChanges() {
			t.Fatal("assertFalse(pool.anyDocValuesChanges())")
		}
		if expectUpdate != writtenToDisk {
			t.Fatalf("expected writtenToDisk %t, got %t", expectUpdate, writtenToDisk)
		}
		if expectUpdate {
			readersAndUpdates = mustPoolGet(t, pool, commitInfo, true)
			updatedReader := mustReadOnlyClone(t, readersAndUpdates)
			if doc == -1 {
				t.Fatal("assertNotSame(-1, doc)")
			}
			number, err := updatedReader.GetNumericDocValues("number")
			if err != nil {
				t.Fatalf("getNumericDocValues: %v", err)
			}
			got, err := number.Advance(doc)
			if err != nil || got != doc {
				t.Fatalf("advance(%d): got %d (%v)", doc, got, err)
			}
			v, err := number.LongValue()
			if err != nil || v != 1000 {
				t.Fatalf("longValue: expected 1000, got %d (%v)", v, err)
			}
			if err := readersAndUpdates.Release(updatedReader); err != nil {
				t.Fatalf("release: %v", err)
			}
			if mustPoolRelease(t, pool, readersAndUpdates, rand.Intn(2) == 0) {
				t.Fatal("assertFalse(pool.release(readersAndUpdates, random().nextBoolean()))")
			}
		}
	}
	readerPoolCloseAll(t, pool, reader, directory)
}

func TestReaderPoolDeletes(t *testing.T) {
	directory := newDirectory()
	fieldNumbers := readerPoolBuildIndex(t, directory)
	reader, segmentInfos := readerPoolFixture(t, directory)
	pool := newTestReaderPool(t, directory, segmentInfos, fieldNumbers, util.NoOpInfoStream)
	id := rand.Intn(10)
	if rand.Intn(2) == 0 {
		pool.EnableReaderPooling()
	}
	for commitInfo := range segmentInfos.Iterator() {
		readersAndUpdates := mustPoolGet(t, pool, commitInfo, true)
		readOnlyClone := mustReadOnlyClone(t, readersAndUpdates)
		postings, found := readerPoolFirstPosting(t, readOnlyClone, id)
		expectUpdate := false
		doc := -1
		if found {
			doc = postings.DocID()
			deleted, err := readersAndUpdates.Delete(doc)
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			if !deleted {
				t.Fatal("assertTrue(readersAndUpdates.delete(doc))")
			}
			expectUpdate = true
			readerPoolAssertExhausted(t, postings)
		}
		if pool.AnyDocValuesChanges() { // deletes are not accounted here
			t.Fatal("assertFalse(pool.anyDocValuesChanges())")
		}
		readerPoolCloseAll(t, readOnlyClone)
		var writtenToDisk bool
		if pool.IsReaderPoolingEnabled() {
			var err error
			writtenToDisk, err = pool.Commit(segmentInfos)
			if err != nil {
				t.Fatalf("commit: %v", err)
			}
			if mustPoolRelease(t, pool, readersAndUpdates, rand.Intn(2) == 0) {
				t.Fatal("assertFalse(pool.release(readersAndUpdates, random().nextBoolean()))")
			}
		} else {
			writtenToDisk = mustPoolRelease(t, pool, readersAndUpdates, rand.Intn(2) == 0)
		}
		if pool.AnyDocValuesChanges() {
			t.Fatal("assertFalse(pool.anyDocValuesChanges())")
		}
		if expectUpdate != writtenToDisk {
			t.Fatalf("expected writtenToDisk %t, got %t", expectUpdate, writtenToDisk)
		}
		if expectUpdate {
			readersAndUpdates = mustPoolGet(t, pool, commitInfo, true)
			updatedReader := mustReadOnlyClone(t, readersAndUpdates)
			if doc == -1 {
				t.Fatal("assertNotSame(-1, doc)")
			}
			if liveDocs := updatedReader.GetLiveDocs(); liveDocs == nil || liveDocs.Get(doc) {
				t.Fatal("assertFalse(updatedReader.getLiveDocs().get(doc))")
			}
			if err := readersAndUpdates.Release(updatedReader); err != nil {
				t.Fatalf("release: %v", err)
			}
			if mustPoolRelease(t, pool, readersAndUpdates, rand.Intn(2) == 0) {
				t.Fatal("assertFalse(pool.release(readersAndUpdates, random().nextBoolean()))")
			}
		}
	}
	readerPoolCloseAll(t, pool, reader, directory)
}

// TestReaderPoolPassReaderToMergePolicyConcurrently ports
// testPassReaderToMergePolicyConcurrently, whose FilterMergePolicy overrides
// MergePolicy#keepFullyDeletedSegment(IOSupplier<CodecReader>) to pull a
// reader from ReadersAndUpdates#getLatestReader. Gocene's
// MergePolicy.KeepFullyDeletedSegment takes a SegmentCommitInfo instead of the
// reader supplier, and ReadersAndUpdates.KeepFullyDeletedSegment is not ported.
func TestReaderPoolPassReaderToMergePolicyConcurrently(t *testing.T) {
	t.Fatal("org.apache.lucene.index.MergePolicy#keepFullyDeletedSegment(IOSupplier<CodecReader>) and " +
		"ReadersAndUpdates#keepFullyDeletedSegment(MergePolicy) (via getLatestReader) are not ported")
}

// readerPoolBuildIndex ports the private buildIndex(Directory).
func readerPoolBuildIndex(t *testing.T, directory store.Directory) *FieldNumbers {
	t.Helper()
	writer, err := NewIndexWriter(directory, newIndexWriterConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		id, err := document.NewStringField("id", strconv.Itoa(i), true)
		if err != nil {
			t.Fatalf("new StringField: %v", err)
		}
		doc.Add(id)
		number, err := document.NewNumericDocValuesField("number", int64(i))
		if err != nil {
			t.Fatalf("new NumericDocValuesField: %v", err)
		}
		doc.Add(number)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
		if rand.Intn(2) == 0 {
			if err := writer.Flush(); err != nil {
				t.Fatalf("flush: %v", err)
			}
		}
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return writer.globalFieldNumberMap
}

func TestReaderPoolGetReaderByRam(t *testing.T) {
	directory := newDirectory()
	fieldNumbers := readerPoolBuildIndex(t, directory)
	reader, segmentInfos := readerPoolFixture(t, directory)
	pool := newTestReaderPool(t, directory, segmentInfos, fieldNumbers, util.NoOpInfoStream)
	if n := len(pool.GetReadersByRam()); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}

	ord := 0
	for commitInfo := range segmentInfos.Iterator() {
		readersAndUpdates := mustPoolGet(t, pool, commitInfo, true)
		test := NewBinaryDocValuesFieldUpdates(0, "test", commitInfo.SegmentInfo().MaxDoc())
		if err := test.AddBinary(0, util.NewBytesRef(make([]byte, ord))); err != nil {
			t.Fatalf("add: %v", err)
		}
		ord++
		if err := test.Finish(); err != nil {
			t.Fatalf("finish: %v", err)
		}
		if err := readersAndUpdates.AddDVUpdate(test); err != nil {
			t.Fatalf("addDVUpdate: %v", err)
		}
	}

	readersByRam := pool.GetReadersByRam()
	if segmentInfos.Size() != len(readersByRam) {
		t.Fatalf("expected %d, got %d", segmentInfos.Size(), len(readersByRam))
	}
	previousRAM := int64(^uint64(0) >> 1)
	for _, rld := range readersByRam {
		if !(previousRAM >= rld.ramBytesUsed.Load()) {
			t.Fatalf("previous: %d now: %d", previousRAM, rld.ramBytesUsed.Load())
		}
		previousRAM = rld.ramBytesUsed.Load()
		rld.DropChanges()
		mustPoolDrop(t, pool, rld.info)
	}
	readerPoolCloseAll(t, pool, reader, directory)
}
