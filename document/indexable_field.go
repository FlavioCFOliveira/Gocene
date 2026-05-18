// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"io"
)

// IndexableField is the interface implemented by all field types that can
// be added to a Document.
//
// This is the Go port of Lucene's org.apache.lucene.index.IndexableField.
//
// Architectural note: Gocene exposes two complementary IndexableField
// interfaces by design:
//
//   - document.IndexableField (this file) is the document-facing surface
//     and carries the full document-side API including ReaderValue.
//   - index.IndexableField is the codec-facing surface, a narrower
//     interface accepted by SegmentWriteState/StoredFieldsWriter chains
//     (see index/codec_interface.go for the full divergence note).
//
// Every concrete field type implements both because every method on
// index.IndexableField is also present on document.IndexableField (with
// a compatible signature once FieldType is dispatched through
// FieldTypeInterface). Callers driving the codec layer use the index
// alias; callers building documents use this one.
type IndexableField interface {
	// Name returns the name of the field.
	Name() string

	// FieldType returns the FieldType for this field.
	// The FieldType describes how the field should be indexed and stored.
	FieldType() *FieldType

	// StringValue returns the string value of the field.
	// Returns empty string if the field has no string value.
	StringValue() string

	// ReaderValue returns a reader for the field value.
	// Returns nil if the field has no reader value.
	ReaderValue() io.Reader

	// BinaryValue returns the binary value of the field.
	// Returns nil if the field has no binary value.
	BinaryValue() []byte

	// NumericValue returns the numeric value of the field.
	// The interface{} can be int, int64, float32, or float64.
	// Returns nil if the field has no numeric value.
	NumericValue() interface{}

	// TokenStream returns a TokenStream for the field value.
	// This is used during indexing to analyze the field content.
	// Returns nil if the field is not tokenized.
	// TokenStream() analysis.TokenStream
}
