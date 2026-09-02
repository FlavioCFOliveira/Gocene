//go:build ignore

package index

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocValuesWriter is a port of org.apache.lucene.index.DocValuesWriter.
// It provides a common interface for writing doc values to a segment.
type DocValuesWriter[T util.DocIdSetIterator] interface {
	// Flush writes the doc values to the consumer.
	Flush(state *spi.SegmentWriteState, sortMap spi.SorterDocMap, consumer spi.DocValuesConsumer) error
	// GetDocValues returns the doc values as a DocIdSetIterator.
	GetDocValues() T
}
