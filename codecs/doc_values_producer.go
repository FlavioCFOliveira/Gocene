// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// DocValuesProducer is an abstract API that produces numeric, binary, sorted,
// sortedset, and sortednumeric docvalues.
//
// Mirrors org.apache.lucene.codecs.DocValuesProducer in Apache Lucene 10.5.0.
type DocValuesProducer interface {
	io.Closer

	// GetNumeric returns NumericDocValues for this field. The returned instance
	// need not be thread-safe: it will only be used by a single thread.
	// The behavior is undefined if the doc values type of the given field
	// is not DocValuesType.Numeric. The return value is never nil.
	GetNumeric(field index.FieldInfo) (spi.NumericDocValues, error)

	// GetBinary returns BinaryDocValues for this field. The returned instance
	// need not be thread-safe: it will only be used by a single thread.
	// The behavior is undefined if the doc values type of the given field
	// is not DocValuesType.Binary. The return value is never nil.
	GetBinary(field index.FieldInfo) (spi.BinaryDocValues, error)

	// GetSorted returns SortedDocValues for this field. The returned instance
	// need not be thread-safe: it will only be used by a single thread.
	// The behavior is undefined if the doc values type of the given field
	// is not DocValuesType.Sorted. The return value is never nil.
	GetSorted(field index.FieldInfo) (spi.SortedDocValues, error)

	// GetSortedNumeric returns SortedNumericDocValues for this field. The returned
	// instance need not be thread-safe: it will only be used by a single thread.
	// The behavior is undefined if the doc values type of the given field
	// is not DocValuesType.SortedNumeric. The return value is never nil.
	GetSortedNumeric(field index.FieldInfo) (spi.SortedNumericDocValues, error)

	// GetSortedSet returns SortedSetDocValues for this field. The returned instance
	// need not be thread-safe: it will only be used by a single thread.
	// The behavior is undefined if the doc values type of the given field
	// is not DocValuesType.SortedSet. The return value is never nil.
	GetSortedSet(field index.FieldInfo) (spi.SortedSetDocValues, error)

	// GetSkipper returns a DocValuesSkipper for this field. The returned instance
	// need not be thread-safe: it will only be used by a single thread.
	// The return value is undefined if field.DocValuesSkipIndexType() returns
	// DocValuesSkipIndexType.None.
	GetSkipper(field index.FieldInfo) (spi.DocValuesSkipper, error)

	// CheckIntegrity checks consistency of this producer.
	// Note that this may be costly in terms of I/O, e.g. may involve computing
	// a checksum value against large data files.
	CheckIntegrity() error

	// GetMergeInstance returns an instance optimized for merging. This instance
	// may only be consumed in the thread that called GetMergeInstance.
	// The default implementation in Lucene returns this.
	GetMergeInstance() DocValuesProducer
}
