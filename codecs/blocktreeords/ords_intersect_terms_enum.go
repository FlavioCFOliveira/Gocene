// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// OrdsIntersectTermsEnum iterates over terms of one field that are accepted
// by a finite automaton, tracking term ordinals.
//
// Port of org.apache.lucene.codecs.blocktreeords.OrdsIntersectTermsEnum
// (Lucene 10.4.0). Full traversal logic is deferred to sprint task 3188
// (OrdsIntersectTermsEnumFrame) and beyond.
type OrdsIntersectTermsEnum struct {
	reader    *OrdsFieldReader
	compiled  *automaton.CompiledAutomaton
	startTerm *util.BytesRef
}

// NewOrdsIntersectTermsEnum constructs an enum restricted to terms accepted
// by compiled.  startTerm positions the enum before the first Next() call;
// it may be nil.
func NewOrdsIntersectTermsEnum(r *OrdsFieldReader, compiled *automaton.CompiledAutomaton, startTerm *util.BytesRef) (*OrdsIntersectTermsEnum, error) {
	return &OrdsIntersectTermsEnum{reader: r, compiled: compiled, startTerm: startTerm}, nil
}

// Next advances to the next accepted term.
func (e *OrdsIntersectTermsEnum) Next() (*index.Term, error) { return nil, nil }

// SeekCeil seeks to term or the next accepted term after it.
func (e *OrdsIntersectTermsEnum) SeekCeil(term *index.Term) (*index.Term, error) {
	return nil, nil
}

// SeekExact seeks to the exact term.
func (e *OrdsIntersectTermsEnum) SeekExact(term *index.Term) (bool, error) { return false, nil }

// Term returns the current term.
func (e *OrdsIntersectTermsEnum) Term() *index.Term { return nil }

// DocFreq returns the document frequency of the current term.
func (e *OrdsIntersectTermsEnum) DocFreq() (int, error) { return 0, nil }

// TotalTermFreq returns the total term frequency of the current term.
func (e *OrdsIntersectTermsEnum) TotalTermFreq() (int64, error) { return 0, nil }

// Postings returns a PostingsEnum for the current term.
func (e *OrdsIntersectTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term filtered by live docs.
func (e *OrdsIntersectTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return &index.EmptyPostingsEnum{}, nil
}

// compile-time assertion.
var _ index.TermsEnum = (*OrdsIntersectTermsEnum)(nil)
