// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

const (
	floatMantissaBits = 24
	maxScaledScore    = (1 << 24) - 1
)

// scalingFactor returns a scaling factor for the given float so that f * 2^scalingFactor
// would be in [2^23, 2^24).
func scalingFactor(f float32) int {
	if f < 0 {
		panic("scores must be positive or null")
	} else if f == 0 {
		return scalingFactor(math.SmallestNonzeroFloat32) + 1
	} else if math.IsInf(float64(f), 1) {
		return scalingFactor(math.MaxFloat32) - 1
	} else {
		d := float64(f)
		return floatMantissaBits - 1 - int(math.Floor(math.Log2(d)))
	}
}

// scaleMaxScore scales max scores in an unsigned integer to avoid overflows
// and floating-point arithmetic errors.
func scaleMaxScore(maxScore float32, scalingFactor int) int64 {
	scaled := math.Scalb(float64(maxScore), float64(scalingFactor))
	if scaled > float64(maxScaledScore) {
		return maxScaledScore
	}
	return int64(math.Ceil(scaled))
}

// scaleMinScore scales min competitive scores similarly but rounds down.
func scaleMinScore(minScore float32, scalingFactor int) int64 {
	scaled := math.Scalb(float64(minScore), float64(scalingFactor))
	return int64(math.Floor(scaled))
}

// WANDScorer implements the Block-Max WAND scoring logic.
type WANDScorer struct {
	scalingFactor int
	minCompetitiveScore int64

	allScorers []Scorer

	// lead: list of scorers positioned on the current doc.
	lead *wandLead
	doc  int
	leadScore float64

	// head: priority queue of scorers beyond the current doc.
	head *DisiPriorityQueue
	// tail: max-heap of scorers behind the current doc.
	tail []*DisiWrapper
	tailMaxScore int64
	tailSize     int

	cost int64
	upTo int
	minShouldMatch int
	freq int
	scoreMode ScoreMode
	leadCost int64
}

type wandLead struct {
	wrapper *DisiWrapper
	next    *wandLead
}

func NewWANDScorer(scorers []Scorer, minShouldMatch int, scoreMode ScoreMode, leadCost int64) (*WANDScorer, error) {
	if minShouldMatch >= len(scorers) {
		return nil, fmt.Errorf("minShouldMatch should be < the number of scorers")
	}

	allScorers := make([]Scorer, len(scorers))
	copy(allScorers, scorers)

	var scalingFactor int
	if scoreMode == TOP_SCORES {
		var maxScoreSumDouble float64
		for _, s := range allScorers {
			s.AdvanceShallow(0)
			maxScoreSumDouble += float64(s.GetMaxScore(NO_MORE_DOCS))
		}
		// Simplified sumUpperBound
		maxScoreSum := float32(maxScoreSumDouble)
		scalingFactor = scalingFactor(maxScoreSum)
	}

	head := NewDisiPriorityQueue(len(scorers))
	tail := make([]*DisiWrapper, len(scorers))

	ws := &WANDScorer{
		scalingFactor:  scalingFactor,
		minCompetitiveScore: 0,
		allScorers:     allScorers,
		doc:            -1,
		upTo:           -1,
		minShouldMatch: minShouldMatch,
		scoreMode:      scoreMode,
		leadCost:       leadCost,
		head:           head,
		tail:           tail,
	}

	for _, s := range allScorers {
		ws.addUnpositionedLead(NewDisiWrapper(s, true))
	}

	// Cost computation (simplified)
	var totalCost int64
	for _, s := range allScorers {
		totalCost += s.Iterator().Cost()
	}
	ws.cost = totalCost

	return ws, nil
}

func (ws *WANDScorer) addUnpositionedLead(w *DisiWrapper) {
	ws.lead = &wandLead{wrapper: w, next: ws.lead}
	ws.freq++
}

func (ws *WANDScorer) DocID() int {
	return ws.doc
}

func (ws *WANDScorer) NextDoc() (int, error) {
	return ws.Advance(ws.doc + 1)
}

func (ws *WANDScorer) Advance(target int) (int, error) {
	ws.pushBackLeads(target)
	ws.advanceHead(target)

	if ws.scoreMode == TOP_SCORES && (ws.head.Top() == nil || ws.head.Top().doc > ws.upTo) {
		ws.moveToNextBlock(target)
	}

	headTop := ws.head.Top()
	if headTop == nil {
		ws.doc = NO_MORE_DOCS
		return ws.doc, nil
	}
	ws.doc = headTop.doc
	return ws.doc, nil
}

func (ws *WANDScorer) Score() float32 {
	ws.advanceAllTail()
	var totalScore float64
	if ws.scoreMode != TOP_SCORES {
		for l := ws.lead; l != nil; l = l.next {
			totalScore += float64(l.wrapper.scorable.Score())
		}
	} else {
		totalScore = ws.leadScore
	}
	return float32(totalScore)
}

func (ws *WANDScorer) SetMinCompetitiveScore(minScore float32) {
	if ws.scoreMode != TOP_SCORES {
		return
	}
	ws.minCompetitiveScore = scaleMinScore(minScore, ws.scalingFactor)
}

