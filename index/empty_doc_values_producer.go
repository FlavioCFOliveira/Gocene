// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "fmt"

// EmptyDocValuesProducer is a DocValuesProducer that returns "not supported"
// for every accessor. It is used by tests and by formats that have no doc
// values to expose. Mirrors
// org.apache.lucene.index.EmptyDocValuesProducer from Apache Lucene 10.4.0.
//
// Gocene models DocValuesProducer-shaped surfaces in the codecs package
// (codecs.DocValuesProducer). This type provides the index-package version
// used by EmptyDocValuesProducer's typical callers, returning a sentinel
// "unsupported" error rather than panicking.
type EmptyDocValuesProducer struct{}

// NewEmptyDocValuesProducer returns the canonical empty producer.
func NewEmptyDocValuesProducer() *EmptyDocValuesProducer { return &EmptyDocValuesProducer{} }

// errUnsupportedEmptyDV is the sentinel error returned by every accessor.
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
func (EmptyDocValuesProducer) GetSkipper(_ *FieldInfo) (DocValuesSkipper, error) {
	return nil, errUnsupportedEmptyDV
}

// CheckIntegrity returns the unsupported error.
func (EmptyDocValuesProducer) CheckIntegrity() error { return errUnsupportedEmptyDV }

// Close returns the unsupported error.
func (EmptyDocValuesProducer) Close() error { return errUnsupportedEmptyDV }
