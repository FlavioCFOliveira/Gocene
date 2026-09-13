// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/util"
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// invalidQueryMessage mirrors ToChildBlockJoinQuery.INVALID_QUERY_MESSAGE: it is
// reported when the supplied parent query in fact returns a child document.
const invalidQueryMessage = "Parent query must not match any docs besides parent filter. " +
	"Combine them as must (+) and must-not (-) clauses to find a problem doc. docID="

// ToChildBlockJoinScorer is a scorer for ToChildBlockJoinQuery.
// It matches child documents whose parent documents match the parent query.
//
// This is the Go port of Lucene's
// org.apache.lucene.search.join.ToChildBlockJoinQuery.ToChildBlockJoinScorer.
// Gocene flattens Lucene's Scorer + DocIdSetIterator into a single iterator, so
// the iteration logic of Lucene's inner DocIdSetIterator is implemented directly
// in NextDoc/Advance/DocID here.
type ToChildBlockJoinScorer struct {
	// BaseScorer carries the concrete members of the abstract classes
	// org.apache.lucene.search.Scorer and Scorable that this scorer does
	// not override.
	search.BaseScorer

	// weight is the parent weight
	weight *ToChildBlockJoinWeight

	// parentScorer is the scorer for parent documents
	parentScorer search.Scorer

	// parentBits is the bitset identifying parent documents
	parentBits util.BitSet

	// doScores reports whether the parent score should be computed and
	// propagated to children. Mirrors Lucene's doScores
	// (scoreMode.needsScores()); the legacy ScoreMode argument is reduced to
	// this boolean so behaviour matches Lucene, where ToChildBlockJoinQuery has
	// no per-child ScoreMode.
	doScores bool

	// boost is the query boost
	boost float32

	// parentScore caches the current parent's score (already includes boost).
	parentScore float32

	// childDoc is the current child document ID (-1 before the first NextDoc).
	childDoc int

	// parentDoc is the current parent document ID (0 before iteration, per
	// Lucene's ToChildBlockJoinScorer which initialises parentDoc = 0).
	parentDoc int
}

// NewToChildBlockJoinScorer creates a new ToChildBlockJoinScorer.
//
// doScores reports whether the parent score must be computed and propagated to
// each child. This is a faithful port of Lucene's
// ToChildBlockJoinScorer(parentScorer, parentBits, doScores): in Lucene
// doScores == scoreMode.needsScores() where scoreMode is the SEARCH-level mode
// passed to createWeight, NOT a per-child aggregation mode (ToChildBlockJoinQuery
// has none). Tying score propagation to the join's None/Avg/Max mode was the
// LUCENE-6588 bug (rmp #4762): a ToChild search that needs scores must still
// score its children even though the join's child-aggregation mode is None.
func NewToChildBlockJoinScorer(weight *ToChildBlockJoinWeight, parentScorer search.Scorer, parentBits util.BitSet, doScores bool, boost float32) *ToChildBlockJoinScorer {
	return &ToChildBlockJoinScorer{
		weight:       weight,
		parentScorer: parentScorer,
		parentBits:   parentBits,
		doScores:     doScores,
		boost:        boost,
		childDoc:     -1,
		parentDoc:    0,
	}
}

// DocID returns the current document ID (the current child).
func (s *ToChildBlockJoinScorer) DocID() int {
	return s.childDoc
}

// validateParentDoc detects mis-use where the supplied parent query in fact
// matches a child document (a non-parent). Mirrors
// ToChildBlockJoinScorer.validateParentDoc.
func (s *ToChildBlockJoinScorer) validateParentDoc() error {
	if s.parentDoc != search.NO_MORE_DOCS && !s.parentBits.Get(s.parentDoc) {
		return fmt.Errorf("%s%d", invalidQueryMessage, s.parentDoc)
	}
	return nil
}

