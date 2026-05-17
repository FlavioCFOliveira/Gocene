// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// BlockTermState holds the postings-level state for a single term as it
// appears inside a term-dictionary block. It is the Go port of
// org.apache.lucene.codecs.BlockTermState (which itself extends OrdTermState)
// from Apache Lucene 10.4.0.
//
// Each PostingsFormat usually defines its own subtype that embeds
// *BlockTermState and adds codec-specific cursors (skip offset, payload
// offset, etc.). Consumers downstream interact with the embedded
// BlockTermState fields directly:
//
//	type myCodecTermState struct {
//	    *BlockTermState
//	    docStartFP int64
//	    posStartFP int64
//	}
//
// This mirrors the Java pattern where Lucene99PostingsReader's IntBlockTermState
// extends BlockTermState.
//
// BlockTermState is intentionally distinct from the unrelated codecs.BlockState
// (chunked stored-fields block tracking). Do not confuse the two.
type BlockTermState struct {
	// Ord is the ordinal of the term inside its enclosing block (set by the
	// block tree reader during iteration). Mirrors OrdTermState.ord (Java
	// long).
	Ord int64

	// DocFreq is the number of documents containing this term.
	DocFreq int

	// TotalTermFreq is the total number of occurrences across all documents.
	// Set to -1 by codecs that do not track this statistic (e.g. when
	// IndexOptions.DOCS is used).
	TotalTermFreq int64

	// TermBlockOrd is the term's ordinal inside the current term-dictionary
	// block. Mirrors Java's termBlockOrd.
	TermBlockOrd int

	// BlockFilePointer is the file pointer (in bytes) of the block that
	// contains this term, relative to the start of the postings file.
	// Mirrors Java's blockFilePointer.
	BlockFilePointer int64
}

// NewBlockTermState returns a zero-valued BlockTermState ready to be filled by
// a PostingsReaderBase implementation.
func NewBlockTermState() *BlockTermState {
	return &BlockTermState{
		TotalTermFreq: -1,
	}
}

// CopyFrom copies the field values from src into the receiver. This mirrors
// Java's BlockTermState#copyFrom(TermState), which subclasses override to
// extend the copy with their own fields. Returns the receiver for chaining.
func (s *BlockTermState) CopyFrom(src *BlockTermState) *BlockTermState {
	if src == nil {
		return s
	}
	s.Ord = src.Ord
	s.DocFreq = src.DocFreq
	s.TotalTermFreq = src.TotalTermFreq
	s.TermBlockOrd = src.TermBlockOrd
	s.BlockFilePointer = src.BlockFilePointer
	return s
}

// Clone returns a deep copy of the BlockTermState. Codec subclasses that
// embed *BlockTermState should override Clone to copy their own fields too.
func (s *BlockTermState) Clone() *BlockTermState {
	cp := *s
	return &cp
}
