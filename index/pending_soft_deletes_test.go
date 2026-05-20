package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// newPSDTestInfo builds a SegmentCommitInfo with maxDoc live documents.
// SegmentCommitInfo.MaxDoc reports docCount-1, so docCount is maxDoc+1.
// delGen follows the canonical convention: -1 when delCount is 0 (so
// HasDeletions reports false and the hard-deletes base initializes),
// otherwise a non-negative generation.
func newPSDTestInfo(t *testing.T, name string, maxDoc, delCount int) *SegmentCommitInfo {
	t.Helper()
	si := NewSegmentInfo(name, maxDoc+1, nil)
	delGen := int64(-1)
	if delCount > 0 {
		delGen = int64(delCount)
	}
	return NewSegmentCommitInfo(si, delCount, delGen)
}

// fixedBits returns a FixedBitSet of length n with every bit set.
func fixedBits(t *testing.T, n int) *util.FixedBitSet {
	t.Helper()
	b, err := util.NewFixedBitSet(n)
	if err != nil {
		t.Fatalf("NewFixedBitSet(%d): %v", n, err)
	}
	b.SetAll()
	return b
}

// stubDISI is a soft-deletes doc-id iterator over a fixed slice, satisfying
// the package-local softDeletesDISI interface.
type stubDISI struct {
	docs []int
	pos  int
}

func (s *stubDISI) NextDoc() (int, error) {
	if s.pos >= len(s.docs) {
		return util.NO_MORE_DOCS, nil
	}
	d := s.docs[s.pos]
	s.pos++
	return d, nil
}

// updateEntry is a single (docID, hasValue) pair for stubUpdates.
type updateEntry struct {
	doc      int
	hasValue bool
}

// stubUpdates is a DocValuesFieldUpdatesIterator over a fixed entry slice.
type stubUpdates struct {
	entries []updateEntry
	pos     int
	cur     updateEntry
}

func (s *stubUpdates) NextDoc() int {
	if s.pos >= len(s.entries) {
		s.cur = updateEntry{doc: util.NO_MORE_DOCS}
		return util.NO_MORE_DOCS
	}
	s.cur = s.entries[s.pos]
	s.pos++
	return s.cur.doc
}

func (s *stubUpdates) DocID() int                  { return s.cur.doc }
func (s *stubUpdates) LongValue() int64            { return 0 }
func (s *stubUpdates) BinaryValue() *util.BytesRef { return nil }
func (s *stubUpdates) DelGen() int64               { return 0 }
func (s *stubUpdates) HasValue() bool              { return s.cur.hasValue }

func TestApplySoftDeletesFromIterator(t *testing.T) {
	tests := []struct {
		name        string
		bitsLen     int
		preClear    []int
		iterDocs    []int
		wantDeletes int
		wantCleared []int
	}{
		{
			name:        "no docs",
			bitsLen:     8,
			iterDocs:    nil,
			wantDeletes: 0,
		},
		{
			name:        "all distinct live",
			bitsLen:     8,
			iterDocs:    []int{1, 3, 5},
			wantDeletes: 3,
			wantCleared: []int{1, 3, 5},
		},
		{
			name:        "already cleared not counted",
			bitsLen:     8,
			preClear:    []int{2},
			iterDocs:    []int{2, 4},
			wantDeletes: 1,
			wantCleared: []int{2, 4},
		},
		{
			name:        "repeated doc counted once",
			bitsLen:     8,
			iterDocs:    []int{6, 6},
			wantDeletes: 1,
			wantCleared: []int{6},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bits := fixedBits(t, tc.bitsLen)
			for _, d := range tc.preClear {
				bits.Clear(d)
			}
			got, err := applySoftDeletesFromIterator(&stubDISI{docs: tc.iterDocs}, bits)
			if err != nil {
				t.Fatalf("applySoftDeletesFromIterator: %v", err)
			}
			if got != tc.wantDeletes {
				t.Errorf("newDeletes = %d, want %d", got, tc.wantDeletes)
			}
			for _, d := range tc.wantCleared {
				if bits.Get(d) {
					t.Errorf("bit %d still set, want cleared", d)
				}
			}
		})
	}
}