// NextDoc advances to the next child document.
//
// Faithful port of the inner DocIdSetIterator.nextDoc() in Lucene's
// ToChildBlockJoinScorer: it walks the children of the current parent block
// one at a time, and when the block is exhausted advances the parent iterator
// (skipping parents with no children) to the first child of the next block.
func (s *ToChildBlockJoinScorer) NextDoc() (int, error) {
	for {
		if s.childDoc+1 == s.parentDoc {
			// Done iterating the children of this parent: advance the parent.
			for {
				next, err := s.parentScorer.Iterator().NextDoc()
				if err != nil {
					return 0, err
				}
				s.parentDoc = next
				if err := s.validateParentDoc(); err != nil {
					return 0, err
				}

				if s.parentDoc == 0 {
					// Degenerate but allowed: the first parent doc has no
					// children, so skip to the following parent.
					next, err = s.parentScorer.Iterator().NextDoc()
					if err != nil {
						return 0, err
					}
					s.parentDoc = next
					if err := s.validateParentDoc(); err != nil {
						return 0, err
					}
				}

				if s.parentDoc == search.NO_MORE_DOCS {
					s.childDoc = search.NO_MORE_DOCS
					return s.childDoc, nil
				}

				// First child of this parent block.
				s.childDoc = 1 + s.parentBits.PrevSetBit(s.parentDoc-1)

				if s.childDoc == s.parentDoc {
					// Parent with no children; continue to the next parent.
					continue
				}
				if s.childDoc < s.parentDoc {
					if s.doScores {
						parentScore, err := s.parentScorer.Score()
						if err != nil {
							return 0, err
						}
						s.parentScore = parentScore * s.boost
					}
					return s.childDoc, nil
				}
				// Degenerate but allowed: parent has no children.
			}
		}
		// Still inside the current parent block.
		s.childDoc++
		return s.childDoc, nil
	}
}

// Advance advances to the first child at or beyond childTarget.
//
// Faithful port of the inner DocIdSetIterator.advance(int) in Lucene's
// ToChildBlockJoinScorer.
func (s *ToChildBlockJoinScorer) Advance(childTarget int) (int, error) {
	if childTarget >= s.parentDoc {
		if childTarget == search.NO_MORE_DOCS {
			s.childDoc = search.NO_MORE_DOCS
			s.parentDoc = search.NO_MORE_DOCS
			return s.childDoc, nil
		}

		next, err := s.parentScorer.Iterator().Advance(childTarget + 1)
		if err != nil {
			return 0, err
		}
		s.parentDoc = next
		if err := s.validateParentDoc(); err != nil {
			return 0, err
		}

		if s.parentDoc == search.NO_MORE_DOCS {
			s.childDoc = search.NO_MORE_DOCS
			return s.childDoc, nil
		}

		// Scan to the first parent that actually has children.
		for {
			firstChild := s.parentBits.PrevSetBit(s.parentDoc-1) + 1
			if firstChild != s.parentDoc {
				if childTarget < firstChild {
					childTarget = firstChild
				}
				break
			}
			next, err = s.parentScorer.Iterator().NextDoc()
			if err != nil {
				return 0, err
			}
			s.parentDoc = next
			if err := s.validateParentDoc(); err != nil {
				return 0, err
			}
			if s.parentDoc == search.NO_MORE_DOCS {
				s.childDoc = search.NO_MORE_DOCS
				return s.childDoc, nil
			}
		}

		if s.doScores {
			parentScore, err := s.parentScorer.Score()
			if err != nil {
				return 0, err
			}
			s.parentScore = parentScore * s.boost
		}
	}

	s.childDoc = childTarget
	return s.childDoc, nil
}

// Score returns the score of the current child document: the parent score
// (which already includes the query boost). When doScores is false the parent
// score was never computed and remains zero.
func (s *ToChildBlockJoinScorer) Score() (float32, error) {
	return s.parentScore, nil
}

