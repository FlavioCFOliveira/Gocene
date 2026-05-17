// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// PostingsWriterBase is the write-side SPI that encodes per-term postings as
// directed by a term-dictionary writer (typically a block-tree terms writer).
// It is the Go port of org.apache.lucene.codecs.PostingsWriterBase from
// Apache Lucene 10.4.0.
//
// The term-dictionary writer owns the term-by-term cursor and the on-disk
// term metadata; the PostingsWriterBase owns the per-document/per-position
// data files (.doc, .pos, .pay in the default codec). The two cooperate
// through a BlockTermState that the writer encodes inline with each term so
// the read side can decode it later.
//
// Lifecycle (per segment):
//  1. Init is called once after the term-dictionary writer has created its
//     output. The PostingsWriterBase may write a codec header and any global
//     metadata at this point.
//  2. For each field SetField is called once.
//  3. For each term within a field NewTermState/StartTerm/<term-pull>/FinishTerm
//     are invoked. The term-dictionary writer drives the cursor and calls
//     EncodeTerm to flush the BlockTermState bytes inline with the term entry.
//  4. Close releases the file handles and writes any tail metadata.
type PostingsWriterBase interface {
	// Init wires the writer to the term-dictionary's IndexOutput so that
	// codec headers and global metadata can share the same file or stream.
	// Implementations typically write a CodecUtil index header here.
	Init(termsOut store.IndexOutput, state *SegmentWriteState) error

	// NewTermState allocates a fresh BlockTermState (or codec subclass)
	// suitable for the writer. Each call returns an independent instance.
	NewTermState() *BlockTermState

	// SetField is called once before the writer is asked to handle any term
	// of the field. Implementations may cache field-level configuration
	// (IndexOptions, omitNorms, etc.) here.
	SetField(fieldInfo *index.FieldInfo) (int, error)

	// StartTerm is called once at the beginning of every term, after
	// NewTermState. The norms argument is the per-document norms for the
	// term's field (nil when norms are omitted) and is used for impact-based
	// posting list compression.
	StartTerm(norms index.NumericDocValues) error

	// FinishTerm is called after the term's postings have been pushed to the
	// writer. The state argument is the BlockTermState the writer should
	// populate with the codec-specific metadata it intends to round-trip
	// through EncodeTerm.
	FinishTerm(state *BlockTermState) error

	// EncodeTerm serializes the codec-specific portion of state into out.
	// absolute indicates whether the term is the first term in a block (and
	// therefore stored absolutely) or a delta against the previous term in
	// the same block. The encoded bytes must round-trip through the matching
	// PostingsReaderBase.DecodeTerm.
	EncodeTerm(out store.IndexOutput, fieldInfo *index.FieldInfo, state *BlockTermState, absolute bool) error

	// Close releases file handles and writes any tail metadata (footers).
	Close() error
}
