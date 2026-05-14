// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"math/rand"
	"sort"
	"testing"
)

func TestSentinelIntSet_BasicPutAndExists(t *testing.T) {
	s := NewSentinelIntSet(8, -1)
	if s.Size() != 0 {
		t.Fatalf("Size()=%d want 0", s.Size())
	}
	for _, k := range []int{1, 2, 3, 4, 5} {
		s.Put(k)
	}
	if s.Size() != 5 {
		t.Fatalf("Size()=%d want 5", s.Size())
	}
	for _, k := range []int{1, 2, 3, 4, 5} {
		if !s.Exists(k) {
			t.Fatalf("Exists(%d) false; want true", k)
		}
	}
	if s.Exists(99) {
		t.Fatalf("Exists(99) true; want false")
	}
}

func TestSentinelIntSet_DuplicatePutIsIdempotent(t *testing.T) {
	s := NewSentinelIntSet(8, -1)
	a := s.Put(42)
	b := s.Put(42)
	if a != b {
		t.Fatalf("duplicate Put returned different slots: %d vs %d", a, b)
	}
	if s.Size() != 1 {
		t.Fatalf("Size()=%d want 1", s.Size())
	}
}

func TestSentinelIntSet_ClearResetsState(t *testing.T) {
	s := NewSentinelIntSet(8, -1)
	for i := 0; i < 5; i++ {
		s.Put(i)
	}
	s.Clear()
	if s.Size() != 0 {
		t.Fatalf("Size after Clear=%d want 0", s.Size())
	}
	for _, v := range s.Keys {
		if v != -1 {
			t.Fatalf("slot %d not reset (=%d) after Clear", v, v)
		}
	}
	if s.Exists(0) {
		t.Fatalf("Exists(0) after Clear=true")
	}
}

func TestSentinelIntSet_RehashKeepsAllKeys(t *testing.T) {
	s := NewSentinelIntSet(4, -1)
	initialCap := len(s.Keys)
	want := make([]int, 0, 64)
	for i := 0; i < 64; i++ {
		k := (i * 2654435761) & 0x7fffffff // pseudo-random positive
		if k == -1 {
			continue
		}
		if !s.Exists(k) {
			want = append(want, k)
		}
		s.Put(k)
	}
	if len(s.Keys) <= initialCap {
		t.Fatalf("expected table growth: initial=%d final=%d", initialCap, len(s.Keys))
	}
	if s.Size() != len(want) {
		t.Fatalf("Size()=%d want %d", s.Size(), len(want))
	}
	for _, k := range want {
		if !s.Exists(k) {
			t.Fatalf("Exists(%d) false after rehash", k)
		}
	}
}

func TestSentinelIntSet_FindReturnsInsertionSlot(t *testing.T) {
	s := NewSentinelIntSet(8, -1)
	s.Put(7)
	slot := s.Find(99)
	if slot >= 0 {
		t.Fatalf("Find(99)=%d expected negative insertion slot", slot)
	}
	insertSlot := -slot - 1
	if s.Keys[insertSlot] != s.EmptyVal {
		t.Fatalf("insertion slot %d not empty (=%d)", insertSlot, s.Keys[insertSlot])
	}
}

func TestSentinelIntSet_EmptyValZeroIsHonoured(t *testing.T) {
	s := NewSentinelIntSet(8, 0)
	for _, k := range []int{1, 2, 3} {
		s.Put(k)
	}
	if s.Size() != 3 {
		t.Fatalf("Size()=%d want 3", s.Size())
	}
	for _, k := range []int{1, 2, 3} {
		if !s.Exists(k) {
			t.Fatalf("Exists(%d) false", k)
		}
	}
}

func TestSentinelIntSet_RandomizedAgainstNativeMap(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	s := NewSentinelIntSet(16, -1)
	ref := make(map[int]struct{})
	for i := 0; i < 5000; i++ {
		k := rng.Intn(1 << 20)
		if k == -1 {
			continue
		}
		s.Put(k)
		ref[k] = struct{}{}
	}
	if s.Size() != len(ref) {
		t.Fatalf("Size()=%d want %d", s.Size(), len(ref))
	}
	got := make([]int, 0, s.Size())
	for _, v := range s.Keys {
		if v == s.EmptyVal {
			continue
		}
		got = append(got, v)
	}
	if len(got) != len(ref) {
		t.Fatalf("iteration count=%d want %d", len(got), len(ref))
	}
	sort.Ints(got)
	want := make([]int, 0, len(ref))
	for k := range ref {
		want = append(want, k)
	}
	sort.Ints(want)
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("mismatch at %d: got %d want %d", i, got[i], want[i])
		}
	}
}

func TestSentinelIntSet_RamBytesUsedNonZero(t *testing.T) {
	s := NewSentinelIntSet(8, -1)
	if s.RamBytesUsed() <= 0 {
		t.Fatalf("RamBytesUsed()=%d want > 0", s.RamBytesUsed())
	}
}
