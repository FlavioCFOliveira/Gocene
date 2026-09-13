// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi


// StoredFieldsFormat encodes and decodes the per-document stored field
// pair (.fdt / .fdx) for a segment.
//
// Mirrors org.apache.lucene.codecs.StoredFieldsFormat in Apache Lucene
// 10.4.0.
type StoredFieldsFormat interface {
	// Name returns the codec name embedded in segment metadata.
	Name() string

	// FieldsReader opens a reader over the .fdt / .fdx pair. The caller
	// closes the returned reader when done.
	FieldsReader(dir Directory, segmentInfo *SegmentInfo, fieldInfos *FieldInfos, context IOContext) (StoredFieldsReader, error)

	// FieldsWriter opens a writer that produces the .fdt / .fdx pair.
	// The caller closes the returned writer when done.
	FieldsWriter(dir Directory, segmentInfo *SegmentInfo, context IOContext) (StoredFieldsWriter, error)
}

// StoredFieldsReader iterates over the stored fields of one segment
// document by document.
//
// Mirrors org.apache.lucene.codecs.StoredFieldsReader.
type StoredFieldsReader interface {
	// VisitDocument invokes the visitor for every stored field of the
	// document at docID.
	VisitDocument(docID int, visitor StoredFieldVisitor) error

	// CheckIntegrity walks the stored-field data and validates the checksum
	// framing.
	CheckIntegrity() error

	// Close releases any resources held by the reader.
	Close() error
}

// StoredFieldsWriter serialises the stored fields of one segment one
// document at a time.
//
// Mirrors org.apache.lucene.codecs.StoredFieldsWriter.
type StoredFieldsWriter interface {
	// StartDocument signals the beginning of a new document.
	StartDocument() error

	// FinishDocument signals the end of the current document.
	FinishDocument() error

	// WriteField serialises one stored field of the current document.
	// info carries the field number the codec stamps into the serialized
	// record; the value is exposed via the narrow spi.IndexableField
	// interface, which every concrete field type implemented by package
	// document satisfies implicitly.
	//
	// Mirrors the org.apache.lucene.codecs.StoredFieldsWriter.writeField
	// overload family (StoredFieldsWriter.java:63-87), every member of
	// which takes the FieldInfo as its first argument. Java dispatches on
	// the static type of the value; Go has no overloading, so the port
	// carries the value behind IndexableField and dispatches on its
	// StoredValue.
	WriteField(info *FieldInfo, field IndexableField) error

	// Finish finalises the segment after numDocs documents have been
	// written. Mirrors codecs.StoredFieldsWriter.finish.
	Finish(numDocs int) error

	// Close releases any resources held by the writer.
	Close() error
}

// StoredFieldVisitor receives one callback per stored field while a
// document is decoded.
//
// Mirrors org.apache.lucene.index.StoredFieldVisitor.
type StoredFieldVisitor interface {
	// StringField is invoked for a stored string field.
	StringField(field string, value string)

	// BinaryField is invoked for a stored binary field.
	BinaryField(field string, value []byte)

	// IntField is invoked for a stored 32-bit integer field.
	IntField(field string, value int)

	// LongField is invoked for a stored 64-bit integer field.
	LongField(field string, value int64)

	// FloatField is invoked for a stored 32-bit float field.
	FloatField(field string, value float32)

	// DoubleField is invoked for a stored 64-bit float field.
	DoubleField(field string, value float64)
}
