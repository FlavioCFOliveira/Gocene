package search

import (
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
	if bs, ok := b.bits.(util.BitSet); ok {
		return util.NewBitSetIterator(bs, int64(b.maxDoc)), nil
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

	// If we already have a BitSet and no deletions, reuse the BitSet.
	if d.liveDocs == nil {
		// Gocene's BitSetIterator stores the narrower util.Bits, so Java's
		// bitSetIterator.getBitSet() is rendered as an assertion back to BitSet.
		if bsi, ok := it.(*util.BitSetIterator); ok {
			if bs, isBitSet := bsi.GetBitSet().(util.BitSet); isBitSet {
				d.acceptBitSet = bs
				d.cardinality = bs.Cardinality()
				return nil
			}
		}
	}

	threshold := d.maxDoc >> 7 // same as BitSet#of
	if it.Cost() >= int64(threshold) {
		// take advantage of Disi#intoBitset and Bits#applyMask
		bitSet, err := util.NewFixedBitSet(d.maxDoc)
		if err != nil {
			return err
		}
		if err := bitSet.OrIterator(it); err != nil {
			return err
		}
		if d.liveDocs != nil {
			util.ApplyMask(d.liveDocs, bitSet, 0)
		}
		d.acceptBitSet = bitSet
	} else {
		// create a sparse bitset
		bitSet, err := util.OfDocIdSetIterator(getFilteredDocIdSetIterator(it, d.liveDocs), d.maxDoc)
		if err != nil {
			return err
		}
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
		return util.NewBitSetIterator(d.acceptBitSet, int64(d.cardinality)), nil
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

// IntoBitSet mirrors the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0
// (DocIdSetIterator.java:205-210):
//
//	for (int doc = docID(); doc < upTo; doc = nextDoc()) bitSet.set(doc - offset);
//
// The walk must go through this iterator's own NextDoc so that the liveDocs
// filter is applied, which is why it is driven off f rather than f.it.
func (f *filteredDocIdSetIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return DefaultIntoBitSet(f, upTo, bitSet, offset)
}

func (f *filteredDocIdSetIterator) DocIDRunEnd() (int, error) {
	return f.it.DocIDRunEnd()
}

// BitSetIterator is declared by org.apache.lucene.util.BitSetIterator and lives
// in util/bit_set_iterator.go; the stub that used to sit here had no Lucene
// counterpart and returned nil.
