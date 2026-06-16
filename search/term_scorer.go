// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermScorer scores documents using a term's postings.
//
// This is the Go port of Lucene's org.apache.lucene.search.TermScorer.
//
// TermScorer iterates over the documents matching a term and scores them
// using the provided Similarity.SimScorer.
//
// liveDocs model: like Lucene 10.4.0, TermScorer's iterator visits EVERY
// document the postings enumerate, including documents deleted via a persisted
// .liv file. Deleted documents (acceptDocs == LeafReader.getLiveDocs()) are
// excluded by the collector layer, centrally in IndexSearcher.searchLeaf, not
// here. Filtering at the scorer would diverge from Lucene and would make
// join.QueryBitSetProducer drop deleted parents from the block-join parent
// bitset (rmp #4762).
//
// Sentinel translation: index.PostingsEnum uses index.NO_MORE_DOCS (-1) as
// its exhaustion sentinel, while search.DocIdSetIterator uses NO_MORE_DOCS
// (math.MaxInt32). TermScorer bridges the two by mapping -1 → NO_MORE_DOCS
// on every return from NextDoc and Advance.
type TermScorer struct {
	*BaseScorer
	postingsEnum index.PostingsEnum
	doc          int
	simScorer    SimScorer
	norms        index.NumericDocValues
	// maxScoreCache derives per-block maximum scores from the term's impacts.
	// It is nil only when no SimScorer is available (needsScores == false), in
	// which case GetMaxScore/AdvanceShallow fall back to the BaseScorer default.
	maxScoreCache *MaxScoreCache
}

// NewTermScorer creates a new TermScorer.
//
// Following Lucene 10.4.0's TermScorer(PostingsEnum, SimScorer, NumericDocValues)
// constructor, the postings enum is wrapped in an index.SlowImpactsEnum so that
// the scorer exposes a legal ImpactsSource even when the search-layer TermsEnum
// does not surface a codec-backed ImpactsEnum. SlowImpactsEnum reports a single
// impact (freq=MaxInt32, norm=1) spanning the whole remaining postings list,
// which makes MaxScoreCache.GetMaxScore return the global upper bound
// simScorer.Score(MaxInt32) — a correct, conservative block-max bound. When a
// codec exposes real per-block impacts in the future, passing that ImpactsEnum
// in place of the SlowImpactsEnum yields tight per-block bounds with no other
// change to this type.
func NewTermScorer(weight Weight, postingsEnum index.PostingsEnum, simScorer SimScorer, norms index.NumericDocValues) *TermScorer {
	s := &TermScorer{
		BaseScorer:   NewBaseScorer(weight),
		postingsEnum: postingsEnum,
		doc:          -1,
		simScorer:    simScorer,
		norms:        norms,
	}
	if simScorer != nil && postingsEnum != nil {
		s.maxScoreCache = newTermMaxScoreCache(postingsEnum, simScorer)
	}
	return s
}

// newTermMaxScoreCache builds a MaxScoreCache over the postings enum's impacts.
//
// The postings enum handed to NewTermScorer comes from TermsEnum.Postings (not
// TermsEnum.Impacts), so it does NOT carry usable per-block impacts: codec enums
// such as Lucene104's blockPostingsEnum only decode impacts when obtained via
// the postings reader's Impacts(...) entry point (their needsImpacts flag is
// false otherwise, and GetImpacts() returns an empty buffer). This is exactly
// the situation Lucene's TermScorer(PostingsEnum, SimScorer, NumericDocValues)
// constructor handles by wrapping the enum in a SlowImpactsEnum:
//
//	ImpactsEnum impactsEnum = new SlowImpactsEnum(postingsEnum);
//	maxScoreCache = new MaxScoreCache(impactsEnum, scorer);
//
// SlowImpactsEnum reports a single impact (freq=Integer.MAX_VALUE, norm=1)
// spanning the whole remaining postings list, so GetMaxScore returns the global
// upper bound simScorer.Score(MAX_VALUE) — a correct, conservative block-max
// bound that never under-estimates any document score. We therefore always wrap
// here rather than type-asserting to index.ImpactsEnum, which would pick up the
// codec enum's empty (zero) impacts and yield an invalid bound of 0. When the
// scorer is one day built from a TermsEnum.Impacts enum (real per-block skip
// data), that enum can be passed straight to newIndexImpactsSource for tight
// bounds with no other change.
//
// A construction error (the initial impacts snapshot could not be read) leaves
// the cache nil and the scorer falls back to the BaseScorer default.
func newTermMaxScoreCache(postingsEnum index.PostingsEnum, simScorer SimScorer) *MaxScoreCache {
	impactsEnum := index.NewSlowImpactsEnum(postingsEnum)
	src, err := newIndexImpactsSource(impactsEnum)
	if err != nil {
		return nil
	}
	return NewMaxScoreCache(src, newLegacySimImpactScorer(simScorer))
}

