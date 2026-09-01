package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermBytes represents a term of a block line.
//
// It contains the term bytes and the minimal distinguishing prefix (MDP) length of this term.
//
// The MDP is the minimal prefix that distinguishes a term from its immediate previous term
// (terms are alphabetically sorted).
//
// The incremental encoding suffix is the suffix starting at the last byte of the MDP
// (inclusive).
//
// Example: For the block
//
//	client
//	color
//	company
//	companies
//
// "color" - MDP is "co" - incremental encoding suffix is "olor".
// "company" - MDP is "com" - incremental encoding suffix is "mpany".
// "companies" - MDP is "compani" - incremental encoding suffix is "ies".
type TermBytes struct {
	mdpLength int
	term      *util.BytesRef
}

// NewTermBytes creates a new TermBytes instance.
func NewTermBytes(mdpLength int, term *util.BytesRef) *TermBytes {
	tb := &TermBytes{}
	tb.Reset(mdpLength, term)
	return tb
}

// Reset resets the TermBytes instance to reuse it.
func (tb *TermBytes) Reset(mdpLength int, term *util.BytesRef) *TermBytes {
	// assert term.length > 0 && mdpLength > 0 || term.length == 0 && mdpLength == 0
	// assert term.length == 0 || mdpLength <= term.length
	// assert term.offset == 0
	tb.mdpLength = mdpLength
	tb.term = term
	return tb
}

// MdpLength returns the term MDP length.
func (tb *TermBytes) MdpLength() int {
	return tb.mdpLength
}

// Term returns the term bytes.
func (tb *TermBytes) Term() *util.BytesRef {
	return tb.term
}

// SuffixOffset returns the offset of this term incremental encoding suffix.
func (tb *TermBytes) SuffixOffset() int {
	if tb.mdpLength-1 < 0 {
		return 0
	}
	return tb.mdpLength - 1
}

// SuffixLength returns the length of this term incremental encoding suffix.
func (tb *TermBytes) SuffixLength() int {
	return tb.term.Length - tb.SuffixOffset()
}

// ComputeMdpLength computes the length of the minimal distinguishing prefix (MDP) between a current term and its
// previous term (terms are alphabetically sorted).
//
// Example: If previous="car" and current="cartridge", then MDP length is 4. It is the length
// of the minimal prefix distinguishing "cartridge" from "car", that is, the length of "cart".
func ComputeMdpLength(previousTerm, currentTerm *util.BytesRef) int {
	var mdpLength int
	if previousTerm == nil {
		mdpLength = 1
	} else {
		mdpLength = util.SortKeyLength(previousTerm.ValidBytes(), currentTerm.ValidBytes())
	}
	if mdpLength > currentTerm.Length {
		return currentTerm.Length
	}
	return mdpLength
}
