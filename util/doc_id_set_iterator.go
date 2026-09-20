package util

import (
	"math"
)

const NO_MORE_DOCS = math.MaxInt32

// DocIdSetIterator defines methods to iterate over a set of non-decreasing doc
// IDs. It is the Go rendering of the abstract class
// org.apache.lucene.search.DocIdSetIterator (Apache Lucene 10.5.0).
//
// It is declared in util rather than in search because util types that Lucene
// also expresses in terms of this class — notably BitSet.or(DocIdSetIterator)
// and BitSet.of(DocIdSetIterator, int) — must refer to it, and util cannot
// import search or spi without creating an import cycle. The search package
// re-exports it under the Java name via a type alias, so search.DocIdSetIterator
// remains the spelling Lucene uses; spi does the same.
//
// The member set below is Java's in full: docID(), nextDoc(int), advance(int)
// and cost() are abstract; intoBitSet(int, FixedBitSet, int) and docIDRunEnd()
// are concrete but overridable, and every Java subclass therefore has them.
// Go has no inherited default, so an implementation that does not override them
// delegates to DefaultIntoBitSet and DefaultDocIDRunEnd below, which carry the
// bodies of the Java base class verbatim.
type DocIdSetIterator interface {
	// DocID returns the current doc ID.
	// -1 if NextDoc() or Advance() were not called yet.
	// NO_MORE_DOCS if the iterator has exhausted.
	DocID() int

	// NextDoc advances to the next document in the set and returns the doc it is currently on.
	NextDoc() (int, error)

	// Advance advances to the first beyond the current whose document number is >= target.
	Advance(target int) (int, error)

	// Cost returns the estimated cost of this iterator.
	Cost() int64

	// IntoBitSet loads doc IDs into a FixedBitSet, shifted down by offset.
	// Mirrors DocIdSetIterator.intoBitSet(int, FixedBitSet, int).
	IntoBitSet(upTo int, bitSet *FixedBitSet, offset int) error

	// DocIDRunEnd returns the end of the run of consecutive doc IDs that match
	// this iterator and that contains the current docID, that is one plus the
	// last doc ID of the run. Mirrors DocIdSetIterator.docIDRunEnd().
	DocIDRunEnd() (int, error)
}

// DefaultIntoBitSet is the body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0:
//
//	for (int doc = docID(); doc < upTo; doc = nextDoc()) {
//	  bitSet.set(doc - offset);
//	}
//
// It is a free function rather than a method on an embeddable struct because
// the Java body dispatches back to the abstract docID() and nextDoc(), which an
// embedded Go struct cannot reach. A subclass with a cheaper implementation —
// as several Java ones have — declares its own IntoBitSet instead of calling
// this.
func DefaultIntoBitSet(it DocIdSetIterator, upTo int, bitSet *FixedBitSet, offset int) error {
	for doc := it.DocID(); doc < upTo; {
		bitSet.Set(doc - offset)
		next, err := it.NextDoc()
		if err != nil {
			return err
		}
		doc = next
	}
	return nil
}

// DefaultDocIDRunEnd is the body of DocIdSetIterator.docIDRunEnd() in Apache
// Lucene 10.5.0, whose default implementation assumes runs of a single doc ID
// and returns docID() + 1. It is a free function for the same reason as
// DefaultIntoBitSet.
func DefaultDocIDRunEnd(it DocIdSetIterator) (int, error) {
	return it.DocID() + 1, nil
}

// SlowAdvance mirrors the protected final helper
// DocIdSetIterator.slowAdvance(int): a linear implementation of advance that
// relies on nextDoc() to advance beyond the target position.
func SlowAdvance(it DocIdSetIterator, target int) (int, error) {
	for {
		doc, err := it.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc >= target {
			return doc, nil
		}
	}
}
