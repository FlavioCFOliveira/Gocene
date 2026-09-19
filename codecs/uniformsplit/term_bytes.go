// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package uniformsplit

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermBytes is the term of a block line.
//
// Contains the term bytes and the minimal distinguishing prefix (MDP) length of
// this term.
//
// The MDP is the minimal prefix that distinguishes a term from its immediate
// previous term (terms are alphabetically sorted).
//
// The incremental encoding suffix is the suffix starting at the last byte of the
// MDP (inclusive).
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
//
// Mirrors org.apache.lucene.codecs.uniformsplit.TermBytes from Apache Lucene
// 10.5.0.
type TermBytes struct {
	mdpLength int
	term      *util.BytesRef
}

// NewTermBytes constructs a TermBytes. Mirrors the TermBytes(int, BytesRef)
// constructor (TermBytes.java:59), whose body is a call to reset.
func NewTermBytes(mdpLength int, term *util.BytesRef) *TermBytes {
	tb := &TermBytes{}
	return tb.Reset(mdpLength, term)
}

// Reset re-initializes this TermBytes and returns the receiver, so that
// BlockLineSerializer.readIncrementallyEncodedTerm can return it directly.
// Mirrors TermBytes.reset (TermBytes.java:63).
func (tb *TermBytes) Reset(mdpLength int, term *util.BytesRef) *TermBytes {
	// assert term.length > 0 && mdpLength > 0 || term.length == 0 && mdpLength == 0
	// assert term.length == 0 || mdpLength <= term.length
	// assert term.offset == 0
	tb.mdpLength = mdpLength
	tb.term = term
	return tb
}

// GetMdpLength returns this term MDP length.
func (tb *TermBytes) GetMdpLength() int {
	return tb.mdpLength
}

// GetTerm returns this term bytes.
func (tb *TermBytes) GetTerm() *util.BytesRef {
	return tb.term
}

// GetSuffixOffset returns the offset of this term incremental encoding suffix.
func (tb *TermBytes) GetSuffixOffset() int {
	return max(tb.mdpLength-1, 0)
}

// GetSuffixLength returns the length of this term incremental encoding suffix.
func (tb *TermBytes) GetSuffixLength() int {
	return tb.term.Length - tb.GetSuffixOffset()
}

// ComputeMdpLength computes the length of the minimal distinguishing prefix
// (MDP) between a current term and its previous term (terms are alphabetically
// sorted).
//
// Example: If previousTerm="car" and currentTerm="cartridge", then MDP length is
// 4. It is the length of the minimal prefix distinguishing "cartridge" from
// "car", that is, the length of "cart".
//
// Mirrors the static TermBytes.computeMdpLength (TermBytes.java:114). Java's
// StringHelper.sortKeyLength raises no checked exception; Gocene's
// util.SortKeyLength reports out-of-order terms as an error, which is
// propagated here.
func ComputeMdpLength(previousTerm, currentTerm *util.BytesRef) (int, error) {
	mdpLength := 1
	if previousTerm != nil {
		var err error
		mdpLength, err = util.SortKeyLength(previousTerm, currentTerm)
		if err != nil {
			return 0, err
		}
	}
	return min(mdpLength, currentTerm.Length), nil
}
