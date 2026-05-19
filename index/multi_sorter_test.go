// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Lucene 10.4.0 ships no TestMultiSorter peer. The tests below are
// Gocene-native and exercise the public behaviour stated in the Lucene
// reference Javadoc of MultiSorter#sort:
//   * returns nil when the merged sequence is already in index-sort order;
//   * returns one DocMap per reader otherwise, mapping live docs to the
//     merged position and deleted docs to -1.
//
// The tests inject per-reader ComparableProvider stubs through
// testComparableProvidersHook so they do not depend on the absent
// per-type IndexSorter subclasses.

// liveEverywhere is a util.Bits where every doc is live.
type liveEverywhere struct{ length int }

func (l liveEverywhere) Get(int) bool { return true }
func (l liveEverywhere) Length() int  { return l.length }

// liveExcept marks the listed docs as deleted, others as live.
type liveExcept struct {
	length int
	dead   map[int]struct{}
}

func (l liveExcept) Get(i int) bool {
	_, hole := l.dead[i]
	return !hole
}
func (l liveExcept) Length() int { return l.length }

// codecReaderStub builds a CodecReader exposing only the fields
// MultiSorter actually consumes: MaxDoc and GetLiveDocs.
func codecReaderStub(t *testing.T, maxDoc int, liveDocs util.Bits) *CodecReader {
	t.Helper()
	ir := NewIndexReader()
	ir.SetDocCount(maxDoc)
	lr := &LeafReader{IndexReader: ir, coreCacheKey: NewCacheKey()}
	return &CodecReader{LeafReader: lr, liveDocs: liveDocs, numDocs: maxDoc}
}

// providersFromValues returns a hook that maps every (reader, docID)
// pair to values[readerIdx][docID]. Out-of-range docIDs return 0 (the
// merge cursor never queries past maxDoc).
func providersFromValues(values [][]int64) func(*IndexSorter, []*CodecReader) []ComparableProvider {
	return func(_ *IndexSorter, readers []*CodecReader) []ComparableProvider {
		out := make([]ComparableProvider, len(readers))
		for i := range readers {
			row := values[i]
			out[i] = func(docID int) (int64, error) {
				if docID < 0 || docID >= len(row) {
					return 0, nil
				}
				return row[docID], nil
			}
		}
		return out
	}
}

func withProvidersHook(t *testing.T, hook func(*IndexSorter, []*CodecReader) []ComparableProvider, fn func()) {
	t.Helper()
	prev := testComparableProvidersHook
	testComparableProvidersHook = hook
	t.Cleanup(func() { testComparableProvidersHook = prev })
	fn()
}

func TestMultiSorter_AlreadySortedReturnsNil(t *testing.T) {
	readers := []*CodecReader{
		codecReaderStub(t, 3, liveEverywhere{3}),
		codecReaderStub(t, 2, liveEverywhere{2}),
	}
	// Values strictly ascending across readers in reader-order:
	// reader 0 yields 0,1,2 then reader 1 yields 3,4 — the merge
	// processes reader 0 first then reader 1, so lastReaderIndex is
	// monotonic and isSorted stays true.
	hook := providersFromValues([][]int64{{0, 1, 2}, {3, 4}})

	sort := &Sort{fields: []SortField{NewSortField("dummy", SortTypeLong)}}
	var dm []DocMap
	withProvidersHook(t, hook, func() {
		var err error
		dm, err = multiSorterSort(sort, readers)
		if err != nil {
			t.Fatalf("multiSorterSort: %v", err)
		}
	})
	if dm != nil {
		t.Fatalf("expected nil DocMaps for already-sorted input, got %d maps", len(dm))
	}
}

