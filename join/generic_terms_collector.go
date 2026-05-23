// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrScoresNotAvailable is returned by GenericTermsCollector.GetScoresPerTerm
// when the underlying collector does not collect scores.
var ErrScoresNotAvailable = errors.New("scores are not available for this GenericTermsCollector")

// GenericTermsCollector is an interface extending search.Collector with two
// accessor methods for the collected terms and their per-term scores.
//
// Mirrors org.apache.lucene.search.join.GenericTermsCollector.
type GenericTermsCollector interface {
	search.Collector

	// GetCollectedTerms returns the BytesRefHash of collected terms.
	GetCollectedTerms() *util.BytesRefHash

	// GetScoresPerTerm returns per-term scores indexed by BytesRefHash ordinal.
	// Returns ErrScoresNotAvailable when the underlying mode does not collect scores.
	GetScoresPerTerm() ([]float32, error)
}

// wrappedTermsCollector adapts a TermsCollector (no-score) into a GenericTermsCollector.
type wrappedTermsCollector struct {
	inner search.Collector
	terms *util.BytesRefHash
}

// GetLeafCollector implements search.Collector.
func (w *wrappedTermsCollector) GetLeafCollector(reader search.IndexReader) (search.LeafCollector, error) {
	return w.inner.GetLeafCollector(reader)
}

// ScoreMode implements search.Collector.
func (w *wrappedTermsCollector) ScoreMode() search.ScoreMode {
	return w.inner.ScoreMode()
}

// GetCollectedTerms implements GenericTermsCollector.
func (w *wrappedTermsCollector) GetCollectedTerms() *util.BytesRefHash { return w.terms }

// GetScoresPerTerm implements GenericTermsCollector.
func (w *wrappedTermsCollector) GetScoresPerTerm() ([]float32, error) {
	return nil, ErrScoresNotAvailable
}

// scoringGenericTermsCollector wraps a scoring collector (SV/MV with scores).
type scoringGenericTermsCollector struct {
	inner  search.Collector
	terms  *util.BytesRefHash
	scores []float32
}

// GetLeafCollector implements search.Collector.
func (s *scoringGenericTermsCollector) GetLeafCollector(reader search.IndexReader) (search.LeafCollector, error) {
	return s.inner.GetLeafCollector(reader)
}

// ScoreMode implements search.Collector.
func (s *scoringGenericTermsCollector) ScoreMode() search.ScoreMode {
	return s.inner.ScoreMode()
}

// GetCollectedTerms implements GenericTermsCollector.
func (s *scoringGenericTermsCollector) GetCollectedTerms() *util.BytesRefHash { return s.terms }

// GetScoresPerTerm implements GenericTermsCollector.
func (s *scoringGenericTermsCollector) GetScoresPerTerm() ([]float32, error) {
	return s.scores, nil
}

// NewGenericTermsCollectorNoScore creates a GenericTermsCollector that wraps an
// existing search.Collector and returns the collected terms. Scores are not
// available from this variant.
func NewGenericTermsCollectorNoScore(collector search.Collector, terms *util.BytesRefHash) GenericTermsCollector {
	return &wrappedTermsCollector{inner: collector, terms: terms}
}

// NewGenericTermsCollectorWithScores creates a GenericTermsCollector that wraps
// a scoring collector and exposes per-term scores.
func NewGenericTermsCollectorWithScores(collector search.Collector, terms *util.BytesRefHash, scores []float32) GenericTermsCollector {
	return &scoringGenericTermsCollector{inner: collector, terms: terms, scores: scores}
}

// interface compliance
var _ GenericTermsCollector = (*wrappedTermsCollector)(nil)
var _ GenericTermsCollector = (*scoringGenericTermsCollector)(nil)
