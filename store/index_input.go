package store

import "github.com/FlavioCFOliveira/Gocene/spi"

// RandomAccess is a port of org.apache.lucene.store.RandomAccess.
type RandomAccess = spi.RandomAccess

// Slicable provides the ability to create subsets of an IndexInput.
type Slicable = spi.Slicable

// Closable provides the ability to release resources.
type Closable = spi.Closable

// Cloneable provides the ability to create independent copies.
type Cloneable = spi.Cloneable

// IndexInput provides random access to an index file.
type IndexInput = spi.IndexInput

// VariableLengthInput provides methods for reading variable-length encoded data.
type VariableLengthInput = spi.VariableLengthInput

// BufferedInput provides buffer management operations for buffered IndexInput implementations.
type BufferedInput = spi.BufferedInput

// RandomAccessInput provides random access to read primitive types from an
// IndexInput.
type RandomAccessInput = spi.RandomAccessInput

// PrefetchableRandomAccessInput is an optional capability that a
// RandomAccessInput may implement to advertise prefetching support.
type PrefetchableRandomAccessInput = spi.PrefetchableRandomAccessInput

// LoadedReporterRandomAccessInput is an optional capability that a
// RandomAccessInput may implement to expose whether its data is currently
// resident in physical memory.
type LoadedReporterRandomAccessInput = spi.LoadedReporterRandomAccessInput

// BaseIndexInput is the base implementation of IndexInput.
type BaseIndexInput = spi.BaseIndexInput

// NewBaseIndexInput creates a new BaseIndexInput.
func NewBaseIndexInput(desc string, length int64) *BaseIndexInput {
	return spi.NewBaseIndexInput(desc, length)
}
