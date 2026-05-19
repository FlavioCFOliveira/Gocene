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

import "sort"

// StateSet mirrors org.apache.lucene.util.automaton.StateSet from Lucene
// 10.4.0. It is a thin wrapper around a state -> reference-count map; a
// state is removed from the set when its count drops to zero. The set's
// observable contents (returned by GetArray) are sorted in ascending order so
// that equality checks via IntSetEquals are well defined.
//
// Lucene's type is package-private; Gocene exports it for parity with
// neighbouring ports such as StatePair, since the determinization machinery
// scheduled for later sprints needs to share it across files within the
// package.
type StateSet struct {
	inner       map[int32]int32
	hashCode    int64
	arrayCache  []int32
	hashUpdated bool
	arrayUpdtd  bool
}

// NewStateSet returns an empty StateSet with the given initial capacity hint.
// The hint mirrors the IntIntHashMap(capacity) constructor used by Lucene; in
// Gocene it sizes the backing Go map.
func NewStateSet(capacity int) *StateSet {
	if capacity < 0 {
		capacity = 0
	}
	return &StateSet{
		inner:       make(map[int32]int32, capacity),
		arrayCache:  nil,
		hashUpdated: true,
		arrayUpdtd:  true,
	}
}

// Incr adds the state into this set; if the state is already present its
// reference count is increased by one.
func (s *StateSet) Incr(state int32) {
	if _, ok := s.inner[state]; ok {
		s.inner[state]++
		return
	}
	s.inner[state] = 1
	s.keyChanged()
}

// Decr decreases the reference count of the state. When the count reaches
// zero the state is removed from the set. The state must be present; callers
// violate the contract by decrementing an absent state.
func (s *StateSet) Decr(state int32) {
	c, ok := s.inner[state]
	if !ok {
		panic("automaton: StateSet.Decr on absent state")
	}
	c--
	if c == 0 {
		delete(s.inner, state)
		s.keyChanged()
		return
	}
	s.inner[state] = c
}

// Reset clears the set without releasing the underlying map storage. It
// mirrors Lucene's StateSet.reset(), which delegates to IntIntHashMap.clear.
func (s *StateSet) Reset() {
	if len(s.inner) == 0 {
		return
	}
	for k := range s.inner {
		delete(s.inner, k)
	}
	s.keyChanged()
}

// Freeze returns an immutable snapshot of this set associated with the given
// state identifier. The snapshot retains existence only; reference counts are
// discarded. The returned FrozenIntSet shares no storage with the receiver.
//
// Mirrors org.apache.lucene.util.automaton.StateSet.freeze(int) in Lucene
// 10.4.0, which constructs a new FrozenIntSet with a copy of the backing
// array, the cached longHashCode, and the supplied state identifier.
func (s *StateSet) Freeze(state int32) *FrozenIntSet {
	arr := append([]int32(nil), s.GetArray()...)
	return NewFrozenIntSet(arr, s.LongHashCode(), state)
}

// FrozenState returns the associated state of a snapshot produced by
// StateSet.Freeze. It returns -1 when v is not a *FrozenIntSet.
//
// Retained for back-compatibility with the pre-FrozenIntSet adapter; new
// code should type-assert to *FrozenIntSet directly and read the State field.
func FrozenState(v IntSet) int32 {
	if f, ok := v.(*FrozenIntSet); ok {
		return f.State
	}
	return -1
}

// GetArray returns the sorted array view of this set's contents. The slice
// is owned by the receiver and is invalidated by the next mutation; callers
// that need to retain it across mutations must copy it.
func (s *StateSet) GetArray() []int32 {
	if s.arrayUpdtd {
		return s.arrayCache
	}
	s.arrayCache = make([]int32, 0, len(s.inner))
	for k := range s.inner {
		s.arrayCache = append(s.arrayCache, k)
	}
	sort.Slice(s.arrayCache, func(i, j int) bool {
		return s.arrayCache[i] < s.arrayCache[j]
	})
	s.arrayUpdtd = true
	return s.arrayCache
}

// Size returns the number of distinct states currently in the set.
func (s *StateSet) Size() int { return len(s.inner) }

// LongHashCode returns a 64-bit hash of the set's contents. It mirrors
// Lucene's StateSet.longHashCode(): seed is the set size and each key is
// folded in via the HPPC bit mixer (MH3 32-bit finalization step,
// widened back to int64 using Java's int->long sign-extension semantics).
func (s *StateSet) LongHashCode() int64 {
	if s.hashUpdated {
		return s.hashCode
	}
	h := int64(len(s.inner))
	for k := range s.inner {
		h += int64(int32(mix32(uint32(k))))
	}
	s.hashCode = h
	s.hashUpdated = true
	return h
}

// keyChanged invalidates the cached array and hash, mirroring Lucene's
// private StateSet.keyChanged().
func (s *StateSet) keyChanged() {
	s.hashUpdated = false
	s.arrayUpdtd = false
}

// mix32 is MurmurHash3's plain 32-bit finalization step, forked from
// org.apache.lucene.internal.hppc.BitMixer.mix32. It is kept local to this
// file because the package's other consumers reach for the 64-bit mixer.
func mix32(k uint32) uint32 {
	k = (k ^ (k >> 16)) * 0x85ebca6b
	k = (k ^ (k >> 13)) * 0xc2b2ae35
	return k ^ (k >> 16)
}
