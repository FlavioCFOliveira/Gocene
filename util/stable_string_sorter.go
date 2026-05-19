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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

// StableStringSorterImpl extends [StringSorterImpl] with the Save and
// Restore hooks required to preserve the relative order of equal
// inputs during the stable reorder and the stable merge-sort fallback.
//
// Mirrors the additional protected methods declared on
// org.apache.lucene.util.StableStringSorter:
//
//   - save(int i, int j)    -> Save(i, j)
//   - restore(int i, int j) -> Restore(i, j)
type StableStringSorterImpl interface {
	StringSorterImpl
	// Save writes the value at slot i into the j-th position in the
	// caller's scratch storage.
	Save(i, j int)
	// Restore copies the scratch values back into slots [i, j) of the
	// caller's primary storage.
	Restore(i, j int)
}

// StableStringSorter is the stable variant of [StringSorter]:
// equal-key entries preserve their input order. When the supplied
// comparator is a [BytesRefComparator] the heavy lifting is delegated
// to a [StableMSBRadixSorter]; otherwise the sort falls back to a
// Save/Restore-aware merge sort that compares each candidate slot
// through the configured comparator over materialized BytesRefs.
//
// Mirrors org.apache.lucene.util.StableStringSorter from Lucene 10.4.0.
// Because Go has no virtual factory hooks, the dispatch normally
// performed by overriding StringSorter.radixSorter / fallbackSorter
// lives directly on StableStringSorter.Sort.
type StableStringSorter struct {
	impl   StableStringSorterImpl
	cmp    BytesRefComparator
	natCmp func(o1, o2 *BytesRef) int

	scratch1      *BytesRefBuilder
	scratch2      *BytesRefBuilder
	scratchBytes1 BytesRef
	scratchBytes2 BytesRef
}

// NewStableStringSorter returns a [StableStringSorter] that uses cmp
// for the radix fast path. cmp must not be nil; for the
// comparator-function fallback use [NewStableStringSorterFn].
func NewStableStringSorter(impl StableStringSorterImpl, cmp BytesRefComparator) *StableStringSorter {
	if cmp == nil {
		panic("util.NewStableStringSorter: cmp must not be nil")
	}
	return &StableStringSorter{
		impl:     impl,
		cmp:      cmp,
		scratch1: NewBytesRefBuilder(),
		scratch2: NewBytesRefBuilder(),
	}
}

// NewStableStringSorterFn returns a [StableStringSorter] that runs the
// stable merge-sort fallback using the supplied comparator function.
// Radix acceleration is unavailable on this path.
func NewStableStringSorterFn(impl StableStringSorterImpl, cmp func(o1, o2 *BytesRef) int) *StableStringSorter {
	if cmp == nil {
		panic("util.NewStableStringSorterFn: cmp must not be nil")
	}
	return &StableStringSorter{
		impl:     impl,
		natCmp:   cmp,
		scratch1: NewBytesRefBuilder(),
		scratch2: NewBytesRefBuilder(),
	}
}

// Sort orders the slice [from, to) using a stable algorithm: equal-key
// entries keep their input order. The radix path is preferred when a
// [BytesRefComparator] is configured; otherwise the merge-sort
// fallback is used directly.
func (s *StableStringSorter) Sort(from, to int) {
	if s.cmp != nil {
		NewStableMSBRadixSorter(&stableStringRadixAdapter{owner: s}, s.cmp.ComparedBytesCount()).Sort(from, to)
		return
	}
	(&stableStringMergeSorter{owner: s}).Sort(from, to)
}

// stableStringRadixAdapter bridges StableStringSorter to
// [StableMSBRadixSorter] by exposing ByteAt / Swap / Save / Restore on
// the configured implementation. ByteAt resolves through the
// configured BytesRefComparator's byte-stream view, matching the
// override pattern in Lucene's StableStringSorter.radixSorter.
type stableStringRadixAdapter struct {
	owner *StableStringSorter
}

func (a *stableStringRadixAdapter) ByteAt(i, k int) int {
	a.owner.impl.Get(a.owner.scratch1, &a.owner.scratchBytes1, i)
	return a.owner.cmp.ByteAt(&a.owner.scratchBytes1, k)
}

func (a *stableStringRadixAdapter) Swap(i, j int)    { a.owner.impl.Swap(i, j) }
func (a *stableStringRadixAdapter) Save(i, j int)    { a.owner.impl.Save(i, j) }
func (a *stableStringRadixAdapter) Restore(i, j int) { a.owner.impl.Restore(i, j) }

// stableStringMergeSorter is the Save/Restore-aware merge sort used
// as the StableStringSorter fallback path. It mirrors
// StableMSBRadixSorter.MergeSorter with compare() overridden to invoke
// the configured comparator over materialized BytesRef scratch values
// (rather than the byte-stream view used inside the radix sort).
type stableStringMergeSorter struct {
	owner *StableStringSorter
}

// Sort dispatches between binary-sort for small ranges and recursive
// merge sort over [from, to).
func (m *stableStringMergeSorter) Sort(from, to int) {
	if to < from {
		panic("StableStringSorter: to < from")
	}
	m.mergeSort(from, to)
}

func (m *stableStringMergeSorter) mergeSort(from, to int) {
	if to-from < BinarySortThreshold {
		m.binarySort(from, to)
		return
	}
	mid := int(uint(from+to) >> 1)
	m.mergeSort(from, mid)
	m.mergeSort(mid, to)
	m.merge(from, to, mid)
}

// compare runs the configured comparator over two materialized slots.
func (m *stableStringMergeSorter) compare(i, j int) int {
	m.owner.impl.Get(m.owner.scratch1, &m.owner.scratchBytes1, i)
	m.owner.impl.Get(m.owner.scratch2, &m.owner.scratchBytes2, j)
	if m.owner.cmp != nil {
		return m.owner.cmp.Compare(&m.owner.scratchBytes1, &m.owner.scratchBytes2)
	}
	return m.owner.natCmp(&m.owner.scratchBytes1, &m.owner.scratchBytes2)
}

// binarySort is a stable insertion sort by binary-search probe.
func (m *stableStringMergeSorter) binarySort(from, to int) {
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
			m.owner.impl.Swap(j-1, j)
		}
	}
}

// merge merges two adjacent sorted halves [from, mid) and [mid, to)
// using Save/Restore.
func (m *stableStringMergeSorter) merge(from, to, mid int) {
	if m.compare(mid-1, mid) <= 0 {
		return
	}
	left, right, index := from, mid, from
	for {
		if m.compare(left, right) <= 0 {
			m.owner.impl.Save(left, index)
			left++
			index++
			if left == mid {
				m.bulkSave(right, index, to-right)
				break
			}
		} else {
			m.owner.impl.Save(right, index)
			right++
			index++
			if right == to {
				m.bulkSave(left, index, mid-left)
				break
			}
		}
	}
	m.owner.impl.Restore(from, to)
}

func (m *stableStringMergeSorter) bulkSave(from, tmpFrom, length int) {
	for i := 0; i < length; i++ {
		m.owner.impl.Save(from+i, tmpFrom+i)
	}
}
