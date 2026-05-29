// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// synonymSub pairs a term's PostingsEnum with its per-term boost. It is the
// Go analogue of the (PostingsEnum, boost) tuple Lucene wraps in a
// DisiWrapperFreq inside SynonymQuery.SynonymWeight.
type synonymSub struct {
	postings index.PostingsEnum
	boost    float32
}

// SynonymScorer scores a SynonymQuery by treating its terms as a single
// synthetic term: it iterates the disjunction (union) of the per-term postings
// and, for each matching document, invokes the similarity a single time over
// the summed per-term frequencies.
//
// This is the Go port of the disjunction scorer built by
// org.apache.lucene.search.SynonymQuery.SynonymWeight (the SynonymScorer /
// DisiWrapperFreq machinery). Lucene's per-document combined frequency is
// sum(boost_i * freq_i) over every sub-iterator positioned on the document
// (SynonymScorer.freq()), and the document score is simScorer.score(freq, norm)
// (SynonymScorer.score()).
//
// Sentinel translation: index.PostingsEnum exhausts with index.NO_MORE_DOCS
// (-1); search.Scorer uses NO_MORE_DOCS (math.MaxInt32). SynonymScorer maps
// between the two via postingsDocToSearchDoc.
type SynonymScorer struct {
	*BaseScorer
	subs      []synonymSub
	simScorer SimScorer
	doc       int
}

// NewSynonymScorer creates a SynonymScorer over the supplied positioned-at-start
// sub-postings. The slice must be non-empty; an empty disjunction is reported by
// the weight as a nil scorer rather than constructed here.
func NewSynonymScorer(weight Weight, subs []synonymSub, simScorer SimScorer) *SynonymScorer {
	return &SynonymScorer{
		BaseScorer: NewBaseScorer(weight),
		subs:       subs,
		simScorer:  simScorer,
		doc:        -1,
	}
}

// DocID returns the current document ID in search-space.
func (s *SynonymScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the smallest document strictly greater than the current
// one that is reached by at least one sub-iterator.
func (s *SynonymScorer) NextDoc() (int, error) {
	target := s.doc + 1
	if s.doc == -1 {
		target = 0
	}
	return s.advanceInternal(target)
}

// Advance advances to the first document at or beyond target that is reached by
// at least one sub-iterator.
func (s *SynonymScorer) Advance(target int) (int, error) {
	return s.advanceInternal(target)
}

// advanceInternal positions every lagging sub-iterator at or beyond target and
// then selects the minimum document any sub currently sits on. It mirrors the
// behaviour of a disjunction DocIdSetIterator over the sub-postings.
func (s *SynonymScorer) advanceInternal(target int) (int, error) {
	minDoc := index.NO_MORE_DOCS
	for i := range s.subs {
		pe := s.subs[i].postings
		cur := pe.DocID()
		if cur < target {
			// FreqProxPostingsEnum cannot Advance(); scan forward sequentially,
			// matching PhraseWeight.postingsAdvanceTo.
			next, err := postingsAdvanceTo(pe, target)
			if err != nil {
				s.doc = NO_MORE_DOCS
				return NO_MORE_DOCS, err
			}
			cur = next
		}
		if cur != index.NO_MORE_DOCS && (minDoc == index.NO_MORE_DOCS || cur < minDoc) {
			minDoc = cur
		}
	}
	s.doc = postingsDocToSearchDoc(minDoc)
	return s.doc, nil
}

// Freq returns the combined synonym frequency for the current document: the sum
// of boost*freq over every sub-iterator positioned on that document. It ports
// SynonymQuery.SynonymScorer.freq().
func (s *SynonymScorer) Freq() float32 {
	if s.doc == -1 || s.doc == NO_MORE_DOCS {
		return 0
	}
	var freq float32
	for i := range s.subs {
		pe := s.subs[i].postings
		if postingsDocToSearchDoc(pe.DocID()) != s.doc {
			continue
		}
		f, err := pe.Freq()
		if err != nil {
			continue
		}
		freq += s.subs[i].boost * float32(f)
	}
	return freq
}

// Score returns the score of the current document. It invokes the similarity a
// single time over the combined synonym frequency, matching
// SynonymQuery.SynonymScorer.score(). Without a SimScorer (scores not needed)
// it falls back to the raw combined frequency.
func (s *SynonymScorer) Score() float32 {
	freq := s.Freq()
	if s.simScorer != nil {
		return s.simScorer.Score(s.doc, freq)
	}
	return freq
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *SynonymScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// Cost returns the estimated cost of iterating the disjunction: the sum of the
// per-sub costs, matching Lucene's accumulation in the scorer supplier.
func (s *SynonymScorer) Cost() int64 {
	var cost int64
	for i := range s.subs {
		cost += s.subs[i].postings.Cost()
	}
	return cost
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *SynonymScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// Ensure SynonymScorer implements Scorer.
var _ Scorer = (*SynonymScorer)(nil)
