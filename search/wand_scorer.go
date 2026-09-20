// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
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
	scaled := math.Ldexp(float64(maxScore), scalingFactor)
	if scaled > float64(maxScaledScore) {
		return maxScaledScore
	}
	return int64(math.Ceil(scaled))
}

// scaleMinScore scales min competitive scores similarly but rounds down.
func scaleMinScore(minScore float32, scalingFactor int) int64 {
	scaled := math.Ldexp(float64(minScore), scalingFactor)
	return int64(math.Floor(scaled))
}

// WANDScorer implements the Block-Max WAND scoring logic.
type WANDScorer struct {
	scalingFactor       int
	minCompetitiveScore int64

	allScorers []Scorer

	// lead: list of scorers positioned on the current doc.
	lead      *wandLead
	doc       int
	leadScore float64

	// head: priority queue of scorers beyond the current doc.
	head DisiPriorityQueue
	// tail: max-heap of scorers behind the current doc.
	tail         []*DisiWrapper
	tailMaxScore int64
	tailSize     int

	cost           int64
	upTo           int
	minShouldMatch int
	freq           int
	scoreMode      ScoreMode
	leadCost       int64
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

	// To avoid accuracy issues with floating-point numbers, this scorer operates on scaled longs.
	// How do you choose the scaling factor? The thing is that we want to retain as many
	// significant bits as possible, but not too many, otherwise operations on longs would be more
	// precise than the equivalent operations on their unscaled counterparts and we might skip too
	// many hits. So we compute the maximum possible score produced by this scorer, which is the
	// sum of the maximum scores of each clause, and compute a scaling factor that would preserve
	// 24 bits of accuracy - the number of mantissa bits of single-precision floating-point
	// numbers.
	var scaling int
	if scoreMode == TOP_SCORES {
		var maxScoreSumDouble float64
		for _, scorer := range allScorers {
			if _, err := scorer.AdvanceShallow(0); err != nil {
				return nil, err
			}
			maxScore, err := scorer.GetMaxScore(NO_MORE_DOCS)
			if err != nil {
				return nil, err
			}
			maxScoreSumDouble += float64(maxScore)
		}
		maxScoreSum := float32(util.MathSumUpperBound(maxScoreSumDouble, len(allScorers)))
		scaling = scalingFactor(maxScoreSum)
	} else {
		scaling = 0
	}

	head := OfMaxSize(len(scorers))
	tail := make([]*DisiWrapper, len(scorers))

	ws := &WANDScorer{
		scalingFactor:       scaling,
		minCompetitiveScore: 0,
		allScorers:          allScorers,
		doc:                 -1,
		upTo:                -1,
		minShouldMatch:      minShouldMatch,
		scoreMode:           scoreMode,
		leadCost:            leadCost,
		head:                head,
		tail:                tail,
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
	if err := ws.pushBackLeads(target); err != nil {
		return 0, err
	}
	if _, err := ws.advanceHead(target); err != nil {
		return 0, err
	}

	if ws.scoreMode == TOP_SCORES && (ws.head.Top() == nil || ws.head.Top().doc > ws.upTo) {
		if err := ws.moveToNextBlock(target); err != nil {
			return 0, err
		}
	}

	headTop := ws.head.Top()
	if headTop == nil {
		ws.doc = NO_MORE_DOCS
		return ws.doc, nil
	}
	ws.doc = headTop.doc
	return ws.doc, nil
}

func (ws *WANDScorer) Score() (float32, error) {
	// we need to know about all matches
	if err := ws.advanceAllTail(); err != nil {
		return 0, err
	}
	var totalScore float64
	if ws.scoreMode != TOP_SCORES {
		for l := ws.lead; l != nil; l = l.next {
			sc0, err := l.wrapper.scorable.Score()
			if err != nil {
				return 0, err
			}
			totalScore += float64(sc0)
		}
	} else {
		totalScore = ws.leadScore
	}
	return float32(totalScore), nil
}

func (ws *WANDScorer) SetMinCompetitiveScore(minScore float32) {
	if ws.scoreMode != TOP_SCORES {
		return
	}
	ws.minCompetitiveScore = scaleMinScore(minScore, ws.scalingFactor)
}

// pushBackLeads moves disis that are in 'lead' back to the tail.
//
// Mirrors WANDScorer.pushBackLeads(int).
func (ws *WANDScorer) pushBackLeads(target int) error {
	for l := ws.lead; l != nil; l = l.next {
		s := l.wrapper
		evicted := ws.insertTailWithOverflow(s)
		if evicted != nil {
			doc, err := evicted.iterator.Advance(target)
			if err != nil {
				return err
			}
			evicted.doc = doc
			ws.head.Add(evicted)
		}
	}
	ws.lead = nil
	return nil
}

// advanceHead makes sure all disis in 'head' are on or after 'target'.
//
// Mirrors WANDScorer.advanceHead(int).
func (ws *WANDScorer) advanceHead(target int) (*DisiWrapper, error) {
	headTop := ws.head.Top()
	for headTop != nil && headTop.doc < target {
		evicted := ws.insertTailWithOverflow(headTop)
		if evicted != nil {
			doc, err := evicted.iterator.Advance(target)
			if err != nil {
				return nil, err
			}
			evicted.doc = doc
			headTop = ws.head.UpdateTopWith(evicted)
		} else {
			ws.head.Pop()
			headTop = ws.head.Top()
		}
	}
	return headTop, nil
}

// moveToNextBlock updates upTo and the maximum scores of sub scorers so that
// upTo is greater than or equal to the next candidate after target, that is,
// the top of 'head'.
//
// Mirrors WANDScorer.moveToNextBlock(int).
func (ws *WANDScorer) moveToNextBlock(target int) error {
	for ws.upTo < NO_MORE_DOCS {
		if ws.head.Size() == 0 {
			// All clauses could fit in the tail, which means that the sum of the
			// maximum scores of sub clauses is less than the minimum competitive score.
			// Move to the next block until this condition becomes false.
			target = max(target, ws.upTo+1)
			if err := ws.updateMaxScores(target); err != nil {
				return err
			}
		} else if ws.head.Top().doc > ws.upTo {
			// We have a next candidate but it's not in the current block. We need to
			// move to the next block in order to not miss any potential hits between
			// `target` and `head.top().doc`.
			if err := ws.updateMaxScores(target); err != nil {
				return err
			}
			break
		} else {
			break
		}
	}
	return nil
}

// updateMaxScores recomputes upTo and the scaled maximum scores of the sub
// scorers of 'head' and 'tail'.
//
// Mirrors WANDScorer.updateMaxScores(int).
func (ws *WANDScorer) updateMaxScores(target int) error {
	newUpTo := NO_MORE_DOCS
	// If we have entries in 'head', we treat them all as leads and take the minimum of their next
	// block boundaries as a next boundary.
	// We don't take entries in 'tail' into account on purpose: 'tail' is supposed to contain the
	// least score contributors, and taking them into account might not move the boundary fast
	// enough, so we'll waste CPU re-computing the next boundary all the time.
	// Likewise, we ignore clauses whose cost is greater than the lead cost to avoid recomputing
	// per-window max scores over and over again. In the event when this makes us compute upTo as
	// NO_MORE_DOCS, this scorer will effectively implement WAND rather than block-max WAND.
	var headErr error
	ws.head.All()(func(w *DisiWrapper) bool {
		if w.doc <= newUpTo && w.cost <= ws.leadCost {
			shallow, err := w.scorer.AdvanceShallow(w.doc)
			if err != nil {
				headErr = err
				return false
			}
			newUpTo = min(shallow, newUpTo)
		}
		return true
	})
	if headErr != nil {
		return headErr
	}

	// Only look at the tail if none of the `head` clauses had a block we could reuse and if its
	// cost is less than or equal to the lead cost.
	if newUpTo == NO_MORE_DOCS && ws.tailSize > 0 && ws.tail[0].cost <= ws.leadCost {
		shallow, err := ws.tail[0].scorer.AdvanceShallow(target)
		if err != nil {
			return err
		}
		newUpTo = shallow
		// upTo must be on or after the least `head` doc
		if headTop := ws.head.Top(); headTop != nil {
			newUpTo = max(newUpTo, headTop.doc)
		}
	}
	ws.upTo = newUpTo

	// Now update the max scores of clauses that are before upTo.
	ws.head.All()(func(w *DisiWrapper) bool {
		if w.doc <= ws.upTo {
			maxScore, err := w.scorer.GetMaxScore(newUpTo)
			if err != nil {
				headErr = err
				return false
			}
			w.scaledMaxScore = scaleMaxScore(maxScore, ws.scalingFactor)
		}
		return true
	})
	if headErr != nil {
		return headErr
	}

	ws.tailMaxScore = 0
	for i := 0; i < ws.tailSize; i++ {
		w := ws.tail[i]
		if _, err := w.scorer.AdvanceShallow(target); err != nil {
			return err
		}
		maxScore, err := w.scorer.GetMaxScore(ws.upTo)
		if err != nil {
			return err
		}
		w.scaledMaxScore = scaleMaxScore(maxScore, ws.scalingFactor)
		ws.upHeapMaxScore(i) // the heap might need to be reordered
		ws.tailMaxScore += w.scaledMaxScore
	}

	// We need to make sure that entries in 'tail' alone cannot match
	// a competitive hit.
	for ws.tailSize > 0 && ws.tailMaxScore >= ws.minCompetitiveScore {
		w := ws.popTail()
		doc, err := w.iterator.Advance(target)
		if err != nil {
			return err
		}
		w.doc = doc
		ws.head.Add(w)
	}
	return nil
}

// popTail pops the entry from 'tail' that has the greatest score contribution.
//
// Mirrors WANDScorer.popTail().
func (ws *WANDScorer) popTail() *DisiWrapper {
	result := ws.tail[0]
	ws.tailSize--
	ws.tail[0] = ws.tail[ws.tailSize]
	ws.downHeapMaxScore(0)
	return result
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

// advanceTail advances one tail entry to the current doc and then adds it to
// 'lead' or 'head' depending on whether it matches.
//
// Mirrors WANDScorer.advanceTail(DisiWrapper).
func (ws *WANDScorer) advanceTail(disi *DisiWrapper) error {
	doc, err := disi.iterator.Advance(ws.doc)
	if err != nil {
		return err
	}
	disi.doc = doc
	if disi.doc == ws.doc {
		return ws.addLead(disi)
	}
	ws.head.Add(disi)
	return nil
}

// advanceAllTail advances every entry of 'tail' to the current doc.
//
// Mirrors WANDScorer.advanceAllTail().
func (ws *WANDScorer) advanceAllTail() error {
	// We need to advance all clauses from the tail, save for the price of the
	// next one
	for i := ws.tailSize - 1; i >= 0; i-- {
		if err := ws.advanceTail(ws.tail[i]); err != nil {
			return err
		}
	}
	ws.tailSize = 0
	ws.tailMaxScore = 0
	return nil
}

// addLead moves a disi to 'lead'.
//
// Mirrors WANDScorer.addLead(DisiWrapper).
func (ws *WANDScorer) addLead(w *DisiWrapper) error {
	ws.lead = &wandLead{wrapper: w, next: ws.lead}
	ws.freq++
	if ws.scoreMode == TOP_SCORES {
		sc1, err := w.scorable.Score()
		if err != nil {
			return err
		}
		ws.leadScore += float64(sc1)
	}
	return nil
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

func (a *wandApproximation) DocIDRunEnd() (int, error) {
	return -1, nil
}

// GetMaxScore mirrors WANDScorer.getMaxScore(int).
func (ws *WANDScorer) GetMaxScore(upTo int) (float32, error) {
	var maxScoreSum float64
	for _, scorer := range ws.allScorers {
		if scorer.DocID() <= upTo {
			maxScore, err := scorer.GetMaxScore(upTo)
			if err != nil {
				return 0, err
			}
			maxScoreSum += float64(maxScore)
		}
	}
	return float32(util.MathSumUpperBound(maxScoreSum, len(ws.allScorers))), nil
}

// IntoBitSet mirrors the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0.
func (w *wandApproximation) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return DefaultIntoBitSet(w, upTo, bitSet, offset)
}
