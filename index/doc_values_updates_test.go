// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// DocValuesUpdateQueue
// ---------------------------------------------------------------------------

func TestDocValuesUpdateQueue_AddAndRetrieve(t *testing.T) {
	q := NewDocValuesUpdateQueue()
	term := NewTerm("id", "doc1")
	update := &DocValuesUpdate{Field: "price", Term: term, Type: DocValuesTypeNumeric}
	q.AddUpdate(update)

	if q.Size() != 1 {
		t.Fatalf("Size = %d, want 1", q.Size())
	}
	pkg, ok := q.GetPackage("price")
	if !ok {
		t.Fatalf("GetPackage(\"price\") not found")
	}
	got, found := pkg.GetUpdate("doc1")
	if !found {
		t.Fatalf("GetUpdate(\"doc1\") not found")
	}
	if got != update {
		t.Fatalf("retrieved update != inserted update")
	}
}

func TestDocValuesUpdateQueue_Clear(t *testing.T) {
	q := NewDocValuesUpdateQueue()
	q.AddUpdate(&DocValuesUpdate{Field: "f", Term: NewTerm("id", "x"), Type: DocValuesTypeNumeric})
	q.Clear()
	if q.Size() != 0 {
		t.Fatalf("Size after Clear = %d, want 0", q.Size())
	}
}

// ---------------------------------------------------------------------------
// DocValuesUpdateWriter / DocValuesUpdateReader round-trip
// ---------------------------------------------------------------------------

func TestDocValuesUpdates_RoundTrip_Numeric(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 3, dir)

	q := NewDocValuesUpdateQueue()
	q.AddUpdate(&DocValuesUpdate{
		Field: "price",
		Term:  NewTerm("id", "doc0"),
		Type:  DocValuesTypeNumeric,
		Value: &NumericDocValuesUpdate{NumericValue: 42},
	})
	q.AddUpdate(&DocValuesUpdate{
		Field: "price",
		Term:  NewTerm("id", "doc1"),
		Type:  DocValuesTypeNumeric,
		Value: &NumericDocValuesUpdate{NumericValue: 99},
	})

	w := NewDocValuesUpdateWriter(dir, info)
	if err := w.WriteUpdates(q); err != nil {
		t.Fatalf("WriteUpdates: %v", err)
	}

	r := NewDocValuesUpdateReader(dir)
	got, err := r.ReadUpdates(info)
	if err != nil {
		t.Fatalf("ReadUpdates: %v", err)
	}
	if got.Size() != 2 {
		t.Fatalf("round-trip Size = %d, want 2", got.Size())
	}
}

func TestDocValuesUpdateReader_MissingFile_ReturnsEmpty(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg_absent", 0, dir)
	r := NewDocValuesUpdateReader(dir)
	q, err := r.ReadUpdates(info)
	if err != nil {
		t.Fatalf("ReadUpdates on absent file: %v", err)
	}
	if q.Size() != 0 {
		t.Fatalf("expected empty queue for absent file, got Size=%d", q.Size())
	}
}

// ---------------------------------------------------------------------------
// DocValuesUpdateMerger.MergeUpdates — AC2 of rmp #4648
//
// Verifies that term-based DV updates survive a segment merge intact.
// Concretely: two source segments each carry their own pending numeric updates.
// After MergeUpdates the resulting queue must contain all updates from both
// sources — no remap, no loss — because DocValuesUpdate is term-based: the
// target document is identified by its Term, not by a pre-resolved docID.
// The merged segment re-resolves the Term against its own postings when the
// update is applied, so the update survives the merge boundary unchanged.
// ---------------------------------------------------------------------------

func TestDocValuesUpdateMerger_MergeUpdates_PreservesTermBasedUpdates(t *testing.T) {
	dir1 := store.NewByteBuffersDirectory()
	info1 := NewSegmentInfo("seg0", 2, dir1)

	dir2 := store.NewByteBuffersDirectory()
	info2 := NewSegmentInfo("seg1", 2, dir2)

	// Write updates for seg0: numeric field "score", terms "a" and "b".
	q1 := NewDocValuesUpdateQueue()
	q1.AddUpdate(&DocValuesUpdate{
		Field: "score",
		Term:  NewTerm("id", "a"),
		Type:  DocValuesTypeNumeric,
		Value: &NumericDocValuesUpdate{NumericValue: 10},
	})
	q1.AddUpdate(&DocValuesUpdate{
		Field: "score",
		Term:  NewTerm("id", "b"),
		Type:  DocValuesTypeNumeric,
		Value: &NumericDocValuesUpdate{NumericValue: 20},
	})
	if err := NewDocValuesUpdateWriter(dir1, info1).WriteUpdates(q1); err != nil {
		t.Fatalf("WriteUpdates seg0: %v", err)
	}

	// Write updates for seg1: same field "score", term "c".
	q2 := NewDocValuesUpdateQueue()
	q2.AddUpdate(&DocValuesUpdate{
		Field: "score",
		Term:  NewTerm("id", "c"),
		Type:  DocValuesTypeNumeric,
		Value: &NumericDocValuesUpdate{NumericValue: 30},
	})
	if err := NewDocValuesUpdateWriter(dir2, info2).WriteUpdates(q2); err != nil {
		t.Fatalf("WriteUpdates seg1: %v", err)
	}

	// Build SegmentInfos with both source segments.
	sis := NewSegmentInfos()
	sis.Add(NewSegmentCommitInfo(info1, 0, -1))
	sis.Add(NewSegmentCommitInfo(info2, 0, -1))

	// Each reader is bound to the directory where its segment lives.
	r1 := NewDocValuesUpdateReader(dir1)
	r2 := NewDocValuesUpdateReader(dir2)

	merger := NewDocValuesUpdateMerger([]*DocValuesUpdateReader{r1, r2})
	merged, err := merger.MergeUpdates(sis)
	if err != nil {
		t.Fatalf("MergeUpdates: %v", err)
	}

	// The merged queue must contain all three term-based updates.
	if merged.Size() != 3 {
		t.Fatalf("merged Size = %d, want 3", merged.Size())
	}

	pkg, ok := merged.GetPackage("score")
	if !ok {
		t.Fatalf("field \"score\" absent in merged queue")
	}

	for _, key := range []string{"a", "b", "c"} {
		if _, found := pkg.GetUpdate(key); !found {
			t.Errorf("update for term %q missing in merged queue", key)
		}
	}
}

func TestDocValuesUpdateMerger_MergeUpdates_EmptySourcesReturnEmptyQueue(t *testing.T) {
	sis := NewSegmentInfos()
	merger := NewDocValuesUpdateMerger(nil)
	merged, err := merger.MergeUpdates(sis)
	if err != nil {
		t.Fatalf("MergeUpdates: %v", err)
	}
	if merged.Size() != 0 {
		t.Fatalf("empty sources: Size = %d, want 0", merged.Size())
	}
}

func TestDocValuesUpdateMerger_MergeUpdates_NoUpdatesFileReturnEmpty(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	info := NewSegmentInfo("seg0", 1, dir)

	sis := NewSegmentInfos()
	sis.Add(NewSegmentCommitInfo(info, 0, -1))

	// No .dvu file written — reader must return empty queue gracefully.
	r := NewDocValuesUpdateReader(dir)
	merger := NewDocValuesUpdateMerger([]*DocValuesUpdateReader{r})
	merged, err := merger.MergeUpdates(sis)
	if err != nil {
		t.Fatalf("MergeUpdates with absent file: %v", err)
	}
	if merged.Size() != 0 {
		t.Fatalf("Size = %d, want 0", merged.Size())
	}
}
