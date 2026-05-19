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

// stableStringFixture is the StableStringSorter test fixture: it keeps
// the keyed entries together so we can verify stability (equal-key
// entries preserve their input id order after the sort).
type stableStringEntry struct {
	key []byte
	id  int
}

type stableStringFixture struct {
	data    []stableStringEntry
	scratch []stableStringEntry
}

func (f *stableStringFixture) Get(b *BytesRefBuilder, result *BytesRef, i int) {
	k := f.data[i].key
	b.GrowNoCopy(len(k))
	copy(b.Bytes(), k)
	result.Bytes = b.Bytes()
	result.Offset = 0
	result.Length = len(k)
}

func (f *stableStringFixture) Swap(i, j int) {
	f.data[i], f.data[j] = f.data[j], f.data[i]
}

func (f *stableStringFixture) Save(i, j int) {
	if cap(f.scratch) < len(f.data) {
		f.scratch = make([]stableStringEntry, len(f.data))
	}
	f.scratch = f.scratch[:len(f.data)]
	f.scratch[j] = f.data[i]
}

func (f *stableStringFixture) Restore(i, j int) {
	for k := i; k < j; k++ {
		f.data[k] = f.scratch[k]
	}
}

func newStableStringFixture(entries []stableStringEntry) *stableStringFixture {
	cp := make([]stableStringEntry, len(entries))
	copy(cp, entries)
	return &stableStringFixture{data: cp, scratch: make([]stableStringEntry, len(entries))}
}

func TestStableStringSorter_RadixPath_Stability(t *testing.T) {
	entries := []stableStringEntry{
		{[]byte("apple"), 0},
		{[]byte("banana"), 1},
		{[]byte("apple"), 2},
		{[]byte("cherry"), 3},
		{[]byte("banana"), 4},
		{[]byte("apple"), 5},
	}
	f := newStableStringFixture(entries)
	NewStableStringSorter(f, NaturalBytesRefComparator).Sort(0, len(f.data))

	for i := 1; i < len(f.data); i++ {
		if bytes.Compare(f.data[i-1].key, f.data[i].key) > 0 {
			t.Fatalf("not sorted at %d: %q > %q", i, f.data[i-1].key, f.data[i].key)
		}
	}
	for i := 1; i < len(f.data); i++ {
		if bytes.Equal(f.data[i-1].key, f.data[i].key) && f.data[i-1].id > f.data[i].id {
			t.Fatalf("instability at %d (key=%q): id %d > %d",
				i, f.data[i].key, f.data[i-1].id, f.data[i].id)
		}
	}
}

func TestStableStringSorter_FallbackPath_Stability(t *testing.T) {
	entries := []stableStringEntry{
		{[]byte("apple"), 0},
		{[]byte("banana"), 1},
		{[]byte("apple"), 2},
		{[]byte("cherry"), 3},
		{[]byte("banana"), 4},
		{[]byte("apple"), 5},
	}
	f := newStableStringFixture(entries)
	cmp := func(a, b *BytesRef) int {
		return bytes.Compare(a.ValidBytes(), b.ValidBytes())
	}
	NewStableStringSorterFn(f, cmp).Sort(0, len(f.data))

	for i := 1; i < len(f.data); i++ {
		if bytes.Compare(f.data[i-1].key, f.data[i].key) > 0 {
			t.Fatalf("not sorted at %d: %q > %q", i, f.data[i-1].key, f.data[i].key)
		}
	}
	for i := 1; i < len(f.data); i++ {
		if bytes.Equal(f.data[i-1].key, f.data[i].key) && f.data[i-1].id > f.data[i].id {
			t.Fatalf("instability at %d (key=%q): id %d > %d",
				i, f.data[i].key, f.data[i-1].id, f.data[i].id)
		}
	}
}

func TestStableStringSorter_RandomLargeInput_AgainstSortSliceStable(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	const n = 800
	entries := make([]stableStringEntry, n)
	for i := range entries {
		l := rng.Intn(8) + 1
		buf := make([]byte, l)
		for j := range buf {
			buf[j] = byte('a' + rng.Intn(4))
		}
		entries[i] = stableStringEntry{key: buf, id: i}
	}

	want := make([]stableStringEntry, n)
	copy(want, entries)
	sort.SliceStable(want, func(i, j int) bool {
		return bytes.Compare(want[i].key, want[j].key) < 0
	})

	// Radix path.
	radix := newStableStringFixture(entries)
	NewStableStringSorter(radix, NaturalBytesRefComparator).Sort(0, n)
	for i := range want {
		if !bytes.Equal(radix.data[i].key, want[i].key) || radix.data[i].id != want[i].id {
			t.Fatalf("radix mismatch at %d: got (%q,%d) want (%q,%d)",
				i, radix.data[i].key, radix.data[i].id, want[i].key, want[i].id)
		}
	}

	// Fallback path.
	fb := newStableStringFixture(entries)
	NewStableStringSorterFn(fb, func(a, b *BytesRef) int {
		return bytes.Compare(a.ValidBytes(), b.ValidBytes())
	}).Sort(0, n)
	for i := range want {
		if !bytes.Equal(fb.data[i].key, want[i].key) || fb.data[i].id != want[i].id {
			t.Fatalf("fallback mismatch at %d: got (%q,%d) want (%q,%d)",
				i, fb.data[i].key, fb.data[i].id, want[i].key, want[i].id)
		}
	}
}

func TestStableStringSorter_EmptyAndSingleton(t *testing.T) {
	NewStableStringSorter(newStableStringFixture(nil), NaturalBytesRefComparator).Sort(0, 0)
	NewStableStringSorterFn(newStableStringFixture(nil), func(a, b *BytesRef) int { return 0 }).Sort(0, 0)

	f := newStableStringFixture([]stableStringEntry{{[]byte("solo"), 0}})
	NewStableStringSorter(f, NaturalBytesRefComparator).Sort(0, 1)
	if string(f.data[0].key) != "solo" || f.data[0].id != 0 {
		t.Fatalf("singleton corrupted: %+v", f.data[0])
	}
}

func TestStableStringSorter_NilComparator_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil comparator")
		}
	}()
	NewStableStringSorter(newStableStringFixture(nil), nil)
}

func TestStableStringSorter_NilComparatorFn_Panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil comparator function")
		}
	}()
	NewStableStringSorterFn(newStableStringFixture(nil), nil)
}

func TestStableStringSorter_SatisfiesInterface(t *testing.T) {
	var _ StringSorterImpl = (*stableStringFixture)(nil)
	var _ StableStringSorterImpl = (*stableStringFixture)(nil)
}
