// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktreeords

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	gfst "github.com/FlavioCFOliveira/Gocene/util/fst"
)

// OrdsIntersectTermsEnum iterates over terms of one field that are accepted
// by a finite automaton, tracking term ordinals.
//
// Port of org.apache.lucene.codecs.blocktreeords.OrdsIntersectTermsEnum
// (Lucene 10.4.0). Full traversal logic is deferred to a later sprint;
// the struct carries all fields required by OrdsIntersectTermsEnumFrame so
// that the frame compiles cleanly.
type OrdsIntersectTermsEnum struct {
	reader    *OrdsFieldReader
	compiled  *automaton.CompiledAutomaton
	startTerm *util.BytesRef

	// in is the cloned terms IndexInput.
	in store.IndexInput

	// byteRunnable is the byte-level run automaton from compiled.
	byteRunnable automaton.ByteRunnable

	// transitionAccessor is the automaton transition API from compiled.
	transitionAccessor automaton.TransitionAccessor

	// currentFrame is the top frame on the stack.
	currentFrame *OrdsIntersectTermsEnumFrame

	// stack is the frame stack, grown on demand.
	stack []*OrdsIntersectTermsEnumFrame

	// arcs caches FST arcs, one per stack depth.
	arcs []*gfst.Arc[*FSTOrdsOutput]

	// term is the current term bytes built up by frame traversal.
	term []byte
}

// NewOrdsIntersectTermsEnum constructs an enum restricted to terms accepted
// by compiled.  startTerm positions the enum before the first Next() call;
// it may be nil.
func NewOrdsIntersectTermsEnum(r *OrdsFieldReader, compiled *automaton.CompiledAutomaton, startTerm *util.BytesRef) (*OrdsIntersectTermsEnum, error) {
	e := &OrdsIntersectTermsEnum{
		reader:    r,
		compiled:  compiled,
		startTerm: startTerm,
		stack:     make([]*OrdsIntersectTermsEnumFrame, 5),
		arcs:      make([]*gfst.Arc[*FSTOrdsOutput], 5),
		term:      make([]byte, 0, 32),
	}
	for i := range e.arcs {
		e.arcs[i] = &gfst.Arc[*FSTOrdsOutput]{}
	}
	if compiled != nil {
		if compiled.RunAutomaton != nil {
			e.byteRunnable = compiled.RunAutomaton
		}
		if compiled.Automaton != nil {
			e.transitionAccessor = compiled.Automaton
		}
	}
	return e, nil
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
