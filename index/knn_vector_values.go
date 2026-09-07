package index

import (
	"github.com/FlavioCFOliveira/Gocene/util"
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

	// GetVectorByteLength returns the vector byte length.
	GetVectorByteLength() int

	// GetAcceptOrds returns a Bits accepting docs accepted by the argument and having a vector value.
	GetAcceptOrds(acceptDocs util.Bits) util.Bits

	// Iterator creates an iterator for this instance.
	Iterator() util.DocIndexIterator
}
