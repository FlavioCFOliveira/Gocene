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

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/java/org/apache/lucene/search/PhrasePositions.java

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// PhrasePositions is the position of a term in a document that takes into
// account the term offset within the phrase.
//
// Mirrors org.apache.lucene.search.PhrasePositions (Lucene 10.5.0), a
// package-private final class. The Go port keeps the exact field and method
// set of the Java original.
type PhrasePositions struct {
	// Position is the position in the doc.
	Position int
	// Count is the remaining pos in this doc.
	Count int
	// Offset is the position in the phrase.
	Offset int
	// Ord is unique across all PhrasePositions instances.
	Ord int
	// Postings is the stream of docs and positions.
	Postings index.PostingsEnum
	// Next is used to make lists.
	Next *PhrasePositions
	// RptGroup is >= 0 to indicate that this is a repeating PP.
	RptGroup int
	// RptInd is the index in the RptGroup.
	RptInd int
	// Terms are the terms, for repetitions initialization.
	Terms []*index.Term
	// Freq is the cached frequency for the current document.
	Freq int
}

// NewPhrasePositions constructs a PhrasePositions for the given PostingsEnum,
// phrase offset o, ordinal ord, and terms slice.
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

// FirstPosition seeds the remaining-position count from the cached frequency
// and advances to the first position.
//
// Mirrors PhrasePositions.firstPosition().
func (pp *PhrasePositions) FirstPosition() error {
	pp.Count = pp.Freq // use cached frequency
	_, err := pp.NextPosition()
	return err
}

// NextPosition goes to the next location of this term in the current document,
// and sets Position as location - Offset, so that a matching exact phrase is
// easily identified when all PhrasePositions have exactly the same Position.
//
// Mirrors PhrasePositions.nextPosition().
func (pp *PhrasePositions) NextPosition() (bool, error) {
	if pp.Count > 0 { // read subsequent pos's
		pp.Count--
		pos, err := pp.Postings.NextPosition()
		if err != nil {
			return false, err
		}
		pp.Position = pos - pp.Offset
		return true, nil
	}
	pp.Count--
	return false, nil
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
