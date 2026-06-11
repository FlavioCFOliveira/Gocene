// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// DirectMonotonicReader retrieves a sequence previously written by
// DirectMonotonicWriter.
//
// This is the Go port of org.apache.lucene.util.packed.DirectMonotonicReader
// in Apache Lucene 10.4.0.
type DirectMonotonicReader struct {
	blockShift int
	blockMask  int64
	readers    []LongValues
	mins       []int64
	avgs       []float32
	bpvs       []byte
}

// DirectMonotonicMeta holds the in-memory metadata that DirectMonotonicReader
// needs to read data from disk. It is populated by LoadDirectMonotonicMeta.
type DirectMonotonicMeta struct {
	blockShift int
	numBlocks  int
	mins       []int64
	avgs       []float32
	bpvs       []byte
	offsets    []int64
}

// singleZeroBlockMeta is the shared all-zero Meta returned by
// LoadDirectMonotonicMeta when every block has min=0, avg=0, bpv=0;
// this lets readers of all-zero sequences share heap (matches Lucene).
var singleZeroBlockMeta = &DirectMonotonicMeta{
	blockShift: 63,
	numBlocks:  1,
	mins:       []int64{0},
	avgs:       []float32{0},
	bpvs:       []byte{0},
	offsets:    []int64{0},
}

// LoadDirectMonotonicMeta reads the per-block metadata previously
// emitted by DirectMonotonicWriter's metaOut stream.
func LoadDirectMonotonicMeta(metaIn store.DataInput, numValues int64, blockShift int) (*DirectMonotonicMeta, error) {
	numBlocks := numValues >> uint(blockShift)
	if (numBlocks << uint(blockShift)) < numValues {
		numBlocks++
	}
	meta := &DirectMonotonicMeta{
		blockShift: blockShift,
		numBlocks:  int(numBlocks),
		mins:       make([]int64, numBlocks),
		avgs:       make([]float32, numBlocks),
		bpvs:       make([]byte, numBlocks),
		offsets:    make([]int64, numBlocks),
	}
	allZero := true
	for i := 0; i < meta.numBlocks; i++ {
		min, err := metaIn.ReadLong()
		if err != nil {
			return nil, err
		}
		meta.mins[i] = min

		avgInt, err := metaIn.ReadInt()
		if err != nil {
			return nil, err
		}
		meta.avgs[i] = math.Float32frombits(uint32(avgInt))

		offset, err := metaIn.ReadLong()
		if err != nil {
			return nil, err
		}
		meta.offsets[i] = offset

		bpv, err := metaIn.ReadByte()
		if err != nil {
			return nil, err
		}
		meta.bpvs[i] = bpv

		if min != 0 || avgInt != 0 || bpv != 0 {
			allZero = false
		}
	}
	if allZero {
		return singleZeroBlockMeta, nil
	}
	return meta, nil
}

// zeroLongValues returns 0 for every Get call.
type zeroLongValues struct{}

func (zeroLongValues) Get(index int64) (int64, error) { return 0, nil }

// NewDirectMonotonicReader constructs a reader from the given meta
// and a RandomAccessInput positioned over the data stream.
func NewDirectMonotonicReader(meta *DirectMonotonicMeta, data RandomAccessInput) (*DirectMonotonicReader, error) {
	readers := make([]LongValues, meta.numBlocks)
	for i := 0; i < meta.numBlocks; i++ {
		if meta.bpvs[i] == 0 {
			readers[i] = zeroLongValues{}
			continue
		}
		r, err := GetDirectReaderAt(data, int(meta.bpvs[i]), meta.offsets[i])
		if err != nil {
			return nil, err
		}
		readers[i] = r
	}
	if len(readers) != len(meta.mins) ||
		len(readers) != len(meta.avgs) ||
		len(readers) != len(meta.bpvs) {
		return nil, fmt.Errorf("packed: inconsistent meta arrays")
	}
	blockMask := int64(0)
	if meta.blockShift < 63 {
		blockMask = (int64(1) << uint(meta.blockShift)) - 1
	} else {
		blockMask = math.MaxInt64
	}
	return &DirectMonotonicReader{
		blockShift: meta.blockShift,
		blockMask:  blockMask,
		readers:    readers,
		mins:       meta.mins,
		avgs:       meta.avgs,
		bpvs:       meta.bpvs,
	}, nil
}

// Get returns the value at the given index. Propagates errors from
// the underlying packed reader.
func (r *DirectMonotonicReader) Get(index int64) (int64, error) {
	block := int(uint64(index) >> uint(r.blockShift))
	blockIndex := index & r.blockMask
	delta, err := r.readers[block].Get(blockIndex)
	if err != nil {
		return 0, err
	}
	return r.mins[block] + int64(r.avgs[block]*float32(blockIndex)) + delta, nil
}

// BinarySearch returns the index of key in [fromIndex, toIndex) if it
// exists, or -(insertionPoint+1) like Java's Arrays.binarySearch.
func (r *DirectMonotonicReader) BinarySearch(fromIndex, toIndex, key int64) (int64, error) {
	if fromIndex < 0 || fromIndex > toIndex {
		return 0, fmt.Errorf("packed: fromIndex=%d, toIndex=%d", fromIndex, toIndex)
	}
	lo := fromIndex
	hi := toIndex - 1
	for lo <= hi {
		mid := int64(uint64(lo+hi) >> 1)
		lower, upper := r.bounds(mid)
		if upper < key {
			lo = mid + 1
		} else if lower > key {
			hi = mid - 1
		} else {
			midVal, err := r.Get(mid)
			if err != nil {
				return 0, fmt.Errorf("packed: direct monotonic binary search: %w", err)
			}
			if midVal < key {
				lo = mid + 1
			} else if midVal > key {
				hi = mid - 1
			} else {
				return mid, nil
			}
		}
	}
	return -1 - lo, nil
}

// bounds returns a lower/upper bound for the value at index without
// hitting the underlying random access input (since a fault there can
// page-fault). Returns [MinInt64, MaxInt64] on overflow.
func (r *DirectMonotonicReader) bounds(index int64) (int64, int64) {
	block := int(uint64(index) >> uint(r.blockShift))
	blockIndex := index & r.blockMask
	lower := r.mins[block] + int64(r.avgs[block]*float32(blockIndex))
	upper := lower + (int64(1) << uint(r.bpvs[block])) - 1
	if r.bpvs[block] == 64 || upper < lower {
		return math.MinInt64, math.MaxInt64
	}
	return lower, upper
}
