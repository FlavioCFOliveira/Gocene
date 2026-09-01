// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

// MSBRadixSorter implements a radix sorter for variable-length strings.
// This class sorts based on the most significant byte first and falls back
// to IntroSorter when the size of the buckets to sort becomes small.
//
// This algorithm is NOT stable.
//
// This is a port of Apache Lucene's MSBRadixSorter class.
type MSBRadixSorter struct {
	maxLength int
	histograms [][]int
	endOffsets []int
	commonPrefix []int
}

const (
	// LevelThreshold is the recursion level at which we fall back to introsort.
	LevelThreshold = 8
	// HistogramSize is the size of the histograms: 256 + 1 to indicate that the string is finished.
	HistogramSize = 257
	// LengthThreshold is the bucket size below which we fallback to introsort.
	LengthThreshold = 100
)

// NewMSBRadixSorter creates a new MSBRadixSorter.
func NewMSBRadixSorter(maxLength int) *MSBRadixSorter {
	return &MSBRadixSorter{
		maxLength:    maxLength,
		histograms:   make([][]int, LevelThreshold),
		endOffsets:   make([]int, HistogramSize),
		commonPrefix: make([]int, 24),
	}
}

// Sort sorts the range [from, to).
func (s *MSBRadixSorter) Sort(rs RadixSortable, from, to int) {
	if to-from <= 1 {
		return
	}
	s.sort(rs, from, to, 0, 0)
}

func (s *MSBRadixSorter) sort(rs RadixSortable, from, to, k, l int) {
	if s.shouldFallback(from, to, l) {
		fallback := &radixFallbackSorter{
			s:         rs,
			k:         k,
			maxLength: s.maxLength,
		}
		NewIntroSorter(fallback).Sort(from, to)
		return
	}

	histogram := s.histograms[l]
	if histogram == nil {
		histogram = make([]int, HistogramSize)
		s.histograms[l] = histogram
	} else {
		for i := range histogram {
			histogram[i] = 0
		}
	}

	commonPrefixLength := s.computeCommonPrefixLengthAndBuildHistogram(rs, from, to, k, histogram)
	if commonPrefixLength > 0 {
		if k+commonPrefixLength < s.maxLength && histogram[0] < to-from {
			s.sort(rs, from, to, k+commonPrefixLength, l)
		}
		return
	}

	startOffsets := histogram
	sumHistogram(histogram, s.endOffsets)
	s.reorder(rs, from, to, startOffsets, s.endOffsets, k)

	if k+1 < s.maxLength {
		for prev, i := 0, 1; i < HistogramSize; i++ {
			h := s.endOffsets[i]
			bucketLen := h - prev
			if bucketLen > 1 {
				s.sort(rs, from+prev, from+h, k+1, l+1)
			}
			prev = h
		}
	}
}

func (s *MSBRadixSorter) shouldFallback(from, to, l int) bool {
	return to-from <= LengthThreshold || l >= LevelThreshold
}

func (s *MSBRadixSorter) computeCommonPrefixLengthAndBuildHistogram(rs RadixSortable, from, to, k int, histogram []int) int {
	commonPrefixLength := len(s.commonPrefix)
	if s.maxLength-k < commonPrefixLength {
		commonPrefixLength = s.maxLength - k
	}
	for j := 0; j < commonPrefixLength; j++ {
		b := rs.ByteAt(from, k+j)
		s.commonPrefix[j] = b
		if b == -1 {
			commonPrefixLength = j + 1
			break
		}
	}

	i := from + 1
	for ; i < to; i++ {
		for j := 0; j < commonPrefixLength; j++ {
			b := rs.ByteAt(i, k+j)
			if b != s.commonPrefix[j] {
				commonPrefixLength = j
				if commonPrefixLength == 0 {
					break
				}
				break
			}
		}
		if commonPrefixLength == 0 {
			break
		}
	}

	if i < to {
		s.buildHistogram(rs, s.commonPrefix[0]+1, i-from, i, to, k, histogram)
	} else {
		histogram[s.commonPrefix[0]+1] = to - from
	}

	return commonPrefixLength
}

func (s *MSBRadixSorter) buildHistogram(rs RadixSortable, prefixCommonBucket, prefixCommonLen, from, to, k int, histogram []int) {
	histogram[prefixCommonBucket] = prefixCommonLen
	for i := from; i < to; i++ {
		histogram[s.getBucket(rs, i, k)]++
	}
}

func sumHistogram(histogram, endOffsets []int) {
	accum := 0
	for i := 0; i < HistogramSize; i++ {
		count := histogram[i]
		histogram[i] = accum
		accum += count
		endOffsets[i] = accum
	}
}

func (s *MSBRadixSorter) reorder(rs RadixSortable, from, to int, startOffsets, endOffsets []int, k int) {
	for i := 0; i < HistogramSize; i++ {
		limit := endOffsets[i]
		for h1 := startOffsets[i]; h1 < limit; h1 = startOffsets[i] {
			b := s.getBucket(rs, from+h1, k)
			h2 := startOffsets[b]
			startOffsets[b]++
			rs.Swap(from+h1, from+h2)
		}
	}
}

func (s *MSBRadixSorter) getBucket(rs RadixSortable, i, k int) int {
	return rs.ByteAt(i, k) + 1
}

type radixFallbackSorter struct {
	s         RadixSortable
	k         int
	maxLength int
}

func (r *radixFallbackSorter) Compare(i, j int) int {
	for o := r.k; o < r.maxLength; o++ {
		b1 := r.s.ByteAt(i, o)
		b2 := r.s.ByteAt(j, o)
		if b1 != b2 {
			return b1 - b2
		} else if b1 == -1 {
			break
		}
	}
	return 0
}

func (r *radixFallbackSorter) Swap(i, j int) {
	r.s.Swap(i, j)
}

func (r *radixFallbackSorter) SetPivot(i int) {
	if p, ok := r.s.(Pivotable); ok {
		p.SetPivot(i)
	} else {
		panic("RadixSortable must implement Pivotable for fallback IntroSort")
	}
}

func (r *radixFallbackSorter) ComparePivot(j int) int {
	if p, ok := r.s.(Pivotable); ok {
		return p.ComparePivot(j)
	}
	panic("RadixSortable must implement Pivotable for fallback IntroSort")
}
