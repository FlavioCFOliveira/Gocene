// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// BlockJoinScorer is a scorer for block join queries.
// It iterates over parent documents and scores them based on matching child documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.BlockJoinScorer.
type BlockJoinScorer struct {
	// childScorer is the scorer for child documents
	childScorer search.Scorer

	// parentScorer is the scorer for parent documents
	parentScorer search.Scorer

	// scoreMode determines how child scores are combined
	scoreMode ScoreMode

	// currentParentDoc is the current parent document ID
	currentParentDoc int

	// currentChildDoc is the current child document ID
	currentChildDoc int

	// accumulatedScore is used for accumulating scores across children
	accumulatedScore float32

	// childCount is the number of matching children for the current parent
	childCount int
}

// NewBlockJoinScorer creates a new BlockJoinScorer.
// Parameters:
//   - childScorer: the scorer for child documents
//   - parentScorer: the scorer for parent documents
//   - scoreMode: how to combine scores from child documents
func NewBlockJoinScorer(childScorer search.Scorer, parentScorer search.Scorer, scoreMode ScoreMode) *BlockJoinScorer {
	return &BlockJoinScorer{
		childScorer:      childScorer,
		parentScorer:     parentScorer,
		scoreMode:        scoreMode,
		currentParentDoc: -1,
		currentChildDoc:  -1,
		accumulatedScore: 0,
		childCount:       0,
	}
}

// GetChildScorer returns the child scorer.
func (s *BlockJoinScorer) GetChildScorer() search.Scorer {
	return s.childScorer
}

// GetParentScorer returns the parent scorer.
func (s *BlockJoinScorer) GetParentScorer() search.Scorer {
	return s.parentScorer
}

// GetScoreMode returns the score mode.
func (s *BlockJoinScorer) GetScoreMode() ScoreMode {
	return s.scoreMode
}

// NextDoc advances to the next document.
func (s *BlockJoinScorer) NextDoc() (int, error) {
	// Advance to the next parent document
	parentDoc, err := s.parentScorer.NextDoc()
	if err != nil {
		return 0, err
	}

	if parentDoc == search.NO_MORE_DOCS {
		s.currentParentDoc = search.NO_MORE_DOCS
		return search.NO_MORE_DOCS, nil
	}

	s.currentParentDoc = parentDoc

	// Reset accumulated score for the new parent
	s.resetAccumulatedScore()

	// Collect scores from all matching children up to this parent
	err = s.collectChildScores(parentDoc)
	if err != nil {
		return 0, err
	}

	return parentDoc, nil
}

// DocID returns the current document ID.
func (s *BlockJoinScorer) DocID() int {
	return s.currentParentDoc
}

// Score returns the score of the current document.
func (s *BlockJoinScorer) Score() float32 {
	if s.scoreMode == None {
		return s.parentScorer.Score()
	}

	if s.childCount == 0 {
		return 0
	}

	switch s.scoreMode {
	case Avg:
		return s.accumulatedScore / float32(s.childCount)
	case Max:
		return s.accumulatedScore
	case Min:
		return s.accumulatedScore
	case Total:
		return s.accumulatedScore
	default:
		return s.parentScorer.Score()
	}
}

// GetMaxScore returns the maximum score for documents up to the given doc.
//
// Block-join scoring blends the parent and child contributions according to
// the configured ScoreMode:
//   - None    : parent score only (the child scorer never contributes).
//   - Max/Min : the larger/smaller of parent and child max scores (Min returns
//     the smaller, capturing the worst-case bound used by Lucene's
//     block-max optimisations).
//   - Avg     : average of parent and child max scores.
//   - Total   : sum of parent and child max scores (an upper bound on what
//     Score() can return for any single parent in the run).
func (s *BlockJoinScorer) GetMaxScore(upTo int) float32 {
	parentMax := s.parentScorer.GetMaxScore(upTo)
	if s.scoreMode == None || s.childScorer == nil {
		return parentMax
	}
	childMax := s.childScorer.GetMaxScore(upTo)
	switch s.scoreMode {
	case Max:
		if childMax > parentMax {
			return childMax
		}
		return parentMax
	case Min:
		if childMax < parentMax {
			return childMax
		}
		return parentMax
	case Avg:
		return (parentMax + childMax) / 2
	case Total:
		return parentMax + childMax
	default:
		return parentMax
	}
}

