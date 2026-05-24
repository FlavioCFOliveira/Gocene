// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.TermAutomatonScorer.
package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// EnumAndScorer holds a PostingsEnum for one term in a TermAutomatonQuery,
// together with its term ID and current position state.
//
// Mirrors org.apache.lucene.sandbox.search.TermAutomatonQuery.EnumAndScorer.
type EnumAndScorer struct {
	// TermID is the ordinal of this term in the automaton alphabet.
	TermID int
	// PosEnum is the PostingsEnum positioned on the current document.
	PosEnum index.PostingsEnum
	// PosLeft is the number of positions remaining in the current document.
	PosLeft int
	// Pos is the current position.
	Pos int
}

// TermAutomatonWeight carries the compiled automaton and similarity scorer
// used by TermAutomatonScorer.
//
// Mirrors org.apache.lucene.sandbox.search.TermAutomatonQuery.TermAutomatonWeight
// (only the fields consumed by TermAutomatonScorer are exposed here).
type TermAutomatonWeight struct {
	// Automaton is the compiled deterministic automaton over term IDs.
	Automaton *automaton.Automaton
}

// termRunAutomaton is a RunAutomaton whose alphabet size equals the number of
// distinct terms in the TermAutomatonQuery.
//
// Mirrors org.apache.lucene.sandbox.search.TermAutomatonScorer.TermRunAutomaton.
type termRunAutomaton struct {
	*automaton.RunAutomaton
}

// newTermRunAutomaton builds a termRunAutomaton from an automaton and a term count.
func newTermRunAutomaton(a *automaton.Automaton, termCount int) *termRunAutomaton {
	return &termRunAutomaton{RunAutomaton: automaton.NewRunAutomaton(a, termCount)}
}

// posState tracks which automaton states are active at a given position.
//
// Mirrors TermAutomatonScorer.PosState.
type posState struct {
	states []int
	count  int
}

