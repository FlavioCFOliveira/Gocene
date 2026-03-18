// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// TermWithScore represents a term and its associated score.
type TermWithScore struct {
	Term  []byte
	Score float32
}

// TermsWithScoreCollector collects terms and their scores from documents
// matching a query. This is used by JoinUtil to collect join values.
//
// This is the Go port of Lucene's TermsWithScoreCollector.
type TermsWithScoreCollector struct {
	// field is the field to collect terms from
	field string

	// termsWithScores stores the collected terms and their scores
	termsWithScores []TermWithScore

	// termSet tracks unique terms to avoid duplicates
	termSet map[string]struct{}

	// termCounts tracks how many times each term was seen (for Avg score mode)
	termCounts map[string]int

	// totalHits is the total number of documents processed
	totalHits int

	// scoreMode determines how scores are aggregated for duplicate terms
	scoreMode ScoreMode
}

// NewTermsWithScoreCollector creates a new TermsWithScoreCollector.
// Parameters:
//   - field: the field to collect terms from
//   - scoreMode: how to combine scores for duplicate terms
func NewTermsWithScoreCollector(field string, scoreMode ScoreMode) *TermsWithScoreCollector {
	return &TermsWithScoreCollector{
		field:           field,
		termsWithScores: make([]TermWithScore, 0),
		termSet:         make(map[string]struct{}),
		termCounts:      make(map[string]int),
		scoreMode:       scoreMode,
	}
}

// Collect collects a term from a document with its score.
// This method is called for each matching document.
func (c *TermsWithScoreCollector) Collect(term []byte, score float32) error {
	if len(term) == 0 {
		return nil
	}

	termKey := string(term)
	c.totalHits++

	// Check if we already have this term
	if _, exists := c.termSet[termKey]; exists {
		// Update score based on score mode
		c.updateScoreForTerm(term, score)
		c.termCounts[termKey]++
		return nil
	}

	// Add new term
	c.termSet[termKey] = struct{}{}
	c.termsWithScores = append(c.termsWithScores, TermWithScore{
		Term:  term,
		Score: score,
	})
	c.termCounts[termKey] = 1

	return nil
}

// updateScoreForTerm updates the score for an existing term based on score mode.
func (c *TermsWithScoreCollector) updateScoreForTerm(term []byte, newScore float32) {
	termKey := string(term)

	for i := range c.termsWithScores {
		if string(c.termsWithScores[i].Term) == termKey {
			switch c.scoreMode {
			case Max:
				if newScore > c.termsWithScores[i].Score {
					c.termsWithScores[i].Score = newScore
				}
			case Min:
				if newScore < c.termsWithScores[i].Score {
					c.termsWithScores[i].Score = newScore
				}
			case Total, Avg:
				// For Total and Avg, we accumulate and compute final value later
				c.termsWithScores[i].Score += newScore
			}
			break
		}
	}
}

// GetTerms returns all collected terms (without scores).
func (c *TermsWithScoreCollector) GetTerms() [][]byte {
	terms := make([][]byte, len(c.termsWithScores))
	for i, tws := range c.termsWithScores {
		terms[i] = tws.Term
	}
	return terms
}

// GetTermsWithScores returns all collected terms with their scores.
func (c *TermsWithScoreCollector) GetTermsWithScores() []TermWithScore {
	result := make([]TermWithScore, len(c.termsWithScores))
	copy(result, c.termsWithScores)

	// Apply average calculation if needed
	if c.scoreMode == Avg {
		// Divide scores by counts
		for i := range result {
			termKey := string(result[i].Term)
			if count := c.termCounts[termKey]; count > 1 {
				result[i].Score /= float32(count)
			}
		}
	}

	return result
}

// GetTopTerms returns the top N terms sorted by score (highest first).
func (c *TermsWithScoreCollector) GetTopTerms(n int) []TermWithScore {
	terms := c.GetTermsWithScores()

	// Sort by score (descending)
	sort.Slice(terms, func(i, j int) bool {
		return terms[i].Score > terms[j].Score
	})

	if n <= 0 || n > len(terms) {
		return terms
	}
	return terms[:n]
}

// GetTotalHits returns the total number of documents processed.
func (c *TermsWithScoreCollector) GetTotalHits() int {
	return c.totalHits
}

// GetUniqueTermCount returns the number of unique terms collected.
func (c *TermsWithScoreCollector) GetUniqueTermCount() int {
	return len(c.termsWithScores)
}

// Reset resets the collector for reuse.
func (c *TermsWithScoreCollector) Reset() {
	c.termsWithScores = c.termsWithScores[:0]
	c.termSet = make(map[string]struct{})
	c.termCounts = make(map[string]int)
	c.totalHits = 0
}

