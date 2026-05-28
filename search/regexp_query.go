// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// numericRangePattern matches Lucene's <lo-hi> numeric range syntax.
var numericRangePattern = regexp.MustCompile(`^<(\d+)-(\d+)>$`)

// RegexpQuery is a query that matches documents containing terms
// that match a regular expression pattern, using full-term (anchored)
// semantics — the pattern must match the entire term, not a substring.
//
// This is the Go port of Lucene's org.apache.lucene.search.RegexpQuery.
//
// Lucene's RegexpQuery uses its own automaton engine; this port compiles the
// pattern to a Go regexp anchored with ^(?:...)$ so that the whole term must
// match. The special Lucene numeric-range syntax <lo-hi> (e.g. <420000-600000>)
// is handled as a separate code path that compares the parsed integer value of
// each term rather than trying to encode the range as a regexp.
type RegexpQuery struct {
	*BaseQuery
	field          string
	pattern        string
	re             *regexp.Regexp
	isNumericRange bool
	numericLo      int64
	numericHi      int64
}

// compileRegexp compiles pattern as a full-match regexp (anchored at both ends).
// An empty pattern compiles to ^(?:)$ which matches only the empty string.
func compileRegexp(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile("^(?:" + pattern + ")$")
}

// NewRegexpQuery creates a new RegexpQuery with full-match semantics.
//
// Parameters:
//   - field: The field to search
//   - pattern: The regular expression pattern; the Lucene <lo-hi> numeric
//     range syntax is recognised and handled via integer comparison.
//
// Returns an error if the pattern is invalid.
func NewRegexpQuery(field, pattern string) (*RegexpQuery, error) {
	q := &RegexpQuery{
		BaseQuery: &BaseQuery{},
		field:     field,
		pattern:   pattern,
	}
	if m := numericRangePattern.FindStringSubmatch(pattern); m != nil {
		lo, errLo := strconv.ParseInt(m[1], 10, 64)
		hi, errHi := strconv.ParseInt(m[2], 10, 64)
		if errLo != nil || errHi != nil {
			return nil, fmt.Errorf("invalid numeric range pattern %q", pattern)
		}
		q.isNumericRange = true
		q.numericLo = lo
		q.numericHi = hi
	} else {
		re, err := compileRegexp(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regexp pattern: %w", err)
		}
		q.re = re
	}
	return q, nil
}

// NewRegexpQueryWithFlags creates a new RegexpQuery with flags.
// The flags argument is accepted for API compatibility but currently unused;
// Go's regexp package exposes flags via inline syntax (e.g. (?i)).
//
// Parameters:
//   - field: The field to search
//   - pattern: The regular expression pattern
//   - flags: Accepted for API parity; unused in this implementation.
//
// Returns an error if the pattern is invalid.
func NewRegexpQueryWithFlags(field, pattern string, _ int) (*RegexpQuery, error) {
	return NewRegexpQuery(field, pattern)
}

// Field returns the field name.
func (q *RegexpQuery) Field() string {
	return q.field
}

// Pattern returns the regexp pattern.
func (q *RegexpQuery) Pattern() string {
	return q.pattern
}

// Matches returns true if the given text matches the pattern with full-term
// semantics. For numeric-range patterns the text is parsed as an integer and
// compared against [lo, hi].
func (q *RegexpQuery) Matches(text string) bool {
	if q.isNumericRange {
		v, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return false
		}
		return v >= q.numericLo && v <= q.numericHi
	}
	if q.re == nil {
		return false
	}
	return q.re.MatchString(text)
}

// Clone creates a copy of this query.
func (q *RegexpQuery) Clone() Query {
	clone := &RegexpQuery{
		BaseQuery:      &BaseQuery{},
		field:          q.field,
		pattern:        q.pattern,
		isNumericRange: q.isNumericRange,
		numericLo:      q.numericLo,
		numericHi:      q.numericHi,
	}
	if q.re != nil {
		clone.re, _ = compileRegexp(q.pattern)
	}
	return clone
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
// For now, returns itself; a full implementation would rewrite simple patterns
// to TermQuery or PrefixQuery.
func (q *RegexpQuery) Rewrite(reader IndexReader) (Query, error) {
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

	// Collect matching document IDs, deduplicating across terms.
	seen := make(map[int]struct{})
	for {
		term, err := termsEnum.Next()
		if err != nil {
			return nil, err
		}
		if term == nil {
			break
		}

		// Check if the term text matches the pattern (full-match semantics).
		if !w.query.Matches(term.Text()) {
			continue
		}

		// Get postings for this term
		postingsEnum, err := termsEnum.Postings(0)
		if err != nil {
			return nil, err
		}
		if postingsEnum == nil {
			continue
		}

		// Collect documents; -1 == index.NO_MORE_DOCS
		for {
			doc, err := postingsEnum.NextDoc()
			if err != nil {
				return nil, err
			}
			if doc == -1 { // index.NO_MORE_DOCS
				break
			}
			seen[doc] = struct{}{}
		}
	}

	if len(seen) == 0 {
		return nil, nil
	}

	// Produce a sorted, deduplicated slice of matching doc IDs.
	matchingDocs := make([]int, 0, len(seen))
	for d := range seen {
		matchingDocs = append(matchingDocs, d)
	}
	sort.Ints(matchingDocs)

	return NewRegexpScorer(w, matchingDocs), nil
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
//
// RegexpQuery is rewritten by Lucene to a constant-score multi-term query, so
// this ports org.apache.lucene.search.ConstantScoreWeight.explain: pull a
// Scorer and advance to doc; a hit yields a match valued at the constant
// scorer score with the query string as description, and a miss yields
// "<query> doesn't match id <doc>". The value is taken from the live Scorer so
// it equals the scored value.
func (w *RegexpWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	matched, score, err := scorerMatch(w, context, doc)
	if err != nil {
		return nil, err
	}
	if matched {
		desc := w.query.String()
		if score != 1.0 {
			desc = fmt.Sprintf("%s^%v", desc, score)
		}
		return MatchExplanation(score, desc), nil
	}
	return NoMatchExplanation(fmt.Sprintf("%s doesn't match id %d", w.query, doc)), nil
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
