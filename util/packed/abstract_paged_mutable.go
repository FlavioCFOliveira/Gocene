// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import "fmt"

// pagedMinBlockSize and pagedMaxBlockSize bound the page sizes accepted
// by AbstractPagedMutable, matching Lucene's MIN_BLOCK_SIZE / MAX_BLOCK_SIZE.
const (
	pagedMinBlockSize = 1 << 6
	pagedMaxBlockSize = 1 << 30
)

// AbstractPagedMutable provides the shared behavior of PagedMutable and
// PagedGrowableWriter: a flat int64-indexed mutable backed by a slice
// of fixed-size pages of PackedInts.Mutable. Concrete subtypes only
// need to provide a newMutable factory and a newUnfilledCopy factory.
//
// This is the Go port of org.apache.lucene.util.packed.AbstractPagedMutable
// in Apache Lucene 10.4.0.
type AbstractPagedMutable struct {
	size         int64
	pageShift    int
	pageMask     int
	bitsPerValue int
	subMutables  []Mutable

	newMutable      func(valueCount, bitsPerValue int) Mutable
	newUnfilledCopy func(newSize int64) *AbstractPagedMutable
}

// initAbstractPagedMutable validates the page size and lays out the
// (still nil) subMutables slice. Subtypes call fillPages once their
// own factories are wired up.
func initAbstractPagedMutable(bitsPerValue int, size int64, pageSize int) (*AbstractPagedMutable, error) {
	pageShift, err := CheckBlockSize(pageSize, pagedMinBlockSize, pagedMaxBlockSize)
	if err != nil {
		return nil, err
	}
	numPages, err := NumBlocks(size, pageSize)
	if err != nil {
		return nil, err
	}
	return &AbstractPagedMutable{
		size:         size,
		pageShift:    pageShift,
		pageMask:     pageSize - 1,
		bitsPerValue: bitsPerValue,
		subMutables:  make([]Mutable, numPages),
	}, nil
}

// fillPages populates subMutables using the configured newMutable
// factory; the last page is sized exactly to the trailing remainder.
func (a *AbstractPagedMutable) fillPages() {
	numPages := len(a.subMutables)
	for i := 0; i < numPages; i++ {
		valueCount := a.pageSize()
		if i == numPages-1 {
			valueCount = a.lastPageSize(a.size)
		}
		a.subMutables[i] = a.newMutable(valueCount, a.bitsPerValue)
	}
}

// pageSize returns the size of every page except the last.
func (a *AbstractPagedMutable) pageSize() int { return a.pageMask + 1 }

// lastPageSize returns the value count for the trailing page so a
// size of e.g. 1000 with pageSize=256 leaves a 232-value last page.
func (a *AbstractPagedMutable) lastPageSize(size int64) int {
	sz := a.indexInPage(size)
	if sz == 0 {
		return a.pageSize()
	}
	return sz
}

// pageIndex maps a flat index to its page.
func (a *AbstractPagedMutable) pageIndex(index int64) int {
	return int(uint64(index) >> uint(a.pageShift))
}

// indexInPage maps a flat index to its position within its page.
func (a *AbstractPagedMutable) indexInPage(index int64) int {
	return int(index) & a.pageMask
}

// Size returns the configured value count.
func (a *AbstractPagedMutable) Size() int64 { return a.size }

// Get returns the value at the given flat index.
func (a *AbstractPagedMutable) Get(index int64) int64 {
	if index < 0 || index >= a.size {
		panic(fmt.Sprintf("packed: index=%d size=%d", index, a.size))
	}
	return a.subMutables[a.pageIndex(index)].Get(a.indexInPage(index))
}

// Set writes the value at the given flat index.
func (a *AbstractPagedMutable) Set(index int64, value int64) {
	if index < 0 || index >= a.size {
		panic(fmt.Sprintf("packed: index=%d size=%d", index, a.size))
	}
	a.subMutables[a.pageIndex(index)].Set(a.indexInPage(index), value)
}

// Resize returns a new paged mutable of size newSize, copying as much
// of the existing content as fits.
func (a *AbstractPagedMutable) Resize(newSize int64) *AbstractPagedMutable {
	copy := a.newUnfilledCopy(newSize)
	commonPages := len(copy.subMutables)
	if len(a.subMutables) < commonPages {
		commonPages = len(a.subMutables)
	}
	buf := make([]int64, 1024)
	for i := 0; i < len(copy.subMutables); i++ {
		valueCount := a.pageSize()
		if i == len(copy.subMutables)-1 {
			valueCount = a.lastPageSize(newSize)
		}
		bpv := a.bitsPerValue
		if i < commonPages {
			bpv = a.subMutables[i].GetBitsPerValue()
		}
		copy.subMutables[i] = a.newMutable(valueCount, bpv)
		if i < commonPages {
			copyLength := valueCount
			if src := a.subMutables[i].Size(); src < copyLength {
				copyLength = src
			}
			copyWithBuf(a.subMutables[i], 0, copy.subMutables[i], 0, copyLength, buf)
		}
	}
	return copy
}

// Grow returns a paged mutable sized to fit at least minSize, copying
// the existing content. Mirrors Lucene's ArrayUtil.grow strategy.
func (a *AbstractPagedMutable) Grow(minSize int64) *AbstractPagedMutable {
	if minSize <= a.size {
		return a
	}
	extra := minSize >> 3
	if extra < 3 {
		extra = 3
	}
	return a.Resize(minSize + extra)
}

// RamBytesUsed estimates the in-memory footprint of the paged mutable
// (shallow page-array + each underlying Mutable's accounting).
func (a *AbstractPagedMutable) RamBytesUsed() int64 {
	const baseOverhead = 16 + 8 + 8 + 4*3
	bytes := int64(baseOverhead) + int64(16+8*len(a.subMutables))
	for _, m := range a.subMutables {
		if m != nil {
			bytes += m.RamBytesUsed()
		}
	}
	return bytes
}
