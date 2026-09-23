// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestPendingDeletes.java
// (Apache Lucene 10.5.0). TestPendingSoftDeletes extends this class and
// overrides newPendingDeletes; the shared test bodies below take the factory.

package index

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// pendingDeletesFactory renders the overridable newPendingDeletes(SegmentCommitInfo).
type pendingDeletesFactory func(commitInfo *SegmentCommitInfo) PendingDeletesInterface

func newPendingDeletesForTest(commitInfo *SegmentCommitInfo) PendingDeletesInterface {
	return NewPendingDeletesFromInfo(commitInfo)
}

// pendingDeletesTestSegment renders
// new SegmentInfo(dir, Version.LATEST, Version.LATEST, "test", maxDoc, false,
// false, Codec.getDefault(), Collections.emptyMap(), StringHelper.randomId(),
// new HashMap<>(), null) and
// new SegmentCommitInfo(si, 0, 0, -1, -1, -1, StringHelper.randomId()).
func pendingDeletesTestSegment(t *testing.T, dir store.Directory, maxDoc int) (*SegmentInfo, *SegmentCommitInfo) {
	t.Helper()
	si := NewSegmentInfo("test", maxDoc, dir)
	si.SetVersion(util.Latest.String())
	si.SetMinVersion(util.Latest.String())
	si.SetUseCompoundFile(false)
	si.SetHasBlocks(false)
	si.SetCodec(GetDefaultCodec())
	si.SetDiagnostics(map[string]string{})
	if err := si.SetID(util.RandomId()); err != nil {
		t.Fatalf("setID: %v", err)
	}
	si.SetAttributes(map[string]string{})
	return si, NewSegmentCommitInfo(si, 0, 0, -1, -1, -1, util.RandomId())
}

func mustPendingDelete(t *testing.T, deletes PendingDeletesInterface, docID int) bool {
	t.Helper()
	deleted, err := deletes.Delete(docID)
	if err != nil {
		t.Fatalf("delete(%d): %v", docID, err)
	}
	return deleted
}

func mustWriteLiveDocs(t *testing.T, deletes PendingDeletesInterface, dir store.Directory) bool {
	t.Helper()
	wrote, err := deletes.WriteLiveDocs(dir)
	if err != nil {
		t.Fatalf("writeLiveDocs: %v", err)
	}
	return wrote
}

func pendingDeletesListAll(t *testing.T, dir store.Directory) []string {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	return files
}

