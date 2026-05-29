// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/util"

// BulkScorer scores a range of documents at once.
//
// This is the Go port of org.apache.lucene.search.BulkScorer (Lucene 10.4.0).
// Only queries that have a more optimised means of scoring across a range of
// documents need to override the default behaviour; otherwise the
// DefaultBulkScorer is wrapped around the Scorer returned by Weight.Scorer.
type BulkScorer interface {
	// Score collects matching documents in [min, max) and returns an
	// estimation of the next matching document on or after max.
	//
	// The return value must be:
	//   - >= max,
	//   - NO_MORE_DOCS if there are no more matches,
	//   - <= the first matching document that is >= max otherwise.
	//
	// min is the minimum document to be considered for matching; all documents
	// strictly before this value must be ignored. acceptDocs filters the
	// documents allowed to match (nil means all documents are allowed). This
	// mirrors org.apache.lucene.search.BulkScorer.score(LeafCollector, Bits,
	// int, int).
	Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error)

	// Cost returns an upper bound on the number of documents this scorer may
	// match, the bulk-scorer analogue of DocIdSetIterator.Cost.
	Cost() int64
}

// DefaultBulkScorer wraps a Scorer and scores a window of documents by driving
// the scorer's iterator, exactly as Lucene's Weight.DefaultBulkScorer does.
type DefaultBulkScorer struct {
	scorer   Scorer
	iterator DocIdSetIterator
}

// NewDefaultBulkScorer creates a new DefaultBulkScorer over scorer.
func NewDefaultBulkScorer(scorer Scorer) *DefaultBulkScorer {
	return &DefaultBulkScorer{scorer: scorer, iterator: scorer}
}

// Score scores documents in [min, max), passing each matching document that is
// allowed by acceptDocs to collector, and returns the first document the
// scorer advanced to at or beyond max.
//
// Faithful port of Weight.DefaultBulkScorer.score(LeafCollector, Bits, int,
// int): it sets the scorer on the collector, positions the iterator at min,
// and collects until max.
func (bs *DefaultBulkScorer) Score(collector LeafCollector, acceptDocs util.Bits, min, max int) (int, error) {
	if err := collector.SetScorer(bs.scorer); err != nil {
		return 0, err
	}

	doc := bs.iterator.DocID()
	if doc < min {
		var err error
		doc, err = bs.iterator.Advance(min)
		if err != nil {
			return 0, err
		}
	}

	for doc < max {
		if acceptDocs == nil || acceptDocs.Get(doc) {
			if err := collector.Collect(doc); err != nil {
				return 0, err
			}
			// Surface a deferred scoring error (e.g. the block-join
			// child-matches-parent invariant) that Score, which returns only a
			// float32, cannot. Mirrors Lucene raising IllegalStateException from
			// Scorer.score() during collection.
			if rep, ok := bs.scorer.(ScoreErrorReporter); ok {
				if scoreErr := rep.ScoreError(); scoreErr != nil {
					return 0, scoreErr
				}
			}
		}
		var err error
		doc, err = bs.iterator.NextDoc()
		if err != nil {
			return 0, err
		}
	}

	return doc, nil
}

// Cost returns the underlying scorer's iteration cost.
func (bs *DefaultBulkScorer) Cost() int64 {
	return bs.scorer.Cost()
}

var _ BulkScorer = (*DefaultBulkScorer)(nil)
