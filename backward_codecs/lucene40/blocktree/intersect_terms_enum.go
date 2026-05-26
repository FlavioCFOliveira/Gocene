// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"bytes"
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

	// currentTransition is the transition being matched at the current frame level.
	currentTransition *automaton.Transition

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
		fr:                fr,
		runAutomaton:      compiled.RunAutomaton,
		automaton:         compiled.Automaton,
		commonSuffix:      compiled.CommonSuffixRef,
		arcs:              make([]*fst.Arc[*util.BytesRef], 5),
		term:              &util.BytesRef{Bytes: make([]byte, 16)},
		currentTransition: automaton.NewTransition(),
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

	// Initialise root frame: load the root block.
	arc := fr.fstIndex.GetFirstArc(e.arcs[0])
	f := e.stack[0]
	f.fp = fr.rootBlockFP
	f.fpOrig = fr.rootBlockFP
	f.prefix = 0
	f.setState(0)
	f.arc = arc
	f.outputPrefix = arc.Output()

	outputs := fst.ByteSequenceOutputs()
	rootData := outputs.Add(arc.Output(), arc.NextFinalOutput())
	if err := f.load(rootData); err != nil {
		return nil, fmt.Errorf("blocktree IntersectTermsEnum load root block: %w", err)
	}

	if startTerm != nil {
		cp := *startTerm
		e.savedStartTerm = &cp
	}

	e.currentFrame = f
	if startTerm != nil {
		if err := e.seekToStartTerm(startTerm); err != nil {
			return nil, fmt.Errorf("blocktree IntersectTermsEnum seekToStartTerm: %w", err)
		}
	}
	*e.currentTransition = e.currentFrame.transition

	return e, nil
}

// Term returns the current term.
func (e *IntersectTermsEnum) Term() *index.Term {
	if e.term == nil || e.term.Length == 0 {
		return nil
	}
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), e.term)
}

// DocFreq returns the document frequency of the current term.
//
// Port of IntersectTermsEnum.docFreq.
func (e *IntersectTermsEnum) DocFreq() (int, error) {
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return 0, err
	}
	if e.currentFrame.termState == nil {
		return 0, fmt.Errorf("blocktree IntersectTermsEnum DocFreq: term state is nil")
	}
	return e.currentFrame.termState.DocFreq, nil
}

// TotalTermFreq returns the total term frequency of the current term.
//
// Port of IntersectTermsEnum.totalTermFreq.
func (e *IntersectTermsEnum) TotalTermFreq() (int64, error) {
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return 0, err
	}
	if e.currentFrame.termState == nil {
		return 0, fmt.Errorf("blocktree IntersectTermsEnum TotalTermFreq: term state is nil")
	}
	return e.currentFrame.termState.TotalTermFreq, nil
}

// Postings returns a PostingsEnum for the current term.
//
// Port of IntersectTermsEnum.postings.
func (e *IntersectTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	if err := e.currentFrame.decodeMetaData(); err != nil {
		return nil, err
	}
	if e.currentFrame.termState == nil {
		return nil, fmt.Errorf("blocktree IntersectTermsEnum Postings: term state is nil")
	}
	return e.fr.parent.postingsReader.Postings(
		e.fr.fieldInfo,
		e.currentFrame.termState,
		nil,
		flags,
	)
}

