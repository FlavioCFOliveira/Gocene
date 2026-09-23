// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "bytes"

// StringSorterImpl declares the abstract methods of
// org.apache.lucene.util.StringSorter that a concrete sorter supplies: get
// resolves the value at slot i into result (builder may serve as backing
// storage) and swap exchanges the values at slots i and j.
type StringSorterImpl interface {
	Get(builder *BytesRefBuilder, result *BytesRef, i int)
	Swap(i, j int)
}

// StringSorter is a BytesRef sorter that uses an efficient radix sorter when
// its comparator is a [BytesRefComparator] and falls back to an introsort
// driven by the comparator otherwise.
//
// Port of org.apache.lucene.util.StringSorter (Apache Lucene 10.5.0,
// lucene/core/src/java/org/apache/lucene/util/StringSorter.java). The Java
// constructor takes a Comparator<BytesRef> and tests it with instanceof
// BytesRefComparator; Go renders the two cases as [NewStringSorter] (a
// BytesRefComparator) and [NewStringSorterFn] (any other comparator). The
// overridable factories radixSorter and fallbackSorter are rendered as
// function fields that [StableStringSorter] replaces.
//
// @lucene.internal
type StringSorter struct {
	impl StringSorterImpl
	// cmp is the Comparator<BytesRef>; bytesRefCmp is the same comparator
	// when it is a BytesRefComparator, nil otherwise.
	cmp         func(o1, o2 *BytesRef) int
	bytesRefCmp BytesRefComparator

	scratch1      *BytesRefBuilder
	scratch2      *BytesRefBuilder
	pivotBuilder  *BytesRefBuilder
	scratchBytes1 *BytesRef
	scratchBytes2 *BytesRef
	pivot         *BytesRef

	radixSorterFn    func(cmp BytesRefComparator) Sorter
	fallbackSorterFn func(cmp func(o1, o2 *BytesRef) int) Sorter
}

// NewStringSorter returns a StringSorter whose comparator is the
// BytesRefComparator cmp, so Sort uses the radix sorter.
func NewStringSorter(impl StringSorterImpl, cmp BytesRefComparator) *StringSorter {
	s := newStringSorter(impl, cmp.Compare)
	s.bytesRefCmp = cmp
	return s
}

// NewStringSorterFn returns a StringSorter whose comparator is not a
// BytesRefComparator, so Sort uses the fallback introsort.
func NewStringSorterFn(impl StringSorterImpl, cmp func(o1, o2 *BytesRef) int) *StringSorter {
	return newStringSorter(impl, cmp)
}

func newStringSorter(impl StringSorterImpl, cmp func(o1, o2 *BytesRef) int) *StringSorter {
	s := &StringSorter{
		impl:          impl,
		cmp:           cmp,
		scratch1:      NewBytesRefBuilder(),
		scratch2:      NewBytesRefBuilder(),
		pivotBuilder:  NewBytesRefBuilder(),
		scratchBytes1: NewBytesRefEmpty(),
		scratchBytes2: NewBytesRefEmpty(),
		pivot:         NewBytesRefEmpty(),
	}
	s.radixSorterFn = s.defaultRadixSorter
	s.fallbackSorterFn = s.defaultFallbackSorter
	return s
}

// Compare compares the values at slots i and j with the comparator.
// Mirrors StringSorter.compare(int, int).
func (s *StringSorter) Compare(i, j int) int {
	s.impl.Get(s.scratch1, s.scratchBytes1, i)
	s.impl.Get(s.scratch2, s.scratchBytes2, j)
	return s.cmp(s.scratchBytes1, s.scratchBytes2)
}

// Swap exchanges the values at slots i and j through the implementation.
func (s *StringSorter) Swap(i, j int) {
	s.impl.Swap(i, j)
}

// Sort sorts the slots between from inclusive and to exclusive, using the
// radix sorter when the comparator is a BytesRefComparator and the fallback
// sorter otherwise. Mirrors StringSorter.sort(int, int).
func (s *StringSorter) Sort(from, to int) {
	if s.bytesRefCmp != nil {
		s.RadixSorter(s.bytesRefCmp).Sort(from, to)
	} else {
		s.FallbackSorter(s.cmp).Sort(from, to)
	}
}

// RadixSorter returns a radix sorter for BytesRef. Mirrors the protected
// StringSorter.radixSorter(BytesRefComparator).
func (s *StringSorter) RadixSorter(cmp BytesRefComparator) Sorter {
	return s.radixSorterFn(cmp)
}

// FallbackSorter returns the comparator-driven sorter. Mirrors the protected
// StringSorter.fallbackSorter(Comparator<BytesRef>).
func (s *StringSorter) FallbackSorter(cmp func(o1, o2 *BytesRef) int) Sorter {
	return s.fallbackSorterFn(cmp)
}

func (s *StringSorter) defaultRadixSorter(cmp BytesRefComparator) Sorter {
	return NewMSBStringRadixSorter(s, cmp)
}

func (s *StringSorter) defaultFallbackSorter(cmp func(o1, o2 *BytesRef) int) Sorter {
	return NewIntroSorter(&stringSorterFallback{owner: s, cmp: cmp})
}

// stringSorterFallback is the anonymous IntroSorter subclass returned by
// StringSorter.fallbackSorter.
type stringSorterFallback struct {
	owner *StringSorter
	cmp   func(o1, o2 *BytesRef) int
}

func (f *stringSorterFallback) Swap(i, j int) {
	f.owner.impl.Swap(i, j)
}

