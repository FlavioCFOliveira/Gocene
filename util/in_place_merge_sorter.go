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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

// InPlaceMergeSorter is a stable mergesort that performs every merge in
// place: it allocates no extra storage beyond a constant amount used
// for binary-search bookkeeping. The trade-off is that the per-merge
// cost becomes O((k1+k2) log min(k1,k2)) instead of the linear cost of
// a buffer-backed merge, so InPlaceMergeSorter is best suited to small
// or moderately sized inputs where the constant factor of zero
// allocation matters more than the asymptotic merge cost.
//
// The implementation delegates the recursive split to itself and uses
// Sorter.MergeInPlace (binary-search rotate) for each merge step.
// Below InsertionSortThreshold elements the algorithm falls back to
// Sorter.BinarySort to take advantage of insertion sort's cache
// friendliness.
//
// This is the Go port of org.apache.lucene.util.InPlaceMergeSorter.
type InPlaceMergeSorter struct {
	Sorter
	impl SorterInterface
}

// NewInPlaceMergeSorter constructs a sorter that operates on impl.
func NewInPlaceMergeSorter(impl SorterInterface) *InPlaceMergeSorter {
	return &InPlaceMergeSorter{impl: impl}
}

// Sort sorts the range [from, to) in place. The sort is stable.
func (s *InPlaceMergeSorter) Sort(from, to int) {
	s.CheckRange(from, to)
	s.mergeSort(from, to)
}

// mergeSort recursively halves the range until insertion sort kicks in,
// then merges the two halves in place.
func (s *InPlaceMergeSorter) mergeSort(from, to int) {
	if to-from < InsertionSortThreshold {
		s.BinarySort(from, to, s.impl)
		return
	}
	mid := (from + to) >> 1
	s.mergeSort(from, mid)
	s.mergeSort(mid, to)
	s.MergeInPlace(from, mid, to, s.impl)
}
