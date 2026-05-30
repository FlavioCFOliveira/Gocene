// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "testing"

// This file ports org.apache.lucene.index.TestIndexWriterMaxDocs
// (Apache Lucene 10.4.0).
//
// The Java suite verifies the global per-index document cap (LUCENE-6299):
// once an index reaches IndexWriter.getActualMaxDocs(), any add/update/
// addIndexes call must fail with an IllegalArgumentException, opening a
// reader on an over-cap index must fail with CorruptIndexException, and
// MultiReader must reject sub-readers whose combined maxDoc exceeds the cap.
// Tests lower the cap with IndexWriter.setMaxDocs(int) and restore it in a
// finally block via restoreIndexWriterMaxDocs().
//
// Status: stubbed (skipped). Gocene currently lacks the primitives this
// suite depends on:
//
//  1. Static, test-overridable document cap. Java exposes
//     IndexWriter.MAX_DOCS, IndexWriter.setMaxDocs(int) and
//     IndexWriter.getActualMaxDocs(). Gocene's IndexWriter has no MaxDocs
//     surface; IndexWriterConfig.MaxDocs is a per-config field, not the
//     global, test-lowerable cap the suite needs.
//  2. Cap enforcement on the indexing and reading paths. Nothing rejects an
//     add/update/addIndexes once the index reaches the cap, MultiReader does
//     not validate aggregate maxDoc, and DirectoryReader does not raise a
//     CorruptIndexException when an existing index exceeds the cap.
//  3. NRT "open directly from writer" entry point. testDeleteAllAfterFlush
//     uses DirectoryReader.open(writer); Gocene models NRT through a distinct
//     NRTDirectoryReader and DirectoryReaderReopener, with no equivalent.
//
// When these land, replace each t.Skip with the real port: lower the cap via
// setMaxDocs, exercise the writer/reader, assert the rejection or corruption
// error, and restore the cap via t.Cleanup.

// skipMaxDocs is the shared skip reason for the MaxDocs cap suite.
const skipMaxDocs = "blocked: IndexWriter MaxDocs cap (MAX_DOCS/setMaxDocs/" +
	"getActualMaxDocs) and its enforcement on the add/update/addIndexes and " +
	"reader-open paths are not implemented; see Sprint 55 GOC-4200"

// TestIndexWriterMaxDocsExactlyAtTrueLimit ports testExactlyAtTrueLimit:
// indexes exactly IndexWriter.MAX_DOCS documents and checks maxDoc, numDocs
// and search totals, before and after forceMerge(1). Marked @Monster in Java.
func TestIndexWriterMaxDocsExactlyAtTrueLimit(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsAddDocument ports testAddDocument: with the cap
// lowered to 10, the 11th addDocument must be rejected.
func TestIndexWriterMaxDocsAddDocument(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsAddDocuments ports testAddDocuments: with the cap
// lowered to 10, the 11th addDocuments must be rejected.
func TestIndexWriterMaxDocsAddDocuments(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsUpdateDocument ports testUpdateDocument: with the cap
// lowered to 10, the 11th updateDocument must be rejected.
func TestIndexWriterMaxDocsUpdateDocument(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsUpdateDocuments ports testUpdateDocuments: with the
// cap lowered to 10, the 11th updateDocuments must be rejected.
func TestIndexWriterMaxDocsUpdateDocuments(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsReclaimedDeletes ports testReclaimedDeletes: deleted
// docs reclaimed by forceMerge free cap headroom, but the count must still be
// enforced once the cap is reached again.
func TestIndexWriterMaxDocsReclaimedDeletes(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsReclaimedDeletesWholeSegments ports
// testReclaimedDeletesWholeSegments: 100% deleted segments dropped entirely by
// IndexWriter must not be mis-counted against the cap.
func TestIndexWriterMaxDocsReclaimedDeletesWholeSegments(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsAddIndexes ports testAddIndexes: addIndexes(Directory)
// and addIndexesSlowly(reader) must both be rejected when they would push the
// index past the cap.
func TestIndexWriterMaxDocsAddIndexes(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsMultiReaderExactLimit ports testMultiReaderExactLimit:
// a MultiReader whose sub-readers sum to exactly MAX_DOCS must be accepted.
func TestIndexWriterMaxDocsMultiReaderExactLimit(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsMultiReaderBeyondLimit ports
// testMultiReaderBeyondLimit: a MultiReader whose sub-readers sum to one past
// MAX_DOCS must be rejected.
func TestIndexWriterMaxDocsMultiReaderBeyondLimit(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsAddTooManyIndexesDir ports testAddTooManyIndexesDir
// (LUCENE-6299, @Nightly): addIndexes(Directory[]) must reject the batch
// before exceeding MAX_DOCS rather than executing the copy.
func TestIndexWriterMaxDocsAddTooManyIndexesDir(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsAddTooManyIndexesCodecReader ports
// testAddTooManyIndexesCodecReader (LUCENE-6299): addIndexes(CodecReader[])
// must reject the batch before exceeding MAX_DOCS.
func TestIndexWriterMaxDocsAddTooManyIndexesCodecReader(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsTooLargeMaxDocs ports testTooLargeMaxDocs:
// setMaxDocs(math.MaxInt32) must be rejected.
func TestIndexWriterMaxDocsTooLargeMaxDocs(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsDeleteAll ports testDeleteAll (LUCENE-6299): deleteAll
// resets the document count so indexing can resume up to the cap.
func TestIndexWriterMaxDocsDeleteAll(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsDeleteAllAfterFlush ports testDeleteAllAfterFlush
// (LUCENE-6299): deleteAll resets the count even after an NRT reader was
// opened directly from the writer.
func TestIndexWriterMaxDocsDeleteAllAfterFlush(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsDeleteAllAfterCommit ports testDeleteAllAfterCommit
// (LUCENE-6299): deleteAll resets the count even after a commit.
func TestIndexWriterMaxDocsDeleteAllAfterCommit(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsDeleteAllMultipleThreads ports
// testDeleteAllMultipleThreads (LUCENE-6299): concurrent addDocument calls up
// to the cap, then deleteAll, must leave the count correctly reset.
func TestIndexWriterMaxDocsDeleteAllMultipleThreads(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsDeleteAllAfterClose ports testDeleteAllAfterClose
// (LUCENE-6299): deleteAll resets the count in a fresh writer reopened on a
// capped index.
func TestIndexWriterMaxDocsDeleteAllAfterClose(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsAcrossTwoIndexWriters ports testAcrossTwoIndexWriters
// (LUCENE-6299): the cap persists across writers, so a second writer opened on
// an at-cap index must reject the next addDocument.
func TestIndexWriterMaxDocsAcrossTwoIndexWriters(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsCorruptIndexExceptionTooLarge ports
// testCorruptIndexExceptionTooLarge (LUCENE-6299): opening a DirectoryReader
// on an index whose docCount exceeds the cap must raise CorruptIndexException.
func TestIndexWriterMaxDocsCorruptIndexExceptionTooLarge(t *testing.T) {
	t.Fatal(skipMaxDocs)
}

// TestIndexWriterMaxDocsCorruptIndexExceptionTooLargeWriter ports
// testCorruptIndexExceptionTooLargeWriter (LUCENE-6299): opening an
// IndexWriter on an index whose docCount exceeds the cap must raise
// CorruptIndexException.
func TestIndexWriterMaxDocsCorruptIndexExceptionTooLargeWriter(t *testing.T) {
	t.Fatal(skipMaxDocs)
}
