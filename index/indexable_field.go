// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// IndexableField represents a single field for indexing. IndexWriter consumes
// Iterable<IndexableField> as a document.
//
// Mirrors org.apache.lucene.index.IndexableField from Apache Lucene 10.5.0.
type IndexableField interface {
	// Name returns the field name.
	Name() string

	// FieldType returns the spi.IndexableFieldType describing the properties of this field.
	FieldType() spi.IndexableFieldType

	// TokenStream creates the TokenStream used for indexing this field.
	// If appropriate, implementations should use the given analyzer to create the TokenStreams.
	//
	// analyzer: Analyzer that should be used to create the TokenStreams from.
	// reuse: TokenStream for a previous instance of this field name. This allows custom
	// field types (like StringField and NumericField) that do not use the analyzer to still have
	// good performance. Note: the passed-in type may be inappropriate, for example if you mix up
	// different types of Fields for the same field name. So it's the responsibility of the
	// implementation to check.
	//
	// Returns TokenStream value for indexing the document. Should always return a non-null
	// value if the field is to be indexed.
	TokenStream(analyzer analysis.Analyzer, reuse analysis.TokenStream) analysis.TokenStream

	// BinaryValue returns the binary value of this field.
	// Returns non-null if this field has a binary value.
	BinaryValue() []byte

	// StringValue returns the string value of this field.
	// Returns non-null if this field has a string value.
	StringValue() string

	// ReaderValue returns the Reader value of this field.
	// Returns non-null if this field has a Reader value.
	ReaderValue() io.Reader

	// NumericValue returns the numeric value of this field.
	// Returns non-null if this field has a numeric value.
	NumericValue() any

	// StoredValue returns the stored value of this field.
	// This method is called to populate stored fields and must return a non-null
	// value if the field is stored.
	StoredValue() StoredValue

	// InvertableType describes how this field should be inverted.
	// This must return a non-null value if the field indexes terms and postings.
	InvertableType() InvertableType
}
