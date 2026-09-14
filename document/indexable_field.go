// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// IndexableField represents a single field for indexing. IndexWriter consumes
// a sequence of IndexableField as a document.
//
// This is the Go port of org.apache.lucene.index.IndexableField from Apache
// Lucene 10.5.0, and it carries Lucene's member set exactly.
//
// # Why this interface lives in package document
//
// Java declares IndexableField in org.apache.lucene.index, and that is also
// where package index's alias spells it (index.IndexableField). The
// declaration itself has to sit here because Java's IndexableField sits on
// both sides of a package cycle that Go forbids:
//
//   - org.apache.lucene.index.IndexableField imports
//     org.apache.lucene.document.StoredValue and
//     org.apache.lucene.document.InvertableType, which appear in its signature;
//   - org.apache.lucene.document.Document imports
//     org.apache.lucene.index.IndexableField, the element type it holds.
//
// Java permits that cycle between packages; Go does not. Package document is
// the only Gocene package that can name every type in Lucene's member set
// (analysis.Analyzer and analysis.TokenStream, spi.IndexableFieldType,
// StoredValue and InvertableType) without importing package index, so the one
// declaration lives here and package index aliases it. Both Lucene spellings
// therefore resolve to a single type with a single member set.
type IndexableField interface {
	// Name returns the field name.
	Name() string

	// FieldType returns the IndexableFieldType describing the properties of
	// this field.
	FieldType() spi.IndexableFieldType

	// TokenStream creates the TokenStream used for indexing this field. If
	// appropriate, implementations should use the given analyzer to create the
	// TokenStreams.
	//
	// analyzer is the Analyzer that should be used to create the TokenStreams
	// from. reuse is the TokenStream for a previous instance of this field
	// name; this allows custom field types (like StringField and NumericField)
	// that do not use the analyzer to still have good performance. Note: the
	// passed-in type may be inappropriate, for example if you mix up different
	// types of Fields for the same field name, so it is the responsibility of
	// the implementation to check.
	//
	// Returns the TokenStream value for indexing the document. Should always
	// return a non-nil value if the field is to be indexed.
	TokenStream(analyzer analysis.Analyzer, reuse analysis.TokenStream) analysis.TokenStream

	// BinaryValue returns a non-nil value if this field has a binary value.
	BinaryValue() []byte

	// StringValue returns a non-empty value if this field has a string value.
	StringValue() string

	// GetCharSequenceValue returns a non-empty value if this field has a
	// string value. Java declares this as a default method returning
	// stringValue(); Go interfaces carry no default bodies, so every
	// implementation states it.
	GetCharSequenceValue() string

	// ReaderValue returns a non-nil value if this field has a Reader value.
	ReaderValue() io.Reader

	// NumericValue returns a non-nil value if this field has a numeric value.
	NumericValue() interface{}

	// StoredValue returns the stored value. This method is called to populate
	// stored fields and must return a non-nil value if the field is stored.
	StoredValue() *StoredValue

	// InvertableType describes how this field should be inverted. This must
	// return a meaningful value if the field indexes terms and postings.
	InvertableType() InvertableType
}
