// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
)

// stableImpl tags each input entry with both a key (the bytes the
// sorter sees) and an opaque id (so we can verify stability after the
// sort: equal keys must stay in input order by id).
type stableEntry struct {
	key []byte
	id  int
}

type stableImpl struct {
	data    []stableEntry
	scratch []stableEntry
}

func (s *stableImpl) ByteAt(i, k int) int {
	key := s.data[i].key
	if k >= len(key) {
		return -1
	}
	return int(key[k])
}

func (s *stableImpl) Swap(i, j int) {
	s.data[i], s.data[j] = s.data[j], s.data[i]
}

func (s *stableImpl) Save(i, j int) {
	if cap(s.scratch) < len(s.data) {
		s.scratch = make([]stableEntry, len(s.data))
	}
	s.scratch = s.scratch[:len(s.data)]
	s.scratch[j] = s.data[i]
}

func (s *stableImpl) Restore(i, j int) {
	for k := i; k < j; k++ {
		s.data[k] = s.scratch[k]
	}
}

func newStableImpl(entries []stableEntry) *stableImpl {
	cp := make([]stableEntry, len(entries))
	copy(cp, entries)
	return &stableImpl{data: cp, scratch: make([]stableEntry, len(entries))}
}

func TestStableMSBRadixSorter_PreservesOrderForEqualKeys(t *testing.T) {
	entries := []stableEntry{
		{[]byte("apple"), 0},
		{[]byte("banana"), 1},
		{[]byte("apple"), 2},
		{[]byte("cherry"), 3},
		{[]byte("banana"), 4},
		{[]byte("apple"), 5},
	}
	si := newStableImpl(entries)
	s := NewStableMSBRadixSorter(si, 16)
	s.Sort(0, len(si.data))

	// Verify sorted by key.
	for i := 1; i < len(si.data); i++ {
		if bytes.Compare(si.data[i-1].key, si.data[i].key) > 0 {
			t.Fatalf("not sorted at %d: %q > %q", i, si.data[i-1].key, si.data[i].key)
		}
	}
	// Verify stability: within equal-key groups, ids are increasing.
	for i := 1; i < len(si.data); i++ {
		if bytes.Equal(si.data[i-1].key, si.data[i].key) && si.data[i-1].id > si.data[i].id {
			t.Fatalf("instability at %d: id %d > %d (key %q)",
				i, si.data[i-1].id, si.data[i].id, si.data[i].key)
		}
	}
}

func TestStableMSBRadixSorter_RandomLargeInput(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	const n = 800
	entries := make([]stableEntry, n)
	for i := range entries {
		l := rng.Intn(8) + 1
		buf := make([]byte, l)
		for j := range buf {
			// Restrict alphabet so many duplicates appear.
			buf[j] = byte('a' + rng.Intn(4))
		}
		entries[i] = stableEntry{key: buf, id: i}
	}
	si := newStableImpl(entries)

	// Reference: stable sort.
	wantOrder := make([]stableEntry, n)
	copy(wantOrder, entries)
	sort.SliceStable(wantOrder, func(i, j int) bool {
		return bytes.Compare(wantOrder[i].key, wantOrder[j].key) < 0
	})

	s := NewStableMSBRadixSorter(si, 16)
	s.Sort(0, n)

	for i := range wantOrder {
		if !bytes.Equal(si.data[i].key, wantOrder[i].key) {
			t.Fatalf("key mismatch at %d: got %q want %q", i, si.data[i].key, wantOrder[i].key)
		}
		if si.data[i].id != wantOrder[i].id {
			t.Fatalf("id mismatch at %d (key=%q): got %d want %d",
				i, si.data[i].key, si.data[i].id, wantOrder[i].id)
		}
	}
}

func TestStableMSBRadixSorter_EmptyAndSingleton(t *testing.T) {
	si := newStableImpl(nil)
	NewStableMSBRadixSorter(si, 4).Sort(0, 0)

	si = newStableImpl([]stableEntry{{[]byte("solo"), 0}})
	NewStableMSBRadixSorter(si, 4).Sort(0, 1)
	if string(si.data[0].key) != "solo" {
		t.Fatalf("singleton got mangled: %q", si.data[0].key)
	}
}

func TestStableMSBRadixSorter_SatisfiesInterface(t *testing.T) {
	var _ MSBRadixSorterImpl = (*stableImpl)(nil)
	var _ StableMSBRadixSorterImpl = (*stableImpl)(nil)
}
