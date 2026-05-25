// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// PostingsReaderBase is the read-side SPI that decodes per-term postings as
// directed by a term-dictionary reader (typically a block-tree terms reader).
// It is the Go port of org.apache.lucene.codecs.PostingsReaderBase from
// Apache Lucene 10.4.0.
//
// The term-dictionary reader owns the term enumeration and the on-disk term
// metadata; the PostingsReaderBase owns the per-document/per-position data
// files (.doc, .pos, .pay in the default codec). The two cooperate through a
// BlockTermState that the writer side encoded inline with each term.
//
// Lifecycle:
//  1. Init is called once per segment after the term-dictionary reader has
//     opened the file containing the postings metadata.
//  2. For each term in iteration order the term-dictionary reader calls
//     NewTermState followed by DecodeTerm to materialize the BlockTermState
//     for the current term.
//  3. Postings/Impacts produce iterators over the encoded postings using the
//     decoded BlockTermState.
//  4. CheckIntegrity validates the underlying file CRCs.
//  5. Close releases the file handles.
type PostingsReaderBase interface {
	// Init wires the reader to the term-dictionary's IndexInput. The supplied
	// termsIn is positioned at the start of the postings reader's metadata
	// block; implementations may read codec headers, footers, and global
	// metadata before returning.
	Init(termsIn store.IndexInput, state *SegmentReadState) error

	// NewTermState allocates a fresh BlockTermState (or a codec subclass)
	// suitable for the calling reader. Each call returns an independent
	// instance; callers must not share BlockTermStates across goroutines.
	NewTermState() *BlockTermState

	// DecodeTerm reads codec-specific term metadata from in, populating
	// termState. absolute indicates whether the term is the first term in a
	// block (and therefore stored absolutely) or a delta against the previous
	// term in the same block.
	//
	// in is typed as DataInput (not IndexInput) because the term-dictionary
	// reader passes a ByteArrayDataInput backed by an in-memory stats blob —
	// not the raw .tim file handle. Mirrors Java's DataInput parameter.
	DecodeTerm(in store.DataInput, fieldInfo *index.FieldInfo, termState *BlockTermState, absolute bool) error

	// Postings returns a PostingsEnum over the term identified by termState.
	// reuse may be the previous PostingsEnum returned for the same field;
	// implementations may reuse it to avoid allocations, or may allocate a new
	// one when the requested flags require richer enumeration capabilities.
	// flags is a bitmask of index.PostingsEnum FLAG_* values.
	Postings(fieldInfo *index.FieldInfo, termState *BlockTermState, reuse index.PostingsEnum, flags int) (index.PostingsEnum, error)

	// Impacts returns an ImpactsEnum for impact-aware scoring (BMW/MAXSCORE).
	// flags is a bitmask of index.PostingsEnum FLAG_* values.
	Impacts(fieldInfo *index.FieldInfo, termState *BlockTermState, flags int) (index.ImpactsEnum, error)

	// CheckIntegrity validates the CRC footers of every file this reader
	// owns. Returns the first CRC mismatch as an error.
	CheckIntegrity() error

	// Close releases file handles owned by the reader.
	Close() error
}
