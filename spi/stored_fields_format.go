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
// Mirrors org.apache.lucene.index.StoredFieldVisitor of Apache Lucene 10.5.0
// (StoredFieldVisitor.java:36-91).
type StoredFieldVisitor interface {
	// BinaryField processes a binary field.
	//
	// Mirrors binaryField(FieldInfo, byte[]) (StoredFieldVisitor.java:62).
	BinaryField(fieldInfo *FieldInfo, value []byte) error

	// StringField processes a string field.
	//
	// Mirrors stringField(FieldInfo, String) (StoredFieldVisitor.java:65).
	StringField(fieldInfo *FieldInfo, value string) error

	// IntField processes an int numeric field.
	//
	// Mirrors intField(FieldInfo, int) (StoredFieldVisitor.java:68).
	IntField(fieldInfo *FieldInfo, value int) error

	// LongField processes a long numeric field.
	//
	// Mirrors longField(FieldInfo, long) (StoredFieldVisitor.java:71).
	LongField(fieldInfo *FieldInfo, value int64) error

	// FloatField processes a float numeric field.
	//
	// Mirrors floatField(FieldInfo, float) (StoredFieldVisitor.java:74).
	FloatField(fieldInfo *FieldInfo, value float32) error

	// DoubleField processes a double numeric field.
	//
	// Mirrors doubleField(FieldInfo, double) (StoredFieldVisitor.java:77).
	DoubleField(fieldInfo *FieldInfo, value float64) error

	// NeedsField is the hook invoked before a field is processed, so that
	// implementations can state whether they need that particular field, or
	// that processing should stop entirely.
	//
	// Mirrors the abstract needsField(FieldInfo) (StoredFieldVisitor.java:84).
	NeedsField(fieldInfo *FieldInfo) (StoredFieldVisitorStatus, error)
}

// StoredFieldVisitorStatus enumerates the possible return values of
// StoredFieldVisitor.NeedsField. It is the Go port of the nested enum
// org.apache.lucene.index.StoredFieldVisitor.Status (StoredFieldVisitor.java:87-94);
// the Go name carries the enclosing class because the Java simple name
// (Status) is shared by several unrelated Lucene nested types.
type StoredFieldVisitorStatus int

const (
	// StoredFieldVisitorStatusYes — the field should be visited.
	StoredFieldVisitorStatusYes StoredFieldVisitorStatus = iota
	// StoredFieldVisitorStatusNo — don't visit this field, but continue
	// processing fields for this document.
	StoredFieldVisitorStatusNo
	// StoredFieldVisitorStatusStop — don't visit this field and stop
	// processing any other fields for this document.
	StoredFieldVisitorStatusStop
)
