// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TermVectorsFormat encodes and decodes the per-segment term-vector
// files (.tvd / .tvx / .tvm in Lucene 10.4.0).
//
// Mirrors org.apache.lucene.codecs.TermVectorsFormat.
type TermVectorsFormat interface {
	// Name returns the codec name embedded in segment metadata.
	Name() string

	// VectorsWriter opens a writer that produces the per-segment
	// term-vector files. The caller closes the writer when done.
	VectorsWriter(state *SegmentWriteState) (TermVectorsWriter, error)

	// VectorsReader opens a reader over the per-segment term-vector
	// files. The caller closes the reader when done.
	VectorsReader(dir store.Directory, segmentInfo *schema.SegmentInfo, fieldInfos *schema.FieldInfos, context store.IOContext) (TermVectorsReader, error)
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
	StartField(fieldInfo *schema.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error

	// StartTerm signals a new term in the current field.
	StartTerm(term []byte) error

	// AddPosition adds one occurrence of the current term.
	AddPosition(position int, startOffset, endOffset int, payload []byte) error

	// FinishTerm closes the current term.
	FinishTerm() error

	// FinishField closes the current field.
	FinishField() error

	// FinishDocument closes the current document.
	FinishDocument() error

	// Close releases any resources held by the writer.
	Close() error
}

// TermVectorsReader exposes per-document term-vector access.
//
// Mirrors org.apache.lucene.codecs.TermVectorsReader.
type TermVectorsReader interface {
	// Get returns the Fields enumeration for the document at docID, or
	// an empty Fields when the document has no term vectors.
	Get(docID int) (schema.Fields, error)

	// GetField returns the Terms enumeration for the named field at
	// docID, or nil when no term vector exists for that field.
	GetField(docID int, field string) (schema.Terms, error)

	// Close releases any resources held by the reader.
	Close() error
}
