// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// newRAUForTest builds a ReadersAndUpdates wired to a fresh in-memory
// segment commit info and a fresh PendingDeletes. The reader is left
// unset so DropReaders does not try to close anything.
func newRAUForTest(t *testing.T, name string, maxDoc int) *ReadersAndUpdates {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	si := NewSegmentInfo(name, maxDoc, dir)
	sci := NewSegmentCommitInfo(si, 0, -1)
	pd := newPendingDeletes()
	rau, err := NewReadersAndUpdates(10, sci, pd)
	if err != nil {
		t.Fatalf("NewReadersAndUpdates: %v", err)
	}
	return rau
}

// newFinishedBinaryUpdate creates a finished binary DV update packet
// against the supplied (field, delGen) bound. One AddBinary call is
// issued so Any() returns true.
func newFinishedBinaryUpdate(t *testing.T, field string, delGen int64, maxDoc int) *BinaryDocValuesFieldUpdates {
	t.Helper()
	up, err := NewBinaryDocValuesFieldUpdates(delGen, field, maxDoc)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := up.AddBinary(0, util.NewBytesRef([]byte("v"))); err != nil {
		t.Fatalf("AddBinary: %v", err)
	}
	if err := up.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	return up
}

func TestReadersAndUpdates_RefCountLifecycle(t *testing.T) {
	rau := newRAUForTest(t, "_0", 16)

	if got, want := rau.RefCount(), 1; got != want {
		t.Fatalf("initial refCount = %d, want %d", got, want)
	}
	if err := rau.IncRef(); err != nil {
		t.Fatalf("IncRef: %v", err)
	}
	if got, want := rau.RefCount(), 2; got != want {
		t.Fatalf("after IncRef = %d, want %d", got, want)
	}
	if err := rau.DecRef(); err != nil {
		t.Fatalf("DecRef: %v", err)
	}
	if err := rau.DecRef(); err != nil {
		t.Fatalf("DecRef (back to 0): %v", err)
	}
	if got := rau.RefCount(); got != 0 {
		t.Fatalf("final refCount = %d, want 0", got)
	}
	// Going negative must surface the invariant violation.
	if err := rau.DecRef(); err == nil {
		t.Fatalf("DecRef below 0 returned nil; want invariant error")
	}
}

func TestReadersAndUpdates_RejectsNilArgs(t *testing.T) {
	if _, err := NewReadersAndUpdates(0, nil, newPendingDeletes()); err == nil {
		t.Fatalf("nil info accepted; want error")
	}
	dir := store.NewByteBuffersDirectory()
	si := NewSegmentInfo("_x", 1, dir)
	sci := NewSegmentCommitInfo(si, 0, -1)
	if _, err := NewReadersAndUpdates(0, sci, nil); err == nil {
		t.Fatalf("nil pendingDeletes accepted; want error")
	}
	if _, err := NewReadersAndUpdatesFromReader(0, nil, newPendingDeletes()); err == nil {
		t.Fatalf("nil reader accepted; want error")
	}
}

func TestReadersAndUpdates_AddDVUpdate_HappyPath(t *testing.T) {
	rau := newRAUForTest(t, "_1", 8)
	update := newFinishedBinaryUpdate(t, "bin", 1, 8)

	if err := rau.AddDVUpdate(&update.BaseDocValuesFieldUpdates); err != nil {
		t.Fatalf("AddDVUpdate: %v", err)
	}
	if got, want := rau.GetNumDVUpdates(), int64(1); got != want {
		t.Fatalf("GetNumDVUpdates = %d, want %d", got, want)
	}
	if rau.RamBytesUsed() <= 0 {
		t.Fatalf("RamBytesUsed = %d, want > 0", rau.RamBytesUsed())
	}
}

func TestReadersAndUpdates_AddDVUpdate_RejectsUnfinished(t *testing.T) {
	rau := newRAUForTest(t, "_2", 8)
	up, err := NewBinaryDocValuesFieldUpdates(1, "bin", 8)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	// No Finish() call: GetFinished() is false.
	if err := rau.AddDVUpdate(&up.BaseDocValuesFieldUpdates); !errors.Is(err, ErrReadersAndUpdatesUpdateNotFinished) {
		t.Fatalf("unfinished packet accepted: err=%v, want ErrReadersAndUpdatesUpdateNotFinished", err)
	}
}