func (ws *WANDScorer) pushBackLeads(target int) {
	for l := ws.lead; l != nil; l = l.next {
		w := l.wrapper
		evicted := ws.insertTailWithOverflow(w)
		if evicted != nil {
			evicted.doc = evicted.iterator.Advance(target)
			ws.head.Add(evicted)
		}
	}
	ws.lead = nil
}

func (ws *WANDScorer) advanceHead(target int) {
	headTop := ws.head.Top()
	for headTop != nil && headTop.doc < target {
		w := headTop
		ws.head.Pop()
		evicted := ws.insertTailWithOverflow(w)
		if evicted != nil {
			evicted.doc = evicted.iterator.Advance(target)
			headTop = ws.head.Top()
		} else {
			headTop = ws.head.Top()
		}
	}
}

func (ws *WANDScorer) moveToNextBlock(target int) {
	for ws.upTo < NO_MORE_DOCS {
		if ws.head.Size() == 0 {
			target = max(target, ws.upTo+1)
			ws.updateMaxScores(target)
		} else if ws.head.Top().doc > ws.upTo {
			target = max(target, ws.upTo+1)
			ws.updateMaxScores(target)
			break
		} else {
			break
		}
	}
}

func (ws *WANDScorer) updateMaxScores(target int) {
	for {
		newUpTo := NO_MORE_DOCS
		var changed bool
		for _, w := range ws.head.heap {
			if w.doc <= ws.upTo {
				changed = true
				doc := w.scorer.AdvanceShallow(ws.upTo + 1)
				w.doc = doc
				if doc < newUpTo {
					newUpTo = doc
				}
			} else if w.doc < newUpTo {
				newUpTo = w.doc
			}
		}

		if !changed {
			ws.upTo = newUpTo
			break
		}
		ws.upTo = newUpTo
		if ws.upTo == NO_MORE_DOCS {
			break
		}
	}
}

func (ws *WANDScorer) insertTailWithOverflow(s *DisiWrapper) *DisiWrapper {
	if ws.tailMaxScore+s.scaledMaxScore < ws.minCompetitiveScore || ws.tailSize+1 < ws.minShouldMatch {
		ws.addTail(s)
		ws.tailMaxScore += s.scaledMaxScore
		return nil
	} else if ws.tailSize == 0 {
		return s
	} else {
		top := ws.tail[0]
		if s.scaledMaxScore > top.scaledMaxScore {
			ws.tail[0] = s
			ws.downHeapMaxScore(0)
			ws.tailMaxScore = ws.tailMaxScore - top.scaledMaxScore + s.scaledMaxScore
			return top
		}
		return s
	}
}

func (ws *WANDScorer) addTail(s *DisiWrapper) {
	ws.tail[ws.tailSize] = s
	ws.upHeapMaxScore(ws.tailSize)
	ws.tailSize++
}

func (ws *WANDScorer) upHeapMaxScore(i int) {
	for i > 0 {
		p := (i - 1) / 2
		if ws.tail[i].scaledMaxScore > ws.tail[p].scaledMaxScore {
			ws.tail[i], ws.tail[p] = ws.tail[p], ws.tail[i]
			i = p
		} else {
			break
		}
	}
}

func (ws *WANDScorer) downHeapMaxScore(i int) {
	for {
		l := 2*i + 1
		r := 2*i + 2
		largest := i
		if l < ws.tailSize && ws.tail[l].scaledMaxScore > ws.tail[largest].scaledMaxScore {
			largest = l
		}
		if r < ws.tailSize && ws.tail[r].scaledMaxScore > ws.tail[largest].scaledMaxScore {
			largest = r
		}
		if largest != i {
			ws.tail[i], ws.tail[largest] = ws.tail[largest], ws.tail[i]
			i = largest
		} else {
			break
		}
	}
}

func (ws *WANDScorer) advanceAllTail() {
	for i := ws.tailSize - 1; i >= 0; i-- {
		w := ws.tail[i]
		w.doc = w.iterator.Advance(ws.doc)
		if w.doc == ws.doc {
			ws.addLead(w)
		} else {
			ws.head.Add(w)
		}
	}
	ws.tailSize = 0
	ws.tailMaxScore = 0
}

func (ws *WANDScorer) addLead(w *DisiWrapper) {
	ws.lead = &wandLead{wrapper: w, next: ws.lead}
	ws.freq++
	if ws.scoreMode == TOP_SCORES {
		ws.leadScore += float64(w.scorable.Score())
	}
}

func (ws *WANDScorer) Iterator() DocIdSetIterator {
	return &wandApproximation{ws: ws}
}

type wandApproximation struct {
	ws *WANDScorer
}

func (a *wandApproximation) NextDoc() (int, error) {
	return a.ws.Advance(a.ws.doc + 1)
}

func (a *wandApproximation) Advance(target int) (int, error) {
	return a.ws.Advance(target)
}

func (a *wandApproximation) DocID() int {
	return a.ws.doc
}

func (a *wandApproximation) Cost() int64 {
	return a.ws.cost
}

func (a *wandApproximation) DocIDRunEnd() int {
	return -1
}

func (ws *WANDScorer) GetMaxScore(upTo int) float32 {
	var sum float64
	for _, s := range ws.allScorers {
		if s.DocID() <= upTo {
			sum += float64(s.GetMaxScore(upTo))
		}
	}
	return float32(sum)
}