// GetMaxScore returns the maximum score for documents up to the given doc.
// Mirrors Lucene, which returns Float.POSITIVE_INFINITY.
func (s *ToChildBlockJoinScorer) GetMaxScore(upTo int) (float32, error) {
	return float32(math.Inf(1)), nil
}

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. Lucene's
// ToChildBlockJoinScorer does not override advanceShallow.
func (s *ToChildBlockJoinScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// Cost returns the estimated cost of this scorer (the parent iterator cost).
func (s *ToChildBlockJoinScorer) Cost() int64 {
	return s.parentScorer.Iterator().Cost()
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
//
// The block-join child stream is not a dense run, so report the next document
// after the current child (the most conservative, always-correct answer).
func (s *ToChildBlockJoinScorer) DocIDRunEnd() (int, error) {
	return s.childDoc + 1, nil
}

// GetParentDoc returns the current parent document ID.
func (s *ToChildBlockJoinScorer) GetParentDoc() int {
	return s.parentDoc
}

// GetChildren mirrors ToChildBlockJoinScorer.getChildren():
// Collections.singleton(new ChildScorable(parentScorer, "BLOCK_JOIN")).
func (s *ToChildBlockJoinScorer) GetChildren() ([]search.ChildScorable, error) {
	return []search.ChildScorable{{Child: s.parentScorer, Relationship: "BLOCK_JOIN"}}, nil
}

// Iterator mirrors ToChildBlockJoinScorer.iterator(). Gocene flattens Lucene's
// Scorer + inner DocIdSetIterator into this one type (see the type comment), so
// the scorer is its own iterator.
func (s *ToChildBlockJoinScorer) Iterator() search.DocIdSetIterator {
	return s
}

// Ensure ToChildBlockJoinScorer implements Scorer
var _ search.Scorer = (*ToChildBlockJoinScorer)(nil)

// ToParentBlockJoinScorer is a scorer for ToParentBlockJoinQuery.
// It matches parent documents that have children matching the child query.
//
// This is the Go port of Lucene's
// org.apache.lucene.search.join.ToParentBlockJoinQuery.BlockJoinScorer together
// with its inner ParentApproximation and Score helpers. Gocene flattens
// Lucene's Scorer + DocIdSetIterator into a single iterator, and Gocene child
// scorers are always exact (no TwoPhaseIterator), so this port implements the
// childTwoPhase == null branch.
type ToParentBlockJoinScorer struct {
	// BaseScorer carries the concrete members of the abstract classes
	// org.apache.lucene.search.Scorer and Scorable that this scorer does
	// not override.
	search.BaseScorer

	// weight is the parent weight
	weight *ToParentBlockJoinWeight

	// childScorer is the scorer for child documents (also the child approximation)
	childScorer search.Scorer

	// parentBits is the bitset identifying parent documents
	parentBits util.BitSet

	// scoreMode determines how child scores are combined
	scoreMode ScoreMode

	// boost is the query boost
	boost float32

	// doc is the current parent document ID (ParentApproximation.doc; -1 before
	// the first advance).
	doc int

	// parentScore is the aggregated score of the current parent's children
	// (already multiplied by boost). parentFreq is the number of children that
	// contributed, used to compute the Avg mode.
	parentScore float32
	parentFreq  int

	// scoreErr stores the block-join "child matches parent" invariant violation
	// detected during Score, so it can be surfaced by the search loop through
	// the search.ScoreErrorReporter interface (Score itself returns only a
	// float32). Reset on every Score call.
	scoreErr error

	// minCompChanged records that SetMinCompetitiveScore lowered the child's
	// competitive bound since the child iterator was last positioned. Gocene
	// flattens Lucene's child TwoPhaseIterator (whose matches() would re-confirm
	// the current child under the new threshold) into an exact iterator that has
	// already advanced (and confirmed) past the block during Score. To reproduce
	// Lucene's lazy re-confirmation, the next Advance re-advances the child to
	// its current docID so the WANDScorer re-evaluates competitiveness.
	minCompChanged bool
}

// childMatchesParentMessage mirrors the IllegalStateException message thrown by
// ToParentBlockJoinQuery.BlockJoinScorer.scoreChildDocs when the child query
// also matches a parent document.
const childMatchesParentMessage = "Child query must not match same docs with parent filter. " +
	"Combine them as must clauses (+) to find a problem doc. docId="

// NewToParentBlockJoinScorer creates a new ToParentBlockJoinScorer.
func NewToParentBlockJoinScorer(weight *ToParentBlockJoinWeight, childScorer search.Scorer, parentBits util.BitSet, scoreMode ScoreMode, boost float32) *ToParentBlockJoinScorer {
	return &ToParentBlockJoinScorer{
		weight:      weight,
		childScorer: childScorer,
		parentBits:  parentBits,
		scoreMode:   scoreMode,
		boost:       boost,
		doc:         -1,
	}
}

// DocID returns the current parent document ID.
func (s *ToParentBlockJoinScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next matching parent document.
//
// Faithful port of ParentApproximation.nextDoc, which delegates to advance.
func (s *ToParentBlockJoinScorer) NextDoc() (int, error) {
	return s.Advance(s.doc + 1)
}

// Advance advances to the first matching parent at or beyond target.
//
// Faithful port of ParentApproximation.advance: it positions the child scorer
// at the first child of the parent block containing target, then returns the
// next set parent bit after that child.
func (s *ToParentBlockJoinScorer) Advance(target int) (int, error) {
	if target >= s.parentBits.Length() {
		s.doc = search.NO_MORE_DOCS
		return s.doc, nil
	}

	firstChildTarget := 0
	if target != 0 {
		firstChildTarget = s.parentBits.PrevSetBit(target-1) + 1
	}

	childDoc := s.childScorer.DocID()

	// If SetMinCompetitiveScore tightened the child's threshold since the child
	// was last positioned, re-advance the child to its current doc so the
	// underlying WANDScorer re-confirms competitiveness under the new bound
	// (Gocene flattens Lucene's child TwoPhaseIterator re-confirmation; see the
	// minCompChanged field). A re-advance target below firstChildTarget is
	// raised to firstChildTarget so we never move the child backwards.
	if s.minCompChanged && childDoc != search.NO_MORE_DOCS && childDoc >= 0 {
		s.minCompChanged = false
		reTarget := childDoc
		if reTarget < firstChildTarget {
			reTarget = firstChildTarget
		}
		var err error
		childDoc, err = s.childScorer.Iterator().Advance(reTarget)
		if err != nil {
			return 0, err
		}
	}

	if childDoc < firstChildTarget {
		var err error
		childDoc, err = s.childScorer.Iterator().Advance(firstChildTarget)
		if err != nil {
			return 0, err
		}
	}

	if childDoc >= s.parentBits.Length()-1 {
		s.doc = search.NO_MORE_DOCS
		return s.doc, nil
	}

	s.doc = s.parentBits.NextSetBitBounded(childDoc + 1)
	return s.doc, nil
}

// Score returns the aggregated score of the current parent document.
//
// Faithful port of BlockJoinScorer.scoreChildDocs + the inner Score class: it
// iterates every child of the current parent block, combining their scores per
// the configured ScoreMode. The child query must never match the parent doc
// itself (the block-join invariant); that mis-use is reported as an error here,
// matching Lucene's IllegalStateException.
func (s *ToParentBlockJoinScorer) Score() (float32, error) {
	s.scoreErr = nil
	childDoc := s.childScorer.DocID()
	if childDoc >= s.doc {
		// Already scored (or no children before this parent).
		return s.parentScore, nil
	}

	s.parentScore = 0
	s.parentFreq = 0

	if s.scoreMode != None {
		// reset(firstChildScorer): seed with the first child's score.
		first, err := s.childScorer.Score()
		if err != nil {
			return 0, err
		}
		score := first
		freq := 1

		for {
			next, err := s.childScorer.Iterator().NextDoc()
			if err != nil {
				// search.Scorer.Score has no error return; surface via panic-free
				// fallback by stopping accumulation. Errors here are not expected
				// from in-memory child scorers, but guard defensively.
				break
			}
			childDoc = next
			if childDoc >= s.doc {
				break
			}
			childScore, err := s.childScorer.Score()
			if err != nil {
				return 0, err
			}
			freq++
			switch s.scoreMode {
			case Total, Avg:
				score += childScore
			case Min:
				if childScore < score {
					score = childScore
				}
			case Max:
				if childScore > score {
					score = childScore
				}
			}
		}

		if s.scoreMode == Avg {
			score /= float32(freq)
		}
		s.parentScore = score * s.boost
		s.parentFreq = freq
	} else {
		// ScoreMode.None: advance the child past the parent block so the
		// invariant check below sees the post-block position, mirroring the
		// scoring loop's net effect without computing any score.
		for childDoc < s.doc {
			next, err := s.childScorer.Iterator().NextDoc()
			if err != nil {
				break
			}
			childDoc = next
		}
	}

	// Block-join invariant: the child query must not match the parent document
	// itself. Faithful port of the check at the end of
	// ToParentBlockJoinQuery.BlockJoinScorer.scoreChildDocs — if the child
	// approximation landed exactly on the parent doc, the child query also
	// matched a parent, which is illegal. Java throws IllegalStateException
	// there; Score now has an error channel, so the violation is returned
	// directly. It is also recorded in scoreErr so that callers reaching for
	// the search.ScoreErrorReporter extension still observe it.
	if childDoc == s.doc {
		s.scoreErr = fmt.Errorf("%s%d, %T", childMatchesParentMessage, s.doc, s.childScorer)
		return 0, s.scoreErr
	}

	return s.parentScore, nil
}

// ScoreError returns the block-join "child matches parent" invariant violation
// detected by the most recent Score call, or nil. It satisfies
// search.ScoreErrorReporter so the search loop can surface the error that
// Score (which returns only a float32) cannot.
func (s *ToParentBlockJoinScorer) ScoreError() error {
	return s.scoreErr
}

// SetMinCompetitiveScore forwards the minimum competitive score to the child
// scorer when the score mode permits early termination.
//
// Faithful port of ToParentBlockJoinQuery.BlockJoinScorer.setMinCompetitiveScore:
// only ScoreMode.None and ScoreMode.Max forward the hint to the child scorer
// (Avg/Min/Total aggregate over all children, so a per-child threshold cannot
// safely skip). The child scorer honours the hint only if it implements the
// optional search.MinCompetitiveScorer interface (e.g. a TOP_SCORES
// ConstantScoreScorer or WANDScorer); otherwise the call is a no-op.
func (s *ToParentBlockJoinScorer) SetMinCompetitiveScore(minScore float32) error {
	if s.scoreMode == None || s.scoreMode == Max {
		if mc, ok := s.childScorer.(search.MinCompetitiveScorer); ok {
			if err := mc.SetMinCompetitiveScore(minScore); err != nil {
				return err
			}
			// Mark the child position stale so the next Advance re-confirms it
			// under the tightened threshold (see minCompChanged).
			s.minCompChanged = true
		}
	}
	return nil
}

// Cost returns the estimated cost of this scorer (the child iterator cost).
func (s *ToParentBlockJoinScorer) Cost() int64 {
	return s.childScorer.Iterator().Cost()
}

// GetMaxScore returns the maximum score for documents up to the given doc.
//
// Faithful port: ScoreMode.None defers to the child's max score; every other
// mode returns +Inf, because aggregating an unbounded number of children
// provides no tighter upper bound (Lucene returns Float.POSITIVE_INFINITY).
func (s *ToParentBlockJoinScorer) GetMaxScore(upTo int) (float32, error) {
	if s.scoreMode == None {
		return s.childScorer.GetMaxScore(upTo)
	}
	return float32(math.Inf(1)), nil
}

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. Lucene's
// ToParentBlockJoinScorer does not override advanceShallow.
func (s *ToParentBlockJoinScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
//
// Parent matches are not a dense run, so report one past the current parent.
func (s *ToParentBlockJoinScorer) DocIDRunEnd() (int, error) {
	return s.doc + 1, nil
}

// GetChildren mirrors BlockJoinScorer.getChildren():
// Collections.singleton(new ChildScorable(childScorer, "BLOCK_JOIN")).
func (s *ToParentBlockJoinScorer) GetChildren() ([]search.ChildScorable, error) {
	return []search.ChildScorable{{Child: s.childScorer, Relationship: "BLOCK_JOIN"}}, nil
}

// Iterator mirrors BlockJoinScorer.iterator(). Gocene flattens Lucene's Scorer
// + parent approximation into this one type (see the type comment), so the
// scorer is its own iterator.
func (s *ToParentBlockJoinScorer) Iterator() search.DocIdSetIterator {
	return s
}

// Ensure ToParentBlockJoinScorer implements Scorer and the optional
// MinCompetitiveScorer / ScoreErrorReporter extensions.
var (
	_ search.Scorer               = (*ToParentBlockJoinScorer)(nil)
	_ search.MinCompetitiveScorer = (*ToParentBlockJoinScorer)(nil)
	_ search.ScoreErrorReporter   = (*ToParentBlockJoinScorer)(nil)
)

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, util.FixedBitSet, int) in Apache Lucene

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, util.FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *ToChildBlockJoinScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, util.FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *ToParentBlockJoinScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// NextDocsAndScores carries the concrete body of Scorer.nextDocsAndScores in
// Apache Lucene 10.5.0, which ToChildBlockJoinScorer does not override.
func (s *ToChildBlockJoinScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// NextDocsAndScores carries the concrete body of Scorer.nextDocsAndScores in
// Apache Lucene 10.5.0, which BlockJoinScorer does not override.
func (s *ToParentBlockJoinScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}
