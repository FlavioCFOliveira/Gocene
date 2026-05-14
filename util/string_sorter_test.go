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

type stringSorterFixture struct {
	keys [][]byte
}

func (s *stringSorterFixture) Get(builder *BytesRefBuilder, result *BytesRef, i int) {
	k := s.keys[i]
	builder.GrowNoCopy(len(k))
	copy(builder.Bytes(), k)
	result.Bytes = builder.Bytes()
	result.Offset = 0
	result.Length = len(k)
}

func (s *stringSorterFixture) Swap(i, j int) {
	s.keys[i], s.keys[j] = s.keys[j], s.keys[i]
}

func TestStringSorter_NaturalComparator_Radix(t *testing.T) {
	keys := [][]byte{
		[]byte("delta"),
		[]byte("alpha"),
		[]byte("charlie"),
		[]byte("alpha"),
		[]byte("bravo"),
	}
	f := &stringSorterFixture{keys: keys}
	cmp := NaturalBytesRefComparator
	NewStringSorter(f, cmp).Sort(0, len(keys))

	for i := 1; i < len(keys); i++ {
		if bytes.Compare(keys[i-1], keys[i]) > 0 {
			t.Fatalf("not sorted at %d: %q > %q", i, keys[i-1], keys[i])
		}
	}
}

func TestStringSorter_FallbackComparator(t *testing.T) {
	keys := [][]byte{[]byte("c"), []byte("a"), []byte("b")}
	f := &stringSorterFixture{keys: keys}
	NewStringSorterFn(f, func(o1, o2 *BytesRef) int {
		return bytes.Compare(o1.ValidBytes(), o2.ValidBytes())
	}).Sort(0, len(keys))
	for i, w := range []string{"a", "b", "c"} {
		if string(keys[i]) != w {
			t.Fatalf("pos %d: got %q want %q", i, keys[i], w)
		}
	}
}

func TestStringSorter_RandomizedAgainstSortSlice(t *testing.T) {
	rng := rand.New(rand.NewSource(123))
	const n = 500
	keys := make([][]byte, n)
	for i := range keys {
		l := rng.Intn(10) + 1
		buf := make([]byte, l)
		for j := range buf {
			buf[j] = byte('a' + rng.Intn(5))
		}
		keys[i] = buf
	}
	wantKeys := make([][]byte, len(keys))
	for i, k := range keys {
		c := make([]byte, len(k))
		copy(c, k)
		wantKeys[i] = c
	}
	sort.Slice(wantKeys, func(i, j int) bool {
		return bytes.Compare(wantKeys[i], wantKeys[j]) < 0
	})

	f := &stringSorterFixture{keys: keys}
	NewStringSorter(f, NaturalBytesRefComparator).Sort(0, n)

	for i := range wantKeys {
		if !bytes.Equal(wantKeys[i], keys[i]) {
			t.Fatalf("mismatch at %d: got %q want %q", i, keys[i], wantKeys[i])
		}
	}
}

func TestStringSorter_EmptyAndSingleton(t *testing.T) {
	NewStringSorter(&stringSorterFixture{keys: nil}, NaturalBytesRefComparator).Sort(0, 0)
	f := &stringSorterFixture{keys: [][]byte{[]byte("solo")}}
	NewStringSorter(f, NaturalBytesRefComparator).Sort(0, 1)
	if string(f.keys[0]) != "solo" {
		t.Fatalf("singleton corrupted: %q", f.keys[0])
	}
}
