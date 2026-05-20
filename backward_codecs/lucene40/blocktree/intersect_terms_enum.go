// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// IntersectTermsEnum implements efficient intersection of the block-tree terms
// dictionary with a compiled automaton.
//
// Port of
// org.apache.lucene.backward_codecs.lucene40.blocktree.IntersectTermsEnum
// (Lucene 10.4.0).
//
// The full block-loading and navigation logic (seekToStartTerm, next,
// nextFloorFrame, etc.) is deferred to a later sprint. Until then, all
// navigation methods return ErrBlockTraversalNotAvailable.
type IntersectTermsEnum struct {
	index.TermsEnumBase

	// in is the cloned terms file input.
	in store.IndexInput

	stack []*intersectTermsEnumFrame

	arcs []*fst.Arc[*util.BytesRef]

	// runAutomaton is the byte-runnable compiled automaton.
	runAutomaton *automaton.ByteRunAutomaton

	// automaton provides transition access for the compiled automaton.
	automaton *automaton.Automaton

	// commonSuffix is the automaton's required common suffix (may be nil).
	commonSuffix *util.BytesRef

	currentFrame *intersectTermsEnumFrame

	// fr is the owning FieldReader.
	fr *FieldReader

	// savedStartTerm remembers the initial seek term (for assertions).
	savedStartTerm *util.BytesRef

	fstReader fst.BytesReader

	term *util.BytesRef
}

// newIntersectTermsEnum constructs an IntersectTermsEnum for the given
// FieldReader and compiled automaton.
//
// Port of IntersectTermsEnum(FieldReader, TransitionAccessor, ByteRunnable,
// BytesRef, BytesRef).
func newIntersectTermsEnum(
	fr *FieldReader,
	compiled *automaton.CompiledAutomaton,
	startTerm *util.BytesRef,
) (*IntersectTermsEnum, error) {
	if compiled == nil {
		return nil, fmt.Errorf("blocktree: compiled automaton must not be nil")
	}

	e := &IntersectTermsEnum{
		fr:           fr,
		runAutomaton: compiled.RunAutomaton,
		automaton:    compiled.Automaton,
		commonSuffix: compiled.CommonSuffixRef,
		arcs:         make([]*fst.Arc[*util.BytesRef], 5),
		term:         &util.BytesRef{},
	}

	e.in = fr.parent.termsIn.Clone()
	if e.in == nil {
		return nil, fmt.Errorf("blocktree: cannot clone terms input for IntersectTermsEnum")
	}

	e.stack = make([]*intersectTermsEnumFrame, 5)
	for i := range e.stack {
		e.stack[i] = newIntersectTermsEnumFrame(e, i)
	}
	for i := range e.arcs {
		e.arcs[i] = &fst.Arc[*util.BytesRef]{}
	}

	e.fstReader = fr.fstIndex.GetBytesReader()

	// Initialise root frame.
	arc := fr.fstIndex.GetFirstArc(e.arcs[0])
	f := e.stack[0]
	f.fp = fr.rootBlockFP
	f.fpOrig = fr.rootBlockFP
	f.prefix = 0
	f.arc = arc
	f.outputPrefix = arc.Output()

	// savedStartTerm is kept for assertion purposes only.
	if startTerm != nil {
		cp := *startTerm
		e.savedStartTerm = &cp
	}

	e.currentFrame = f

	// Defer seekToStartTerm to a later sprint.
	// Until then IntersectTermsEnum is constructed but navigation returns
	// ErrBlockTraversalNotAvailable.

	return e, nil
}

// Term returns the current term.
func (e *IntersectTermsEnum) Term() *index.Term {
	if e.term == nil || e.term.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term)
}

// Next advances to the next term matching the automaton.
//
// Deferred: requires full block traversal logic.
func (e *IntersectTermsEnum) Next() (*index.Term, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// SeekCeil is not supported by IntersectTermsEnum.
func (e *IntersectTermsEnum) SeekCeil(_ *index.Term) (*index.Term, error) {
	return nil, fmt.Errorf("blocktree: IntersectTermsEnum does not support SeekCeil")
}

// SeekExact is not supported by IntersectTermsEnum.
func (e *IntersectTermsEnum) SeekExact(_ *index.Term) (bool, error) {
	return false, fmt.Errorf("blocktree: IntersectTermsEnum does not support SeekExact")
}

// DocFreq returns the document frequency of the current term.
//
// Deferred: requires metadata decoding.
func (e *IntersectTermsEnum) DocFreq() (int, error) {
	return 0, ErrBlockTraversalNotAvailable
}

// TotalTermFreq returns the total term frequency of the current term.
//
// Deferred: requires metadata decoding.
func (e *IntersectTermsEnum) TotalTermFreq() (int64, error) {
	return 0, ErrBlockTraversalNotAvailable
}

// Postings returns a PostingsEnum for the current term.
//
// Deferred: requires metadata decoding and PostingsReaderBase wiring.
func (e *IntersectTermsEnum) Postings(_ int) (index.PostingsEnum, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term with live
// docs applied.
//
// Deferred: requires metadata decoding and PostingsReaderBase wiring.
func (e *IntersectTermsEnum) PostingsWithLiveDocs(_ util.Bits, _ int) (index.PostingsEnum, error) {
	return nil, ErrBlockTraversalNotAvailable
}

// compile-time assertion
var _ index.TermsEnum = (*IntersectTermsEnum)(nil)
