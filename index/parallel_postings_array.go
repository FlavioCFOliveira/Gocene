package index

import "github.com/FlavioCFOliveira/Gocene/util"

const bytesPerPosting = 3 * 4 // 3 * Integer.BYTES

// parallelPostingsArray stores state for parallel postings.
// It is a port of org.apache.lucene.index.ParallelPostingsArray.
type parallelPostingsArray struct {
	size          int
	textStarts    []int32 // maps term ID to the terms's text start in the bytesHash
	addressOffset []int32 // maps term ID to current stream address
	byteStarts    []int32 // maps term ID to stream start offset in the byte pool
}

// NewParallelPostingsArray creates a new parallelPostingsArray with the given size.
func NewParallelPostingsArray(size int) *parallelPostingsArray {
	return &parallelPostingsArray{
		size:          size,
		textStarts:    make([]int32, size),
		addressOffset: make([]int32, size),
		byteStarts:    make([]int32, size),
	}
}

func (p *parallelPostingsArray) bytesPerPosting() int {
	return bytesPerPosting
}

func (p *parallelPostingsArray) newInstance(size int) *parallelPostingsArray {
	return NewParallelPostingsArray(size)
}

func (p *parallelPostingsArray) grow() *parallelPostingsArray {
	newSize := util.Oversize(p.size+1, p.bytesPerPosting())
	newArray := p.newInstance(newSize)
	p.copyTo(newArray, p.size)
	return newArray
}

func (p *parallelPostingsArray) copyTo(toArray *parallelPostingsArray, numToCopy int) {
	copy(toArray.textStarts, p.textStarts[:numToCopy])
	copy(toArray.addressOffset, p.addressOffset[:numToCopy])
	copy(toArray.byteStarts, p.byteStarts[:numToCopy])
}
