package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// KnnVectorValues abstracts addressing of document vector values. It is the
// Go port of org.apache.lucene.index.KnnVectorValues from Apache Lucene
// 10.5.0.
//
// Lucene declares the class once; spi carries the declaration because
// spi.LeafReader and spi.KnnVectorsReader must name it, so the index spelling
// is an alias.
type KnnVectorValues = spi.KnnVectorValues

// DocIndexIterator is a DocIdSetIterator that also provides an Index method
// tracking a distinct ordinal for a vector associated with each doc. It is the
// Go port of the nested class
// org.apache.lucene.index.KnnVectorValues.DocIndexIterator, declared in spi
// alongside [KnnVectorValues].
type DocIndexIterator = spi.DocIndexIterator
