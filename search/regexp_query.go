// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"regexp"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// RegexpQuery is a query that matches documents containing terms
// that match a regular expression pattern.
//
// This is the Go port of Lucene's org.apache.lucene.search.RegexpQuery.
type RegexpQuery struct {
	*BaseQuery
	field   string
	pattern string
	re      *regexp.Regexp
}

// NewRegexpQuery creates a new RegexpQuery.
//
// Parameters:
//   - field: The field to search
//   - pattern: The regular expression pattern
//
// Returns an error if the pattern is invalid.
func NewRegexpQuery(field, pattern string) (*RegexpQuery, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regexp pattern: %w", err)
	}

	return &RegexpQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		pattern:   pattern,
		re:        re,
	}, nil
}

// NewRegexpQueryWithFlags creates a new RegexpQuery with flags.
//
// Parameters:
//   - field: The field to search
//   - pattern: The regular expression pattern
//   - flags: The regexp flags (e.g., regexp.CaseInsensitive)
//
// Returns an error if the pattern is invalid.
func NewRegexpQueryWithFlags(field, pattern string, flags int) (*RegexpQuery, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regexp pattern: %w", err)
	}

	return &RegexpQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		pattern:   pattern,
		re:        re,
	}, nil
}

// Field returns the field name.
func (q *RegexpQuery) Field() string {
	return q.field
}

// Pattern returns the regexp pattern.
func (q *RegexpQuery) Pattern() string {
	return q.pattern
}

// Matches returns true if the given text matches the pattern.
func (q *RegexpQuery) Matches(text string) bool {
	if q.re == nil {
		return false
	}
	return q.re.MatchString(text)
}

// Clone creates a copy of this query.
func (q *RegexpQuery) Clone() Query {
	// Re-compile the regexp
	re, _ := regexp.Compile(q.pattern)
	return &RegexpQuery{
		BaseQuery: &BaseQuery{},
		field:     q.field,
		pattern:   q.pattern,
		re:        re,
	}
}

// Equals checks if this query equals another.
func (q *RegexpQuery) Equals(other Query) bool {
	if o, ok := other.(*RegexpQuery); ok {
		return q.field == o.field && q.pattern == o.pattern
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *RegexpQuery) HashCode() int {
	hash := 0
	for _, c := range q.field {
		hash = hash*31 + int(c)
	}
	for _, c := range q.pattern {
		hash = hash*31 + int(c)
	}
	return hash
}

// Rewrite rewrites the query to a simpler form.
// For simple patterns, this may rewrite to a TermQuery or BooleanQuery.
func (q *RegexpQuery) Rewrite(reader IndexReader) (Query, error) {
	// For now, return itself
	// A full implementation would analyze the pattern and potentially
	// rewrite to more efficient queries (e.g., prefix queries for ^prefix)
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *RegexpQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewRegexpWeight(q, searcher, needsScores), nil
}

// String returns a string representation of this query.
func (q *RegexpQuery) String() string {
	return fmt.Sprintf("%s:/%s/", q.field, q.pattern)
}

// Ensure RegexpQuery implements Query
var _ Query = (*RegexpQuery)(nil)

// RegexpWeight is the Weight implementation for RegexpQuery.
type RegexpWeight struct {
	*BaseWeight
	query       *RegexpQuery
	searcher    *IndexSearcher
	needsScores bool
}

// NewRegexpWeight creates a new RegexpWeight.
func NewRegexpWeight(query *RegexpQuery, searcher *IndexSearcher, needsScores bool) *RegexpWeight {
	return &RegexpWeight{
		BaseWeight:  NewBaseWeight(query),
		query:       query,
		searcher:    searcher,
		needsScores: needsScores,
	}
}

// Scorer creates a scorer for this weight.
func (w *RegexpWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	leafReader := context.LeafReader()
	if leafReader == nil {
		return nil, nil
	}

	// Get the terms for the field
	terms, err := leafReader.Terms(w.query.field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}

	// Get the terms enum iterator
	termsEnum, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}

	// Collect matching terms
	var matchingDocs []int
	for {
		term, err := termsEnum.Next()
		if err != nil {
			return nil, err
		}
		if term == nil {
			break
		}

		// Check if the term text matches the pattern
		if w.query.Matches(term.Text()) {
			// Get postings for this term
			postingsEnum, err := termsEnum.Postings(0)
			if err != nil {
				return nil, err
			}
			if postingsEnum == nil {
				continue
			}

			// Collect documents
			for {
				doc, err := postingsEnum.NextDoc()
				if err != nil {
					return nil, err
				}
				if doc == -1 { // NO_MORE_DOCS
					break
				}
				matchingDocs = append(matchingDocs, doc)
			}
		}
	}

	// Create a scorer for the matching documents
	if len(matchingDocs) > 0 {
		return NewRegexpScorer(w, matchingDocs), nil
	}

	return nil, nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *RegexpWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

// Explain returns an explanation of the score for the given document.
func (w *RegexpWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(false, 0, "RegexpWeight explanation not implemented"), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *RegexpWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *RegexpWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *RegexpWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *RegexpWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure RegexpWeight implements Weight
var _ Weight = (*RegexpWeight)(nil)

// RegexpScorer is a scorer for regexp queries.
type RegexpScorer struct {
	*BaseScorer
	docs []int
	pos  int
	doc  int
}

// NewRegexpScorer creates a new RegexpScorer.
func NewRegexpScorer(weight Weight, docs []int) *RegexpScorer {
	return &RegexpScorer{
		BaseScorer: NewBaseScorer(weight),
		docs:       docs,
		pos:        -1,
		doc:        -1,
	}
}

// DocID returns the current document ID.
func (s *RegexpScorer) DocID() int {
	return s.doc
}

// NextDoc advances to the next document.
func (s *RegexpScorer) NextDoc() (int, error) {
	s.pos++
	if s.pos >= len(s.docs) {
		s.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	s.doc = s.docs[s.pos]
	return s.doc, nil
}

// Advance advances to the target document.
func (s *RegexpScorer) Advance(target int) (int, error) {
	for s.pos+1 < len(s.docs) {
		s.pos++
		if s.docs[s.pos] >= target {
			s.doc = s.docs[s.pos]
			return s.doc, nil
		}
	}
	s.doc = NO_MORE_DOCS
	return NO_MORE_DOCS, nil
}

// Cost returns the estimated cost.
func (s *RegexpScorer) Cost() int64 {
	return int64(len(s.docs))
}

// DocIDRunEnd returns the end of the current run.
func (s *RegexpScorer) DocIDRunEnd() int {
	return s.doc + 1
}

// Score returns the score for the current document.
func (s *RegexpScorer) Score() float32 {
	return 1.0
}

// GetMaxScore returns the maximum score for documents up to the given doc.
func (s *RegexpScorer) GetMaxScore(upTo int) float32 {
	return 1.0
}

// Ensure RegexpScorer implements Scorer
var _ Scorer = (*RegexpScorer)(nil)