func (f *stringSorterFallback) Compare(i, j int) int {
	o := f.owner
	o.impl.Get(o.scratch1, o.scratchBytes1, i)
	o.impl.Get(o.scratch2, o.scratchBytes2, j)
	return f.cmp(o.scratchBytes1, o.scratchBytes2)
}

func (f *stringSorterFallback) SetPivot(i int) {
	o := f.owner
	o.impl.Get(o.pivotBuilder, o.pivot, i)
}

func (f *stringSorterFallback) ComparePivot(j int) int {
	o := f.owner
	o.impl.Get(o.scratch1, o.scratchBytes1, j)
	return f.cmp(o.pivot, o.scratchBytes1)
}

// MSBStringRadixSorter is the radix sorter for BytesRef returned by
// StringSorter.radixSorter. Mirrors the protected inner class
// StringSorter.MSBStringRadixSorter, which extends MSBRadixSorter.
type MSBStringRadixSorter struct {
	*MSBRadixSorter
	owner *StringSorter
	cmp   BytesRefComparator
}

// NewMSBStringRadixSorter returns the radix sorter of owner over cmp; its
// maximum length is cmp.ComparedBytesCount().
func NewMSBStringRadixSorter(owner *StringSorter, cmp BytesRefComparator) *MSBStringRadixSorter {
	r := &MSBStringRadixSorter{
		MSBRadixSorter: NewMSBRadixSorter(cmp.ComparedBytesCount()),
		owner:          owner,
		cmp:            cmp,
	}
	r.MSBRadixSorter.fallbackSorterFn = func(_ RadixSortable, from, to, k int) {
		r.GetFallbackSorter(k).Sort(from, to)
	}
	return r
}

// Sort sorts the slots between from inclusive and to exclusive.
func (r *MSBStringRadixSorter) Sort(from, to int) {
	r.MSBRadixSorter.Sort(r, from, to)
}

// Swap delegates to the enclosing StringSorter.
func (r *MSBStringRadixSorter) Swap(i, j int) {
	r.owner.impl.Swap(i, j)
}

// ByteAt returns the k-th byte of the value at slot i as seen by the
// comparator.
func (r *MSBStringRadixSorter) ByteAt(i, k int) int {
	o := r.owner
	o.impl.Get(o.scratch1, o.scratchBytes1, i)
	return r.cmp.ByteAt(o.scratchBytes1, k)
}

// Compare is MSBRadixSorter.compare, which is final and unsupported: a radix
// sort is not comparison based. It panics like the Java
// UnsupportedOperationException.
func (r *MSBStringRadixSorter) Compare(i, j int) int {
	panic("unused: not a comparison-based sort")
}

// GetFallbackSorter returns the owner's fallback sorter comparing from the
// k-th byte on. Mirrors MSBStringRadixSorter.getFallbackSorter(int).
func (r *MSBStringRadixSorter) GetFallbackSorter(k int) Sorter {
	return r.owner.FallbackSorter(func(o1, o2 *BytesRef) int { return r.cmp.CompareK(o1, o2, k) })
}

// StringSortable implements Sortable and RadixSortable for sorting indices of BytesRefs.
type StringSortable struct {
	data     []BytesRef
	indices  []int
	cmp      func(a, b *BytesRef) int
	scratch1 *BytesRef
	scratch2 *BytesRef
	pivot    *BytesRef
}

func NewStringSortable(data []BytesRef, indices []int, cmp func(a, b *BytesRef) int) *StringSortable {
	return &StringSortable{
		data:     data,
		indices:  indices,
		cmp:      cmp,
		scratch1: NewBytesRefEmpty(),
		scratch2: NewBytesRefEmpty(),
		pivot:    NewBytesRefEmpty(),
	}
}

func (s *StringSortable) Compare(i, j int) int {
	s.get(s.scratch1, i)
	s.get(s.scratch2, j)
	return s.cmp(s.scratch1, s.scratch2)
}

func (s *StringSortable) Swap(i, j int) {
	s.indices[i], s.indices[j] = s.indices[j], s.indices[i]
}

func (s *StringSortable) ByteAt(i, k int) int {
	s.get(s.scratch1, i)
	if k >= s.scratch1.Length {
		return -1
	}
	return int(s.scratch1.Bytes[s.scratch1.Offset+k])
}

func (s *StringSortable) SetPivot(i int) {
	s.get(s.pivot, i)
}

func (s *StringSortable) ComparePivot(j int) int {
	s.get(s.scratch1, j)
	return s.cmp(s.pivot, s.scratch1)
}

func (s *StringSortable) get(dest *BytesRef, i int) {
	ref := s.data[s.indices[i]]
	dest.Bytes = ref.Bytes
	dest.Offset = ref.Offset
	dest.Length = ref.Length
}

// SortBytesRefs sorts a slice of BytesRefs using the most efficient sorter available.
func SortBytesRefs(data []BytesRef, cmp func(a, b *BytesRef) int) {
	indices := make([]int, len(data))
	for i := range indices {
		indices[i] = i
	}

	ss := NewStringSortable(data, indices, cmp)

	maxLength := 0
	for _, ref := range data {
		if ref.Length > maxLength {
			maxLength = ref.Length
		}
	}

	// Try MSBRadixSort
	radix := NewMSBRadixSorter(maxLength)
	radix.Sort(ss, 0, len(indices))

	// Copy sorted indices back to data
	sortedData := make([]BytesRef, len(data))
	for i, idx := range indices {
		sortedData[i] = data[idx]
	}
	copy(data, sortedData)
}

// NaturalComparator provides the natural byte-order comparison for BytesRefs.
func NaturalComparator(a, b *BytesRef) int {
	return bytes.Compare(a.ValidBytes(), b.ValidBytes())
}
