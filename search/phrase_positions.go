// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/PhrasePositions.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhrasePositions tracks the position of a term in a document relative
// to its phrase offset. It is used by phrase scorers to detect when all
// constituent terms appear at the positions required by the phrase.
//
// Mirrors org.apache.lucene.search.PhrasePositions (Lucene 10.4.0).
//
// In Java this class is package-private (final). The Go port keeps it
// unexported-field but exposes only what phrase scorers need.
type PhrasePositions struct {
	// Position is the current position in the document, adjusted for the
	// phrase offset: position = postings.nextPosition() - offset.
	Position int
	// Count is the number of remaining positions in the current document.
	Count int
	// Offset is the position of this term within the phrase (0-based).
	Offset int
	// Ord is a unique ordinal across all PhrasePositions in a query.
	Ord int
	// Postings is the PostingsEnum providing doc and position data.
	Postings index.PostingsEnum
	// Next links PhrasePositions in a linked list (used internally).
	Next *PhrasePositions
	// RptGroup is ≥ 0 when this is a repeating term group member, or -1.
	RptGroup int
	// RptInd is the index within RptGroup.
	RptInd int
	// Terms are the Term values for repetition initialisation.
	Terms []*index.Term
}

// NewPhrasePositions constructs a PhrasePositions for the given
// PostingsEnum, phrase offset o, ordinal ord, and terms slice.
//
// Mirrors PhrasePositions(PostingsEnum, int, int, Term[]).
func NewPhrasePositions(postings index.PostingsEnum, o, ord int, terms []*index.Term) *PhrasePositions {
	return &PhrasePositions{
		Postings: postings,
		Offset:   o,
		Ord:      ord,
		Terms:    terms,
		RptGroup: -1,
	}
}

// FirstPosition reads the frequency of the current document and advances
// to the first position.
//
// Mirrors PhrasePositions.firstPosition().
func (pp *PhrasePositions) FirstPosition() error {
	freq, err := pp.Postings.Freq()
	if err != nil {
		return err
	}
	pp.Count = freq
	_, err = pp.NextPosition()
	return err
}

// NextPosition advances to the next position of this term in the current
// document and sets Position = nextPosition() - Offset.
// Returns true when a position was consumed, false when exhausted.
//
// Mirrors PhrasePositions.nextPosition().
func (pp *PhrasePositions) NextPosition() (bool, error) {
	if pp.Count <= 0 {
		return false, nil
	}
	pp.Count--
	pos, err := pp.Postings.NextPosition()
	if err != nil {
		return false, err
	}
	pp.Position = pos - pp.Offset
	return true, nil
}

// String returns a debug representation.
//
// Mirrors PhrasePositions.toString().
func (pp *PhrasePositions) String() string {
	s := fmt.Sprintf("o:%d p:%d c:%d", pp.Offset, pp.Position, pp.Count)
	if pp.RptGroup >= 0 {
		s += fmt.Sprintf(" rpt:%d,i%d", pp.RptGroup, pp.RptInd)
	}
	return s
}