// postingsDocToSearchDoc translates index.NO_MORE_DOCS (-1) to the search
// package sentinel (NO_MORE_DOCS = math.MaxInt32). All other values are
// returned as-is.
func postingsDocToSearchDoc(doc int) int {
	if doc == index.NO_MORE_DOCS {
		return NO_MORE_DOCS
	}
	return doc
}

// NextDoc advances to the next document the postings enumerate (deleted docs
// included; liveDocs is applied centrally by the collector, see the type doc).
func (s *TermScorer) NextDoc() (int, error) {
	if s.postingsEnum == nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	nextDoc, err := s.postingsEnum.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = postingsDocToSearchDoc(nextDoc)
	return s.doc, nil
}

// DocID returns the current document ID.
func (s *TermScorer) DocID() int {
	return s.doc
}

// Advance advances to the first document at or beyond the target (deleted docs
// included; liveDocs is applied centrally by the collector, see the type doc).
func (s *TermScorer) Advance(target int) (int, error) {
	if s.postingsEnum == nil {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	advancedDoc, err := s.postingsEnum.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = postingsDocToSearchDoc(advancedDoc)
	return s.doc, nil
}

// Score returns the score of the current document.
func (s *TermScorer) Score() float32 {
	if s.simScorer == nil {
		return 1.0
	}
	freq, err := s.postingsEnum.Freq()
	if err != nil {
		return 0.0
	}
	norm := int64(1)
	if s.norms != nil {
		if ok, err := s.norms.AdvanceExact(s.doc); err == nil && ok {
			if v, err := s.norms.LongValue(); err == nil {
				norm = v
			}
		}
	}
	return s.simScorer.Score(s.doc, float32(freq), norm)
}

// GetMaxScore returns an upper bound on the score of any document from the last
// shallow-advanced target up to and including upTo, computed from the term's
// impacts via the MaxScoreCache. It ports
// org.apache.lucene.search.TermScorer.getMaxScore, which delegates to
// maxScoreCache.getMaxScore(upTo).
//
// When no SimScorer is wired (needsScores == false) there is no cache; the
// inherited BaseScorer default (1.0) is returned, matching the constant score
// the scorer produces in that mode.
func (s *TermScorer) GetMaxScore(upTo int) float32 {
	if s.maxScoreCache == nil {
		return s.BaseScorer.GetMaxScore(upTo)
	}
	return s.maxScoreCache.GetMaxScore(upTo)
}

// AdvanceShallow advances the impacts source to the block containing target and
// returns the inclusive upper doc id that shares target's block-max bound. It
// ports org.apache.lucene.search.TermScorer.advanceShallow, which delegates to
// maxScoreCache.advanceShallow(target).
//
// When no SimScorer is wired there is no cache; the inherited BaseScorer default
// (NO_MORE_DOCS) is returned.
func (s *TermScorer) AdvanceShallow(target int) (int, error) {
	if s.maxScoreCache == nil {
		return s.BaseScorer.AdvanceShallow(target)
	}
	return s.maxScoreCache.AdvanceShallow(target)
}

// Freq returns the term frequency of the current document, mirroring Lucene's
// org.apache.lucene.search.TermScorer.freq(). It is used by TermWeight.Explain
// to build the frequency sub-explanation. Returns 0 if the underlying postings
// enum cannot supply a frequency.
func (s *TermScorer) Freq() (int, error) {
	if s.postingsEnum == nil {
		return 0, nil
	}
	return s.postingsEnum.Freq()
}

// Cost returns the estimated cost of iterating through all documents.
func (s *TermScorer) Cost() int64 {
	if s.postingsEnum == nil {
		return 0
	}
	return s.postingsEnum.Cost()
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *TermScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// Ensure TermScorer implements Scorer
var _ Scorer = (*TermScorer)(nil)
