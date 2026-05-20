// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/ReqExclBulkScorer.java

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// reqExclBulkScorer is a windowed bulk scorer that computes the set difference
// of a required scorer and an exclusion iterator: it scores only those documents
// that match req but NOT excl.
//
// The Java original is final and package-private; the Go port follows the same
// visibility model (unexported).
//
// Like disjunctionMaxBulkScorer, reqExclBulkScorer implements the internal
// windowedBulkScorer contract rather than the public BulkScorer interface, so
// it can participate in the per-window scoring pipeline used elsewhere in the
// search package.
//
// Ported from org.apache.lucene.search.ReqExclBulkScorer.
type reqExclBulkScorer struct {
	req          windowedBulkScorer
	exclApprox   DocIdSetIterator
	exclTwoPhase *TwoPhaseIterator
}

// newReqExclBulkScorerFromDISI creates a reqExclBulkScorer with a plain
// DocIdSetIterator as the exclusion source.
//
// Mirrors ReqExclBulkScorer(BulkScorer, DocIdSetIterator).
func newReqExclBulkScorerFromDISI(req windowedBulkScorer, excl DocIdSetIterator) *reqExclBulkScorer {
	return &reqExclBulkScorer{
		req:          req,
		exclApprox:   excl,
		exclTwoPhase: nil,
	}
}

// newReqExclBulkScorerFromTwoPhase creates a reqExclBulkScorer with a
// TwoPhaseIterator as the exclusion source.  The approximation is extracted
// for fast doc-ID advancement; the two-phase confirmer is applied when the
// approximation lands on a required document.
//
// Mirrors ReqExclBulkScorer(BulkScorer, TwoPhaseIterator).
func newReqExclBulkScorerFromTwoPhase(req windowedBulkScorer, excl *TwoPhaseIterator) *reqExclBulkScorer {
	return &reqExclBulkScorer{
		req:          req,
		exclApprox:   excl.Approximation(),
		exclTwoPhase: excl,
	}
}

// ScoreWindow scores documents in [min, max) excluding those matched by the
// exclusion DISI.  Returns the first document at or beyond max that req
// advanced to, or NO_MORE_DOCS when exhausted.
//
// The algorithm mirrors org.apache.lucene.search.ReqExclBulkScorer.score(
// LeafCollector, Bits, int, int) exactly:
//
//  1. Advance exclApprox to at least upTo.
//  2. If exclDoc == upTo: the doc is excluded.
//     — Without two-phase: use docIDRunEnd() to skip the whole run, then
//     advance exclDoc.
//     — With two-phase: if matches(), advance upTo by 1 and exclDoc; if
//     not matches(), only advance exclDoc (upTo stays, req can score it).
//  3. If exclDoc > upTo: the range [upTo, min(exclDoc,max)) is exclusion-free;
//     score req over that range with the outer collector directly.
func (s *reqExclBulkScorer) ScoreWindow(
	collector LeafCollector,
	acceptDocs util.Bits,
	min, max int,
) (int, error) {
	upTo := min
	exclDoc := s.exclApprox.DocID()

	for upTo < max {
		// Advance the exclusion approximation to at least upTo.
		if exclDoc < upTo {
			var err error
			exclDoc, err = s.exclApprox.Advance(upTo)
			if err != nil {
				return 0, err
			}
		}

		if exclDoc == upTo {
			// upTo is excluded.
			if s.exclTwoPhase == nil {
				// All docs in the run [upTo, runEnd) are excluded.
				// Note: Gocene's DocIDRunEnd() may advance the underlying iterator
				// to runEnd-1 as a side effect.  After calling it we re-read the
				// iterator position so nextDoc() starts from the correct place.
				runEnd := s.exclApprox.DocIDRunEnd()
				// Do NOT clamp to max here: if the run extends past the window
				// boundary, we advance upTo past max so the caller knows where
				// to resume.  This avoids re-processing excluded docs that fall
				// inside the run but beyond the current window.
				upTo = runEnd
				// Re-read exclDoc in case DocIDRunEnd() advanced the iterator
				// (Gocene's BitSetIterator.DocIDRunEnd has a position-advancing
				// side effect unlike the Java reference).
				_ = s.exclApprox.DocID() // ensure iterator is settled
			} else {
				matched, err := s.exclTwoPhase.Matches()
				if err != nil {
					return 0, err
				}
				if matched {
					// upTo is a confirmed exclusion; skip it.
					upTo++
				}
				// Advance exclDoc regardless of whether it confirmed.
			}
			var err error
			exclDoc, err = s.exclApprox.NextDoc()
			if err != nil {
				return 0, err
			}
		} else {
			// exclDoc > upTo: the range [upTo, min(exclDoc,max)) is exclusion-free.
			// Score req directly with the outer collector over that range.
			limit := exclDoc
			if limit > max {
				limit = max
			}
			var err error
			upTo, err = s.req.ScoreWindow(collector, acceptDocs, upTo, limit)
			if err != nil {
				return 0, err
			}
		}
	}

	// Flush: when upTo==max, prime the req iterator by scoring a zero-length
	// window so it is correctly positioned for the next call.
	if upTo == max {
		var err error
		upTo, err = s.req.ScoreWindow(collector, acceptDocs, upTo, upTo)
		if err != nil {
			return 0, err
		}
	}

	return upTo, nil
}

// Cost returns the cost of the required scorer (exclusions only reduce matches,
// never increase them).
func (s *reqExclBulkScorer) Cost() int64 { return s.req.Cost() }

// reqExclFullScorer wraps a reqExclBulkScorer to satisfy the public
// BulkScorer interface by driving the entire document space in one window.
type reqExclFullScorer struct {
	inner *reqExclBulkScorer
}

// Score implements BulkScorer by scoring all documents via a single window.
func (r *reqExclFullScorer) Score(collector Collector, acceptDocs DocIdSetIterator) error {
	lc, ok := collector.(LeafCollector)
	if !ok {
		return nil
	}
	_, err := r.inner.ScoreWindow(lc, nil, 0, math.MaxInt32)
	return err
}

// newReqExclBulkScorer exposes a public BulkScorer from a req windowedBulkScorer
// and a plain exclusion DocIdSetIterator.
func newReqExclBulkScorer(req windowedBulkScorer, excl DocIdSetIterator) BulkScorer {
	return &reqExclFullScorer{inner: newReqExclBulkScorerFromDISI(req, excl)}
}