func (p *posState) add(state int) {
	if p.count == len(p.states) {
		newStates := make([]int, max(p.count*2, 4))
		copy(newStates, p.states)
		p.states = newStates
	}
	p.states[p.count] = state
	p.count++
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TermAutomatonScorer is a search.Scorer that matches documents whose
// term-position sequences are accepted by a compiled automaton. It uses a
// two-pass approach: a DocID priority queue to efficiently advance to the
// next candidate document, and a position priority queue to merge-sort all
// positions within each candidate document.
//
// Mirrors org.apache.lucene.sandbox.search.TermAutomatonScorer.
type TermAutomatonScorer struct {
	subsOnDoc    []*EnumAndScorer
	docIDQueue   *util.PriorityQueue[*EnumAndScorer]
	posQueue     *util.PriorityQueue[*EnumAndScorer]
	runAutomaton *termRunAutomaton

	positions []posState
	posShift  int

	anyTermID int
	scorer    search.SimScorer
	norms     index.NumericDocValues

	numSubsOnDoc int
	cost         int64

	docID int
	freq  int

	originalSubsOnDoc []*EnumAndScorer
}

// NewTermAutomatonScorer constructs a TermAutomatonScorer.
//
//   - weight supplies the compiled automaton.
//   - subs are the per-term EnumAndScorer instances (nil entries are skipped).
//   - anyTermID is the wildcard term ID, or -1 if none.
//   - scorer is the similarity scorer.
//   - norms provides per-document norms (may be nil).
func NewTermAutomatonScorer(
	weight *TermAutomatonWeight,
	subs []*EnumAndScorer,
	anyTermID int,
	scorer search.SimScorer,
	norms index.NumericDocValues,
) (*TermAutomatonScorer, error) {
	docIDQueue, err := util.NewPriorityQueue[*EnumAndScorer](len(subs), func(a, b *EnumAndScorer) bool {
		return a.PosEnum.DocID() < b.PosEnum.DocID()
	})
	if err != nil {
		return nil, err
	}
	posQueue, err := util.NewPriorityQueue[*EnumAndScorer](len(subs), func(a, b *EnumAndScorer) bool {
		return a.Pos < b.Pos
	})
	if err != nil {
		return nil, err
	}

	s := &TermAutomatonScorer{
		runAutomaton:      newTermRunAutomaton(weight.Automaton, len(subs)),
		scorer:            scorer,
		norms:             norms,
		docIDQueue:        docIDQueue,
		posQueue:          posQueue,
		anyTermID:         anyTermID,
		subsOnDoc:         make([]*EnumAndScorer, len(subs)),
		positions:         make([]posState, 4),
		originalSubsOnDoc: subs,
		docID:             -1,
	}
	for i := range s.positions {
		s.positions[i].states = make([]int, 2)
	}

	// Collect non-nil subs into subsOnDoc; docIDQueue starts empty.
	// On the first NextDoc call, all subsOnDoc entries will be advanced and
	// then pushed to docIDQueue — mirroring Java's lazy initialisation.
	for _, sub := range subs {
		if sub != nil {
			s.cost += sub.PosEnum.Cost()
			s.subsOnDoc[s.numSubsOnDoc] = sub
			s.numSubsOnDoc++
		}
	}
	return s, nil
}

// popCurrentDoc pops all entries at the current minimum docID from docIDQueue
// into subsOnDoc.
func (s *TermAutomatonScorer) popCurrentDoc() {
	s.numSubsOnDoc = 0
	top := s.docIDQueue.Pop()
	s.subsOnDoc[s.numSubsOnDoc] = top
	s.numSubsOnDoc++
	s.docID = top.PosEnum.DocID()
	for s.docIDQueue.Size() > 0 && s.docIDQueue.Top().PosEnum.DocID() == s.docID {
		s.subsOnDoc[s.numSubsOnDoc] = s.docIDQueue.Pop()
		s.numSubsOnDoc++
	}
}

// pushCurrentDoc re-inserts all subsOnDoc into docIDQueue, including those
// positioned at NO_MORE_DOCS — this preserves the invariant that every
// sub remains in the queue so popCurrentDoc can detect exhaustion.
func (s *TermAutomatonScorer) pushCurrentDoc() {
	for i := 0; i < s.numSubsOnDoc; i++ {
		s.docIDQueue.Add(s.subsOnDoc[i])
	}
	s.numSubsOnDoc = 0
}

// DocID returns the current document ID.
func (s *TermAutomatonScorer) DocID() int { return s.docID }

// DocIDRunEnd returns docID+1.
func (s *TermAutomatonScorer) DocIDRunEnd() int { return s.docID + 1 }

// NextDoc advances to the next matching document.
func (s *TermAutomatonScorer) NextDoc() (int, error) {
	// Advance all sub-scorers that were positioned on the current doc.
	// Subs that reach NO_MORE_DOCS are kept (pushed back to docIDQueue)
	// so that popCurrentDoc can detect global exhaustion.
	for i := 0; i < s.numSubsOnDoc; i++ {
		sub := s.subsOnDoc[i]
		next, err := sub.PosEnum.NextDoc()
		if err != nil {
			return 0, err
		}
		if next != search.NO_MORE_DOCS {
			freq, err := sub.PosEnum.Freq()
			if err != nil {
				return 0, err
			}
			sub.PosLeft = freq - 1
			pos, err := sub.PosEnum.NextPosition()
			if err != nil {
				return 0, err
			}
			sub.Pos = pos
		}
	}
	s.pushCurrentDoc()
	return s.doNext()
}

// Advance advances to the first document at or beyond target.
func (s *TermAutomatonScorer) Advance(target int) (int, error) {
	// Advance the PQ entries that are behind target.
	if s.docIDQueue.Size() > 0 {
		top := s.docIDQueue.Top()
		for top.PosEnum.DocID() < target {
			advanced, err := top.PosEnum.Advance(target)
			if err != nil {
				return 0, err
			}
			if advanced != search.NO_MORE_DOCS {
				freq, err := top.PosEnum.Freq()
				if err != nil {
					return 0, err
				}
				top.PosLeft = freq - 1
				pos, err := top.PosEnum.NextPosition()
				if err != nil {
					return 0, err
				}
				top.Pos = pos
			}
			s.docIDQueue.UpdateTop()
			if s.docIDQueue.Size() == 0 {
				break
			}
			top = s.docIDQueue.Top()
		}
	}
	// Advance any subsOnDoc entries that are behind target.
	for i := 0; i < s.numSubsOnDoc; i++ {
		sub := s.subsOnDoc[i]
		advanced, err := sub.PosEnum.Advance(target)
		if err != nil {
			return 0, err
		}
		if advanced != search.NO_MORE_DOCS {
			freq, err := sub.PosEnum.Freq()
			if err != nil {
				return 0, err
			}
			sub.PosLeft = freq - 1
			pos, err := sub.PosEnum.NextPosition()
			if err != nil {
				return 0, err
			}
			sub.Pos = pos
		}
	}
	s.pushCurrentDoc()
	return s.doNext()
}

// Cost returns the total iteration cost.
func (s *TermAutomatonScorer) Cost() int64 { return s.cost }

// doNext iterates until a document with freq > 0 is found.
func (s *TermAutomatonScorer) doNext() (int, error) {
	for {
		s.popCurrentDoc()
		if s.docID == search.NO_MORE_DOCS {
			return s.docID, nil
		}
		if err := s.countMatches(); err != nil {
			return 0, err
		}
		if s.freq > 0 {
			return s.docID, nil
		}
		// Advance all sub-scorers on this doc to the next doc.
		for i := 0; i < s.numSubsOnDoc; i++ {
			sub := s.subsOnDoc[i]
			next, err := sub.PosEnum.NextDoc()
			if err != nil {
				return 0, err
			}
			if next != search.NO_MORE_DOCS {
				freq, err := sub.PosEnum.Freq()
				if err != nil {
					return 0, err
				}
				sub.PosLeft = freq - 1
				pos, err := sub.PosEnum.NextPosition()
				if err != nil {
					return 0, err
				}
				sub.Pos = pos
			}
		}
		s.pushCurrentDoc()
	}
}

// getPosition returns a pointer to the posState at the given absolute position.
func (s *TermAutomatonScorer) getPosition(pos int) *posState {
	return &s.positions[pos-s.posShift]
}

// shift zeroes out positions below pos and updates posShift.
func (s *TermAutomatonScorer) shift(pos int) {
	limit := pos - s.posShift
	for i := 0; i < limit; i++ {
		s.positions[i].count = 0
	}
	s.posShift = pos
}

// countMatches runs the automaton over all positions in the current document
// and sets s.freq to the number of accepting paths found.
func (s *TermAutomatonScorer) countMatches() error {
	s.freq = 0
	for i := 0; i < s.numSubsOnDoc; i++ {
		s.posQueue.Add(s.subsOnDoc[i])
	}

	lastPos := -1
	s.posShift = -1

	for s.posQueue.Size() != 0 {
		sub := s.posQueue.Pop()
		pos := sub.Pos

		if s.posShift == -1 {
			s.posShift = pos
		}

		// Grow positions array if needed.
		needed := pos + 1 - s.posShift
		if needed >= len(s.positions) {
			newLen := needed * 2
			if newLen < 8 {
				newLen = 8
			}
			newPositions := make([]posState, newLen)
			copy(newPositions, s.positions)
			for i := len(s.positions); i < newLen; i++ {
				newPositions[i].states = make([]int, 2)
			}
			s.positions = newPositions
		}

		// Advance any-term arcs from lastPos to pos.
		if lastPos != -1 && s.anyTermID != -1 {
			startLastPos := lastPos
			for lastPos < pos {
				posState := s.getPosition(lastPos)
				if posState.count == 0 && lastPos > startLastPos {
					lastPos = pos
					break
				}
				nextPosState := s.getPosition(lastPos + 1)
				for i := 0; i < posState.count; i++ {
					state := s.runAutomaton.Step(posState.states[i], s.anyTermID)
					if state != -1 {
						nextPosState.add(state)
					}
				}
				lastPos++
			}
		}

		posState := s.getPosition(pos)
		nextPosState := s.getPosition(pos + 1)

		// If both this slot and the next are empty, shift back to save memory.
		if posState.count == 0 && nextPosState.count == 0 {
			s.shift(pos)
			posState = s.getPosition(pos)
			nextPosState = s.getPosition(pos + 1)
		}

		// Match current token against all active states.
		for i := 0; i < posState.count; i++ {
			state := s.runAutomaton.Step(posState.states[i], sub.TermID)
			if state != -1 {
				nextPosState.add(state)
				if s.runAutomaton.IsAccept(state) {
					s.freq++
				}
			}
		}

		// Also start a new match from the initial state.
		state := s.runAutomaton.Step(0, sub.TermID)
		if state != -1 {
			nextPosState.add(state)
			if s.runAutomaton.IsAccept(state) {
				s.freq++
			}
		}

		// Re-enqueue this sub if it has more positions on the current doc.
		if sub.PosLeft > 0 {
			nextPos, err := sub.PosEnum.NextPosition()
			if err != nil {
				return err
			}
			sub.Pos = nextPos
			sub.PosLeft--
			s.posQueue.Add(sub)
		}

		lastPos = pos
	}

	// Reset active position slots.
	if lastPos >= 0 {
		limit := lastPos + 1 - s.posShift
		for i := 0; i <= limit && i < len(s.positions); i++ {
			s.positions[i].count = 0
		}
	}
	return nil
}

// GetOriginalSubsOnDoc returns the original sub-scorer slice (for explain).
func (s *TermAutomatonScorer) GetOriginalSubsOnDoc() []*EnumAndScorer {
	return s.originalSubsOnDoc
}

// Score returns the similarity score for the current document.
func (s *TermAutomatonScorer) Score() float32 {
	return s.scorer.Score(s.docID, float32(s.freq))
}

// GetMaxScore returns an upper bound on the score.
func (s *TermAutomatonScorer) GetMaxScore(_ int) float32 {
	return s.scorer.Score(s.docID, float32(1e30))
}

var _ search.Scorer = (*TermAutomatonScorer)(nil)
