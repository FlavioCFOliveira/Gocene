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
//     http://www.apache.org/licenses/LICENSE-2.0

package util

import (
	"cmp"
	"fmt"
)

// Iterator is a generic iterator interface similar to Java's Iterator.
type Iterator[T any] interface {
	HasNext() bool
	Next() T
}

// IntIterator is an iterator over int values.
type IntIterator interface {
	HasNext() bool
	Next() int
}

// MergedIterator merges multiple sorted iterators into a single sorted iterator.
// It supports optional duplicate removal.
//
// This is the Go port of Lucene's org.apache.lucene.util.MergedIterator.
type MergedIterator struct {
	current          int
	queue            *PriorityQueue[*subIterator]
	top              []*subIterator
	removeDuplicates bool
	numTop           int
	hasCurrent       bool
}

// subIterator wraps an iterator with its current value and index.
type subIterator struct {
	iterator   IntIterator
	current    int
	index      int
	hasCurrent bool
}

// IntSliceIterator is an iterator over a slice of ints.
type IntSliceIterator struct {
	slice []int
	index int
}

// NewIntSliceIterator creates a new iterator over the given int slice.
func NewIntSliceIterator(slice []int) *IntSliceIterator {
	return &IntSliceIterator{
		slice: slice,
		index: 0,
	}
}

// HasNext returns true if there are more elements in the slice.
func (si *IntSliceIterator) HasNext() bool {
	return si.index < len(si.slice)
}

// Next returns the next element from the slice.
// Panics if there are no more elements.
func (si *IntSliceIterator) Next() int {
	if si.index >= len(si.slice) {
		panic("no more elements")
	}
	val := si.slice[si.index]
	si.index++
	return val
}

// NewMergedIterator creates a new MergedIterator that removes duplicates by default.
// The input iterators must be sorted in ascending order.
func NewMergedIterator(iterators ...IntIterator) (*MergedIterator, error) {
	return NewMergedIteratorWithOptions(true, iterators...)
}

