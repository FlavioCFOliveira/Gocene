package suggest

import (
	"bytes"
	"sort"
)

// SortedInputIterator buffers an inner iterator and re-emits its tuples in
// term-sorted order. Mirrors
// org.apache.lucene.search.suggest.SortedInputIterator.
type SortedInputIterator struct{ *BufferedInputIterator }

// NewSortedInputIterator buffers and sorts inner.
func NewSortedInputIterator(inner InputIterator) (*SortedInputIterator, error) {
	buf, err := NewBufferedInputIterator(inner)
	if err != nil {
		return nil, err
	}
	idx := make([]int, len(buf.terms))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		return bytes.Compare(buf.terms[idx[i]], buf.terms[idx[j]]) < 0
	})
	terms := make([][]byte, len(idx))
	weights := make([]int64, len(idx))
	payloads := make([][]byte, len(idx))
	contexts := make([][][]byte, len(idx))
	for newPos, oldPos := range idx {
		terms[newPos] = buf.terms[oldPos]
		weights[newPos] = buf.weights[oldPos]
		payloads[newPos] = buf.payloads[oldPos]
		contexts[newPos] = buf.contexts[oldPos]
	}
	buf.terms = terms
	buf.weights = weights
	buf.payloads = payloads
	buf.contexts = contexts
	buf.idx = -1
	return &SortedInputIterator{BufferedInputIterator: buf}, nil
}

var _ InputIterator = (*SortedInputIterator)(nil)