func TestReadersAndUpdates_AddDVUpdate_DuplicateDelGen(t *testing.T) {
	rau := newRAUForTest(t, "_3", 8)
	first := newFinishedBinaryUpdate(t, "bin", 7, 8)
	dup := newFinishedBinaryUpdate(t, "bin", 7, 8) // same field+delGen

	if err := rau.AddDVUpdate(&first.BaseDocValuesFieldUpdates); err != nil {
		t.Fatalf("first AddDVUpdate: %v", err)
	}
	err := rau.AddDVUpdate(&dup.BaseDocValuesFieldUpdates)
	if err == nil {
		t.Fatalf("duplicate delGen accepted; want error")
	}
	if !strings.Contains(err.Error(), "duplicate delGen") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadersAndUpdates_AddDVUpdate_MirrorsToMergingWhenMerging(t *testing.T) {
	rau := newRAUForTest(t, "_4", 8)
	if err := rau.SetIsMerging(); err != nil {
		t.Fatalf("SetIsMerging: %v", err)
	}
	if !rau.IsMerging() {
		t.Fatalf("IsMerging = false after SetIsMerging")
	}

	up := newFinishedBinaryUpdate(t, "bin", 1, 8)
	if err := rau.AddDVUpdate(&up.BaseDocValuesFieldUpdates); err != nil {
		t.Fatalf("AddDVUpdate: %v", err)
	}

	merging := rau.GetMergingDVUpdates()
	if len(merging["bin"]) != 1 {
		t.Fatalf("merging[bin] len = %d, want 1", len(merging["bin"]))
	}
	// GetMergingDVUpdates atomically clears isMerging.
	if rau.IsMerging() {
		t.Fatalf("IsMerging still true after GetMergingDVUpdates")
	}
}

func TestReadersAndUpdates_SetIsMerging_RejectsNonEmptyMergingMap(t *testing.T) {
	rau := newRAUForTest(t, "_5", 8)
	// Force a non-empty mergingDVUpdates without going through SetIsMerging.
	rau.mu.Lock()
	rau.mergingDVUpdates["x"] = []*readersAndUpdatesPacket{
		{inner: &newFinishedBinaryUpdate(t, "x", 1, 8).BaseDocValuesFieldUpdates},
	}
	rau.mu.Unlock()
	if err := rau.SetIsMerging(); err == nil {
		t.Fatalf("SetIsMerging with non-empty mergingDVUpdates accepted; want error")
	}
}

func TestReadersAndUpdates_DropMergingUpdates(t *testing.T) {
	rau := newRAUForTest(t, "_6", 8)
	if err := rau.SetIsMerging(); err != nil {
		t.Fatalf("SetIsMerging: %v", err)
	}
	up := newFinishedBinaryUpdate(t, "bin", 1, 8)
	if err := rau.AddDVUpdate(&up.BaseDocValuesFieldUpdates); err != nil {
		t.Fatalf("AddDVUpdate: %v", err)
	}
	rau.DropMergingUpdates()
	if rau.IsMerging() {
		t.Fatalf("IsMerging = true after DropMergingUpdates")
	}
	if got := rau.GetMergingDVUpdates(); len(got) != 0 {
		t.Fatalf("GetMergingDVUpdates non-empty after drop: %v", got)
	}
	// Pending updates must NOT be dropped — only merging.
	if rau.GetNumDVUpdates() != 1 {
		t.Fatalf("pending updates lost on DropMergingUpdates")
	}
}

func TestReadersAndUpdates_PruneAppliedDVUpdates(t *testing.T) {
	rau := newRAUForTest(t, "_7", 8)
	// Three packets: delGens 1, 2, 5. Prune with maxDelGen=2 must drop 1+2.
	for _, gen := range []int64{1, 2, 5} {
		up := newFinishedBinaryUpdate(t, "bin", gen, 8)
		if err := rau.AddDVUpdate(&up.BaseDocValuesFieldUpdates); err != nil {
			t.Fatalf("AddDVUpdate gen=%d: %v", gen, err)
		}
	}
	before := rau.RamBytesUsed()
	freed := rau.PruneAppliedDVUpdates(2)
	if freed <= 0 {
		t.Fatalf("PruneAppliedDVUpdates returned %d, want > 0", freed)
	}
	if got := rau.GetNumDVUpdates(); got != 1 {
		t.Fatalf("remaining packets = %d, want 1", got)
	}
	after := rau.RamBytesUsed()
	if after != before-freed {
		t.Fatalf("RamBytesUsed accounting drift: before=%d freed=%d after=%d", before, freed, after)
	}
}

func TestReadersAndUpdates_PruneAllRemovesField(t *testing.T) {
	rau := newRAUForTest(t, "_8", 8)
	up := newFinishedBinaryUpdate(t, "only", 1, 8)
	if err := rau.AddDVUpdate(&up.BaseDocValuesFieldUpdates); err != nil {
		t.Fatalf("AddDVUpdate: %v", err)
	}
	rau.PruneAppliedDVUpdates(1)

	rau.mu.Lock()
	_, present := rau.pendingDVUpdates["only"]
	rau.mu.Unlock()
	if present {
		t.Fatalf("field key not removed after pruning all packets")
	}
}

func TestReadersAndUpdates_WriteFieldUpdatesFastPath(t *testing.T) {
	rau := newRAUForTest(t, "_9", 8)

	// Empty: returns (false, nil) like Lucene.
	wrote, err := rau.WriteFieldUpdates(nil, nil, 100)
	if err != nil {
		t.Fatalf("empty WriteFieldUpdates: err=%v", err)
	}
	if wrote {
		t.Fatalf("empty WriteFieldUpdates reported wrote=true")
	}

	// Packet with delGen=3, maxDelGen=2 → not eligible → fast path again.
	up := newFinishedBinaryUpdate(t, "bin", 3, 8)
	if err := rau.AddDVUpdate(&up.BaseDocValuesFieldUpdates); err != nil {
		t.Fatalf("AddDVUpdate: %v", err)
	}
	wrote, err = rau.WriteFieldUpdates(nil, nil, 2)
	if err != nil {
		t.Fatalf("ineligible WriteFieldUpdates: err=%v", err)
	}
	if wrote {
		t.Fatalf("ineligible WriteFieldUpdates reported wrote=true")
	}
}

func TestReadersAndUpdates_WriteFieldUpdatesGapError(t *testing.T) {
	rau := newRAUForTest(t, "_10", 8)
	up := newFinishedBinaryUpdate(t, "bin", 1, 8)
	if err := rau.AddDVUpdate(&up.BaseDocValuesFieldUpdates); err != nil {
		t.Fatalf("AddDVUpdate: %v", err)
	}
	_, err := rau.WriteFieldUpdates(nil, nil, 5)
	if !errors.Is(err, ErrReadersAndUpdatesDVWriteUnsupported) {
		t.Fatalf("WriteFieldUpdates with eligible packet: err=%v, want ErrReadersAndUpdatesDVWriteUnsupported", err)
	}
}

func TestReadersAndUpdates_GapErrors(t *testing.T) {
	rau := newRAUForTest(t, "_11", 8)
	if _, err := rau.GetReadOnlyClone(); !errors.Is(err, ErrReadersAndUpdatesReadOnlyCloneUnsupported) {
		t.Fatalf("GetReadOnlyClone: err=%v", err)
	}
	if _, err := rau.NumDeletesToMerge(nil); !errors.Is(err, ErrReadersAndUpdatesMergeReaderUnsupported) {
		t.Fatalf("NumDeletesToMerge: err=%v", err)
	}
	if _, err := rau.GetLiveDocs(); !errors.Is(err, ErrReadersAndUpdatesLiveDocsUnsupported) {
		t.Fatalf("GetLiveDocs: err=%v", err)
	}
	if _, err := rau.GetHardLiveDocs(); !errors.Is(err, ErrReadersAndUpdatesLiveDocsUnsupported) {
		t.Fatalf("GetHardLiveDocs: err=%v", err)
	}
	if _, err := rau.WriteLiveDocs(nil); !errors.Is(err, ErrReadersAndUpdatesLiveDocsUnsupported) {
		t.Fatalf("WriteLiveDocs: err=%v", err)
	}
	if _, err := rau.GetReaderForMerge(); !errors.Is(err, ErrReadersAndUpdatesMergeReaderUnsupported) {
		t.Fatalf("GetReaderForMerge: err=%v", err)
	}
	if _, err := rau.IsFullyDeleted(); !errors.Is(err, ErrReadersAndUpdatesLiveDocsUnsupported) {
		t.Fatalf("IsFullyDeleted: err=%v", err)
	}
	if _, err := rau.KeepFullyDeletedSegment(nil); !errors.Is(err, ErrReadersAndUpdatesLiveDocsUnsupported) {
		t.Fatalf("KeepFullyDeletedSegment: err=%v", err)
	}
}

func TestReadersAndUpdates_DeleteAndDropChanges(t *testing.T) {
	rau := newRAUForTest(t, "_12", 8)
	first, err := rau.Delete(3)
	if err != nil {
		t.Fatalf("Delete(3): %v", err)
	}
	if !first {
		t.Fatalf("first Delete(3) returned false; want true")
	}
	second, err := rau.Delete(3)
	if err != nil {
		t.Fatalf("Delete(3) repeat: %v", err)
	}
	if second {
		t.Fatalf("repeat Delete(3) returned true; want false")
	}
	if got, want := rau.GetDelCount(), 1; got != want {
		t.Fatalf("GetDelCount = %d, want %d", got, want)
	}
	rau.DropChanges()
	if got, want := rau.GetDelCount(), 0; got != want {
		t.Fatalf("GetDelCount after DropChanges = %d, want %d", got, want)
	}
}

func TestReadersAndUpdates_Delete_RejectsNegative(t *testing.T) {
	rau := newRAUForTest(t, "_13", 8)
	if _, err := rau.Delete(-1); err == nil {
		t.Fatalf("Delete(-1) accepted; want error")
	}
}

func TestReadersAndUpdates_String(t *testing.T) {
	rau := newRAUForTest(t, "_14", 8)
	s := rau.String()
	if !strings.HasPrefix(s, "ReadersAndLiveDocs(seg=") {
		t.Fatalf("String() = %q, want ReadersAndLiveDocs(seg=... prefix", s)
	}
	if !strings.Contains(s, "pendingDeletes=") {
		t.Fatalf("String() missing pendingDeletes: %q", s)
	}
}

func TestReadersAndUpdates_GetReader_LazyOpen(t *testing.T) {
	rau := newRAUForTest(t, "_15", 8)
	r1, err := rau.GetReader()
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	if r1 == nil {
		t.Fatalf("GetReader returned nil reader")
	}
	r2, err := rau.GetReader()
	if err != nil {
		t.Fatalf("GetReader (cached): %v", err)
	}
	if r1 != r2 {
		t.Fatalf("GetReader did not cache: r1=%p r2=%p", r1, r2)
	}
	if err := rau.Release(r1); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if err := rau.Release(nil); err == nil {
		t.Fatalf("Release(nil) accepted; want error")
	}
}

func TestReadersAndUpdates_SortMap(t *testing.T) {
	rau := newRAUForTest(t, "_16", 8)
	if rau.SortMap() != nil {
		t.Fatalf("initial SortMap non-nil")
	}
	m := stubSortMap{newToOld: []int{2, 0, 1}}
	rau.SetSortMap(m)
	if rau.SortMap() == nil {
		t.Fatalf("SortMap nil after SetSortMap")
	}
	if rau.SortMap().NewToOld(0) != 2 {
		t.Fatalf("stub map NewToOld broken")
	}
}

// stubSortMap is a trivial sortDocMap stand-in for the SortMap accessor
// test; the real Sorter.DocMap port lands in a later sprint.
type stubSortMap struct{ newToOld []int }

func (s stubSortMap) NewToOld(i int) int { return s.newToOld[i] }
func (s stubSortMap) OldToNew(i int) int {
	for newID, oldID := range s.newToOld {
		if oldID == i {
			return newID
		}
	}
	return -1
}
func (s stubSortMap) Size() int { return len(s.newToOld) }

func TestReadersAndUpdates_ConcurrentAddDVUpdate(t *testing.T) {
	rau := newRAUForTest(t, "_17", 8)
	const n = 32
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := int64(0); i < n; i++ {
		wg.Add(1)
		go func(gen int64) {
			defer wg.Done()
			up := newFinishedBinaryUpdate(t, "bin", gen, 8)
			errs <- rau.AddDVUpdate(&up.BaseDocValuesFieldUpdates)
		}(i + 1)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent AddDVUpdate: %v", err)
		}
	}
	if got, want := rau.GetNumDVUpdates(), int64(n); got != want {
		t.Fatalf("GetNumDVUpdates = %d, want %d", got, want)
	}
}
