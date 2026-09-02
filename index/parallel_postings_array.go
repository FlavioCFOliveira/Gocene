// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// This file holds the constructor for ParallelPostingsArray, the Go port of
// org.apache.lucene.index.ParallelPostingsArray from Apache Lucene 10.5.0.
// The struct itself is declared in terms_hash_per_field.go, next to
// TermsHashPerField, its only consumer — and next to the growth machinery that
// the concrete postings arrays (FreqProx, TermVectors) hook into through the
// wrapper field.

// BytesPerPosting is the number of bytes a single term posting occupies in the
// side arrays: three ints, one each for the text start, the address offset and
// the byte start. Mirrors ParallelPostingsArray.BYTES_PER_POSTING.
const BytesPerPosting = 3 * 4

// NewParallelPostingsArray allocates a base ParallelPostingsArray with the
// given number of term slots. Mirrors ParallelPostingsArray(int size).
//
// The wrapper field is left nil: the concrete postings-array subtypes set it
// to themselves after embedding this value, so that array growth can copy
// their own side arrays.
func NewParallelPostingsArray(size int) *ParallelPostingsArray {
	return &ParallelPostingsArray{
		Size:          size,
		TextStarts:    make([]int, size),
		AddressOffset: make([]int, size),
		ByteStarts:    make([]int, size),
	}
}