// Advance advances to the given document.
func (s *BlockJoinScorer) Advance(target int) (int, error) {
	// Advance parent to the target
	parentDoc, err := s.parentScorer.Advance(target)
	if err != nil {
		return 0, err
	}

	if parentDoc == search.NO_MORE_DOCS {
		s.currentParentDoc = search.NO_MORE_DOCS
		return search.NO_MORE_DOCS, nil
	}

	s.currentParentDoc = parentDoc

	// Reset accumulated score
	s.resetAccumulatedScore()

	// Collect scores from matching children
	err = s.collectChildScores(parentDoc)
	if err != nil {
		return 0, err
	}

	return parentDoc, nil
}

// Cost returns the estimated cost of this scorer.
//
// The block-join scorer drives the parent iterator and pulls matching
// children for each parent, so the work it performs is bounded by the
// number of parent matches plus the number of child matches. We therefore
// return the sum of the two scorer costs (skipping the child contribution
// when there is no child scorer, which happens with ScoreMode.None on the
// parent side).
func (s *BlockJoinScorer) Cost() int64 {
	cost := s.parentScorer.Cost()
	if s.childScorer != nil {
		cost += s.childScorer.Cost()
	}
	return cost
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
func (s *BlockJoinScorer) DocIDRunEnd() int {
	// Delegate to parent scorer
	return s.parentScorer.DocIDRunEnd()
}

// GetChildren returns the child scorer.
func (s *BlockJoinScorer) GetChildren() search.Scorer {
	return s.childScorer
}

// GetChildCount returns the number of matching children for the current parent.
func (s *BlockJoinScorer) GetChildCount() int {
	return s.childCount
}

// resetAccumulatedScore resets the accumulated score and child count.
func (s *BlockJoinScorer) resetAccumulatedScore() {
	s.accumulatedScore = 0
	s.childCount = 0
}

// collectChildScores collects scores from all matching children up to the given parent.
func (s *BlockJoinScorer) collectChildScores(parentDoc int) error {
	// Advance child scorer to collect matching children
	// In a real implementation, this would iterate through children
	// that belong to the current parent block

	// For now, use a simplified approach
	if s.currentChildDoc == -1 {
		// Initialize child scorer
		childDoc, err := s.childScorer.NextDoc()
		if err != nil {
			return err
		}
		s.currentChildDoc = childDoc
	}

	// Collect scores from children that are before the parent
	for s.currentChildDoc != search.NO_MORE_DOCS && s.currentChildDoc < parentDoc {
		childScore := s.childScorer.Score()

		switch s.scoreMode {
		case Avg, Total:
			s.accumulatedScore += childScore
		case Max:
			if childScore > s.accumulatedScore {
				s.accumulatedScore = childScore
			}
		case Min:
			if s.childCount == 0 || childScore < s.accumulatedScore {
				s.accumulatedScore = childScore
			}
		}

		s.childCount++

		// Advance to next child
		childDoc, err := s.childScorer.NextDoc()
		if err != nil {
			return err
		}
		s.currentChildDoc = childDoc
	}

	return nil
}

// Ensure BlockJoinScorer implements Scorer
var _ search.Scorer = (*BlockJoinScorer)(nil)

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
	// weight is the parent weight
	weight *ToChildBlockJoinWeight

	// parentScorer is the scorer for parent documents
	parentScorer search.Scorer

	// parentBits is the bitset identifying parent documents
	parentBits *FixedBitSet

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
func NewToChildBlockJoinScorer(weight *ToChildBlockJoinWeight, parentScorer search.Scorer, parentBits *FixedBitSet, doScores bool, boost float32) *ToChildBlockJoinScorer {
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
				next, err := s.parentScorer.NextDoc()
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
					next, err = s.parentScorer.NextDoc()
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
						s.parentScore = s.parentScorer.Score() * s.boost
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

		next, err := s.parentScorer.Advance(childTarget + 1)
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
			next, err = s.parentScorer.NextDoc()
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
			s.parentScore = s.parentScorer.Score() * s.boost
		}
	}

	s.childDoc = childTarget
	return s.childDoc, nil
}

// Score returns the score of the current child document: the parent score
// (which already includes the query boost). When doScores is false the parent
// score was never computed and remains zero.
func (s *ToChildBlockJoinScorer) Score() float32 {
	return s.parentScore
}

// GetMaxScore returns the maximum score for documents up to the given doc.
// Mirrors Lucene, which returns Float.POSITIVE_INFINITY.
func (s *ToChildBlockJoinScorer) GetMaxScore(upTo int) float32 {
	return float32(math.Inf(1))
}