func TestMultiSorter_InterleavedProducesDocMaps(t *testing.T) {
	readers := []*CodecReader{
		codecReaderStub(t, 2, liveEverywhere{2}),
		codecReaderStub(t, 2, liveEverywhere{2}),
	}
	// reader 0: 10, 30   reader 1: 20, 40
	// merge order by ascending value: r0d0, r1d0, r0d1, r1d1 -> mapped
	// 0,1,2,3. lastReaderIndex flips 0->1->0->1 so isSorted=false.
	hook := providersFromValues([][]int64{{10, 30}, {20, 40}})

	sort := &Sort{fields: []SortField{NewSortField("dummy", SortTypeLong)}}
	var dm []DocMap
	withProvidersHook(t, hook, func() {
		var err error
		dm, err = multiSorterSort(sort, readers)
		if err != nil {
			t.Fatalf("multiSorterSort: %v", err)
		}
	})
	if len(dm) != 2 {
		t.Fatalf("expected 2 DocMaps, got %d", len(dm))
	}
	// reader 0: doc 0 -> 0, doc 1 -> 2
	if got := dm[0].Get(0); got != 0 {
		t.Errorf("dm[0].Get(0) = %d, want 0", got)
	}
	if got := dm[0].Get(1); got != 2 {
		t.Errorf("dm[0].Get(1) = %d, want 2", got)
	}
	// reader 1: doc 0 -> 1, doc 1 -> 3
	if got := dm[1].Get(0); got != 1 {
		t.Errorf("dm[1].Get(0) = %d, want 1", got)
	}
	if got := dm[1].Get(1); got != 3 {
		t.Errorf("dm[1].Get(1) = %d, want 3", got)
	}
}

func TestMultiSorter_DeletedDocsMapToMinusOne(t *testing.T) {
	readers := []*CodecReader{
		codecReaderStub(t, 2, liveExcept{length: 2, dead: map[int]struct{}{1: {}}}),
		codecReaderStub(t, 2, liveEverywhere{2}),
	}
	// reader 0: 10, 30   reader 1: 20, 40   (reader 0 doc 1 is deleted)
	// merge order: r0d0(live,->0), r1d0(live,->1), r0d1(dead,no inc),
	// r1d1(live,->2). lastReaderIndex flips so isSorted=false.
	hook := providersFromValues([][]int64{{10, 30}, {20, 40}})

	sort := &Sort{fields: []SortField{NewSortField("dummy", SortTypeLong)}}
	var dm []DocMap
	withProvidersHook(t, hook, func() {
		var err error
		dm, err = multiSorterSort(sort, readers)
		if err != nil {
			t.Fatalf("multiSorterSort: %v", err)
		}
	})
	if len(dm) != 2 {
		t.Fatalf("expected 2 DocMaps, got %d", len(dm))
	}
	if got := dm[0].Get(0); got != 0 {
		t.Errorf("dm[0].Get(0 live) = %d, want 0", got)
	}
	if got := dm[0].Get(1); got != -1 {
		t.Errorf("dm[0].Get(1 deleted) = %d, want -1", got)
	}
	if got := dm[1].Get(0); got != 1 {
		t.Errorf("dm[1].Get(0) = %d, want 1", got)
	}
	if got := dm[1].Get(1); got != 2 {
		t.Errorf("dm[1].Get(1) = %d, want 2", got)
	}
}

func TestMultiSorter_ReverseFieldReordersDescending(t *testing.T) {
	readers := []*CodecReader{
		codecReaderStub(t, 2, liveEverywhere{2}),
		codecReaderStub(t, 2, liveEverywhere{2}),
	}
	// reader 0: 40, 20   reader 1: 30, 10   (each reader already
	// descending in docID order, satisfying MultiSorter's precondition).
	// With reverse=true the merge order by descending value is:
	// r0d0(40)->0, r1d0(30)->1, r0d1(20)->2, r1d1(10)->3.
	hook := providersFromValues([][]int64{{40, 20}, {30, 10}})

	sf := NewSortField("dummy", SortTypeLong)
	sf.SetReverse(true)
	sort := &Sort{fields: []SortField{sf}}
	var dm []DocMap
	withProvidersHook(t, hook, func() {
		var err error
		dm, err = multiSorterSort(sort, readers)
		if err != nil {
			t.Fatalf("multiSorterSort: %v", err)
		}
	})
	if len(dm) != 2 {
		t.Fatalf("expected 2 DocMaps, got %d", len(dm))
	}
	if got := dm[0].Get(0); got != 0 {
		t.Errorf("dm[0].Get(0) = %d, want 0", got)
	}
	if got := dm[0].Get(1); got != 2 {
		t.Errorf("dm[0].Get(1) = %d, want 2", got)
	}
	if got := dm[1].Get(0); got != 1 {
		t.Errorf("dm[1].Get(0) = %d, want 1", got)
	}
	if got := dm[1].Get(1); got != 3 {
		t.Errorf("dm[1].Get(1) = %d, want 3", got)
	}
}

func TestMultiSorter_NilSortRejected(t *testing.T) {
	if _, err := multiSorterSort(nil, nil); err == nil {
		t.Fatal("expected error for nil sort, got nil")
	}
}
