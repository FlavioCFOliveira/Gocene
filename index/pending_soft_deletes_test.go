// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestPendingSoftDeletes.java
// (Apache Lucene 10.5.0), which extends TestPendingDeletes: the inherited
// test methods run with the overriding newPendingDeletes factory.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func newPendingSoftDeletesForTest(commitInfo *SegmentCommitInfo) PendingDeletesInterface {
	return NewPendingSoftDeletes("_soft_deletes", commitInfo)
}

func TestPendingSoftDeletesDeleteDoc(t *testing.T) {
	checkPendingDeletesDeleteDoc(t, newPendingSoftDeletesForTest)
}

func TestPendingSoftDeletesWriteLiveDocs(t *testing.T) {
	checkPendingDeletesWriteLiveDocs(t, newPendingSoftDeletesForTest)
}

func TestPendingSoftDeletesIsFullyDeleted(t *testing.T) {
	checkPendingDeletesIsFullyDeleted(t, newPendingSoftDeletesForTest)
}

// softDeletesTestWriter opens the IndexWriter configured with
// setSoftDeletesField("_soft_deletes"), setMaxBufferedDocs(maxBufferedDocs),
// NoMergePolicy and setRAMBufferSizeMB(DISABLE_AUTO_FLUSH), and soft-updates
// the documents 1, 2, 2 as the Java tests do.
func softDeletesTestWriter(t *testing.T, dir store.Directory, maxBufferedDocs int) *IndexWriter {
	t.Helper()
	iwc := newIndexWriterConfig()
	iwc.SetSoftDeletesField("_soft_deletes")
	// make sure all docs will end up in the same segment
	iwc.SetMaxBufferedDocs(maxBufferedDocs)
	iwc.SetMergePolicy(NewNoMergePolicy())
	iwc.SetRAMBufferSizeMB(DISABLE_AUTO_FLUSH)
	writer, err := NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	for _, id := range []string{"1", "2", "2"} {
		doc := document.NewDocument()
		f, err := document.NewStringField("id", id, true)
		if err != nil {
			t.Fatalf("new StringField: %v", err)
		}
		doc.Add(f)
		softDelete, err := document.NewNumericDocValuesField("_soft_deletes", 1)
		if err != nil {
			t.Fatalf("new NumericDocValuesField: %v", err)
		}
		if _, err := writer.SoftUpdateDocument(NewTerm("id", id), doc, []*document.Field{softDelete.Field}); err != nil {
			t.Fatalf("softUpdateDocument: %v", err)
		}
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return writer
}

// softDeletesOnlySegmentReader renders
// (SegmentReader) DirectoryReader.open(dir).leaves().get(0).reader().
func softDeletesOnlySegmentReader(t *testing.T, dir store.Directory) (*DirectoryReader, *SegmentReader) {
	t.Helper()
	reader, err := OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	segmentReader, ok := leaves[0].LeafReader().(*SegmentReader)
	if !ok {
		t.Fatalf("leaf is a %T, not a SegmentReader", leaves[0].LeafReader())
	}
	return reader, segmentReader
}

func assertLive(t *testing.T, bits util.Bits, want ...bool) {
	t.Helper()
	if bits == nil {
		t.Fatal("live docs are null")
	}
	for i, w := range want {
		if bits.Get(i) != w {
			t.Fatalf("liveDocs.get(%d): expected %t, got %t", i, w, bits.Get(i))
		}
	}
}

func mustOnNewReader(t *testing.T, deletes PendingDeletesInterface, reader CodecReader, info *SegmentCommitInfo) {
	t.Helper()
	if err := deletes.OnNewReader(reader, info); err != nil {
		t.Fatalf("onNewReader: %v", err)
	}
}

func pendingSoftDeletesCloseAll(t *testing.T, closers ...interface{ Close() error }) {
	t.Helper()
	for _, c := range closers {
		if err := c.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

func TestPendingSoftDeletesHardDeleteSoftDeleted(t *testing.T) {
	dir := newDirectory()
	writer := softDeletesTestWriter(t, dir, 10)
	reader, segmentReader := softDeletesOnlySegmentReader(t, dir)
	segmentInfo := segmentReader.GetSegmentCommitInfo()
	pendingSoftDeletes := NewPendingSoftDeletes("_soft_deletes", segmentInfo)
	mustOnNewReader(t, pendingSoftDeletes, segmentReader, segmentInfo)
	if pendingSoftDeletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", pendingSoftDeletes.NumPendingDeletes())
	}
	if pendingSoftDeletes.GetDelCount() != 1 {
		t.Fatalf("expected 1, got %d", pendingSoftDeletes.GetDelCount())
	}
	assertLive(t, pendingSoftDeletes.GetLiveDocs(), true, false, true)
	if pendingSoftDeletes.GetHardLiveDocs() != nil {
		t.Fatal("assertNull(pendingSoftDeletes.getHardLiveDocs())")
	}
	if !mustPendingDelete(t, pendingSoftDeletes, 1) {
		t.Fatal("assertTrue(pendingSoftDeletes.delete(1))")
	}
	if pendingSoftDeletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", pendingSoftDeletes.NumPendingDeletes())
	}
	if pendingSoftDeletes.pendingDeleteCount != -1 { // transferred the delete
		t.Fatalf("expected pendingDeleteCount -1, got %d", pendingSoftDeletes.pendingDeleteCount)
	}
	if pendingSoftDeletes.GetDelCount() != 1 {
		t.Fatalf("expected 1, got %d", pendingSoftDeletes.GetDelCount())
	}
	pendingSoftDeletesCloseAll(t, reader, writer, dir)
}

func TestPendingSoftDeletesDeleteSoft(t *testing.T) {
	dir := newDirectory()
	writer := softDeletesTestWriter(t, dir, 10)
	reader, segmentReader := softDeletesOnlySegmentReader(t, dir)
	segmentInfo := segmentReader.GetSegmentCommitInfo()
	pendingSoftDeletes := NewPendingSoftDeletes("_soft_deletes", segmentInfo)
	mustOnNewReader(t, pendingSoftDeletes, segmentReader, segmentInfo)
	if pendingSoftDeletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", pendingSoftDeletes.NumPendingDeletes())
	}
	if pendingSoftDeletes.GetDelCount() != 1 {
		t.Fatalf("expected 1, got %d", pendingSoftDeletes.GetDelCount())
	}
	assertLive(t, pendingSoftDeletes.GetLiveDocs(), true, false, true)
	if pendingSoftDeletes.GetHardLiveDocs() != nil {
		t.Fatal("assertNull(pendingSoftDeletes.getHardLiveDocs())")
	}
	// pass reader again
	liveDocs := pendingSoftDeletes.GetLiveDocs()
	mustOnNewReader(t, pendingSoftDeletes, segmentReader, segmentInfo)
	if pendingSoftDeletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", pendingSoftDeletes.NumPendingDeletes())
	}
	if pendingSoftDeletes.GetDelCount() != 1 {
		t.Fatalf("expected 1, got %d", pendingSoftDeletes.GetDelCount())
	}
	if !sameBits(liveDocs, pendingSoftDeletes.GetLiveDocs()) {
		t.Fatal("assertSame(liveDocs, pendingSoftDeletes.getLiveDocs())")
	}

	// now apply a hard delete
	if _, err := writer.DeleteDocuments([]Term{*NewTerm("id", "1")}); err != nil {
		t.Fatalf("deleteDocuments: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	pendingSoftDeletesCloseAll(t, reader)
	reader, segmentReader = softDeletesOnlySegmentReader(t, dir)
	segmentInfo = segmentReader.GetSegmentCommitInfo()
	pendingSoftDeletes = NewPendingSoftDeletes("_soft_deletes", segmentInfo)
	mustOnNewReader(t, pendingSoftDeletes, segmentReader, segmentInfo)
	if pendingSoftDeletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", pendingSoftDeletes.NumPendingDeletes())
	}
	if pendingSoftDeletes.GetDelCount() != 2 {
		t.Fatalf("expected 2, got %d", pendingSoftDeletes.GetDelCount())
	}
	assertLive(t, pendingSoftDeletes.GetLiveDocs(), false, false, true)
	if pendingSoftDeletes.GetHardLiveDocs() == nil {
		t.Fatal("assertNotNull(pendingSoftDeletes.getHardLiveDocs())")
	}
	assertLive(t, pendingSoftDeletes.GetHardLiveDocs(), false, true, true)
	pendingSoftDeletesCloseAll(t, reader, writer, dir)
}

