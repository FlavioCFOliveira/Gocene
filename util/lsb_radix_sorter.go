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

// LSBRadixSorter is a least-significant-bit radix sorter for unsigned int
// values. It mirrors org.apache.lucene.util.LSBRadixSorter byte-for-byte:
// the histogram is 256 buckets wide, each pass shifts by 8 bits, and any
// pass that observes a single bucket is skipped without buffer swap.
//
// Inputs are treated as unsigned: negative int32 values (Java has no
// unsigned) are still sorted correctly relative to one another but will
// fall after non-negative values in the output, which matches Lucene
// because the LSB pipeline operates on the raw bit pattern.
type LSBRadixSorter struct {
	histogram [lsbHistogramSize]int32
	buffer    []int32
}

const (
	lsbInsertionSortThreshold = 30
	lsbHistogramSize          = 256
)

// NewLSBRadixSorter creates a fresh LSBRadixSorter. Instances may be
// reused across many Sort calls; the internal buffer grows on demand.
func NewLSBRadixSorter() *LSBRadixSorter { return &LSBRadixSorter{} }

// Sort sorts arr[0:length] in place. numBits indicates how many bits
// are required to represent any of the values in arr[0:length]; pass
// 32 if unknown. Passing a too-small numBits is a programming error and
// will leave high-bit values out of place. The semantics match Lucene.
func (s *LSBRadixSorter) Sort(numBits int, arr []int32, length int) {
	if length < lsbInsertionSortThreshold {
		lsbInsertionSort(arr, 0, length)
		return
	}

	if cap(s.buffer) < length {
		s.buffer = make([]int32, length)
	} else {
		s.buffer = s.buffer[:length]
	}

	src := arr
	dst := s.buffer

	for shift := 0; shift < numBits; shift += 8 {
		if s.passSort(src, length, shift, dst) {
			src, dst = dst, src
		}
	}

	// If the final result ended up in the buffer, copy it back so that
	// the original arr slice always holds the sorted output.
	if &arr[0] != &src[0] {
		copy(arr[:length], src[:length])
	}
}

// passSort performs a single LSB pass for the given shift. Returns true
// if the pass actually reordered values; returns false when every value
// shares the same byte at this position so the buffer was untouched
// and src/dst should not be swapped.
func (s *LSBRadixSorter) passSort(src []int32, length, shift int, dst []int32) bool {
	for i := range s.histogram {
		s.histogram[i] = 0
	}
	lsbBuildHistogram(src, length, &s.histogram, shift)
	if s.histogram[0] == int32(length) {
		return false
	}
	lsbSumHistogram(&s.histogram)
	lsbReorder(src, length, &s.histogram, shift, dst)
	return true
}

// lsbBuildHistogram fills the 256-bucket histogram with counts of the
// byte at position `shift` of each value.
func lsbBuildHistogram(arr []int32, length int, hist *[lsbHistogramSize]int32, shift int) {
	for i := 0; i < length; i++ {
		b := int((uint32(arr[i]) >> uint(shift)) & 0xFF)
		hist[b]++
	}
}

// lsbSumHistogram converts the histogram from counts to exclusive
// prefix sums (bucket starts).
func lsbSumHistogram(hist *[lsbHistogramSize]int32) {
	var accum int32
	for i := 0; i < lsbHistogramSize; i++ {
		count := hist[i]
		hist[i] = accum
		accum += count
	}
}

// lsbReorder scatters src into dst according to the histogram cursors.
// This stable scatter preserves the order of equal-byte values.
func lsbReorder(src []int32, length int, hist *[lsbHistogramSize]int32, shift int, dst []int32) {
	for i := 0; i < length; i++ {
		v := src[i]
		b := int((uint32(v) >> uint(shift)) & 0xFF)
		dst[hist[b]] = v
		hist[b]++
	}
}

// lsbInsertionSort is the in-place fall-back used when length is below
// the insertion-sort threshold. Lucene uses signed comparison here
// (Java has no unsigned int); for the documented use case of unsigned
// values in the non-negative range this is identical to unsigned. The
// signed path is preserved to be byte-for-byte compatible with Lucene
// on every possible input.
func lsbInsertionSort(arr []int32, off, length int) {
	end := off + length
	for i := off + 1; i < end; i++ {
		for j := i; j > off; j-- {
			if arr[j-1] > arr[j] {
				arr[j-1], arr[j] = arr[j], arr[j-1]
			} else {
				break
			}
		}
	}
}
