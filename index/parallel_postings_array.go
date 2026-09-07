// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// NewParallelPostingsArray allocates a base ParallelPostingsArray (declared
// in terms_hash_per_field.go) with the given number of term slots. Mirrors
// the constructor of Lucene's org.apache.lucene.index.ParallelPostingsArray.
func NewParallelPostingsArray(size int) *ParallelPostingsArray {
	return &ParallelPostingsArray{
		Size:          size,
		TextStarts:    make([]int, size),
		AddressOffset: make([]int, size),
		ByteStarts:    make([]int, size),
	}
}
