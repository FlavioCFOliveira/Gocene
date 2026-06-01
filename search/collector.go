// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// LeafCollector collects matching documents in a segment.
//
// This is the Go port of Lucene's org.apache.lucene.search.LeafCollector.
type LeafCollector interface {
	// SetScorer sets the scorer for this collector.
	SetScorer(scorer Scorer) error

	// Collect collects the given document.
	Collect(doc int) error
}

// Collector collects matching documents during search.
//
// This is the Go port of Lucene's org.apache.lucene.search.Collector.
//
// Collector is the base class for all collectors. Collectors receive
// matching documents and typically store them in some data structure.
// Common collectors include TopDocsCollector (for top-N results) and
// TotalHitCountCollector (for counting total hits).
type Collector interface {
	// GetLeafCollector returns a LeafCollector for the given leaf reader
	// context. The context carries the segment's docBase, ordinal and reader,
	// so collectors that need to rebase document ids or bind to the segment's
	// DocValues can do so without the searcher poking at the returned leaf
	// collector afterwards.
	//
	// This mirrors org.apache.lucene.search.Collector#getLeafCollector, which
	// takes a LeafReaderContext. A collector may return a
	// CollectionTerminatedException (as an error) to signal that it does not
	// need the given segment; MultiCollector and the search loop detect this
	// with IsCollectionTerminated.
	GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error)

	// ScoreMode returns the ScoreMode indicating how scores are needed.
	ScoreMode() ScoreMode
}

// ScoreMode indicates how scores are needed by the collector.
type ScoreMode int

const (
	// COMPLETE - scores are needed and must be complete.
	COMPLETE ScoreMode = iota
	// COMPLETE_NO_SCORES - scores are not needed.
	COMPLETE_NO_SCORES
	// TOP_SCORES - only top scores are needed.
	TOP_SCORES
	// TOP_DOCS - only top docs are needed (no scores).
	TOP_DOCS
)

// needsScores reports whether this score mode requires document scores.
//
// This mirrors org.apache.lucene.search.ScoreMode#needsScores: COMPLETE and
// TOP_SCORES need scores, while COMPLETE_NO_SCORES and TOP_DOCS do not.
func (m ScoreMode) needsScores() bool {
	return m == COMPLETE || m == TOP_SCORES
}

// String returns the canonical name of the score mode, matching the constant
// names used by org.apache.lucene.search.ScoreMode so test diagnostics read the
// same as the Lucene source.
func (m ScoreMode) String() string {
	switch m {
	case COMPLETE:
		return "COMPLETE"
	case COMPLETE_NO_SCORES:
		return "COMPLETE_NO_SCORES"
	case TOP_SCORES:
		return "TOP_SCORES"
	case TOP_DOCS:
		return "TOP_DOCS"
	default:
		return "ScoreMode(" + strconv.Itoa(int(m)) + ")"
	}
}

// isExhaustive reports whether this score mode requires processing all matching
// documents (true) rather than allowing dynamic pruning down to the top hits
// (false).
//
// This mirrors org.apache.lucene.search.ScoreMode#isExhaustive: COMPLETE and
// COMPLETE_NO_SCORES are exhaustive, while TOP_SCORES and TOP_DOCS are not
// (they may skip non-competitive hits). It is consulted by ConstantScoreQuery
// when choosing the ScoreMode to forward to its wrapped query.
func (m ScoreMode) isExhaustive() bool {
	return m == COMPLETE || m == COMPLETE_NO_SCORES
}

// SimpleCollector provides a base implementation for collectors.
type SimpleCollector struct {
	scoreMode ScoreMode
}

// NewSimpleCollector creates a new SimpleCollector.
func NewSimpleCollector(scoreMode ScoreMode) *SimpleCollector {
	return &SimpleCollector{scoreMode: scoreMode}
}

// ScoreMode returns the score mode.
func (c *SimpleCollector) ScoreMode() ScoreMode {
	return c.scoreMode
}

// BaseLeafCollector provides a base implementation for LeafCollector.
type BaseLeafCollector struct {
	scorer Scorer
}

// NewBaseLeafCollector creates a new BaseLeafCollector.
func NewBaseLeafCollector() *BaseLeafCollector {
	return &BaseLeafCollector{}
}

// SetScorer sets the scorer.
func (c *BaseLeafCollector) SetScorer(scorer Scorer) error {
	c.scorer = scorer
	return nil
}

// Collect collects a document.
func (c *BaseLeafCollector) Collect(doc int) error {
	return nil
}
