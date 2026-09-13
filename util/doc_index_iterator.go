package util

// DocIndexIterator is a DocIdSetIterator that also provides an Index() method
// tracking a distinct ordinal for a vector associated with each doc.
type DocIndexIterator interface {
	DocIdSetIterator
	// Index returns the value index (aka "ordinal" or "ord") corresponding to the current doc.
	Index() int
}

type denseDocIndexIterator struct {
	n   int
	idx int
}

// NewDenseDocIndexIterator returns a DocIndexIterator that iterates over a dense
// identity range [0, n).
func NewDenseDocIndexIterator(n int) DocIndexIterator {
	return &denseDocIndexIterator{n: n, idx: -1}
}

func (it *denseDocIndexIterator) DocID() int { return it.idx }

func (it *denseDocIndexIterator) NextDoc() (int, error) {
	it.idx++
	if it.idx >= it.n {
		it.idx = NO_MORE_DOCS
		return it.idx, nil
	}
	return it.idx, nil
}

func (it *denseDocIndexIterator) Advance(target int) (int, error) {
	if target > it.idx {
		it.idx = target
	}
	if it.idx >= it.n {
		it.idx = NO_MORE_DOCS
	}
	return it.idx, nil
}

func (it *denseDocIndexIterator) DocIDRunEnd() (int, error) {
	return it.n, nil
}

func (it *denseDocIndexIterator) Cost() int64 { return 1 }

func (it *denseDocIndexIterator) Index() int { return it.idx }

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (it *denseDocIndexIterator) IntoBitSet(upTo int, bitSet *FixedBitSet, offset int) error {
	return DefaultIntoBitSet(it, upTo, bitSet, offset)
}
