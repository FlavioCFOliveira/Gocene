// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// TermVectorsFormat encodes and decodes the per-segment term-vector
// files (.tvd / .tvx / .tvm in Lucene 10.4.0).
//
// Mirrors org.apache.lucene.codecs.TermVectorsFormat.
type TermVectorsFormat interface {
	// Name returns the codec name embedded in segment metadata.
	Name() string

	// VectorsReader opens a reader over the per-segment term-vector
	// files. The caller closes the reader when done.
	//
	// Mirrors TermVectorsFormat.vectorsReader(Directory, SegmentInfo,
	// FieldInfos, IOContext) (TermVectorsFormat.java:30-32).
	VectorsReader(directory Directory, segmentInfo *SegmentInfo, fieldInfos *FieldInfos, context IOContext) (TermVectorsReader, error)

	// VectorsWriter opens a writer that produces the per-segment
	// term-vector files. The caller closes the writer when done.
	//
	// Mirrors TermVectorsFormat.vectorsWriter(Directory, SegmentInfo,
	// IOContext) (TermVectorsFormat.java:35-36).
	VectorsWriter(directory Directory, segmentInfo *SegmentInfo, context IOContext) (TermVectorsWriter, error)
}

// TermVectorsWriter serialises term vectors document by document,
// field by field, term by term.
//
// Mirrors org.apache.lucene.codecs.TermVectorsWriter.
type TermVectorsWriter interface {
	// StartDocument signals the beginning of a new document; numFields
	// is the number of fields with term vectors in this document.
	StartDocument(numFields int) error

	// StartField signals the beginning of a new field within the current
	// document; the flags describe what per-position data follows.
	StartField(fieldInfo *FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error

	// StartTerm signals a new term in the current field.
	StartTerm(term []byte, freq int) error

	// AddPosition adds one occurrence of the current term.
	AddPosition(position int, startOffset, endOffset int, payload []byte) error

	// FinishTerm closes the current term.
	FinishTerm() error

	// FinishField closes the current field.
	FinishField() error

	// FinishDocument closes the current document.
	FinishDocument() error

	// Finish is called before Close(), passing in the number of documents written.
	Finish(numDocs int) error

	// Close releases any resources held by the writer.
	Close() error
}

// TermVectorsReader exposes per-document term-vector access.
//
// Mirrors org.apache.lucene.codecs.TermVectorsReader.
type TermVectorsReader interface {
	// Get returns the Fields enumeration for the document at docID, or
	// an empty Fields when the document has no term vectors.
	Get(docID int) (Fields, error)

	// GetField returns the Terms enumeration for the named field at
	// docID, or nil when no term vector exists for that field.
	GetField(docID int, field string) (Terms, error)

	// CheckIntegrity walks the term-vector data and validates the checksum
	// framing.
	CheckIntegrity() error

	// Close releases any resources held by the reader.
	Close() error
}
