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

// StringSorterImpl provides the abstract operations a [StringSorter]
// needs from the caller: fetching a BytesRef-keyed entry by slot and
// swapping entries.
//
// Mirrors the abstract get(BytesRefBuilder, BytesRef, int) /
// swap(int, int) methods of org.apache.lucene.util.StringSorter.
type StringSorterImpl interface {
	// Get loads slot i into the supplied scratch storage. Implementations
	// must populate result (which is the BytesRef returned to the
	// caller) by reusing builder when possible to avoid allocations.
	Get(builder *BytesRefBuilder, result *BytesRef, i int)
	// Swap exchanges the values stored at slots i and j.
	Swap(i, j int)
}

// StringSorter sorts BytesRef-keyed inputs. When the supplied
// comparator implements [BytesRefComparator] the sort is delegated to
// an [MSBRadixSorter] for the heavy lifting; otherwise it falls back
// to an IntroSorter. Mirrors org.apache.lucene.util.StringSorter.
type StringSorter struct {
	impl   StringSorterImpl
	cmp    BytesRefComparator
	natCmp func(o1, o2 *BytesRef) int

	scratch1      *BytesRefBuilder
	scratch2      *BytesRefBuilder
	pivotBuilder  *BytesRefBuilder
	scratchBytes1 BytesRef
	scratchBytes2 BytesRef
	pivot         BytesRef
}

// NewStringSorter returns a [StringSorter] using cmp as the
// underlying ordering. cmp may be a [BytesRefComparator] (radix fast
// path) or any function-style comparator (introsort fallback).
//
// The signature accepts both shapes via the variadic radix flag:
// pass a non-nil BytesRefComparator if available; otherwise pass nil
// and provide a simple comparator function via [NewStringSorterFn].
func NewStringSorter(impl StringSorterImpl, cmp BytesRefComparator) *StringSorter {
	return &StringSorter{
		impl:         impl,
		cmp:          cmp,
		scratch1:     NewBytesRefBuilder(),
		scratch2:     NewBytesRefBuilder(),
		pivotBuilder: NewBytesRefBuilder(),
	}
}

// NewStringSorterFn returns a [StringSorter] that compares with the
// supplied function. Radix acceleration is not available on this path.
func NewStringSorterFn(impl StringSorterImpl, cmp func(o1, o2 *BytesRef) int) *StringSorter {
	if cmp == nil {
		panic("util.NewStringSorterFn: cmp must not be nil")
	}
	return &StringSorter{
		impl:         impl,
		natCmp:       cmp,
		scratch1:     NewBytesRefBuilder(),
		scratch2:     NewBytesRefBuilder(),
		pivotBuilder: NewBytesRefBuilder(),
	}
}

// Sort orders the slice [from, to) using the configured comparator.
// If a BytesRefComparator is in play, an MSBRadixSorter is used;
// otherwise an IntroSorter falls back to byte-by-byte comparison.
func (s *StringSorter) Sort(from, to int) {
	if s.cmp != nil {
		s.radixSort(from, to)
		return
	}
	s.fallbackSort(from, to)
}

func (s *StringSorter) compareSlots(i, j int) int {
	s.impl.Get(s.scratch1, &s.scratchBytes1, i)
	s.impl.Get(s.scratch2, &s.scratchBytes2, j)
	if s.cmp != nil {
		return s.cmp.Compare(&s.scratchBytes1, &s.scratchBytes2)
	}
	return s.natCmp(&s.scratchBytes1, &s.scratchBytes2)
}

// radixSort runs the MSBRadixSorter path with byteAt routed through
// the BytesRefComparator.
func (s *StringSorter) radixSort(from, to int) {
	adapter := &stringRadixAdapter{owner: s}
	NewMSBRadixSorter(adapter, s.cmp.ComparedBytesCount()).Sort(from, to)
}

// fallbackSort runs the IntroSorter path.
func (s *StringSorter) fallbackSort(from, to int) {
	NewIntroSorter(&stringIntroAdapter{owner: s, fromK: 0}).Sort(from, to)
}

// stringRadixAdapter bridges StringSorter to MSBRadixSorter via the
// BytesRefComparator's byte-stream view.
type stringRadixAdapter struct {
	owner *StringSorter
}

func (a *stringRadixAdapter) ByteAt(i, k int) int {
	a.owner.impl.Get(a.owner.scratch1, &a.owner.scratchBytes1, i)
	return a.owner.cmp.ByteAt(&a.owner.scratchBytes1, k)
}

func (a *stringRadixAdapter) Swap(i, j int) { a.owner.impl.Swap(i, j) }

// stringIntroAdapter implements IntroSorterInterface using the
// owner's comparator. When fromK is non-zero we are operating inside
// an MSBRadixSorter fallback and skip the first fromK bytes via
// CompareK.
type stringIntroAdapter struct {
	owner *StringSorter
	fromK int
}

func (a *stringIntroAdapter) Compare(i, j int) int {
	a.owner.impl.Get(a.owner.scratch1, &a.owner.scratchBytes1, i)
	a.owner.impl.Get(a.owner.scratch2, &a.owner.scratchBytes2, j)
	if a.owner.cmp != nil {
		if a.fromK > 0 {
			return a.owner.cmp.CompareK(&a.owner.scratchBytes1, &a.owner.scratchBytes2, a.fromK)
		}
		return a.owner.cmp.Compare(&a.owner.scratchBytes1, &a.owner.scratchBytes2)
	}
	return a.owner.natCmp(&a.owner.scratchBytes1, &a.owner.scratchBytes2)
}

func (a *stringIntroAdapter) Swap(i, j int) { a.owner.impl.Swap(i, j) }

func (a *stringIntroAdapter) SetPivot(i int) {
	a.owner.impl.Get(a.owner.pivotBuilder, &a.owner.pivot, i)
}

func (a *stringIntroAdapter) ComparePivot(j int) int {
	a.owner.impl.Get(a.owner.scratch1, &a.owner.scratchBytes1, j)
	if a.owner.cmp != nil {
		if a.fromK > 0 {
			return a.owner.cmp.CompareK(&a.owner.pivot, &a.owner.scratchBytes1, a.fromK)
		}
		return a.owner.cmp.Compare(&a.owner.pivot, &a.owner.scratchBytes1)
	}
	return a.owner.natCmp(&a.owner.pivot, &a.owner.scratchBytes1)
}

func (a *stringIntroAdapter) Sort(from, to int) {}