// singleUpdateIterator renders the anonymous DocValuesFieldUpdates.Iterator of
// singleUpdate(List<Integer>, int, boolean).
type singleUpdateIterator struct {
	docsChanged []int
	next        int
	doc         int
	hasValue    bool
}

func (it *singleUpdateIterator) NextDoc() int {
	it.doc = it.docsChanged[it.next]
	it.next++
	return it.doc
}
func (it *singleUpdateIterator) DocID() int       { return it.doc }
func (it *singleUpdateIterator) LongValue() int64 { return 1 }
func (it *singleUpdateIterator) BinaryValue() *util.BytesRef {
	panic("UnsupportedOperationException")
}
func (it *singleUpdateIterator) DelGen() int64  { return 0 }
func (it *singleUpdateIterator) HasValue() bool { return it.hasValue }

// singleUpdate renders the private singleUpdate(List<Integer>, int, boolean):
// a NUMERIC DocValuesFieldUpdates for "_soft_deletes" whose iterator walks
// docsChanged. Only its iterator() is used by the tests.
func singleUpdate(docsChanged []int, maxDoc int, hasValue bool) func() DocValuesFieldUpdatesIterator {
	_ = maxDoc // the Java DocValuesFieldUpdates(maxDoc, 0, "_soft_deletes", NUMERIC) super-constructor argument
	return func() DocValuesFieldUpdatesIterator {
		return &singleUpdateIterator{docsChanged: docsChanged, doc: -1, hasValue: hasValue}
	}
}

