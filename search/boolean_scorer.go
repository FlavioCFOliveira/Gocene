// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	booleanScorerShift = 12
	booleanScorerSize  = 1 << booleanScorerShift
	booleanScorerMask  = booleanScorerSize - 1
)

type bucket struct {
	score float64
	freq  int
}

type disiWrapper struct {
	scorer   Scorer
	iterator util.DocIdSetIterator
	doc      int
	cost     int64
}

func newDisiWrapper(scorer Scorer) *disiWrapper {
	return &disiWrapper{
		scorer:   scorer,
		iterator: scorer.Iterator(),
		doc:      scorer.DocID(),
		cost:     scorer.Iterator().Cost(),
	}
}

type BooleanScorer struct {
	buckets    []bucket
	matching   *util.FixedBitSet
	leads      []*disiWrapper
	head       *util.PriorityQueue[*disiWrapper]
	tail       *util.PriorityQueue[*disiWrapper]
	score      *SimpleScorable
	minShouldMatch int
	cost       int64
	needsScores    bool
	docAndScoreBuffer *DocAndFloatFeatureBuffer
}

// NewBooleanScorer creates a new BooleanScorer.
// Mirrors org.apache.lucene.search.BooleanScorer (Lucene 10.5.0).
func NewBooleanScorer(scorers []Scorer, minShouldMatch int, needsScores bool) (*BooleanScorer, error) {
	if minShouldMatch < 1 || minShouldMatch > len(scorers) {
		return nil, fmt.Errorf("minShouldMatch should be within 1..num_scorers. Got %d", minShouldMatch)
	}
	if len(scorers) <= 1 {
		return nil, fmt.Errorf("this scorer can only be used with two scorers or more, got %d", len(scorers))
	}

	var buckets []bucket
	if needsScores || minShouldMatch > 1 {
		buckets = make([]bucket, booleanScorerSize)
	}

	leads := make([]*disiWrapper, len(scorers))
	head, _ := util.NewPriorityQueue(len(scorers)-minShouldMatch+1, func(a, b *disiWrapper) bool {
		return a.doc < b.doc
	})
	tail, _ := util.NewPriorityQueue(minShouldMatch-1, func(a, b *disiWrapper) bool {
		return a.cost < b.cost
	})

	costs := make([]int64, 0, len(scorers))
	for _, s := range scorers {
		w := newDisiWrapper(s)
		costs = append(costs, w.cost)
		if evicted, overflow := tail.InsertWithOverflow(w); overflow {
			if evicted != nil {
				head.Add(evicted)
			}
		}
	}

	return &BooleanScorer{
		buckets:           buckets,
		matching:          func() *util.FixedBitSet {
			fs, _ := util.NewFixedBitSet(booleanScorerSize)
			return fs
		}(),
		leads:             leads,
		head:               head,
		tail:               tail,
		score:              &SimpleScorable{},
		minShouldMatch:     minShouldMatch,
		cost:               CostWithMinShouldMatch(costs, len(scorers), minShouldMatch),
		needsScores:        needsScores,
		docAndScoreBuffer:  NewDocAndFloatFeatureBuffer(),
	}, nil
}

func (bs *BooleanScorer) Cost() int64 {
	return bs.cost
}

func (bs *BooleanScorer) score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	collector.SetScorer(bs.score)

	top := bs.advance(min)
	for top.doc < max {
		var err error
		top, err = bs.scoreWindow(top, collector, acceptDocs, min, max)
		if err != nil {
			return top.doc, err
		}
	}

	return top.doc, nil
}

func (bs *BooleanScorer) advance(min int) *disiWrapper {
	headTop := bs.head.Top()
	tailTop := bs.tail.Top()
	for headTop.doc < min {
		if tailTop == nil || headTop.cost <= tailTop.cost {
			headTop.doc = bs.safeAdvance(headTop.iterator, min)
			bs.head.UpdateTop()
			headTop = bs.head.Top()
		} else {
			previousHeadTop := headTop
			tailTop.doc = bs.safeAdvance(tailTop.iterator, min)
			bs.head.UpdateTopWith(tailTop)
			headTop = bs.head.Top()
			bs.tail.UpdateTopWith(previousHeadTop)
			tailTop = bs.tail.Top()
		}
	}
	return headTop
}

func (bs *BooleanScorer) safeAdvance(it util.DocIdSetIterator, target int) int {
	doc, err := it.Advance(target)
	if err != nil {
		return util.NO_MORE_DOCS
	}
	return doc
}

func (bs *BooleanScorer) scoreWindow(top *disiWrapper, collector LeafCollector, acceptDocs util.Bits, min, max int) (*disiWrapper, error) {
	windowBase := top.doc & ^booleanScorerMask
	windowMin := min
	if windowBase > min {
		windowMin = windowBase
	}
	windowMax := windowBase + booleanScorerSize
	if max < windowMax {
		windowMax = max
	}

	bs.leads[0] = bs.head.Pop()
	maxFreq := 1
	for bs.head.Size() > 0 && bs.head.Top().doc < windowMax {
		bs.leads[maxFreq] = bs.head.Pop()
		maxFreq++
	}

	if bs.minShouldMatch == 1 && maxFreq == 1 {
		bulkScorer := bs.leads[0]
		if err := bs.scoreWindowSingleScorer(bulkScorer, collector, acceptDocs, windowMin, windowMax, max); err != nil {
			return bs.head.Top(), err
		}
		bs.head.Add(bulkScorer)
		return bs.head.Top(), nil
	}

	if err := bs.scoreWindowMultipleScorers(collector, acceptDocs, windowBase, windowMin, windowMax, maxFreq); err != nil {
		return bs.head.Top(), err
	}

	return bs.head.Top(), nil
}

