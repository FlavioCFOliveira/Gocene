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

// radixSelectorLevelThreshold is the recursion depth after which the
// radix selector falls back to IntroSelector (the algorithm degrades
// when there are long common prefixes due to cache locality, so we
// switch to a comparison-based selector below this level).
const radixSelectorLevelThreshold = 8

// radixSelectorHistogramSize is the per-level histogram width: 256
// bytes plus one extra bucket (index 0) representing "string is
// finished".
const radixSelectorHistogramSize = 257

// radixSelectorLengthThreshold is the size below which a partition is
// finished off with IntroSelector instead of more radix recursion.
const radixSelectorLengthThreshold = 100

// RadixSelectorInterface is the dynamic-dispatch contract a concrete
// [RadixSelector] consumer must satisfy: a swap operation plus a
// byte-at-(i,k) probe.
//
// ByteAt must return the k-th byte of slot i, or -1 if its length is
// less than or equal to k. Callers may assume k is in the range
// [0, maxLength).
type RadixSelectorInterface interface {
	// Swap exchanges slots i and j.
	Swap(i, j int)
	// ByteAt returns the k-th byte of slot i, or -1 if the slot has no
	// byte at that position.
	ByteAt(i, k int) int
}

// RadixSelector is the Go port of org.apache.lucene.util.RadixSelector.
// It performs in-place k-th element selection on byte-array keys via a
// MSB radix sort that only recurses into the partition holding k,
// with an IntroSelector fallback for short partitions and deep
// recursion.
type RadixSelector struct {
	Selector
	impl         RadixSelectorInterface
	maxLength    int
	histogram    [radixSelectorHistogramSize]int
	commonPrefix []int
}

// NewRadixSelector constructs a RadixSelector. impl supplies the
// concrete Swap/ByteAt operations; maxLength is the maximum key
// length in bytes (pass a very large value like math.MaxInt32 when
// the key length is unbounded).
func NewRadixSelector(impl RadixSelectorInterface, maxLength int) *RadixSelector {
	cp := 24
	if maxLength < cp {
		cp = maxLength
	}
	return &RadixSelector{
		impl:         impl,
		maxLength:    maxLength,
		commonPrefix: make([]int, cp),
	}
}

// Select reorders the range [from, to) so the element at slot k is
// the same as if all elements had been fully sorted.
func (s *RadixSelector) Select(from, to, k int) {
	s.CheckArgs(from, to, k)
	s.selectRecur(from, to, k, 0, 0)
}

// selectRecur dispatches between the radix and fallback paths.
func (s *RadixSelector) selectRecur(from, to, k, d, l int) {
	if to-from <= radixSelectorLengthThreshold || l >= radixSelectorLevelThreshold {
		s.fallbackSelector(d).Select(from, to, k)
		return
	}
	s.radixSelect(from, to, k, d, l)
}

// radixSelect is the core of the algorithm. It builds a histogram of
// the d-th byte across [from, to), partitions the range into 257
// buckets, and recurses only into the bucket containing k.
func (s *RadixSelector) radixSelect(from, to, k, d, l int) {
	for i := range s.histogram {
		s.histogram[i] = 0
	}

	commonPrefixLen := s.computeCommonPrefixLengthAndBuildHistogram(from, to, d)
	if commonPrefixLen > 0 {
		// All entries in [from, to) share a common prefix at level d.
		// If we have more bytes to compare and the prefix did not
		// terminate every string, recurse past it.
		if d+commonPrefixLen < s.maxLength && s.histogram[0] < to-from {
			s.radixSelect(from, to, k, d+commonPrefixLen, l)
		}
		return
	}

	bucketFrom := from
	for bucket := 0; bucket < radixSelectorHistogramSize; bucket++ {
		bucketTo := bucketFrom + s.histogram[bucket]
		if bucketTo > k {
			s.partition(from, to, bucket, bucketFrom, bucketTo, d)
			if bucket != 0 && d+1 < s.maxLength {
				// Bucket 0 means the string finished at this byte —
				// everything in it is equal, so no further recursion.
				s.selectRecur(bucketFrom, bucketTo, k, d+1, l+1)
			}
			return
		}
		bucketFrom = bucketTo
	}
	panic("RadixSelector.radixSelect: unreachable")
}

// getBucket returns the histogram bucket for slot i at level k. The
// "-1 means end-of-string" semantic from ByteAt is folded into bucket
// 0 by adding one.
func (s *RadixSelector) getBucket(i, k int) int { return s.impl.ByteAt(i, k) + 1 }

// computeCommonPrefixLengthAndBuildHistogram detects a shared prefix
// across [from, to) starting at byte k. When the prefix terminates,
// it builds the histogram for the remaining bytes.
func (s *RadixSelector) computeCommonPrefixLengthAndBuildHistogram(from, to, k int) int {
	commonPrefixLen := s.computeInitialCommonPrefixLength(from, k)
	return s.computeCommonPrefixLengthAndBuildHistogramPart1(from, to, k, commonPrefixLen)
}

// computeInitialCommonPrefixLength reads the first element's prefix
// into s.commonPrefix and trims the buffer at the first end-of-string
// marker.
func (s *RadixSelector) computeInitialCommonPrefixLength(from, k int) int {
	commonPrefixLen := len(s.commonPrefix)
	if rem := s.maxLength - k; rem < commonPrefixLen {
		commonPrefixLen = rem
	}
	for j := 0; j < commonPrefixLen; j++ {
		b := s.impl.ByteAt(from, k+j)
		s.commonPrefix[j] = b
		if b == -1 {
			commonPrefixLen = j + 1
			break
		}
	}
	return commonPrefixLen
}

