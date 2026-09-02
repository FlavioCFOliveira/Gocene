//go:build ignore

package index

import "github.com/FlavioCFOliveira/Gocene/util"

const bytesPerPosting = 3 * 4 // 3 * Integer.BYTES

// ParallelPostingsArray stores state for parallel postings.
// It is a port of org.apache.lucene.index.ParallelPostingsArray.
type ParallelPostingsArray struct {
	size          int
	textStarts    []int32 // maps term ID to the terms's text start in the bytesHash
	addressOffset []int32 // maps term ID to current stream address
	byteStarts    []int32 // maps term ID to stream start offset in the byte pool
}

// NewParallelPostingsArray creates a new ParallelPostingsArray with the given size.
func NewParallelPostingsArray(size int) *ParallelPostingsArray {
	return &ParallelPostingsArray{
		size:          size,
		textStarts:    make([]int32, size),
		addressOffset: make([]int32, size),
		byteStarts:    make([]int32, size),
	}
}

func (p *ParallelPostingsArray) bytesPerPosting() int {
	return bytesPerPosting
}

func (p *ParallelPostingsArray) newInstance(size int) *ParallelPostingsArray {
	return NewParallelPostingsArray(size)
}

func (p *ParallelPostingsArray) grow() *ParallelPostingsArray {
	newSize := util.Oversize(p.size+1, p.bytesPerPosting())
	newArray := p.newInstance(newSize)
	p.copyTo(newArray, p.size)
	return newArray
}

func (p *ParallelPostingsArray) copyTo(toArray *ParallelPostingsArray, numToCopy int) {
	copy(toArray.textStarts, p.textStarts[:numToCopy])
	copy(toArray.addressOffset, p.addressOffset[:numToCopy])
	copy(toArray.byteStarts, p.byteStarts[:numToCopy])
}
