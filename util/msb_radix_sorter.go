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

// MSBRadixSorterImpl defines the abstract operations a concrete
// MSBRadixSorter needs from its caller. The contract is identical to
// the Java abstract methods of org.apache.lucene.util.MSBRadixSorter:
//   - ByteAt returns the k-th byte of slot i, or -1 if slot i has fewer
//     than k+1 bytes.
//   - Swap swaps the entries at slots i and j.
//
// Implementations may apply MSBRadixSorter to any indexable container
// whose entries can be addressed by a contiguous int range.
type MSBRadixSorterImpl interface {
	// ByteAt returns the k-th byte of slot i as an int in [0,255], or
	// -1 if slot i has fewer than k+1 bytes. Called only for 0 <= k < maxLength.
	ByteAt(i, k int) int
	// Swap exchanges the values stored at slots i and j.
	Swap(i, j int)
}

const (
	msbLevelThreshold  = 8
	msbHistogramSize   = 257
	msbLengthThreshold = 100
)

// MSBRadixSorter is a most-significant-byte radix sorter for
// variable-length byte strings, falling back to IntroSorter once a
// bucket becomes smaller than LENGTH_THRESHOLD (100) or recursion
// reaches LEVEL_THRESHOLD (8). This is a direct port of Lucene 10.4.0
// MSBRadixSorter and preserves the bucket-order semantics that
// downstream serialisation paths (points/BKD) depend on.
type MSBRadixSorter struct {
	Sorter

	impl         MSBRadixSorterImpl
	maxLength    int
	histograms   [msbLevelThreshold][]int32
	endOffsets   [msbHistogramSize]int32
	commonPrefix []int
}

// NewMSBRadixSorter creates a fresh MSBRadixSorter for keys of at most
// maxLength bytes. Pass math.MaxInt32 (or any large sentinel) when the
// maximum length is unknown.
func NewMSBRadixSorter(impl MSBRadixSorterImpl, maxLength int) *MSBRadixSorter {
	cpLen := 24
	if maxLength < cpLen {
		cpLen = maxLength
	}
	return &MSBRadixSorter{
		impl:         impl,
		maxLength:    maxLength,
		commonPrefix: make([]int, cpLen),
	}
}

// Sort sorts the range [from, to) of the underlying container.
func (s *MSBRadixSorter) Sort(from, to int) {
	s.CheckRange(from, to)
	s.sort(from, to, 0, 0)
}

// sort dispatches between radix sort and the introsort fallback.
func (s *MSBRadixSorter) sort(from, to, k, l int) {
	if s.shouldFallback(from, to, l) {
		s.fallbackSort(from, to, k)
	} else {
		s.radixSort(from, to, k, l)
	}
}

func (s *MSBRadixSorter) shouldFallback(from, to, l int) bool {
	return to-from <= msbLengthThreshold || l >= msbLevelThreshold
}

// fallbackSort runs an IntroSorter that assumes the first k bytes of
// every entry in [from, to) are equal. Mirrors getFallbackSorter(k) in
// the Java reference, which constructs an inline anonymous IntroSorter.
func (s *MSBRadixSorter) fallbackSort(from, to, k int) {
	adapter := &msbIntroAdapter{owner: s, k: k}
	NewIntroSorter(adapter).Sort(from, to)
}

// radixSort runs one pass of the MSB algorithm at byte index k and
// recursion level l.
func (s *MSBRadixSorter) radixSort(from, to, k, l int) {
	hist := s.histograms[l]
	if hist == nil {
		hist = make([]int32, msbHistogramSize)
		s.histograms[l] = hist
	} else {
		for i := range hist {
			hist[i] = 0
		}
	}

	commonPrefixLength := s.computeCommonPrefixLengthAndBuildHistogram(from, to, k, hist)
	if commonPrefixLength > 0 {
		if k+commonPrefixLength < s.maxLength && int(hist[0]) < to-from {
			s.radixSort(from, to, k+commonPrefixLength, l)
		}
		return
	}

	startOffsets := hist
	endOffsets := s.endOffsets[:]
	msbSumHistogram(hist, endOffsets)
	s.reorder(from, to, startOffsets, endOffsets, k)
	endOffsets = startOffsets

	if k+1 < s.maxLength {
		prev := int(endOffsets[0])
		for i := 1; i < msbHistogramSize; i++ {
			h := int(endOffsets[i])
			bucketLen := h - prev
			if bucketLen > 1 {
				s.sort(from+prev, from+h, k+1, l+1)
			}
			prev = h
		}
	}
}

// computeCommonPrefixLengthAndBuildHistogram returns the length of the
// common prefix shared by all entries in [from, to) starting at offset
// k. As a side effect it populates the histogram for byte k. The
// three-part split mirrors the Java reference, where it was introduced
// as a JVM-crash workaround (LUCENE-12898). We keep the same structure
// for parity with the Lucene source.
func (s *MSBRadixSorter) computeCommonPrefixLengthAndBuildHistogram(from, to, k int, histogram []int32) int {
	commonPrefixLength := s.computeInitialCommonPrefixLength(from, k)
	return s.computeCommonPrefixLengthAndBuildHistogramPart1(from, to, k, histogram, commonPrefixLength)
}