// softDeletesFieldInfo renders new FieldInfo("_soft_deletes", 1, false,
// false, false, IndexOptions.NONE, DocValuesType.NUMERIC,
// DocValuesSkipIndexType.NONE, dvGen, Collections.emptyMap(), 0, 0, 0, 0,
// VectorEncoding.FLOAT32, VectorSimilarityFunction.EUCLIDEAN, true, false).
func softDeletesFieldInfo(dvGen int64) *FieldInfo {
	opts := DefaultFieldInfoOptions()
	opts.DocValuesType = DocValuesTypeNumeric
	opts.DocValuesGen = dvGen
	opts.IsSoftDeletesField = true
	return NewFieldInfo("_soft_deletes", 1, opts)
}

func mustOnDocValuesUpdate(t *testing.T, deletes PendingDeletesInterface, fi *FieldInfo, updates ...func() DocValuesFieldUpdatesIterator) {
	t.Helper()
	for _, update := range updates {
		if err := deletes.OnDocValuesUpdate(fi, update()); err != nil {
			t.Fatalf("onDocValuesUpdate: %v", err)
		}
	}
}

func TestPendingSoftDeletesApplyUpdates(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	si, commitInfo := pendingDeletesTestSegment(t, dir, 10)
	writer, err := NewIndexWriter(dir, newIndexWriterConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	for i := 0; i < si.MaxDoc(); i++ {
		if _, err := writer.AddDocument(document.NewDocument()); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	reader, err := OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("DirectoryReader.open(writer): %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	segmentReader, ok := leaves[0].LeafReader().(*SegmentReader)
	if !ok {
		t.Fatalf("leaf is a %T, not a SegmentReader", leaves[0].LeafReader())
	}
	deletes := NewPendingSoftDeletes("_soft_deletes", commitInfo)
	mustOnNewReader(t, deletes, segmentReader, commitInfo)
	pendingSoftDeletesCloseAll(t, reader, writer)
	fieldInfo := softDeletesFieldInfo(0)
	docsDeleted := []int{1, 3, 7, 8, util.NO_MORE_DOCS}
	mustOnDocValuesUpdate(t, deletes, fieldInfo, singleUpdate(docsDeleted, 10, true))
	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}
	if deletes.GetDelCount() != 4 {
		t.Fatalf("expected 4, got %d", deletes.GetDelCount())
	}
	assertLive(t, deletes.GetLiveDocs(), true, false, true, false, true, true, true, false, false, true)

	docsDeleted = []int{1, 2, util.NO_MORE_DOCS}
	fieldInfo = softDeletesFieldInfo(1)
	mustOnDocValuesUpdate(t, deletes, fieldInfo, singleUpdate(docsDeleted, 10, true))
	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}
	if deletes.GetDelCount() != 5 {
		t.Fatalf("expected 5, got %d", deletes.GetDelCount())
	}
	assertLive(t, deletes.GetLiveDocs(), true, false, false, false, true, true, true, false, false, true)
}

func TestPendingSoftDeletesUpdateAppliedOnlyOnce(t *testing.T) {
	dir := newDirectory()
	writer := softDeletesTestWriter(t, dir, 3)
	reader, segmentReader := softDeletesOnlySegmentReader(t, dir)
	segmentInfo := segmentReader.GetSegmentCommitInfo()
	var deletes PendingDeletesInterface = NewPendingSoftDeletes("_soft_deletes", segmentInfo)
	mustOnNewReader(t, deletes, segmentReader, segmentInfo)
	fieldInfo := softDeletesFieldInfo(segmentInfo.NextDocValuesGen())
	docsDeleted := []int{1, util.NO_MORE_DOCS}
	mustOnDocValuesUpdate(t, deletes, fieldInfo, singleUpdate(docsDeleted, 3, true))
	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}
	if deletes.GetDelCount() != 1 {
		t.Fatalf("expected 1, got %d", deletes.GetDelCount())
	}
	assertLive(t, deletes.GetLiveDocs(), true, false, true)
	liveDocs := deletes.GetLiveDocs()
	mustOnNewReader(t, deletes, segmentReader, segmentInfo)
	// no changes we don't apply updates twice
	if !sameBits(liveDocs, deletes.GetLiveDocs()) {
		t.Fatal("assertSame(liveDocs, deletes.getLiveDocs())")
	}
	assertLive(t, deletes.GetLiveDocs(), true, false, true)
	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}
	if deletes.GetDelCount() != 1 {
		t.Fatalf("expected 1, got %d", deletes.GetDelCount())
	}
	pendingSoftDeletesCloseAll(t, reader, writer, dir)
}

