// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"io"
)

// PhraseMatcher is the base for exact and sloppy phrase matching.
type PhraseMatcher interface {
	// Approximation returns a DocIdSetIterator that only matches documents containing all terms.
	Approximation() DocIdSetIterator
	// ImpactsApproximation returns an ImpactsDISI view of the approximation.
	ImpactsApproximation() ImpactsDISI
	// MaxFreq returns an upper bound on the number of possible matches on the current document.
	MaxFreq() (float32, error)
	// ResetPositions loads positions for matching after the approximation has been advanced.
	ResetPositions() error
	// NextMatch finds the next match on the current document, returning false if there are none.
	NextMatch() (bool, error)
	// SloppyWeight returns the slop-adjusted weight of the current match.
	SloppyWeight() float32
	// StartPosition returns the start position of the current match.
	StartPosition() int
	// EndPosition returns the end position of the current match.
	EndPosition() int
	// StartOffset returns the start offset of the current match.
	StartOffset() (int, error)
	// EndOffset returns the end offset of the current match.
	EndOffset() (int, error)
	// GetMatchCost returns an estimate of the average cost of finding all matches on a document.
	GetMatchCost() float32
}

// PhrasePositions wraps a postings enum and tracks the current position and offset.
type PhrasePositions struct {
	postings index.PostingsEnum
	offset   int
	position int
	ord      int
	terms    []byte // For simplicity, we can use the term bytes or a Term object
	rptGroup int
	rptInd   int
	freq     float32
}

func NewPhrasePositions(postings index.PostingsEnum, offset int, ord int, terms []byte) *PhrasePositions {
	return &PhrasePositions{
		postings: postings,
		offset:   offset,
		position: 0,
		ord:      ord,
		terms:    terms,
		rptGroup: -1,
		rptInd:   -1,
	}
}

func (pp *PhrasePositions) firstPosition() (bool, error) {
	pos, err := pp.postings.NextPosition()
	if err != nil || pos == index.NO_MORE_POSITIONS {
		return false, err
	}
	pp.position = pos
	return true, nil
}

func (pp *PhrasePositions) nextPosition() (bool, error) {
	pos, err := pp.postings.NextPosition()
	if err != nil || pos == index.NO_MORE_POSITIONS {
		return false, err
	}
	pp.position = pos
	return true, nil
}

func (pp *PhrasePositions) tpPos() int {
	return pp.position + pp.offset
}
