// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// IndexableField is the narrow, codec-facing contract for fields that the
// stored-fields write path serialises.
//
// # Divergence from Apache Lucene 10.4.0
//
// Apache Lucene 10.4.0's org.apache.lucene.index.IndexableField is a wider
// interface that also carries TokenStream(Analyzer, TokenStream),
// ReaderValue(), StoredValue(), InvertableType(), and FieldType()
// accessors. Gocene's codec stored-fields writers (Lucene104,
// Lucene90Compressing) only consume Name, StringValue, BinaryValue and
// NumericValue when producing the .fdt/.fdx pair. Restricting the SPI
// surface to those four methods keeps spi/ a leaf package — it does not
// need to know about the document or analysis layers — while still
// accepting every concrete field type implemented in package document
// without conversion (document.IndexableField is a structural superset
// of spi.IndexableField, so values satisfy this interface implicitly).
//
// If a future codec needs richer field metadata, the corresponding
// accessors should be added here first and then implemented by the
// document side.
type IndexableField interface {
	// Name returns the name of the field.
	Name() string

	// StringValue returns the string value of the field, or "" if the
	// field carries no string payload.
	StringValue() string

	// BinaryValue returns the binary value of the field, or nil if the
	// field carries no binary payload.
	BinaryValue() []byte

	// NumericValue returns the numeric value of the field, or nil if the
	// field carries no numeric payload. Concrete dynamic types include
	// int, int32, int64, float32 and float64.
	NumericValue() interface{}
}