// NewMergedIteratorWithOptions creates a new MergedIterator with configurable duplicate removal.
// If removeDuplicates is true, duplicate values across iterators will be returned only once.
// The input iterators must be sorted in ascending order.
func NewMergedIteratorWithOptions(removeDuplicates bool, iterators ...IntIterator) (*MergedIterator, error) {
	mi := &MergedIterator{
		removeDuplicates: removeDuplicates,
		top:              make([]*subIterator, len(iterators)),
	}

	// Create priority queue with comparator
	// Min-heap: smaller values have higher priority
	queue, err := NewPriorityQueue(len(iterators), func(a, b *subIterator) bool {
		if a.current != b.current {
			return a.current < b.current
		}
		// Equal values, compare by index for stability
		return a.index < b.index
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create priority queue: %w", err)
	}
	mi.queue = queue

	// Initialize sub-iterators
	index := 0
	for _, iterator := range iterators {
		if iterator.HasNext() {
			sub := &subIterator{
				iterator: iterator,
				index:    index,
			}
			sub.current = iterator.Next()
			sub.hasCurrent = true
			mi.queue.Add(sub)
			index++
		}
	}

	return mi, nil
}

// HasNext returns true if there are more elements to iterate.
func (mi *MergedIterator) HasNext() bool {
	if mi.queue.Size() > 0 {
		return true
	}

	for i := 0; i < mi.numTop; i++ {
		if mi.top[i].iterator.HasNext() {
			return true
		}
	}
	return false
}

// Next returns the next element in the merged iteration.
// Panics if there are no more elements.
func (mi *MergedIterator) Next() int {
	// Restore queue by pushing top elements back
	mi.pushTop()

	// Gather equal top elements
	if mi.queue.Size() > 0 {
		mi.pullTop()
	} else {
		mi.hasCurrent = false
	}

	if !mi.hasCurrent {
		panic("no more elements")
	}
	return mi.current
}

// pullTop extracts the top element(s) from the queue.
// If removeDuplicates is true, all elements with the same value are extracted.
func (mi *MergedIterator) pullTop() {
	mi.numTop = 0
	mi.top[mi.numTop] = mi.queue.Pop()
	mi.numTop++

	if mi.removeDuplicates {
		// Extract all subs from the queue that have the same top element
		for mi.queue.Size() > 0 {
			top := mi.queue.Top()
			if mi.top[0].current == top.current {
				mi.top[mi.numTop] = mi.queue.Pop()
				mi.numTop++
			} else {
				break
			}
		}
	}

	mi.current = mi.top[0].current
	mi.hasCurrent = true
}

// pushTop pushes the top elements back into the queue after advancing them.
func (mi *MergedIterator) pushTop() {
	for i := 0; i < mi.numTop; i++ {
		if mi.top[i].iterator.HasNext() {
			mi.top[i].current = mi.top[i].iterator.Next()
			mi.top[i].hasCurrent = true
			mi.queue.Add(mi.top[i])
		} else {
			// No more elements
			mi.top[i].hasCurrent = false
		}
	}
	mi.numTop = 0
}

// -----------------------------------------------------------------------------
// Generic MergedIteratorG[T] — mirrors the Java MergedIterator<T extends Comparable<T>>
// public contract. The legacy int-specialised type above is preserved for
// backwards compatibility with existing call sites.

// IteratorG is the generic equivalent of [Iterator] over T.
type IteratorG[T any] interface {
	HasNext() bool
	Next() T
}

// SliceIteratorG is a [IteratorG] backed by a slice.
type SliceIteratorG[T any] struct {
	slice []T
	index int
}

// NewSliceIteratorG returns a new generic slice iterator.
func NewSliceIteratorG[T any](s []T) *SliceIteratorG[T] {
	return &SliceIteratorG[T]{slice: s}
}

// HasNext reports whether more elements are available.
func (s *SliceIteratorG[T]) HasNext() bool { return s.index < len(s.slice) }

// Next returns the next element. Panics when no element is available.
func (s *SliceIteratorG[T]) Next() T {
	if s.index >= len(s.slice) {
		panic("no more elements")
	}
	v := s.slice[s.index]
	s.index++
	return v
}

// subIteratorG is the generic SubIterator analogue.
type subIteratorG[T cmp.Ordered] struct {
	iterator IteratorG[T]
	current  T
	index    int
}

// MergedIteratorG provides a merged sorted view over several sorted
// generic iterators. It mirrors the contract of the Java MergedIterator<T>
// for any [cmp.Ordered] T.
//
// Caveats are the same as the Java original:
//   - Behavior is undefined if input iterators are not sorted.
//   - Within a single iterator duplicates are not deduplicated.
//   - When duplicates are removed across iterators it is undefined which
//     one is returned.
//   - With removeDuplicates=false the order in which duplicates are
//     returned across iterators is undefined.
type MergedIteratorG[T cmp.Ordered] struct {
	current          T
	hasCurrent       bool
	queue            *PriorityQueue[*subIteratorG[T]]
	top              []*subIteratorG[T]
	removeDuplicates bool
	numTop           int
}

// NewMergedIteratorG creates a new generic MergedIteratorG that
// deduplicates equal values across iterators.
func NewMergedIteratorG[T cmp.Ordered](iterators ...IteratorG[T]) (*MergedIteratorG[T], error) {
	return NewMergedIteratorGWithOptions(true, iterators...)
}

// NewMergedIteratorGWithOptions creates a new generic MergedIteratorG
// configured by removeDuplicates.
func NewMergedIteratorGWithOptions[T cmp.Ordered](removeDuplicates bool, iterators ...IteratorG[T]) (*MergedIteratorG[T], error) {
	mi := &MergedIteratorG[T]{
		removeDuplicates: removeDuplicates,
		top:              make([]*subIteratorG[T], len(iterators)),
	}

	q, err := NewPriorityQueue(len(iterators), func(a, b *subIteratorG[T]) bool {
		if a.current != b.current {
			return a.current < b.current
		}
		return a.index < b.index
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create priority queue: %w", err)
	}
	mi.queue = q

	idx := 0
	for _, it := range iterators {
		if it.HasNext() {
			sub := &subIteratorG[T]{iterator: it, index: idx}
			sub.current = it.Next()
			mi.queue.Add(sub)
			idx++
		}
	}
	return mi, nil
}

// HasNext reports whether more elements are available.
func (mi *MergedIteratorG[T]) HasNext() bool {
	if mi.queue.Size() > 0 {
		return true
	}
	for i := 0; i < mi.numTop; i++ {
		if mi.top[i].iterator.HasNext() {
			return true
		}
	}
	return false
}

// Next returns the next element in merged order.
func (mi *MergedIteratorG[T]) Next() T {
	mi.pushTop()
	if mi.queue.Size() > 0 {
		mi.pullTop()
	} else {
		mi.hasCurrent = false
	}
	if !mi.hasCurrent {
		panic("no more elements")
	}
	return mi.current
}

func (mi *MergedIteratorG[T]) pullTop() {
	mi.numTop = 0
	mi.top[mi.numTop] = mi.queue.Pop()
	mi.numTop++

	if mi.removeDuplicates {
		for mi.queue.Size() > 0 {
			top := mi.queue.Top()
			if mi.top[0].current == top.current {
				mi.top[mi.numTop] = mi.queue.Pop()
				mi.numTop++
			} else {
				break
			}
		}
	}
	mi.current = mi.top[0].current
	mi.hasCurrent = true
}

func (mi *MergedIteratorG[T]) pushTop() {
	for i := 0; i < mi.numTop; i++ {
		if mi.top[i].iterator.HasNext() {
			mi.top[i].current = mi.top[i].iterator.Next()
			mi.queue.Add(mi.top[i])
		}
	}
	mi.numTop = 0
}
