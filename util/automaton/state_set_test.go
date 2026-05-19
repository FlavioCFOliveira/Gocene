// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package automaton

import (
	"reflect"
	"testing"
)

func TestStateSet_IncrAddsAndCounts(t *testing.T) {
	s := NewStateSet(0)
	s.Incr(7)
	s.Incr(3)
	s.Incr(7) // bumps refcount, must not duplicate the array entry.

	if got, want := s.Size(), 2; got != want {
		t.Fatalf("Size after Incr: got %d, want %d", got, want)
	}
	if got, want := s.GetArray(), []int32{3, 7}; !reflect.DeepEqual(got, want) {
		t.Errorf("GetArray: got %v, want %v (must be sorted ascending)", got, want)
	}
}

func TestStateSet_DecrRemovesOnZero(t *testing.T) {
	s := NewStateSet(4)
	s.Incr(1)
	s.Incr(1) // refcount = 2
	s.Incr(2)

	s.Decr(1) // refcount = 1, still present
	if got := s.GetArray(); !reflect.DeepEqual(got, []int32{1, 2}) {
		t.Errorf("after Decr to refcount 1: got %v, want [1 2]", got)
	}
	s.Decr(1) // refcount = 0, removed
	if got := s.GetArray(); !reflect.DeepEqual(got, []int32{2}) {
		t.Errorf("after Decr to refcount 0: got %v, want [2]", got)
	}
	if s.Size() != 1 {
		t.Errorf("Size after removal: got %d, want 1", s.Size())
	}
}

func TestStateSet_DecrAbsentPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on Decr of absent state")
		}
	}()
	s := NewStateSet(0)
	s.Decr(42)
}

func TestStateSet_ResetClears(t *testing.T) {
	s := NewStateSet(0)
	s.Incr(10)
	s.Incr(20)
	s.Reset()
	if s.Size() != 0 {
		t.Fatalf("Size after Reset: got %d, want 0", s.Size())
	}
	if got := s.GetArray(); len(got) != 0 {
		t.Errorf("GetArray after Reset: got %v, want empty", got)
	}
	// Reset on empty must be a no-op.
	s.Reset()
	if s.Size() != 0 {
		t.Errorf("Size after second Reset: got %d, want 0", s.Size())
	}
}

func TestStateSet_HashCacheInvalidatedOnKeyChange(t *testing.T) {
	s := NewStateSet(0)
	s.Incr(5)
	h1 := s.LongHashCode()
	s.Incr(5) // refcount-only change: keys unchanged, hash stable.
	if got := s.LongHashCode(); got != h1 {
		t.Errorf("hash changed on refcount-only Incr: %d -> %d", h1, got)
	}
	s.Incr(8) // new key: hash must update.
	if got := s.LongHashCode(); got == h1 {
		t.Errorf("hash unchanged after new key added: still %d", got)
	}
}

func TestStateSet_EqualsAcrossDifferentInsertionOrders(t *testing.T) {
	a := NewStateSet(0)
	a.Incr(2)
	a.Incr(9)
	a.Incr(4)

	b := NewStateSet(0)
	b.Incr(9)
	b.Incr(4)
	b.Incr(2)

	if !IntSetEquals(a, b) {
		t.Fatalf("IntSetEquals on permutations: got false, want true")
	}
	if a.LongHashCode() != b.LongHashCode() {
		t.Errorf("LongHashCode mismatch on permutations: %d vs %d",
			a.LongHashCode(), b.LongHashCode())
	}
}

func TestStateSet_NotEqualsWhenContentsDiffer(t *testing.T) {
	a := NewStateSet(0)
	a.Incr(1)
	a.Incr(2)

	b := NewStateSet(0)
	b.Incr(1)
	b.Incr(3)

	if IntSetEquals(a, b) {
		t.Fatal("IntSetEquals on distinct sets: got true, want false")
	}
}

func TestStateSet_FreezeIsSnapshot(t *testing.T) {
	s := NewStateSet(0)
	s.Incr(1)
	s.Incr(2)
	s.Incr(3)
	frozen := s.Freeze(99)

	// Snapshot is decoupled from subsequent mutations.
	s.Incr(4)
	if got, want := frozen.Size(), 3; got != want {
		t.Errorf("frozen Size after upstream mutation: got %d, want %d", got, want)
	}
	if got, want := frozen.GetArray(), []int32{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("frozen GetArray: got %v, want %v", got, want)
	}
	if got := FrozenState(frozen); got != 99 {
		t.Errorf("FrozenState: got %d, want 99", got)
	}
	// IntSetEquals must work between StateSet and a frozen snapshot of itself.
	snap2 := s.Freeze(0)
	if !IntSetEquals(snap2, s) {
		t.Error("IntSetEquals(frozen, source): got false after refreezing")
	}
}

func TestStateSet_HashFoldingSeedIsSize(t *testing.T) {
	// Reproduces Lucene's seed: longHashCode() == size() + sum(mix32(key))
	// signed-widened to int64. An empty set must hash to zero.
	s := NewStateSet(0)
	if got := s.LongHashCode(); got != 0 {
		t.Fatalf("empty LongHashCode: got %d, want 0", got)
	}
	// Single-element set: hash == 1 + sign-extended mix32(key).
	s.Incr(0)
	want := int64(1) + int64(int32(mix32(0)))
	if got := s.LongHashCode(); got != want {
		t.Errorf("LongHashCode for {0}: got %d, want %d", got, want)
	}
}
