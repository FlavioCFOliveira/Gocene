package index

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// KnnVectorValues abstracts addressing of document vector values.
type KnnVectorValues interface {
	// Dimension returns the dimension of the vectors.
	Dimension() int

	// Size returns the number of vectors for this field.
	Size() int

	// OrdToDoc returns the docid of the document indexed with the given vector ordinal.
	OrdToDoc(ord int) int

	// Prefetch prefetches the provided ordinals.
	Prefetch(ordsToPrefetch []int, numOrds int) error

	// Copy creates a new copy of this KnnVectorValues.
	Copy() (KnnVectorValues, error)

	// GetEncoding returns the vector encoding of these values.
	GetEncoding() VectorEncoding

	// Iterator creates an iterator for this instance.
	Iterator() search.DocIndexIterator
}

// DocIndexIterator is a DocIdSetIterator that also provides an Index() method
// tracking a distinct ordinal for a vector associated with each doc.
type DocIndexIterator interface {
	search.DocIdSetIterator
	// Index returns the value index (aka "ordinal" or "ord") corresponding to the current doc.
	Index() int
}
