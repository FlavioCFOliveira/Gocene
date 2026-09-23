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

// StableStringSorterImpl declares the abstract methods of
// org.apache.lucene.util.StableStringSorter: those of [StringSorterImpl] plus
// save and restore.
type StableStringSorterImpl interface {
	StringSorterImpl
	// Save saves the i-th value into the j-th position in temporary storage.
	Save(i, j int)
	// Restore restores the values between the i-th and the j-th (excluded)
	// positions of temporary storage into original storage.
	Restore(i, j int)
}

// StableStringSorter is a [StringSorter] that keeps equal values in their
// input order: its radix sorter is a [StableMSBRadixSorter] and its fallback
// sorter is the StableMSBRadixSorter merge sort.
//
// Port of the package-private abstract class
// org.apache.lucene.util.StableStringSorter (Apache Lucene 10.5.0,
// lucene/core/src/java/org/apache/lucene/util/StableStringSorter.java), which
// extends StringSorter and overrides radixSorter and fallbackSorter. Go
// renders the inheritance as embedding and the overrides as replacements of
// the embedded sorter's factory fields.
type StableStringSorter struct {
	*StringSorter
	stableImpl StableStringSorterImpl
}

// NewStableStringSorter returns a StableStringSorter whose comparator is the
// BytesRefComparator cmp, so Sort uses the stable radix sorter.
func NewStableStringSorter(impl StableStringSorterImpl, cmp BytesRefComparator) *StableStringSorter {
	return newStableStringSorter(NewStringSorter(impl, cmp), impl)
}

// NewStableStringSorterFn returns a StableStringSorter whose comparator is not
// a BytesRefComparator, so Sort uses the stable merge-sort fallback.
func NewStableStringSorterFn(impl StableStringSorterImpl, cmp func(o1, o2 *BytesRef) int) *StableStringSorter {
	return newStableStringSorter(NewStringSorterFn(impl, cmp), impl)
}

func newStableStringSorter(base *StringSorter, impl StableStringSorterImpl) *StableStringSorter {
	s := &StableStringSorter{StringSorter: base, stableImpl: impl}
	base.radixSorterFn = s.stableRadixSorter
	base.fallbackSorterFn = s.stableFallbackSorter
	return s
}

// Save delegates to the implementation. Mirrors StableStringSorter.save.
func (s *StableStringSorter) Save(i, j int) {
	s.stableImpl.Save(i, j)
}

// Restore delegates to the implementation. Mirrors StableStringSorter.restore.
func (s *StableStringSorter) Restore(i, j int) {
	s.stableImpl.Restore(i, j)
}

// stableRadixSorter mirrors the StableStringSorter.radixSorter override: an
// anonymous StableMSBRadixSorter whose save, restore, swap and byteAt reach
// this sorter and whose fallback sorter is fallbackSorter comparing from the
// k-th byte on.
func (s *StableStringSorter) stableRadixSorter(cmp BytesRefComparator) Sorter {
	adapter := &stableStringRadixAdapter{owner: s, cmp: cmp}
	radix := NewStableMSBRadixSorter(adapter, cmp.ComparedBytesCount())
	radix.MSBRadixSorter.fallbackSorterFn = func(_ RadixSortable, from, to, k int) {
		s.FallbackSorter(func(o1, o2 *BytesRef) int { return cmp.CompareK(o1, o2, k) }).Sort(from, to)
	}
	adapter.radix = radix
	return adapter
}

// stableFallbackSorter mirrors the StableStringSorter.fallbackSorter
// override: a StableMSBRadixSorter.MergeSorter whose compare runs cmp over
// the materialised values.
func (s *StableStringSorter) stableFallbackSorter(cmp func(o1, o2 *BytesRef) int) Sorter {
	return &stableStringMergeSorter{owner: s, cmp: cmp}
}

// stableStringRadixAdapter is the anonymous StableMSBRadixSorter subclass of
// StableStringSorter.radixSorter.
type stableStringRadixAdapter struct {
	owner *StableStringSorter
	cmp   BytesRefComparator
	radix *StableMSBRadixSorter
}

func (a *stableStringRadixAdapter) Sort(from, to int) {
	a.radix.Sort(a, from, to)
}

func (a *stableStringRadixAdapter) ByteAt(i, k int) int {
	o := a.owner.StringSorter
	o.impl.Get(o.scratch1, o.scratchBytes1, i)
	return a.cmp.ByteAt(o.scratchBytes1, k)
}

// Compare is the final MSBRadixSorter.compare, unsupported for a radix sort.
func (a *stableStringRadixAdapter) Compare(i, j int) int {
	panic("unused: not a comparison-based sort")
}

func (a *stableStringRadixAdapter) Swap(i, j int)    { a.owner.impl.Swap(i, j) }
func (a *stableStringRadixAdapter) Save(i, j int)    { a.owner.Save(i, j) }
func (a *stableStringRadixAdapter) Restore(i, j int) { a.owner.Restore(i, j) }

// stableStringMergeSorter is the anonymous StableMSBRadixSorter.MergeSorter
// subclass of StableStringSorter.fallbackSorter: a stable merge sort through
// save and restore whose compare runs the comparator over the materialised
// values.
type stableStringMergeSorter struct {
	owner *StableStringSorter
	cmp   func(o1, o2 *BytesRef) int
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

// compare runs the comparator over two materialised slots.
func (m *stableStringMergeSorter) compare(i, j int) int {
	o := m.owner.StringSorter
	o.impl.Get(o.scratch1, o.scratchBytes1, i)
	o.impl.Get(o.scratch2, o.scratchBytes2, j)
	return m.cmp(o.scratchBytes1, o.scratchBytes2)
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
			m.owner.Save(left, index)
			left++
			index++
			if left == mid {
				m.bulkSave(right, index, to-right)
				break
			}
		} else {
			m.owner.Save(right, index)
			right++
			index++
			if right == to {
				m.bulkSave(left, index, mid-left)
				break
			}
		}
	}
	m.owner.Restore(from, to)
}

func (m *stableStringMergeSorter) bulkSave(from, tmpFrom, length int) {
	for i := 0; i < length; i++ {
		m.owner.Save(from+i, tmpFrom+i)
	}
}