func TestPendingSoftDeletesResetOnUpdate(t *testing.T) {
	dir := newDirectory()
	writer := softDeletesTestWriter(t, dir, 3)
	reader, segmentReader := softDeletesOnlySegmentReader(t, dir)
	segmentInfo := segmentReader.GetSegmentCommitInfo()
	var deletes PendingDeletesInterface = NewPendingSoftDeletes("_soft_deletes", segmentInfo)
	mustOnNewReader(t, deletes, segmentReader, segmentInfo)
	fieldInfo := softDeletesFieldInfo(segmentInfo.NextDocValuesGen())
	mustOnDocValuesUpdate(t, deletes, fieldInfo, singleUpdate([]int{0, 1, util.NO_MORE_DOCS}, 3, false))
	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}
	assertLive(t, deletes.GetLiveDocs(), true, true, true)
	liveDocs := deletes.GetLiveDocs()
	mustOnNewReader(t, deletes, segmentReader, segmentInfo)
	// no changes we keep this update
	if !sameBits(liveDocs, deletes.GetLiveDocs()) {
		t.Fatal("assertSame(liveDocs, deletes.getLiveDocs())")
	}
	assertLive(t, deletes.GetLiveDocs(), true, true, true)
	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}

	segmentInfo.AdvanceDocValuesGen()
	fieldInfo = softDeletesFieldInfo(segmentInfo.NextDocValuesGen())
	mustOnDocValuesUpdate(t, deletes, fieldInfo, singleUpdate([]int{1, util.NO_MORE_DOCS}, 3, true))
	// no changes we keep this update
	if sameBits(liveDocs, deletes.GetLiveDocs()) {
		t.Fatal("assertNotSame(liveDocs, deletes.getLiveDocs())")
	}
	assertLive(t, deletes.GetLiveDocs(), true, false, true)
	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}
	if deletes.GetDelCount() != 1 {
		t.Fatalf("expected 1, got %d", deletes.GetDelCount())
	}
	pendingSoftDeletesCloseAll(t, reader, writer, dir)
}