func TestApplySoftDeletesFromUpdates(t *testing.T) {
	tests := []struct {
		name        string
		bitsLen     int
		preClear    []int
		entries     []updateEntry
		wantDeletes int
	}{
		{
			name:        "values clear live bits",
			bitsLen:     8,
			entries:     []updateEntry{{0, true}, {2, true}},
			wantDeletes: 2,
		},
		{
			name:        "reset re-sets a cleared bit",
			bitsLen:     8,
			preClear:    []int{3},
			entries:     []updateEntry{{3, false}},
			wantDeletes: -1,
		},
		{
			name:        "mixed value and reset net zero",
			bitsLen:     8,
			preClear:    []int{5},
			entries:     []updateEntry{{1, true}, {5, false}},
			wantDeletes: 0,
		},
		{
			name:        "value on already-cleared bit not counted",
			bitsLen:     8,
			preClear:    []int{4},
			entries:     []updateEntry{{4, true}},
			wantDeletes: 0,
		},
		{
			name:        "reset on already-live bit not counted",
			bitsLen:     8,
			entries:     []updateEntry{{6, false}},
			wantDeletes: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bits := fixedBits(t, tc.bitsLen)
			for _, d := range tc.preClear {
				bits.Clear(d)
			}
			got := applySoftDeletesFromUpdates(&stubUpdates{entries: tc.entries}, bits)
			if got != tc.wantDeletes {
				t.Errorf("newDeletes = %d, want %d", got, tc.wantDeletes)
			}
		})
	}
}

