// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// GC-1004: SpanScorer implementation
// SpanScorer scores span queries with support for sloppy frequency calculation.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanScorer.

import "math"

// SpanScorer scores span queries.
// It computes sloppy frequency based on span positions and supports
// similarity-based scoring with position-aware adjustments.
type SpanScorer struct {
	// spans is the underlying Spans iterator
	spans *Spans

	// simScorer is the similarity scorer for computing term weights
	simScorer SimScorer

	// score is the base score weight
	score float32

	// doc is the current document ID
	doc int

	// freq is the sloppy frequency (slop-adjusted)
	freq float32

	// numMatches is the number of matches in current document
	numMatches int

	// matchWidth is the width of the current span match
	matchWidth int

	// positioned tracks if spans are positioned on current doc
	positioned bool
}

// NewSpanScorer creates a new SpanScorer.
// Parameters:
//   - spans: the Spans iterator providing (doc, start, end) tuples
//   - score: base score weight for this scorer
func NewSpanScorer(spans *Spans, score float32) *SpanScorer {
	return &SpanScorer{
		spans:      spans,
		score:      score,
		doc:        -1,
		freq:       0,
		numMatches: 0,
		matchWidth: 0,
		positioned: false,
	}
}

// NewSpanScorerWithSimilarity creates a SpanScorer with similarity scoring.
func NewSpanScorerWithSimilarity(spans *Spans, score float32, simScorer SimScorer) *SpanScorer {
	return &SpanScorer{
		spans:      spans,
		simScorer:  simScorer,
		score:      score,
		doc:        -1,
		freq:       0,
		numMatches: 0,
		matchWidth: 0,
		positioned: false,
	}
}

// Score returns the score for the current document.
// The score is computed using the sloppy frequency and similarity scorer.
func (s *SpanScorer) Score() float32 {
	if s.simScorer != nil {
		// Use similarity scorer if available
		return s.simScorer.Score(s.doc, s.freq)
	}
	// Fallback to simple score * freq calculation
	return s.score * s.freq
}

// DocID returns the current document ID.
func (s *SpanScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next document.
// It also updates the frequency by calling setFreqCurrentDoc.
func (s *SpanScorer) NextDoc() (int, error) {
	doc, err := s.spans.NextDoc()
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = doc
	if doc != NO_MORE_DOCS {
		s.setFreqCurrentDoc()
	}
	return doc, nil
}

// Advance advances to the specified document.
func (s *SpanScorer) Advance(target int) (int, error) {
	doc, err := s.spans.Advance(target)
	if err != nil {
		return NO_MORE_DOCS, err
	}
	s.doc = doc
	if doc != NO_MORE_DOCS {
		s.setFreqCurrentDoc()
	}
	return doc, nil
}

// setFreqCurrentDoc computes the sloppy frequency for the current document.
// It iterates through all positions in the current document and accumulates
// the slop-adjusted frequency.
func (s *SpanScorer) setFreqCurrentDoc() {
	s.freq = 0
	s.numMatches = 0
	s.positioned = false

	if s.doc == NO_MORE_DOCS {
		return
	}

	// Start processing the current document
	s.doStartCurrentDoc()

	// Iterate through all positions in the current document
	for {
		// Get next position
		pos, err := s.spans.NextStartPosition()
		if err != nil {
			break
		}
		if pos == -1 {
			// No more positions in this document
			break
		}

		s.positioned = true

		// Calculate match width (span length)
		start := s.spans.StartPosition()
		end := s.spans.EndPosition()
		s.matchWidth = end - start

		// Process this span occurrence
		s.doCurrentSpans()

		// Increment match count
		s.numMatches++

		// Calculate sloppy frequency contribution
		// Formula: freq += 1.0 / (1.0 + matchWidth)
		// This penalizes wider spans (more slop)
		sloppyContribution := float32(1.0 / (1.0 + float64(s.matchWidth)))
		s.freq += sloppyContribution
	}

	// If no positions were found, set freq to 0
	if !s.positioned {
		s.freq = 0
	}

	s.doEndCurrentDoc()
}

// doStartCurrentDoc is called at the start of processing a document.
// Subclasses can override this hook.
func (s *SpanScorer) doStartCurrentDoc() {
	// Hook for subclasses
}

// doCurrentSpans is called for each span occurrence.
// Subclasses can override this hook to perform custom processing.
func (s *SpanScorer) doCurrentSpans() {
	// Hook for subclasses
}

// doEndCurrentDoc is called after all spans in the document are processed.
// Subclasses can override this hook.
func (s *SpanScorer) doEndCurrentDoc() {
	// Hook for subclasses
}

// Spans returns the underlying Spans.
func (s *SpanScorer) Spans() *Spans {
	return s.spans
}

// SloppyFreq returns the sloppy frequency (slop-adjusted frequency).
// This is the sum of 1/(1+matchWidth) for all matches.
func (s *SpanScorer) SloppyFreq() float32 {
	return s.freq
}

// NumMatches returns the number of matches in the current document.
func (s *SpanScorer) NumMatches() int {
	return s.numMatches
}

// MatchWidth returns the width of the last span match.
func (s *SpanScorer) MatchWidth() int {
	return s.matchWidth
}

// Cost returns the estimated cost of iterating through all documents.
func (s *SpanScorer) Cost() int64 {
	return s.spans.Cost()
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *SpanScorer) DocIDRunEnd() int {
	return s.spans.DocIDRunEnd()
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *SpanScorer) GetMaxScore(upTo int) float32 {
	// For span queries, we use the current score as an estimate
	// In practice, this could be improved with better heuristics
	return s.score * s.freq
}

// SetSimScorer sets the similarity scorer for this SpanScorer.
func (s *SpanScorer) SetSimScorer(simScorer SimScorer) {
	s.simScorer = simScorer
}

// GetSimScorer returns the similarity scorer.
func (s *SpanScorer) GetSimScorer() SimScorer {
	return s.simScorer
}

// Ensure SpanScorer implements Scorer
var _ Scorer = (*SpanScorer)(nil)

// SpanScorerUtils provides utility methods for span scoring.
var SpanScorerUtils = &spanScorerUtils{}

type spanScorerUtils struct{}

// ComputeSloppyFreq calculates sloppy frequency from match width.
// Formula: 1.0 / (1.0 + width)
// This penalizes wider matches (more slop).
func (u *spanScorerUtils) ComputeSloppyFreq(width int) float32 {
	if width < 0 {
		return 0
	}
	return float32(1.0 / (1.0 + float64(width)))
}

// ComputeSloppyFreqWithDecay calculates sloppy frequency with exponential decay.
// This is an alternative formula that uses exponential decay: k^width
// where k is typically around 0.95 (less aggressive penalty).
func (u *spanScorerUtils) ComputeSloppyFreqWithDecay(width int, k float64) float32 {
	if width < 0 {
		return 0
	}
	if k <= 0 || k >= 1 {
		k = 0.95
	}
	return float32(math.Pow(k, float64(width)))
}

// ComputePositionSlop calculates the slop between two positions.
// Slop is the distance between the expected and actual positions.
func (u *spanScorerUtils) ComputePositionSlop(expected, actual int) int {
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	return diff
}

// IsWithinSlop checks if the actual position is within maxSlop of expected.
func (u *spanScorerUtils) IsWithinSlop(expected, actual, maxSlop int) bool {
	return u.ComputePositionSlop(expected, actual) <= maxSlop
}