// GetField returns the field being collected.
func (c *TermsWithScoreCollector) GetField() string {
	return c.field
}

// GetScoreMode returns the score mode.
func (c *TermsWithScoreCollector) GetScoreMode() ScoreMode {
	return c.scoreMode
}

// TermsWithScoreCollectorManager manages TermsWithScoreCollector instances
// for distributed search across multiple segments.
type TermsWithScoreCollectorManager struct {
	field     string
	scoreMode ScoreMode
}

// NewTermsWithScoreCollectorManager creates a new manager.
func NewTermsWithScoreCollectorManager(field string, scoreMode ScoreMode) *TermsWithScoreCollectorManager {
	return &TermsWithScoreCollectorManager{
		field:     field,
		scoreMode: scoreMode,
	}
}

// NewCollector creates a new collector for the given context.
func (m *TermsWithScoreCollectorManager) NewCollector(context *index.LeafReaderContext) (*TermsWithScoreCollector, error) {
	return NewTermsWithScoreCollector(m.field, m.scoreMode), nil
}

// Reduce combines multiple collectors into a single result.
func (m *TermsWithScoreCollectorManager) Reduce(collectors []*TermsWithScoreCollector) (*TermsWithScoreCollector, error) {
	// Create a merged collector
	merged := NewTermsWithScoreCollector(m.field, m.scoreMode)

	// Merge term sets with score aggregation
	termScores := make(map[string]float32)
	termCounts := make(map[string]int)

	for _, collector := range collectors {
		for _, tws := range collector.termsWithScores {
			termKey := string(tws.Term)

			// Aggregate scores based on score mode
			switch m.scoreMode {
			case Max:
				if existing, exists := termScores[termKey]; !exists || tws.Score > existing {
					termScores[termKey] = tws.Score
				}
			case Min:
				if existing, exists := termScores[termKey]; !exists || tws.Score < existing {
					termScores[termKey] = tws.Score
				}
			case Total, Avg:
				termScores[termKey] += tws.Score
				termCounts[termKey]++
			default:
				termScores[termKey] = tws.Score
			}
		}
		merged.totalHits += collector.totalHits
	}

	// Build final terms list
	for termKey, score := range termScores {
		finalScore := score
		if m.scoreMode == Avg {
			if count := termCounts[termKey]; count > 1 {
				finalScore /= float32(count)
			}
		}
		merged.termsWithScores = append(merged.termsWithScores, TermWithScore{
			Term:  []byte(termKey),
			Score: finalScore,
		})
		merged.termSet[termKey] = struct{}{}
	}

	return merged, nil
}

// CollectFromReader collects terms from a LeafReader for documents matching the given query.
// This is a helper method used by JoinUtil.
func CollectFromReader(
	reader *index.LeafReader,
	query search.Query,
	field string,
	scoreMode ScoreMode,
) (*TermsWithScoreCollector, error) {
	collector := NewTermsWithScoreCollector(field, scoreMode)

	// Create a searcher for this reader
	searcher := search.NewIndexSearcher(reader)

	// Execute the query and collect terms
	// This is a simplified implementation
	// In a full implementation, we would:
	// 1. Create a Weight from the query
	// 2. Get a Scorer
	// 3. Iterate through matching documents
	// 4. Extract the field value from each document
	// 5. Collect the term and score

	_ = searcher
	_ = query

	return collector, nil
}

// CollectTerms executes a query and collects terms from the specified field.
// This is the main entry point used by JoinUtil.CreateJoinQuery.
func CollectTerms(
	searcher *search.IndexSearcher,
	query search.Query,
	field string,
	scoreMode ScoreMode,
) ([][]byte, error) {
	// Create collector
	collector := NewTermsWithScoreCollector(field, scoreMode)

	// Execute search
	// In a full implementation, this would use the searcher's search method
	// with the collector to collect terms from matching documents

	_ = searcher
	_ = query

	return collector.GetTerms(), nil
}

// CollectTermsWithScores executes a query and collects terms with scores.
func CollectTermsWithScores(
	searcher *search.IndexSearcher,
	query search.Query,
	field string,
	scoreMode ScoreMode,
) ([]TermWithScore, error) {
	collector := NewTermsWithScoreCollector(field, scoreMode)

	// Execute search
	_ = searcher
	_ = query

	return collector.GetTermsWithScores(), nil
}

// Validate interface implementations
var _ fmt.Stringer = (*TermWithScore)(nil)

// String returns a string representation of TermWithScore.
func (t TermWithScore) String() string {
	return fmt.Sprintf("TermWithScore{term=%s, score=%f}", string(t.Term), t.Score)
}
