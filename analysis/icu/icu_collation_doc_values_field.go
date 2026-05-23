// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"github.com/FlavioCFOliveira/Gocene/analysis/icu/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ICUCollationDocValuesField indexes collation keys as a single-valued
// SortedDocValuesField.
//
// Go port of org.apache.lucene.analysis.icu.ICUCollationDocValuesField
// (Apache Lucene 10.4.0).
//
// This is more efficient than ICUCollationKeyAnalyzer if the field has only
// one value: no uninversion is necessary to sort on the field, and the
// underlying data structure built at index time is likely more efficient.
//
// Deviation: The Java original clones the Collator at construction time to
// ensure thread safety. In Go the Collator is an interface; callers are
// responsible for providing a thread-safe implementation (or cloning it
// themselves if needed).
type ICUCollationDocValuesField struct {
	*document.SortedDocValuesField
	name     string
	collator tokenattributes.Collator
	bytes    *util.BytesRef
}

// NewICUCollationDocValuesField creates a new ICUCollationDocValuesField.
//
// NOTE: do not create a new one for each document; instead create one and
// reuse it during indexing, updating the value via SetStringValue.
func NewICUCollationDocValuesField(name string, collator tokenattributes.Collator) (*ICUCollationDocValuesField, error) {
	f := &ICUCollationDocValuesField{
		name:     name,
		collator: collator,
		bytes:    &util.BytesRef{},
	}
	// Create an initial SortedDocValuesField with an empty value.
	sdvf, err := document.NewSortedDocValuesField(name, []byte{})
	if err != nil {
		return nil, err
	}
	f.SortedDocValuesField = sdvf
	return f, nil
}

// Name returns the field name.
func (f *ICUCollationDocValuesField) Name() string { return f.name }

// SetStringValue computes the collation key for value and updates the
// underlying SortedDocValuesField.
func (f *ICUCollationDocValuesField) SetStringValue(value string) error {
	key := f.collator.GetRawCollationKey(value)
	f.bytes.Bytes = key
	f.bytes.Offset = 0
	f.bytes.Length = len(key)
	// Update the underlying field value.
	sdvf, err := document.NewSortedDocValuesField(f.name, key)
	if err != nil {
		return err
	}
	f.SortedDocValuesField = sdvf
	return nil
}
