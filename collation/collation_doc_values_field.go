// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
// analysis/common/src/java/org/apache/lucene/collation/CollationDocValuesField.java

package collation

import (
	"github.com/FlavioCFOliveira/Gocene/collation/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/document"
)

// CollationDocValuesField is the Go port of
// org.apache.lucene.collation.CollationDocValuesField.
//
// Indexes collation keys as a single-valued SortedDocValuesField.
//
// This is more efficient than CollationKeyAnalyzer if the field only has
// one value: no uninversion is necessary to sort on the field,
// locale-sensitive range queries can work via DocValuesRangeQuery, and
// the underlying data structures built at index-time are likely more
// efficient than FieldCache.
//
// NOTE: You should not create a new CollationDocValuesField for each
// document — reuse a single instance and call SetStringValue for each
// document, matching the Java recommendation.
type CollationDocValuesField struct {
	*document.SortedDocValuesField
	name     string
	collator tokenattributes.Collator
}

// NewCollationDocValuesField creates a new CollationDocValuesField.
//
// The Collator should be concurrency-safe or used from a single goroutine.
// (The Java version clones Collator in the constructor to avoid contention.)
func NewCollationDocValuesField(name string, collator tokenattributes.Collator) (*CollationDocValuesField, error) {
	// Start with empty bytes; SetStringValue fills them in.
	sdvf, err := document.NewSortedDocValuesField(name, []byte{})
	if err != nil {
		return nil, err
	}
	f := &CollationDocValuesField{
		SortedDocValuesField: sdvf,
		name:                 name,
		collator:             collator,
	}
	return f, nil
}

// Name returns the field name.
func (f *CollationDocValuesField) Name() string {
	return f.name
}

// SetStringValue encodes the string as collation-key bytes and stores
// them as the binary value of this SortedDocValuesField. This mirrors
// the Java override of Field.setStringValue.
func (f *CollationDocValuesField) SetStringValue(value string) {
	key := f.collator.CollationKey(value)
	// Replace the underlying binary value in the embedded Field.
	// document.SortedDocValuesField embeds *document.Field; we reach
	// into it via SetBinaryValue if present, or via direct mutation.
	f.SortedDocValuesField.Field.SetBinaryValue(key)
}
