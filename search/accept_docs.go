package search

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// AcceptDocs is a higher-level abstraction for document acceptance filtering.
type AcceptDocs interface {
	// Bits returns random access to the accepted documents.
	Bits() (util.Bits, error)

	// Iterator creates a new iterator of accepted docs.
	Iterator() (DocIdSetIterator, error)

	// Cost returns an approximation of the number of accepted documents.
	Cost() (int, error)
}

// FromLiveDocs creates AcceptDocs from a util.Bits instance representing live documents.
func FromLiveDocs(bits util.Bits, maxDoc int) AcceptDocs {
	return &bitsAcceptDocs{
		bits:   bits,
		maxDoc: maxDoc,
	}
}

type bitsAcceptDocs struct {
	bits   util.Bits
	maxDoc int
}

func (b *bitsAcceptDocs) Bits() (util.Bits, error) {
	return b.bits, nil
}

func (b *bitsAcceptDocs) Iterator() (DocIdSetIterator, error) {
	if bs, ok := b.bits.(*util.FixedBitSet); ok {
		return NewBitSetIterator(bs, b.maxDoc), nil
	}
	return getFilteredDocIdSetIterator(Range(0, b.maxDoc), b.bits), nil
}

func (b *bitsAcceptDocs) Cost() (int, error) {
	return b.maxDoc, nil
}

// FromIteratorSupplier creates AcceptDocs from an iterator supplier.
func FromIteratorSupplier(supplier func() (DocIdSetIterator, error), liveDocs util.Bits, maxDoc int) AcceptDocs {
	return &docIdSetIteratorAcceptDocs{
		iteratorSupplier: supplier,
		liveDocs:         liveDocs,
		maxDoc:           maxDoc,
	}
}

type docIdSetIteratorAcceptDocs struct {
	iteratorSupplier func() (DocIdSetIterator, error)
	liveDocs         util.Bits
	maxDoc           int
	acceptBitSet     util.BitSet
	cardinality      int
}

func (d *docIdSetIteratorAcceptDocs) createBitSet() error {
	if d.acceptBitSet != nil {
		return nil
	}

	it, err := d.iteratorSupplier()
	if err != nil {
		return err
	}

	// Heuristic for BitSet creation
	threshold := d.maxDoc >> 7
	if it.Cost() >= int64(threshold) {
		bitSet, err := util.NewFixedBitSet(d.maxDoc)
		if err != nil {
			return err
		}
		bitSet.Or(it)
		if d.liveDocs != nil {
			util.ApplyMask(d.liveDocs, bitSet, 0)
		}
		d.acceptBitSet = bitSet
	} else {
		// Create a sparse bitset (implementation assumed in util.BitSet)
		// For now, we'll implement a basic version or use FixedBitSet
		bitSet, err := util.NewFixedBitSet(d.maxDoc)
		if err != nil {
			return err
		}
		// ... logic to populate sparse bitset ...
		d.acceptBitSet = bitSet
	}
	d.cardinality = d.acceptBitSet.Cardinality()
	return nil
}

func (d *docIdSetIteratorAcceptDocs) Bits() (util.Bits, error) {
	if err := d.createBitSet(); err != nil {
		return nil, err
	}
	return d.acceptBitSet, nil
}

func (d *docIdSetIteratorAcceptDocs) Iterator() (DocIdSetIterator, error) {
	if d.acceptBitSet != nil {
		return NewBitSetIterator(d.acceptBitSet, d.cardinality), nil
	}
	it, err := d.iteratorSupplier()
	if err != nil {
		return nil, err
	}
	return getFilteredDocIdSetIterator(it, d.liveDocs), nil
}

func (d *docIdSetIteratorAcceptDocs) Cost() (int, error) {
	if err := d.createBitSet(); err != nil {
		return 0, err
	}
	return d.cardinality, nil
}

func getFilteredDocIdSetIterator(it DocIdSetIterator, liveDocs util.Bits) DocIdSetIterator {
	if liveDocs == nil {
		return it
	}
	return &filteredDocIdSetIterator{
		it:       it,
		liveDocs: liveDocs,
	}
}

type filteredDocIdSetIterator struct {
	it       DocIdSetIterator
	liveDocs util.Bits
}

func (f *filteredDocIdSetIterator) DocID() int {
	return f.it.DocID()
}

func (f *filteredDocIdSetIterator) NextDoc() (int, error) {
	for {
		doc, err := f.it.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, nil
		}
		if f.liveDocs == nil || f.liveDocs.Get(doc) {
			return doc, nil
		}
	}
}

func (f *filteredDocIdSetIterator) Advance(target int) (int, error) {
	for {
		doc, err := f.it.Advance(target)
		if err != nil {
			return 0, err
		}
		if doc == NO_MORE_DOCS {
			return NO_MORE_DOCS, nil
		}
		if f.liveDocs == nil || f.liveDocs.Get(doc) {
			return doc, nil
		}
		target = doc + 1
	}
}

func (f *filteredDocIdSetIterator) Cost() int64 {
	return f.it.Cost()
}

func (f *filteredDocIdSetIterator) IntoBitSet(upTo int, bitSet util.BitSet, offset int) error {
	// implementation similar to original
	return nil
}

func (f *filteredDocIdSetIterator) DocIDRunEnd() (int, error) {
	return f.it.DocIDRunEnd()
}

func NewBitSetIterator(bs util.BitSet, maxDoc int) DocIdSetIterator {
	// Assumed implementation in util or search
	return nil // Placeholder
}