func (bs *BooleanScorer) scoreWindowSingleScorer(w *disiWrapper, collector LeafCollector, acceptDocs util.Bits, windowMin, windowMax, max int) error {
	nextWindowBase := bs.head.Top().doc & ^booleanScorerMask
	end := windowMax
	if max < end {
		end = max
	}
	if nextWindowBase < end {
		end = nextWindowBase
	}

	it := w.iterator
	doc := w.doc
	if doc < windowMin {
		var err error
		doc, err = it.Advance(windowMin)
		if err != nil {
			return err
		}
	}

	collector.SetScorer(w.scorer)
	for doc < end {
		if acceptDocs == nil || acceptDocs.Get(doc) {
			if err := collector.Collect(doc); err != nil {
				return err
			}
		}
		var nextDoc int
		var err error
		nextDoc, err = it.NextDoc()
		if err != nil {
			return err
		}
		doc = nextDoc
	}
	w.doc = doc

	collector.SetScorer(bs.score)
	return nil
}

func (bs *BooleanScorer) scoreWindowMultipleScorers(collector LeafCollector, acceptDocs util.Bits, windowBase, windowMin, windowMax, maxFreq int) error {
	for maxFreq < bs.minShouldMatch && maxFreq+bs.tail.Size() >= bs.minShouldMatch {
		candidate := bs.tail.Pop()
		if candidate.doc < windowMin {
			var err error
			candidate.doc, err = candidate.iterator.Advance(windowMin)
			if err != nil {
				return err
			}
		}
		if candidate.doc < windowMax {
			bs.leads[maxFreq] = candidate
			maxFreq++
		} else {
			bs.head.Add(candidate)
		}
	}

	if maxFreq >= bs.minShouldMatch {
		for i := 0; i < bs.tail.Size(); i++ {
			val, _ := bs.tail.Get(i)
			bs.leads[maxFreq] = val
			maxFreq++
		}
		bs.tail.Clear()

		if err := bs.scoreWindowIntoBitSetAndReplay(collector, acceptDocs, windowBase, windowMin, windowMax, bs.leads, maxFreq); err != nil {
			return err
		}
	}

	for i := 0; i < maxFreq; i++ {
		if evicted, overflow := bs.head.InsertWithOverflow(bs.leads[i]); overflow {
			if evicted != nil {
				bs.tail.Add(evicted)
			}
		}
	}
	return nil
}

func (bs *BooleanScorer) scoreWindowIntoBitSetAndReplay(collector LeafCollector, acceptDocs util.Bits, base, min, max int, scorers []*disiWrapper, numScorers int) error {
	for i := 0; i < numScorers; i++ {
		w := scorers[i]
		it := w.iterator
		if w.doc < min {
			var err error
			w.doc, err = it.Advance(min)
			if err != nil {
				return err
			}
		}
		if bs.buckets == nil {
			// minShouldMatch=1 and scores not needed
			// In Lucene, this is it.intoBitSet(max, matching, base)
			// Since Gocene's FixedBitSet doesn't have intoBitSet, we iterate.
			for {
				doc, err := it.NextDoc()
				if err != nil {
					return err
				}
				if doc >= max {
					break
				}
				bs.matching.Set(doc & booleanScorerMask)
			}
		} else if bs.needsScores {
			for {
				err := w.scorer.NextDocsAndScores(max, acceptDocs, bs.docAndScoreBuffer)
				if err != nil {
					return err
				}
				if bs.docAndScoreBuffer.Size == 0 {
					break
				}
				for index := 0; index < bs.docAndScoreBuffer.Size; index++ {
					doc := bs.docAndScoreBuffer.Docs[index]
					score := bs.docAndScoreBuffer.Features[index]
					d := doc & booleanScorerMask
					bs.matching.Set(d)
					bucket := &bs.buckets[d]
					bucket.freq++
					bucket.score += float64(score)
				}
				// Reset buffer size for next call if it doesn't do it internally
				bs.docAndScoreBuffer.Size = 0
			}
		} else {
			// minShouldMatch > 1, scores not needed
			for {
				doc, err := it.NextDoc()
				if err != nil {
					return err
				}
				if doc >= max {
					break
				}
				if acceptDocs == nil || acceptDocs.Get(doc) {
					d := doc & booleanScorerMask
					bs.matching.Set(d)
					bs.buckets[d].freq++
				}
			}
		}
		w.doc = it.DocID()
	}

	if bs.buckets == nil {
		if acceptDocs != nil {
			// acceptDocs.applyMask(matching, base)
			// This is complex to implement if FixedBitSet doesn't have it.
			// For now, I'll iterate the matching bitset and check acceptDocs.
		}
		// collector.collect(new BitSetDocIdStream(matching, base));
		// I'll implement the replay manually here to avoid creating a new stream type.
		for i := 0; i < booleanScorerSize; i++ {
			if bs.matching.Get(i) {
				if acceptDocs == nil || acceptDocs.Get(base|i) {
					if err := collector.Collect(base | i); err != nil {
						return err
					}
				}
			}
		}
	} else {
		bitArray := bs.matching.GetBits()
		for idx, word := range bitArray {
			for word != 0 {
				ntz := bits.TrailingZeros64(word)
				indexInWindow := (idx << 6) | ntz
				if indexInWindow >= booleanScorerSize {
					break
				}
				bucket := &bs.buckets[indexInWindow]
				if bucket.freq >= bs.minShouldMatch {
					bs.score.SetScore(float32(bucket.score))
					if err := collector.Collect(base | indexInWindow); err != nil {
						return err
					}
				}
				bucket.freq = 0
				bucket.score = 0
				word &= ^(uint64(1) << ntz)
			}
		}
	}

	bs.matching.ClearAll()
	return nil
}