// PostingsWithLiveDocs returns a PostingsEnum for the current term with live
// docs applied.
func (e *IntersectTermsEnum) PostingsWithLiveDocs(_ util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// SeekCeil is not supported by IntersectTermsEnum.
func (e *IntersectTermsEnum) SeekCeil(_ *index.Term) (*index.Term, error) {
	return nil, fmt.Errorf("blocktree: IntersectTermsEnum does not support SeekCeil")
}

// SeekExact is not supported by IntersectTermsEnum.
func (e *IntersectTermsEnum) SeekExact(_ *index.Term) (bool, error) {
	return false, fmt.Errorf("blocktree: IntersectTermsEnum does not support SeekExact")
}

// Next advances to the next term matching the automaton.
//
// Port of IntersectTermsEnum.next (via _next + NoMoreTermsException).
func (e *IntersectTermsEnum) Next() (*index.Term, error) {
	if e.currentFrame == nil {
		return nil, nil
	}
	ref, err := e.nextInternal()
	if err == errNoMoreTerms {
		// Provoke nil-frame so illegal second call fails quickly.
		e.currentFrame = nil
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ref == nil {
		e.currentFrame = nil
		return nil, nil
	}
	return index.NewTermFromBytesRef(e.fr.fieldInfo.Name(), ref), nil
}

// nextInternal is the inner loop of Next().
//
// Port of IntersectTermsEnum._next.
func (e *IntersectTermsEnum) nextInternal() (*util.BytesRef, error) {
	isSubBlock, err := e.popPushNext()
	if err != nil {
		return nil, err
	}

	for {
		var state int
		var lastState int

		if e.currentFrame.suffix != 0 {
			suffixBytes := e.currentFrame.suffixBytes
			label := int(suffixBytes[e.currentFrame.startBytePos]) & 0xFF

			if label < e.currentTransition.Min {
				// Scan forward to catch up with the transition minimum.
				minTrans := e.currentTransition.Min
				for e.currentFrame.nextEnt < e.currentFrame.entCount {
					isSubBlock = e.currentFrame.next()
					if int(suffixBytes[e.currentFrame.startBytePos])&0xFF >= minTrans {
						goto continueNextTerm
					}
				}
				isSubBlock, err = e.popPushNext()
				if err != nil {
					return nil, err
				}
				continue
			}

			// Advance the automaton transition to cover this label.
			for label > e.currentTransition.Max {
				if e.currentFrame.transitionIndex >= e.currentFrame.transitionCount-1 {
					// No further transitions: pop frame.
					if e.currentFrame.ord == 0 {
						e.currentFrame = nil
						return nil, nil
					}
					e.currentFrame = e.stack[e.currentFrame.ord-1]
					*e.currentTransition = e.currentFrame.transition
					isSubBlock, err = e.popPushNext()
					if err != nil {
						return nil, err
					}
					goto continueNextTerm
				}
				e.currentFrame.transitionIndex++
				e.automaton.GetNextTransition(&e.currentFrame.transition)
				*e.currentTransition = e.currentFrame.transition

				if label < e.currentTransition.Min {
					minTrans := e.currentTransition.Min
					for e.currentFrame.nextEnt < e.currentFrame.entCount {
						isSubBlock = e.currentFrame.next()
						if int(suffixBytes[e.currentFrame.startBytePos])&0xFF >= minTrans {
							goto continueNextTerm
						}
					}
					isSubBlock, err = e.popPushNext()
					if err != nil {
						return nil, err
					}
					goto continueNextTerm
				}
			}

			// Check common-suffix filter (skip sub-blocks).
			if e.commonSuffix != nil && !isSubBlock {
				termLen := e.currentFrame.prefix + e.currentFrame.suffix
				if termLen < e.commonSuffix.Length {
					isSubBlock, err = e.popPushNext()
					if err != nil {
						return nil, err
					}
					goto continueNextTerm
				}
				csBytes := e.commonSuffix.Bytes
				lenInPrefix := e.commonSuffix.Length - e.currentFrame.suffix

				var csBytesPos int
				var suffixBytesPos int

				if lenInPrefix > 0 {
					termBytesPos := e.currentFrame.prefix - lenInPrefix
					termBytesPosEnd := e.currentFrame.prefix
					for termBytesPos < termBytesPosEnd {
						if e.term.Bytes[termBytesPos] != csBytes[csBytesPos] {
							isSubBlock, err = e.popPushNext()
							if err != nil {
								return nil, err
							}
							goto continueNextTerm
						}
						termBytesPos++
						csBytesPos++
					}
					suffixBytesPos = e.currentFrame.startBytePos
				} else {
					suffixBytesPos = e.currentFrame.startBytePos + e.currentFrame.suffix - e.commonSuffix.Length
				}

				csEnd := e.commonSuffix.Length
				for csBytesPos < csEnd {
					if suffixBytes[suffixBytesPos] != csBytes[csBytesPos] {
						isSubBlock, err = e.popPushNext()
						if err != nil {
							return nil, err
						}
						goto continueNextTerm
					}
					suffixBytesPos++
					csBytesPos++
				}
			}

			// Walk the automaton over the suffix (starting from the 2nd byte,
			// since we already confirmed label == suffixBytes[startBytePos]).
			lastState = e.currentFrame.state
			state = e.currentTransition.Dest

			end := e.currentFrame.startBytePos + e.currentFrame.suffix
			for idx := e.currentFrame.startBytePos + 1; idx < end; idx++ {
				lastState = state
				state = e.runAutomaton.Step(state, int(suffixBytes[idx])&0xFF)
				if state == -1 {
					isSubBlock, err = e.popPushNext()
					if err != nil {
						return nil, err
					}
					goto continueNextTerm
				}
			}
		} else {
			state = e.currentFrame.state
			lastState = e.currentFrame.lastState
		}

		if isSubBlock {
			// Match: recurse into sub-block.
			e.copyTerm()
			f, err2 := e.pushFrame(state)
			if err2 != nil {
				return nil, err2
			}
			e.currentFrame = f
			*e.currentTransition = e.currentFrame.transition
			e.currentFrame.lastState = lastState
		} else if e.runAutomaton.IsAccept(state) {
			e.copyTerm()
			return e.term, nil
		}
		// else: prefix of an accepted term but not itself accepted; continue.

		isSubBlock, err = e.popPushNext()
		if err != nil {
			return nil, err
		}
		continue

	continueNextTerm:
	}
}

// popPushNext pops exhausted frames and calls next() on the current frame.
//
// Port of IntersectTermsEnum.popPushNext.
func (e *IntersectTermsEnum) popPushNext() (bool, error) {
	for e.currentFrame.nextEnt == e.currentFrame.entCount {
		if !e.currentFrame.isLastInFloor {
			if err := e.currentFrame.loadNextFloorBlock(); err != nil {
				return false, err
			}
			break
		}
		if e.currentFrame.ord == 0 {
			// Exhausted root: no more terms.
			return false, errNoMoreTerms
		}
		e.currentFrame = e.stack[e.currentFrame.ord-1]
		*e.currentTransition = e.currentFrame.transition
	}
	return e.currentFrame.next(), nil
}

// errNoMoreTerms is a sentinel used internally by popPushNext to signal
// end-of-iteration, identical in purpose to Java's NoMoreTermsException.
var errNoMoreTerms = fmt.Errorf("blocktree intersect: no more terms")

// getFrame grows the stack to at least ord+1 entries and returns stack[ord].
func (e *IntersectTermsEnum) getFrame(ord int) (*intersectTermsEnumFrame, error) {
	if ord >= len(e.stack) {
		newLen := util.Oversize(ord+1, 8)
		grown := make([]*intersectTermsEnumFrame, newLen)
		copy(grown, e.stack)
		for i := len(e.stack); i < len(grown); i++ {
			grown[i] = newIntersectTermsEnumFrame(e, i)
		}
		e.stack = grown
	}
	return e.stack[ord], nil
}

// getArc grows the arcs slice to at least ord+1 entries and returns arcs[ord].
func (e *IntersectTermsEnum) getArc(ord int) *fst.Arc[*util.BytesRef] {
	if ord >= len(e.arcs) {
		newLen := util.Oversize(ord+1, 8)
		grown := make([]*fst.Arc[*util.BytesRef], newLen)
		copy(grown, e.arcs)
		for i := len(e.arcs); i < len(grown); i++ {
			grown[i] = &fst.Arc[*util.BytesRef]{}
		}
		e.arcs = grown
	}
	return e.arcs[ord]
}

// pushFrame pushes a new frame for the sub-block reachable at lastSubFP,
// loading floor data from the FST output.
//
// Port of IntersectTermsEnum.pushFrame(int).
func (e *IntersectTermsEnum) pushFrame(state int) (*intersectTermsEnumFrame, error) {
	f, err := e.getFrame(1 + e.currentFrame.ord)
	if err != nil {
		return nil, err
	}

	f.fp = e.currentFrame.lastSubFP
	f.fpOrig = f.fp
	f.prefix = e.currentFrame.prefix + e.currentFrame.suffix
	f.setState(state)

	// Walk FST arcs to get the floor data for this block.
	arc := e.currentFrame.arc
	idx := e.currentFrame.prefix
	outputs := fst.ByteSequenceOutputs()
	output := e.currentFrame.outputPrefix
	for idx < f.prefix {
		target := int(e.term.Bytes[idx]) & 0xFF
		nextArc, err2 := e.fr.fstIndex.FindTargetArc(
			target,
			arc,
			e.getArc(1+idx),
			e.fstReader,
		)
		if err2 != nil || nextArc == nil {
			break
		}
		arc = nextArc
		output = outputs.Add(output, arc.Output())
		idx++
	}
	f.arc = arc
	f.outputPrefix = output

	frameData := outputs.Add(output, arc.NextFinalOutput())
	if err3 := f.load(frameData); err3 != nil {
		return nil, fmt.Errorf("blocktree intersect pushFrame load: %w", err3)
	}
	return f, nil
}

// getState returns the automaton state after processing the current frame's
// suffix from the 2nd byte onward (the first byte was handled by the caller).
func (e *IntersectTermsEnum) getState() int {
	state := e.currentFrame.state
	for idx := 0; idx < e.currentFrame.suffix; idx++ {
		state = e.runAutomaton.Step(
			state,
			int(e.currentFrame.suffixBytes[e.currentFrame.startBytePos+idx])&0xFF,
		)
	}
	return state
}

// seekToStartTerm positions the enum at or just before startTerm so that the
// first call to Next() returns the first term >= startTerm that matches the
// automaton.
//
// Port of IntersectTermsEnum.seekToStartTerm.
func (e *IntersectTermsEnum) seekToStartTerm(startTerm *util.BytesRef) error {
	if startTerm.Length > len(e.term.Bytes) {
		e.term.Bytes = util.GrowExactByte(e.term.Bytes, util.Oversize(startTerm.Length, 1))
	}
	arc := e.arcs[0]

	for idx := 0; idx <= startTerm.Length; idx++ {
		for {
			saveNextEnt := e.currentFrame.nextEnt
			savePos := e.currentFrame.suffixesReader.GetPosition()
			saveLengthPos := e.currentFrame.suffixLengthsReader.GetPosition()
			saveStartBytePos := e.currentFrame.startBytePos
			saveSuffix := e.currentFrame.suffix
			saveLastSubFP := e.currentFrame.lastSubFP
			var saveTermBlockOrd int
			if e.currentFrame.termState != nil {
				saveTermBlockOrd = e.currentFrame.termState.TermBlockOrd
			}

			isSubBlock := e.currentFrame.next()

			newTermLen := e.currentFrame.prefix + e.currentFrame.suffix
			if newTermLen > len(e.term.Bytes) {
				e.term.Bytes = util.GrowExactByte(e.term.Bytes, util.Oversize(newTermLen, 1))
			}
			copy(
				e.term.Bytes[e.currentFrame.prefix:],
				e.currentFrame.suffixBytes[e.currentFrame.startBytePos:e.currentFrame.startBytePos+e.currentFrame.suffix],
			)
			e.term.Length = newTermLen

			if isSubBlock && bytes.HasPrefix(startTerm.Bytes[startTerm.Offset:startTerm.Offset+startTerm.Length], e.term.Bytes[:e.term.Length]) {
				// Recurse into this sub-block.
				subState := e.getState()
				f, err := e.pushFrame(subState)
				if err != nil {
					return err
				}
				e.currentFrame = f
				break
			}

			cmp := bytes.Compare(
				e.term.Bytes[:e.term.Length],
				startTerm.Bytes[startTerm.Offset:startTerm.Offset+startTerm.Length],
			)
			if cmp < 0 {
				// Keep scanning.
				if e.currentFrame.nextEnt == e.currentFrame.entCount {
					if !e.currentFrame.isLastInFloor {
						if err := e.currentFrame.loadNextFloorBlock(); err != nil {
							return err
						}
						continue
					}
					return nil
				}
				continue
			} else if cmp == 0 {
				return nil
			} else {
				// Restore prior entry.
				e.currentFrame.nextEnt = saveNextEnt
				e.currentFrame.lastSubFP = saveLastSubFP
				e.currentFrame.startBytePos = saveStartBytePos
				e.currentFrame.suffix = saveSuffix
				e.currentFrame.suffixesReader.SetPosition(savePos)            //nolint:errcheck
				e.currentFrame.suffixLengthsReader.SetPosition(saveLengthPos) //nolint:errcheck
				if e.currentFrame.termState != nil {
					e.currentFrame.termState.TermBlockOrd = saveTermBlockOrd
				}
				copy(
					e.term.Bytes[e.currentFrame.prefix:],
					e.currentFrame.suffixBytes[e.currentFrame.startBytePos:e.currentFrame.startBytePos+e.currentFrame.suffix],
				)
				e.term.Length = e.currentFrame.prefix + e.currentFrame.suffix
				return nil
			}
		}
		_ = arc // suppress "declared and not used" for arc when idx > 0
	}
	return nil
}

// copyTerm copies the current suffix + prefix into e.term.
func (e *IntersectTermsEnum) copyTerm() {
	l := e.currentFrame.prefix + e.currentFrame.suffix
	if len(e.term.Bytes) < l {
		e.term.Bytes = util.GrowExactByte(e.term.Bytes, util.Oversize(l, 1))
	}
	copy(
		e.term.Bytes[e.currentFrame.prefix:],
		e.currentFrame.suffixBytes[e.currentFrame.startBytePos:e.currentFrame.startBytePos+e.currentFrame.suffix],
	)
	e.term.Length = l
}

// compile-time assertion
var _ index.TermsEnum = (*IntersectTermsEnum)(nil)