func checkPendingDeletesDeleteDoc(t *testing.T, newPendingDeletes pendingDeletesFactory) {
	dir := store.NewByteBuffersDirectory()
	_, commitInfo := pendingDeletesTestSegment(t, dir, 10)
	deletes := newPendingDeletes(commitInfo)
	if deletes.GetLiveDocs() != nil {
		t.Fatal("assertNull(deletes.getLiveDocs())")
	}
	docToDelete := rand.Intn(8)
	if !mustPendingDelete(t, deletes, docToDelete) {
		t.Fatal("assertTrue(deletes.delete(docToDelete))")
	}
	if deletes.GetLiveDocs() == nil {
		t.Fatal("assertNotNull(deletes.getLiveDocs())")
	}
	if deletes.NumPendingDeletes() != 1 {
		t.Fatalf("expected 1, got %d", deletes.NumPendingDeletes())
	}

	liveDocs := deletes.GetLiveDocs()
	if liveDocs.Get(docToDelete) {
		t.Fatal("assertFalse(liveDocs.get(docToDelete))")
	}
	if mustPendingDelete(t, deletes, docToDelete) { // delete again
		t.Fatal("assertFalse(deletes.delete(docToDelete))")
	}

	if !liveDocs.Get(8) {
		t.Fatal("assertTrue(liveDocs.get(8))")
	}
	if !mustPendingDelete(t, deletes, 8) {
		t.Fatal("assertTrue(deletes.delete(8))")
	}
	if !liveDocs.Get(8) { // we have a snapshot
		t.Fatal("assertTrue(liveDocs.get(8))")
	}
	if deletes.NumPendingDeletes() != 2 {
		t.Fatalf("expected 2, got %d", deletes.NumPendingDeletes())
	}

	if !liveDocs.Get(9) {
		t.Fatal("assertTrue(liveDocs.get(9))")
	}
	if !mustPendingDelete(t, deletes, 9) {
		t.Fatal("assertTrue(deletes.delete(9))")
	}
	if !liveDocs.Get(9) {
		t.Fatal("assertTrue(liveDocs.get(9))")
	}

	// now make sure new live docs see the deletions
	liveDocs = deletes.GetLiveDocs()
	if liveDocs.Get(9) || liveDocs.Get(8) || liveDocs.Get(docToDelete) {
		t.Fatal("new live docs must see the deletions")
	}
	if deletes.NumPendingDeletes() != 3 {
		t.Fatalf("expected 3, got %d", deletes.NumPendingDeletes())
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func checkPendingDeletesWriteLiveDocs(t *testing.T, newPendingDeletes pendingDeletesFactory) {
	dir := store.NewByteBuffersDirectory()
	_, commitInfo := pendingDeletesTestSegment(t, dir, 6)
	deletes := newPendingDeletes(commitInfo)
	if mustWriteLiveDocs(t, deletes, dir) {
		t.Fatal("assertFalse(deletes.writeLiveDocs(dir))")
	}
	if n := len(pendingDeletesListAll(t, dir)); n != 0 {
		t.Fatalf("expected 0 files, got %d", n)
	}
	secondDocDeletes := rand.Intn(2) == 0
	mustPendingDelete(t, deletes, 5)
	if secondDocDeletes {
		deletes.GetLiveDocs()
		mustPendingDelete(t, deletes, 2)
	}
	if commitInfo.DelGen() != -1 {
		t.Fatalf("expected delGen -1, got %d", commitInfo.DelGen())
	}
	if commitInfo.DelCount() != 0 {
		t.Fatalf("expected delCount 0, got %d", commitInfo.DelCount())
	}

	want := 1
	if secondDocDeletes {
		want = 2
	}
	if deletes.NumPendingDeletes() != want {
		t.Fatalf("expected %d, got %d", want, deletes.NumPendingDeletes())
	}
	if !mustWriteLiveDocs(t, deletes, dir) {
		t.Fatal("assertTrue(deletes.writeLiveDocs(dir))")
	}
	if n := len(pendingDeletesListAll(t, dir)); n != 1 {
		t.Fatalf("expected 1 file, got %d", n)
	}
	liveDocs, err := GetDefaultCodec().LiveDocsFormat().ReadLiveDocs(dir, commitInfo, store.IOContextDefault)
	if err != nil {
		t.Fatalf("readLiveDocs: %v", err)
	}
	if liveDocs.Get(5) {
		t.Fatal("assertFalse(liveDocs.get(5))")
	}
	if liveDocs.Get(2) == secondDocDeletes {
		t.Fatalf("liveDocs.get(2) = %t with secondDocDeletes=%t", liveDocs.Get(2), secondDocDeletes)
	}
	for _, doc := range []int{0, 1, 3, 4} {
		if !liveDocs.Get(doc) {
			t.Fatalf("assertTrue(liveDocs.get(%d))", doc)
		}
	}

	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}
	if commitInfo.DelCount() != want {
		t.Fatalf("expected delCount %d, got %d", want, commitInfo.DelCount())
	}
	if commitInfo.DelGen() != 1 {
		t.Fatalf("expected delGen 1, got %d", commitInfo.DelGen())
	}

	mustPendingDelete(t, deletes, 0)
	if !mustWriteLiveDocs(t, deletes, dir) {
		t.Fatal("assertTrue(deletes.writeLiveDocs(dir))")
	}
	if n := len(pendingDeletesListAll(t, dir)); n != 2 {
		t.Fatalf("expected 2 files, got %d", n)
	}
	liveDocs, err = GetDefaultCodec().LiveDocsFormat().ReadLiveDocs(dir, commitInfo, store.IOContextDefault)
	if err != nil {
		t.Fatalf("readLiveDocs: %v", err)
	}
	if liveDocs.Get(5) {
		t.Fatal("assertFalse(liveDocs.get(5))")
	}
	if liveDocs.Get(2) == secondDocDeletes {
		t.Fatalf("liveDocs.get(2) = %t with secondDocDeletes=%t", liveDocs.Get(2), secondDocDeletes)
	}
	if liveDocs.Get(0) {
		t.Fatal("assertFalse(liveDocs.get(0))")
	}
	for _, doc := range []int{1, 3, 4} {
		if !liveDocs.Get(doc) {
			t.Fatalf("assertTrue(liveDocs.get(%d))", doc)
		}
	}

	if deletes.NumPendingDeletes() != 0 {
		t.Fatalf("expected 0, got %d", deletes.NumPendingDeletes())
	}
	if commitInfo.DelCount() != want+1 {
		t.Fatalf("expected delCount %d, got %d", want+1, commitInfo.DelCount())
	}
	if commitInfo.DelGen() != 2 {
		t.Fatalf("expected delGen 2, got %d", commitInfo.DelGen())
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func checkPendingDeletesIsFullyDeleted(t *testing.T, newPendingDeletes pendingDeletesFactory) {
	dir := store.NewByteBuffersDirectory()
	si, commitInfo := pendingDeletesTestSegment(t, dir, 3)
	fieldInfos := EmptyFieldInfos
	if err := si.Codec().FieldInfosFormat().Write(dir, si, "", fieldInfos, store.IOContextDefault); err != nil {
		t.Fatalf("fieldInfosFormat().write: %v", err)
	}
	deletes := newPendingDeletes(commitInfo)
	for i := 0; i < 3; i++ {
		if !mustPendingDelete(t, deletes, i) {
			t.Fatalf("assertTrue(deletes.delete(%d))", i)
		}
		if rand.Intn(2) == 0 {
			if !mustWriteLiveDocs(t, deletes, dir) {
				t.Fatal("assertTrue(deletes.writeLiveDocs(dir))")
			}
		}
		fullyDeleted, err := deletes.IsFullyDeleted(func() (CodecReader, error) { return nil, nil })
		if err != nil {
			t.Fatalf("isFullyDeleted: %v", err)
		}
		if fullyDeleted != (i == 2) {
			t.Fatalf("isFullyDeleted at %d: expected %t, got %t", i, i == 2, fullyDeleted)
		}
	}
}

func TestPendingDeletesDeleteDoc(t *testing.T) {
	checkPendingDeletesDeleteDoc(t, newPendingDeletesForTest)
}

func TestPendingDeletesWriteLiveDocs(t *testing.T) {
	checkPendingDeletesWriteLiveDocs(t, newPendingDeletesForTest)
}

func TestPendingDeletesIsFullyDeleted(t *testing.T) {
	checkPendingDeletesIsFullyDeleted(t, newPendingDeletesForTest)
}
