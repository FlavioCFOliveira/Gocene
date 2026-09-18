// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermBytes contains the term bytes and the minimal distinguishing prefix (MDP) length of this term.
// Mirrors org.apache.lucene.codecs.uniformsplit.TermBytes from Apache Lucene 10.5.0.
type TermBytes struct {
	// mdpLength is the minimal prefix that distinguishes a term from its immediate previous term.
	mdpLength int32

	// term is the actual term bytes.
	term *util.BytesRef
}

// NewTermBytes constructs a new TermBytes.
func NewTermBytes(mdpLength int32, term *util.BytesRef) *TermBytes {
	return &TermBytes{
		mdpLength: mdpLength,
		term:      term,
	}
}

// GetSuffixLength returns the length of the incremental encoding suffix.
func (tb *TermBytes) GetSuffixLength() int {
	if tb.term == nil || tb.term.Length() == 0 {
		return 0
	}
	return tb.term.Length() - int(tb.mdpLength-1)
}

// GetSuffixOffset returns the offset of the incremental encoding suffix.
func (tb *TermBytes) GetSuffixOffset() int {
	return int(tb.mdpLength - 1)
}

// GetTerm returns the full term bytes.
func (tb *TermBytes) GetTerm() *util.BytesRef {
	return tb.term
}

// GetMdpLength returns the MDP length.
func (tb *TermBytes) GetMdpLength() int32 {
	return tb.mdpLength
}

// BlockLine represents one line in a block's term list.
// Mirrors org.apache.lucene.codecs.uniformsplit.BlockLine from Apache Lucene 10.5.0.
type BlockLine struct {
	// termStateRelativeOffset is the relative offset to the term state in the details region.
	termStateRelativeOffset int32

	// term is the term associated with this line.
	term *TermBytes
}

// Reset resets the BlockLine fields.
func (bl *BlockLine) Reset(termBytes *TermBytes, termStateRelativeOffset int32) *BlockLine {
	bl.term = termBytes
	bl.termStateRelativeOffset = termStateRelativeOffset
	return bl
}

// FieldMetadata contains metadata and stats for one field in the index.
// Mirrors org.apache.lucene.codecs.uniformsplit.FieldMetadata from Apache Lucene 10.5.0.
type FieldMetadata struct {
	// numTerms is the number of terms in the field.
	numTerms int64

	// sumDocFreq is the sum of doc frequencies for all terms in the field.
	sumDocFreq int64

	// sumTotalTermFreq is the sum of total term frequencies for all terms in the field.
	sumTotalTermFreq int64

	// docCount is the number of documents that contain at least one term from this field.
	docCount int32

	// dictionaryStartFP is the file pointer to the start of the dictionary for this field.
	dictionaryStartFP int64

	// firstBlockStartFP is the file pointer to the start of the first block.
	firstBlockStartFP int64

	// lastBlockStartFP is the file pointer to the end of the last block.
	lastBlockStartFP int64

	// lastTerm is the last term in the field.
	lastTerm *util.BytesRef
}
