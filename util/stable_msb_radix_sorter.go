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

// StableMSBRadixSorterImpl extends the base [MSBRadixSorterImpl] with
// Save/Restore hooks that allow temporary storage of values during
// the stable reorder + fallback merge sort. Save(i, j) writes the
// value at slot i to scratch slot j; Restore(i, j) copies the scratch
// values back into slots [i, j).
type StableMSBRadixSorterImpl interface {
	MSBRadixSorterImpl
	// Save writes the value at slot i into the j-th position in the
	// caller's scratch storage.
	Save(i, j int)
	// Restore copies the scratch values back into slots [i, j) of the
	// caller's primary storage.
	Restore(i, j int)
}

// StableMSBRadixSorter is the stable variant of [MSBRadixSorter]:
// equal-key elements preserve their input order. The algorithm
// matches Lucene 10.4.0 byte-for-byte and uses a merge-sort fallback
// (rather than the unstable IntroSorter used by MSBRadixSorter).
//
// Callers supply scratch storage via the [StableMSBRadixSorterImpl]
// Save/Restore hooks.
type StableMSBRadixSorter struct {
	*MSBRadixSorter

	stableImpl        StableMSBRadixSorterImpl
	fixedStartOffsets [msbHistogramSize]int32
}

// NewStableMSBRadixSorter returns a stable MSB radix sorter that
// preserves the input order of equal keys. impl must implement the
// extended StableMSBRadixSorterImpl with Save/Restore hooks.
func NewStableMSBRadixSorter(impl StableMSBRadixSorterImpl, maxLength int) *StableMSBRadixSorter {
	base := NewMSBRadixSorter(impl, maxLength)
	s := &StableMSBRadixSorter{
		MSBRadixSorter: base,
		stableImpl:     impl,
	}
	base.reorderFn = s.stableReorder
	base.fallbackSorterFn = s.stableFallback
	return s
}

// stableReorder shuffles the elements in [from, to) into their
// bucket positions, preserving the relative order of equal-bucket
// entries. The algorithm copies values to scratch storage via Save
// and then restores them in-place via Restore, mirroring the Java
// StableMSBRadixSorter.reorder().
func (s *StableMSBRadixSorter) stableReorder(from, to int, startOffsets, endOffsets []int32, k int) {
	copy(s.fixedStartOffsets[:], startOffsets)
	for i := 0; i < msbHistogramSize; i++ {
		limit := endOffsets[i]
		for h1 := s.fixedStartOffsets[i]; h1 < limit; h1++ {
			b := s.MSBRadixSorter.getBucket(from+int(h1), k)
			h2 := startOffsets[b]
			startOffsets[b]++
			s.stableImpl.Save(from+int(h1), from+int(h2))
		}
	}
	s.stableImpl.Restore(from, to)
}

// stableFallback runs a Save/Restore-aware merge sort over [from, to)
// assuming the first k bytes of every entry are equal. The result is
// stable for equal keys; the merge step relies on Save/Restore for
// the temporary buffer.
func (s *StableMSBRadixSorter) stableFallback(from, to, k int) {
	(&stableMergeSorter{owner: s, k: k}).Sort(from, to)
}

// stableMergeSorter is the Save/Restore-aware merge sort used as the
// short-bucket fallback inside StableMSBRadixSorter. It does not need
// the Sorter heavy helpers; the merge loop calls Save / Restore on
// the surrounding StableMSBRadixSorter directly.
type stableMergeSorter struct {
	owner *StableMSBRadixSorter
	k     int
}

// Sort dispatches between binary-sort for small ranges and recursive
// merge sort.
func (m *stableMergeSorter) Sort(from, to int) {
	if to < from {
		panic("StableMSBRadixSorter: to < from")
	}
	m.mergeSort(from, to)
}

func (m *stableMergeSorter) mergeSort(from, to int) {
	if to-from < BinarySortThreshold {
		m.binarySort(from, to)
		return
	}
	mid := int(uint(from+to) >> 1)
	m.mergeSort(from, mid)
	m.mergeSort(mid, to)
	m.merge(from, to, mid)
}

// compare two slots using the MSBRadixSorter's byte-stream view.
func (m *stableMergeSorter) compare(i, j int) int {
	for o := m.k; o < m.owner.maxLength; o++ {
		b1 := m.owner.stableImpl.ByteAt(i, o)
		b2 := m.owner.stableImpl.ByteAt(j, o)
		if b1 != b2 {
			return b1 - b2
		}
		if b1 == -1 {
			break
		}
	}
	return 0
}

// binarySort is a stable insertion sort by binary-search probe.
func (m *stableMergeSorter) binarySort(from, to int) {
	for i := from + 1; i < to; i++ {
		l := from
		h := i - 1
		for l <= h {
			mid := int(uint(l+h) >> 1)
			if m.compare(i, mid) < 0 {
				h = mid - 1
			} else {
				l = mid + 1
			}
		}
		for j := i; j > l; j-- {
			m.owner.stableImpl.Swap(j-1, j)
		}
	}
}

// merge merges two adjacent sorted halves [from, mid) and [mid, to)
// using Save/Restore.
func (m *stableMergeSorter) merge(from, to, mid int) {
	if m.compare(mid-1, mid) <= 0 {
		return
	}
	left, right, index := from, mid, from
	for {
		if m.compare(left, right) <= 0 {
			m.owner.stableImpl.Save(left, index)
			left++
			index++
			if left == mid {
				m.bulkSave(right, index, to-right)
				break
			}
		} else {
			m.owner.stableImpl.Save(right, index)
			right++
			index++
			if right == to {
				m.bulkSave(left, index, mid-left)
				break
			}
		}
	}
	m.owner.stableImpl.Restore(from, to)
}

func (m *stableMergeSorter) bulkSave(from, tmpFrom, length int) {
	for i := 0; i < length; i++ {
		m.owner.stableImpl.Save(from+i, tmpFrom+i)
	}
}
