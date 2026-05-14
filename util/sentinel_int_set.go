// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package util

import "unsafe"

// SentinelIntSet is an open-addressing int hash set that reserves one
// value (EmptyVal) as the internal "empty" sentinel. The space
// overhead is low: a single power-of-two-sized int slice. The table
// is rehashed when load factor reaches 75%.
//
// To iterate, walk Keys and skip elements equal to EmptyVal.
//
// Port of org.apache.lucene.util.SentinelIntSet. Internal fields are
// exported to mirror the Java public-field idiom used by callers such
// as the FST and BKD code paths. Method names follow Go conventions.
//
// Not safe for concurrent use.
type SentinelIntSet struct {
	// Keys is a power-of-two oversized array holding the integers in
	// the set, with EmptyVal marking unused slots.
	Keys []int
	// Count is the number of integers currently in the set.
	Count int
	// EmptyVal is the value reserved to represent an empty slot.
	EmptyVal int
	// RehashCount is the Count threshold at which the table will be
	// rehashed to double its capacity.
	RehashCount int
}

// NewSentinelIntSet returns a set able to hold at least size elements
// without rehashing. emptyVal is the value reserved as the internal
// empty sentinel; callers must never insert it.
func NewSentinelIntSet(size, emptyVal int) *SentinelIntSet {
	tsize := NextHighestPowerOfTwo(size)
	if tsize < 1 {
		tsize = 1
	}
	rehashCount := tsize - (tsize >> 2)
	if size >= rehashCount {
		tsize <<= 1
		rehashCount = tsize - (tsize >> 2)
	}
	s := &SentinelIntSet{
		Keys:        make([]int, tsize),
		EmptyVal:    emptyVal,
		RehashCount: rehashCount,
	}
	if emptyVal != 0 {
		s.Clear()
	}
	return s
}

// Clear empties the set, restoring every slot to EmptyVal.
func (s *SentinelIntSet) Clear() {
	for i := range s.Keys {
		s.Keys[i] = s.EmptyVal
	}
	s.Count = 0
}

// Hash returns the hash value for key. The default implementation is
// the identity function, matching Lucene's default; callers with
// poor-quality keys can subclass via composition and override this
// behaviour at the call site.
func (s *SentinelIntSet) Hash(key int) int { return key }

// Size returns the number of integers in this set.
func (s *SentinelIntSet) Size() int { return s.Count }

// GetSlot computes the slot index that key currently occupies or that
// it would occupy if inserted. key must not equal EmptyVal.
func (s *SentinelIntSet) GetSlot(key int) int {
	h := s.Hash(key)
	mask := len(s.Keys) - 1
	slot := h & mask
	if s.Keys[slot] == key || s.Keys[slot] == s.EmptyVal {
		return slot
	}
	increment := (h >> 7) | 1
	for {
		slot = (slot + increment) & mask
		if s.Keys[slot] == key || s.Keys[slot] == s.EmptyVal {
			return slot
		}
	}
}

// Find returns the slot index of key when present, or -slot-1
// indicating the insertion slot when absent. key must not equal
// EmptyVal.
func (s *SentinelIntSet) Find(key int) int {
	h := s.Hash(key)
	mask := len(s.Keys) - 1
	slot := h & mask
	if s.Keys[slot] == key {
		return slot
	}
	if s.Keys[slot] == s.EmptyVal {
		return -slot - 1
	}
	increment := (h >> 7) | 1
	for {
		slot = (slot + increment) & mask
		if s.Keys[slot] == key {
			return slot
		}
		if s.Keys[slot] == s.EmptyVal {
			return -slot - 1
		}
	}
}

// Exists reports whether key is currently in the set.
func (s *SentinelIntSet) Exists(key int) bool { return s.Find(key) >= 0 }

// Put inserts key and returns the slot index it occupies after the
// call. The table is rehashed if Put would push the load factor past
// 75%.
func (s *SentinelIntSet) Put(key int) int {
	slot := s.Find(key)
	if slot < 0 {
		s.Count++
		if s.Count >= s.RehashCount {
			s.Rehash()
			slot = s.GetSlot(key)
		} else {
			slot = -slot - 1
		}
		s.Keys[slot] = key
	}
	return slot
}

// Rehash doubles the underlying table and re-inserts every live key.
// Public for parity with the Java surface; the typical caller does
// not invoke this directly.
func (s *SentinelIntSet) Rehash() {
	newSize := len(s.Keys) << 1
	oldKeys := s.Keys
	s.Keys = make([]int, newSize)
	if s.EmptyVal != 0 {
		for i := range s.Keys {
			s.Keys[i] = s.EmptyVal
		}
	}
	for _, key := range oldKeys {
		if key == s.EmptyVal {
			continue
		}
		s.Keys[s.GetSlot(key)] = key
	}
	s.RehashCount = newSize - (newSize >> 2)
}

// RamBytesUsed returns the approximate RAM footprint in bytes.
func (s *SentinelIntSet) RamBytesUsed() int64 {
	headers := int64(3)*int64(unsafe.Sizeof(int(0))) + NumBytesObjectRef
	return AlignObjectSize(headers) + SizeOfIntSlice(s.Keys)
}