// computeCommonPrefixLengthAndBuildHistogramPart1 scans [from+1, to)
// trimming commonPrefixLen at the first divergence. If a divergence
// is found, it short-circuits the loop and dispatches to the partial
// histogram builder.
func (s *RadixSelector) computeCommonPrefixLengthAndBuildHistogramPart1(from, to, k, commonPrefixLen int) int {
	i := from + 1
	for ; i < to; i++ {
		for j := 0; j < commonPrefixLen; j++ {
			b := s.impl.ByteAt(i, k+j)
			if b != s.commonPrefix[j] {
				commonPrefixLen = j
				if commonPrefixLen == 0 {
					// No common prefix at all — record the histogram
					// entries for the first byte of the previously-seen
					// entries and for the current entry, then break the
					// outer scan.
					s.histogram[s.commonPrefix[0]+1] = i - from
					s.histogram[b+1] = 1
					i++
					goto histogramBuild
				}
				break
			}
		}
	}
histogramBuild:
	return s.computeCommonPrefixLengthAndBuildHistogramPart2(from, to, k, commonPrefixLen, i)
}

// computeCommonPrefixLengthAndBuildHistogramPart2 finishes the
// histogram. When the outer scan completed without finding a
// divergence, the whole range shares a prefix; otherwise the
// remainder of the range is bucketed.
func (s *RadixSelector) computeCommonPrefixLengthAndBuildHistogramPart2(from, to, k, commonPrefixLen, i int) int {
	if i < to {
		// The outer loop broke because there is no common prefix.
		s.buildHistogram(i, to, k)
	} else {
		s.histogram[s.commonPrefix[0]+1] = to - from
	}
	return commonPrefixLen
}

// buildHistogram increments the bucket count for every slot in
// [from, to) at byte k.
func (s *RadixSelector) buildHistogram(from, to, k int) {
	for i := from; i < to; i++ {
		s.histogram[s.getBucket(i, k)]++
	}
}

// partition reorders [from, to) so that every slot whose d-th byte
// equals bucket lies in [bucketFrom, bucketTo). Other slots are
// pushed outside that window using a two-pointer scan.
func (s *RadixSelector) partition(from, to, bucket, bucketFrom, bucketTo, d int) {
	left := from
	right := to - 1
	slot := bucketFrom

	for {
		leftBucket := s.getBucket(left, d)
		rightBucket := s.getBucket(right, d)

		for leftBucket <= bucket && left < bucketFrom {
			if leftBucket == bucket {
				s.impl.Swap(left, slot)
				slot++
			} else {
				left++
			}
			leftBucket = s.getBucket(left, d)
		}
		for rightBucket >= bucket && right >= bucketTo {
			if rightBucket == bucket {
				s.impl.Swap(right, slot)
				slot++
			} else {
				right--
			}
			rightBucket = s.getBucket(right, d)
		}

		if left < bucketFrom && right >= bucketTo {
			s.impl.Swap(left, right)
			left++
			right--
		} else {
			// invariants: left == bucketFrom, right == bucketTo - 1
			return
		}
	}
}

// fallbackSelector returns an IntroSelector that assumes the first d
// bytes of every key are equal. Used for short partitions and after
// the radix recursion limit is hit.
func (s *RadixSelector) fallbackSelector(d int) *IntroSelector {
	return NewIntroSelector(&radixFallbackImpl{owner: s, d: d})
}

// radixFallbackImpl satisfies IntroSelectorInterface by delegating to
// the radix selector's owner and decoding byte streams via ByteAt at
// every probe.
type radixFallbackImpl struct {
	owner *RadixSelector
	d     int
	pivot []byte
}

// Swap delegates to the owner.
func (r *radixFallbackImpl) Swap(i, j int) { r.owner.impl.Swap(i, j) }

// Select must be implemented to satisfy SelectorInterface; the
// IntroSelector wrapping this impl performs the recursive selection,
// so this method is unreachable.
func (r *radixFallbackImpl) Select(_, _, _ int) {
	panic("radixFallbackImpl.Select: unreachable; use IntroSelector.Select")
}

// Compare two keys by their suffix starting at byte d.
func (r *radixFallbackImpl) Compare(i, j int) int {
	for o := r.d; o < r.owner.maxLength; o++ {
		b1 := r.owner.impl.ByteAt(i, o)
		b2 := r.owner.impl.ByteAt(j, o)
		if b1 != b2 {
			return b1 - b2
		}
		if b1 == -1 {
			break
		}
	}
	return 0
}

// SetPivot snapshots slot i's suffix into r.pivot so subsequent
// ComparePivot calls don't have to consult the original slot.
func (r *radixFallbackImpl) SetPivot(i int) {
	r.pivot = r.pivot[:0]
	for o := r.d; o < r.owner.maxLength; o++ {
		b := r.owner.impl.ByteAt(i, o)
		if b == -1 {
			break
		}
		r.pivot = append(r.pivot, byte(b))
	}
}

// ComparePivot compares the saved pivot with slot j.
func (r *radixFallbackImpl) ComparePivot(j int) int {
	for o := 0; o < len(r.pivot); o++ {
		b1 := int(r.pivot[o]) & 0xff
		b2 := r.owner.impl.ByteAt(j, r.d+o)
		if b1 != b2 {
			return b1 - b2
		}
	}
	if r.d+len(r.pivot) == r.owner.maxLength {
		return 0
	}
	return -1 - r.owner.impl.ByteAt(j, r.d+len(r.pivot))
}
