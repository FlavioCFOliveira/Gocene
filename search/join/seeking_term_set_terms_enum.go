// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SeekingTermSetTermsEnum is a filtered TermsEnum that uses a BytesRefHash as a
// filter.
//
// Port of org.apache.lucene.search.join.SeekingTermSetTermsEnum
// (lucene/join/src/java/org/apache/lucene/search/join/SeekingTermSetTermsEnum.java,
// Apache Lucene 10.5.0). The Java class extends FilteredTermsEnum and
// overrides nextSeekTerm(BytesRef) and accept(BytesRef); Gocene renders the
// subclass as the FilteredTermsEnumAcceptor installed on the embedded
// *index.FilteredTermsEnum.
//
// Gocene's TermsEnum is keyed on *Term; the seek terms it hands to the
// delegate carry an empty field name, since only the bytes take part in
// seekCeil, exactly as the Java BytesRef does.
//
// @lucene.internal
type SeekingTermSetTermsEnum struct {
	*index.FilteredTermsEnum

	terms       *util.BytesRefHash
	ords        []int
	lastElement int

	lastTerm *util.BytesRef
	spare    *util.BytesRef

	seekTerm *util.BytesRef
	upto     int
}

// NewSeekingTermSetTermsEnum renders the constructor
// SeekingTermSetTermsEnum(TermsEnum tenum, BytesRefHash terms, int[] ords).
func NewSeekingTermSetTermsEnum(tenum index.TermsEnum, terms *util.BytesRefHash, ords []int) *SeekingTermSetTermsEnum {
	e := &SeekingTermSetTermsEnum{
		terms: terms,
		ords:  ords,
		spare: util.NewBytesRefEmpty(),
	}
	e.FilteredTermsEnum = index.NewFilteredTermsEnum(tenum, e)
	e.lastElement = terms.Size() - 1
	e.lastTerm = terms.Get(ords[e.lastElement], util.NewBytesRefEmpty())
	e.seekTerm = terms.Get(ords[e.upto], e.spare)
	return e
}

// NextSeekTerm renders the nextSeekTerm(BytesRef currentTerm) override.
func (e *SeekingTermSetTermsEnum) NextSeekTerm(currentTerm *spi.Term) (*spi.Term, error) {
	temp := e.seekTerm
	e.seekTerm = nil
	if temp == nil {
		return nil, nil
	}
	return spi.NewTermFromBytes("", temp.ValidBytes()), nil
}

// Accept renders the accept(BytesRef term) override.
func (e *SeekingTermSetTermsEnum) Accept(t *spi.Term) (index.AcceptStatus, error) {
	term := t.BytesValue()
	if util.BytesRefCompare(term, e.lastTerm) > 0 {
		return index.AcceptEnd, nil
	}

	currentTerm := e.terms.Get(e.ords[e.upto], e.spare)
	if util.BytesRefCompare(term, currentTerm) == 0 {
		if e.upto == e.lastElement {
			return index.AcceptYes, nil
		}
		e.upto++
		e.seekTerm = e.terms.Get(e.ords[e.upto], e.spare)
		return index.AcceptYesAndSeek, nil
	}
	if e.upto == e.lastElement {
		return index.AcceptNo, nil
	}
	// Our current term doesn't match the given term.
	var cmp int
	for { // We maybe are behind the given term by more than one step. Keep incrementing till
		// we're the same or higher.
		if e.upto == e.lastElement {
			return index.AcceptNo, nil
		}
		// typically the terms dict is a superset of query's terms so it's unusual that we have to
		// skip many of
		// our terms so we don't do a binary search here
		e.upto++
		e.seekTerm = e.terms.Get(e.ords[e.upto], e.spare)
		if cmp = util.BytesRefCompare(e.seekTerm, term); cmp >= 0 {
			break
		}
	}
	if cmp == 0 {
		if e.upto == e.lastElement {
			return index.AcceptYes, nil
		}
		e.upto++
		e.seekTerm = e.terms.Get(e.ords[e.upto], e.spare)
		return index.AcceptYesAndSeek, nil
	}
	return index.AcceptNoAndSeek, nil
}