func TestCountSoftDeletes(t *testing.T) {
	tests := []struct {
		name        string
		softDeleted *stubDISI
		hardDeletes util.Bits
		want        int
	}{
		{
			name:        "nil iterator yields zero",
			softDeleted: nil,
			hardDeletes: nil,
			want:        0,
		},
		{
			name:        "nil hardDeletes counts every soft doc",
			softDeleted: &stubDISI{docs: []int{0, 2, 4}},
			hardDeletes: nil,
			want:        3,
		},
		{
			name:        "only hard-live soft docs counted",
			softDeleted: &stubDISI{docs: []int{0, 1, 2}},
			// doc 1 is hard-deleted (bit cleared), so it must not count.
			hardDeletes: func() util.Bits {
				b := fixedBits(t, 8)
				b.Clear(1)
				return b
			}(),
			want: 2,
		},
		{
			name:        "all soft docs hard-deleted yields zero",
			softDeleted: &stubDISI{docs: []int{3, 5}},
			hardDeletes: func() util.Bits {
				b := fixedBits(t, 8)
				b.Clear(3)
				b.Clear(5)
				return b
			}(),
			want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var disi softDeletesDISI
			if tc.softDeleted != nil {
				disi = tc.softDeleted
			}
			got, err := CountSoftDeletes(disi, tc.hardDeletes)
			if err != nil {
				t.Fatalf("CountSoftDeletes: %v", err)
			}
			if got != tc.want {
				t.Errorf("count = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestNewPendingSoftDeletes(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	if psd.field != "_soft_" {
		t.Errorf("field = %q, want %q", psd.field, "_soft_")
	}
	if psd.dvGeneration != pendingSoftDeletesUninitializedGen {
		t.Errorf("dvGeneration = %d, want %d", psd.dvGeneration, pendingSoftDeletesUninitializedGen)
	}
	if psd.hardDeletes == nil {
		t.Fatal("hardDeletes is nil")
	}
	// numPendingDeletesHook must be installed so getDelCount sees the
	// soft+hard override.
	if psd.pendingDeletesBase.numPendingDeletesHook == nil {
		t.Error("numPendingDeletesHook not installed")
	}
	if got := psd.numPendingDeletes(); got != 0 {
		t.Errorf("numPendingDeletes = %d, want 0", got)
	}
}

func TestPendingSoftDeletesNumPendingDeletes(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	// Apply a soft delete through the update path.
	if err := psd.OnDocValuesUpdate(
		NewFieldInfo("_soft_", 0, DefaultFieldInfoOptions()),
		&stubUpdates{entries: []updateEntry{{0, true}}},
	); err != nil {
		t.Fatalf("OnDocValuesUpdate: %v", err)
	}
	// WriteLiveDocs/OnDocValuesUpdate drop the soft pending count after
	// folding it into SoftDelCount, so the soft component is back to 0.
	if got := psd.numPendingDeletes(); got != 0 {
		t.Errorf("numPendingDeletes after update = %d, want 0", got)
	}

	// A hard delete raises hardDeletes.numPendingDeletes, and the override
	// sums soft+hard.
	if _, err := psd.Delete(1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := psd.numPendingDeletes(); got != 1 {
		t.Errorf("numPendingDeletes after hard delete = %d, want 1", got)
	}
}

func TestPendingSoftDeletesDelete(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	// Delete always reports the hard-delete outcome; a fresh doc is hard
	// deleted successfully.
	ok, err := psd.Delete(2)
	if err != nil {
		t.Fatalf("Delete(2): %v", err)
	}
	if !ok {
		t.Error("Delete(2) = false, want true")
	}
	if got := psd.hardDeletes.numPendingDeletes(); got != 1 {
		t.Errorf("hardDeletes pending = %d, want 1", got)
	}

	// Out-of-bounds doc id is an error.
	if _, err := psd.Delete(999); err == nil {
		t.Error("Delete(999): expected out-of-bounds error, got nil")
	}
}

func TestPendingSoftDeletesDeleteDecrementsAlreadySoftDeleted(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	// Materialize the soft live-docs bitset, then soft-delete doc 0 by
	// clearing its bit and counting it. A subsequent hard delete on the
	// same doc must decrement the soft pending count so the doc is not
	// counted twice, mirroring the Java accounting in delete().
	mutable, err := psd.getMutableBits()
	if err != nil {
		t.Fatalf("getMutableBits: %v", err)
	}
	mutable.Clear(0)
	psd.pendingDeleteCount = 1

	ok, err := psd.Delete(0)
	if err != nil {
		t.Fatalf("Delete(0): %v", err)
	}
	if !ok {
		t.Error("Delete(0) = false, want true")
	}
	if psd.pendingDeleteCount != 0 {
		t.Errorf("pendingDeleteCount = %d, want 0 (soft count decremented)", psd.pendingDeleteCount)
	}
}

func TestPendingSoftDeletesDeleteKeepsSoftCountOnSoftLiveDoc(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	// Hard-deleting a doc that is still soft-live clears its soft bit but
	// leaves the soft pending count unchanged: the doc was never counted
	// as a soft delete, so there is nothing to decrement.
	if _, err := psd.getMutableBits(); err != nil {
		t.Fatalf("getMutableBits: %v", err)
	}
	psd.pendingDeleteCount = 1

	ok, err := psd.Delete(0)
	if err != nil {
		t.Fatalf("Delete(0): %v", err)
	}
	if !ok {
		t.Error("Delete(0) = false, want true")
	}
	if psd.pendingDeleteCount != 1 {
		t.Errorf("pendingDeleteCount = %d, want 1 (untouched)", psd.pendingDeleteCount)
	}
}

func TestPendingSoftDeletesOnDocValuesUpdate(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	// An update on an unrelated field only advances dvGeneration.
	other := NewFieldInfo("other", 1, FieldInfoOptions{DocValuesGen: 7})
	if err := psd.OnDocValuesUpdate(other, &stubUpdates{}); err != nil {
		t.Fatalf("OnDocValuesUpdate(other): %v", err)
	}
	if info.SoftDelCount() != 0 {
		t.Errorf("SoftDelCount after unrelated update = %d, want 0", info.SoftDelCount())
	}

	// An update on the soft-deletes field applies the soft deletes and
	// folds them into SoftDelCount.
	soft := NewFieldInfo("_soft_", 0, FieldInfoOptions{DocValuesGen: 9})
	if err := psd.OnDocValuesUpdate(soft, &stubUpdates{
		entries: []updateEntry{{0, true}, {1, true}},
	}); err != nil {
		t.Fatalf("OnDocValuesUpdate(soft): %v", err)
	}
	if info.SoftDelCount() != 2 {
		t.Errorf("SoftDelCount = %d, want 2", info.SoftDelCount())
	}
	if psd.dvGeneration != 9 {
		t.Errorf("dvGeneration = %d, want 9", psd.dvGeneration)
	}
	// The pending count is dropped after folding.
	if psd.pendingDeleteCount != 0 {
		t.Errorf("pendingDeleteCount = %d, want 0", psd.pendingDeleteCount)
	}
}

func TestPendingSoftDeletesWriteLiveDocs(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	// Stage two soft pending deletes directly.
	if _, err := psd.getMutableBits(); err != nil {
		t.Fatalf("getMutableBits: %v", err)
	}
	psd.pendingDeleteCount = 2

	// No hard pending deletes -> WriteLiveDocs reports false and folds the
	// soft count into SoftDelCount.
	if psd.WriteLiveDocs() {
		t.Error("WriteLiveDocs = true, want false (no hard pending deletes)")
	}
	if info.SoftDelCount() != 2 {
		t.Errorf("SoftDelCount = %d, want 2", info.SoftDelCount())
	}
	if psd.pendingDeleteCount != 0 {
		t.Errorf("pendingDeleteCount = %d, want 0", psd.pendingDeleteCount)
	}

	// With a hard pending delete present, WriteLiveDocs reports true.
	if _, err := psd.hardDeletes.delete(3); err != nil {
		t.Fatalf("hardDeletes.delete: %v", err)
	}
	if !psd.WriteLiveDocs() {
		t.Error("WriteLiveDocs = false, want true (hard pending delete present)")
	}
}

func TestPendingSoftDeletesDropChanges(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	// Stage a soft and a hard pending delete.
	if _, err := psd.getMutableBits(); err != nil {
		t.Fatalf("getMutableBits: %v", err)
	}
	psd.pendingDeleteCount = 5
	if _, err := psd.hardDeletes.delete(1); err != nil {
		t.Fatalf("hardDeletes.delete: %v", err)
	}

	// DropChanges resets only the hard pending count; the soft count is
	// left intact.
	psd.DropChanges()
	if psd.hardDeletes.numPendingDeletes() != 0 {
		t.Errorf("hardDeletes pending = %d, want 0", psd.hardDeletes.numPendingDeletes())
	}
	if psd.pendingDeleteCount != 5 {
		t.Errorf("soft pendingDeleteCount = %d, want 5 (untouched)", psd.pendingDeleteCount)
	}
}

func TestPendingSoftDeletesMustInitOnDelete(t *testing.T) {
	// A segment with deletions starts uninitialized -> MustInitOnDelete true.
	withDel := newPSDTestInfo(t, "_0", 9, 3)
	if got := NewPendingSoftDeletes("_soft_", withDel).MustInitOnDelete(); !got {
		t.Error("MustInitOnDelete (delCount>0) = false, want true")
	}

	// A segment without deletions is initialized -> MustInitOnDelete false.
	noDel := newPSDTestInfo(t, "_1", 9, 0)
	if got := NewPendingSoftDeletes("_soft_", noDel).MustInitOnDelete(); got {
		t.Error("MustInitOnDelete (delCount==0) = true, want false")
	}
}

func TestPendingSoftDeletesGetHardLiveDocs(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)

	// Before any hard live-docs are materialized the snapshot is nil.
	if psd.GetHardLiveDocs() != nil {
		t.Error("GetHardLiveDocs = non-nil, want nil before initialization")
	}

	// After a hard delete the snapshot reflects the cleared bit.
	if _, err := psd.hardDeletes.delete(4); err != nil {
		t.Fatalf("hardDeletes.delete: %v", err)
	}
	live := psd.GetHardLiveDocs()
	if live == nil {
		t.Fatal("GetHardLiveDocs = nil after hard delete, want non-nil")
	}
	if live.Get(4) {
		t.Error("hard live docs bit 4 set, want cleared")
	}
	if !live.Get(0) {
		t.Error("hard live docs bit 0 cleared, want set")
	}
}

func TestPendingSoftDeletesString(t *testing.T) {
	info := newPSDTestInfo(t, "_0", 9, 0)
	psd := NewPendingSoftDeletes("_soft_", info)
	if s := psd.String(); s == "" {
		t.Error("String returned empty")
	}
}
