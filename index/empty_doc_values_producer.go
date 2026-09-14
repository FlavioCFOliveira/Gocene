// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
)

// EmptyDocValuesProducer is the abstract base class implementing a
// DocValuesProducer that has no doc values. Mirrors
// org.apache.lucene.index.EmptyDocValuesProducer from Apache Lucene 10.5.0.
//
// Every accessor of the Java class throws UnsupportedOperationException; the
// Go rendering returns errUnsupportedEmptyDV instead. Java subclasses it
// anonymously and overrides the single accessor they serve (for example the
// doc-values writers' flush and DocValuesConsumer's merge members); Go
// subclasses embed this struct and define that one method, together with
// GetMergeInstance so that the inherited default returns the subclass.
type EmptyDocValuesProducer struct{}

// NewEmptyDocValuesProducer returns the canonical empty producer.
func NewEmptyDocValuesProducer() *EmptyDocValuesProducer { return &EmptyDocValuesProducer{} }

// errUnsupportedEmptyDV is the rendering of the UnsupportedOperationException
// thrown by every accessor.
var errUnsupportedEmptyDV = fmt.Errorf("operation not supported on EmptyDocValuesProducer")

// GetNumeric returns the unsupported error.
func (EmptyDocValuesProducer) GetNumeric(_ *FieldInfo) (NumericDocValues, error) {
	return nil, errUnsupportedEmptyDV
}

// GetBinary returns the unsupported error.
func (EmptyDocValuesProducer) GetBinary(_ *FieldInfo) (BinaryDocValues, error) {
	return nil, errUnsupportedEmptyDV
}

// GetSorted returns the unsupported error.
func (EmptyDocValuesProducer) GetSorted(_ *FieldInfo) (SortedDocValues, error) {
	return nil, errUnsupportedEmptyDV
}

// GetSortedNumeric returns the unsupported error.
func (EmptyDocValuesProducer) GetSortedNumeric(_ *FieldInfo) (SortedNumericDocValues, error) {
	return nil, errUnsupportedEmptyDV
}

// GetSortedSet returns the unsupported error.
func (EmptyDocValuesProducer) GetSortedSet(_ *FieldInfo) (SortedSetDocValues, error) {
	return nil, errUnsupportedEmptyDV
}

// GetSkipper returns the unsupported error.
func (EmptyDocValuesProducer) GetSkipper(_ *FieldInfo) (spi.DocValuesSkipper, error) {
	return nil, errUnsupportedEmptyDV
}

// CheckIntegrity returns the unsupported error.
func (EmptyDocValuesProducer) CheckIntegrity() error { return errUnsupportedEmptyDV }

// GetMergeInstance returns the receiver: EmptyDocValuesProducer does not
// override DocValuesProducer.getMergeInstance(), whose default returns this.
func (p EmptyDocValuesProducer) GetMergeInstance() DocValuesProducer { return p }

// Close returns the unsupported error.
func (EmptyDocValuesProducer) Close() error { return errUnsupportedEmptyDV }