func (s *MSBRadixSorter) computeInitialCommonPrefixLength(from, k int) int {
	commonPrefix := s.commonPrefix
	cap := len(commonPrefix)
	if rem := s.maxLength - k; rem < cap {
		cap = rem
	}
	commonPrefixLength := cap
	for j := 0; j < commonPrefixLength; j++ {
		b := s.impl.ByteAt(from, k+j)
		commonPrefix[j] = b
		if b == -1 {
			commonPrefixLength = j + 1
			break
		}
	}
	return commonPrefixLength
}

func (s *MSBRadixSorter) computeCommonPrefixLengthAndBuildHistogramPart1(
	from, to, k int, histogram []int32, commonPrefixLength int,
) int {
	commonPrefix := s.commonPrefix
	i := from + 1
outer:
	for ; i < to; i++ {
		for j := 0; j < commonPrefixLength; j++ {
			b := s.impl.ByteAt(i, k+j)
			if b != commonPrefix[j] {
				commonPrefixLength = j
				if commonPrefixLength == 0 {
					break outer
				}
				break
			}
		}
	}
	return s.computeCommonPrefixLengthAndBuildHistogramPart2(from, to, k, histogram, commonPrefixLength, i)
}

func (s *MSBRadixSorter) computeCommonPrefixLengthAndBuildHistogramPart2(
	from, to, k int, histogram []int32, commonPrefixLength, i int,
) int {
	if i < to {
		s.buildHistogram(s.commonPrefix[0]+1, i-from, i, to, k, histogram)
	} else {
		histogram[s.commonPrefix[0]+1] = int32(to - from)
	}
	return commonPrefixLength
}

// buildHistogram populates the histogram for byte k over entries in
// [from, to), seeding it with prefixCommonLen entries at the common
// prefix bucket.
func (s *MSBRadixSorter) buildHistogram(prefixCommonBucket, prefixCommonLen, from, to, k int, histogram []int32) {
	histogram[prefixCommonBucket] = int32(prefixCommonLen)
	for i := from; i < to; i++ {
		histogram[s.getBucket(i, k)]++
	}
}

// getBucket returns the 1-indexed bucket id (0 reserved for "end of
// string") for byte k of slot i.
func (s *MSBRadixSorter) getBucket(i, k int) int {
	return s.impl.ByteAt(i, k) + 1
}

// msbSumHistogram converts counts to (start, end) offsets in place,
// storing end offsets in the supplied endOffsets slice.
func msbSumHistogram(histogram, endOffsets []int32) {
	var accum int32
	for i := 0; i < msbHistogramSize; i++ {
		count := histogram[i]
		histogram[i] = accum
		accum += count
		endOffsets[i] = accum
	}
}

// reorder shuffles the entries in [from, to) so that each bucket
// occupies its contiguous slot. After this returns, startOffsets and
// endOffsets are equal element-wise. The Dutch-national-flag style
// algorithm matches Lucene's reorder() exactly.
func (s *MSBRadixSorter) reorder(from, to int, startOffsets, endOffsets []int32, k int) {
	for i := 0; i < msbHistogramSize; i++ {
		limit := endOffsets[i]
		for h1 := startOffsets[i]; h1 < limit; h1 = startOffsets[i] {
			b := s.getBucket(from+int(h1), k)
			h2 := startOffsets[b]
			startOffsets[b]++
			s.impl.Swap(from+int(h1), from+int(h2))
		}
	}
}

// msbIntroAdapter adapts MSBRadixSorter to IntroSorter for the
// short-bucket fallback. It compares two entries by reading bytes from
// the underlying MSBRadixSorterImpl starting at byte index k.
type msbIntroAdapter struct {
	owner *MSBRadixSorter
	k     int
	pivot []byte
}

func (a *msbIntroAdapter) Compare(i, j int) int {
	for o := a.k; o < a.owner.maxLength; o++ {
		b1 := a.owner.impl.ByteAt(i, o)
		b2 := a.owner.impl.ByteAt(j, o)
		if b1 != b2 {
			return b1 - b2
		}
		if b1 == -1 {
			break
		}
	}
	return 0
}

func (a *msbIntroAdapter) Swap(i, j int)    { a.owner.impl.Swap(i, j) }
func (a *msbIntroAdapter) Sort(from, to int) {}

func (a *msbIntroAdapter) SetPivot(i int) {
	a.pivot = a.pivot[:0]
	for o := a.k; o < a.owner.maxLength; o++ {
		b := a.owner.impl.ByteAt(i, o)
		if b == -1 {
			break
		}
		a.pivot = append(a.pivot, byte(b))
	}
}

func (a *msbIntroAdapter) ComparePivot(j int) int {
	for o := 0; o < len(a.pivot); o++ {
		b1 := int(a.pivot[o]) & 0xFF
		b2 := a.owner.impl.ByteAt(j, a.k+o)
		if b1 != b2 {
			return b1 - b2
		}
	}
	if a.k+len(a.pivot) == a.owner.maxLength {
		return 0
	}
	return -1 - a.owner.impl.ByteAt(j, a.k+len(a.pivot))
}