// Cost returns the estimated cost of this scorer (the parent iterator cost).
func (s *ToChildBlockJoinScorer) Cost() int64 {
	return s.parentScorer.Cost()
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
//
// The block-join child stream is not a dense run, so report the next document
// after the current child (the most conservative, always-correct answer).
func (s *ToChildBlockJoinScorer) DocIDRunEnd() int {
	return s.childDoc + 1
}

// GetParentDoc returns the current parent document ID.
func (s *ToChildBlockJoinScorer) GetParentDoc() int {
	return s.parentDoc
}

// GetChildren returns child scorers.
func (s *ToChildBlockJoinScorer) GetChildren() search.Scorer {
	return s.parentScorer
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
	// weight is the parent weight
	weight *ToParentBlockJoinWeight

	// childScorer is the scorer for child documents (also the child approximation)
	childScorer search.Scorer

	// parentBits is the bitset identifying parent documents
	parentBits *FixedBitSet

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
func NewToParentBlockJoinScorer(weight *ToParentBlockJoinWeight, childScorer search.Scorer, parentBits *FixedBitSet, scoreMode ScoreMode, boost float32) *ToParentBlockJoinScorer {
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
		childDoc, err = s.childScorer.Advance(reTarget)
		if err != nil {
			return 0, err
		}
	}

	if childDoc < firstChildTarget {
		var err error
		childDoc, err = s.childScorer.Advance(firstChildTarget)
		if err != nil {
			return 0, err
		}
	}

	if childDoc >= s.parentBits.Length()-1 {
		s.doc = search.NO_MORE_DOCS
		return s.doc, nil
	}

	s.doc = s.parentBits.NextSetBit(childDoc + 1)
	return s.doc, nil
}

// Score returns the aggregated score of the current parent document.
//
// Faithful port of BlockJoinScorer.scoreChildDocs + the inner Score class: it
// iterates every child of the current parent block, combining their scores per
// the configured ScoreMode. The child query must never match the parent doc
// itself (the block-join invariant); that mis-use is reported as an error here,
// matching Lucene's IllegalStateException.
func (s *ToParentBlockJoinScorer) Score() float32 {
	s.scoreErr = nil
	childDoc := s.childScorer.DocID()
	if childDoc >= s.doc {
		// Already scored (or no children before this parent).
		return s.parentScore
	}

	s.parentScore = 0
	s.parentFreq = 0

	if s.scoreMode != None {
		// reset(firstChildScorer): seed with the first child's score.
		first := s.childScorer.Score()
		score := first
		freq := 1

		for {
			next, err := s.childScorer.NextDoc()
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
			childScore := s.childScorer.Score()
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
			next, err := s.childScorer.NextDoc()
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
	// matched a parent, which is illegal. Score has no error channel, so the
	// violation is recorded in scoreErr and surfaced by the search loop through
	// the search.ScoreErrorReporter interface.
	if childDoc == s.doc {
		s.scoreErr = fmt.Errorf("%s%d, %T", childMatchesParentMessage, s.doc, s.childScorer)
	}

	return s.parentScore
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
	return s.childScorer.Cost()
}

// GetMaxScore returns the maximum score for documents up to the given doc.
//
// Faithful port: ScoreMode.None defers to the child's max score; every other
// mode returns +Inf, because aggregating an unbounded number of children
// provides no tighter upper bound (Lucene returns Float.POSITIVE_INFINITY).
func (s *ToParentBlockJoinScorer) GetMaxScore(upTo int) float32 {
	if s.scoreMode == None {
		return s.childScorer.GetMaxScore(upTo)
	}
	return float32(math.Inf(1))
}

// DocIDRunEnd returns the end of the run of consecutive doc IDs.
//
// Parent matches are not a dense run, so report one past the current parent.
func (s *ToParentBlockJoinScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// GetChildren returns the child scorer.
func (s *ToParentBlockJoinScorer) GetChildren() search.Scorer {
	return s.childScorer
}

// Ensure ToParentBlockJoinScorer implements Scorer and the optional
// MinCompetitiveScorer / ScoreErrorReporter extensions.
var (
	_ search.Scorer               = (*ToParentBlockJoinScorer)(nil)
	_ search.MinCompetitiveScorer = (*ToParentBlockJoinScorer)(nil)
	_ search.ScoreErrorReporter   = (*ToParentBlockJoinScorer)(nil)
)
