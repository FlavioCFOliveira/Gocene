// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// sortInts sorts a slice of ints in ascending order.
func sortInts(s []int) { sort.Ints(s) }

// TermRangeQuery matches documents containing terms within a range.
type TermRangeQuery struct {
	*BaseQuery
	field        string
	lowerTerm    []byte
	upperTerm    []byte
	includeLower bool
	includeUpper bool
}

// NewTermRangeQuery creates a new TermRangeQuery.
func NewTermRangeQuery(field string, lowerTerm, upperTerm []byte, includeLower, includeUpper bool) *TermRangeQuery {
	return &TermRangeQuery{
		BaseQuery:    &BaseQuery{},
		field:        field,
		lowerTerm:    lowerTerm,
		upperTerm:    upperTerm,
		includeLower: includeLower,
		includeUpper: includeUpper,
	}
}

// Field returns the field name.
func (q *TermRangeQuery) Field() string {
	return q.field
}

// LowerTerm returns the lower bound term.
func (q *TermRangeQuery) LowerTerm() []byte {
	return q.lowerTerm
}

// UpperTerm returns the upper bound term.
func (q *TermRangeQuery) UpperTerm() []byte {
	return q.upperTerm
}

// IncludesLower returns true if the lower bound is inclusive.
func (q *TermRangeQuery) IncludesLower() bool {
	return q.includeLower
}

// IncludesUpper returns true if the upper bound is inclusive.
func (q *TermRangeQuery) IncludesUpper() bool {
	return q.includeUpper
}

// Clone creates a copy of this query.
func (q *TermRangeQuery) Clone() Query {
	lowerCopy := make([]byte, len(q.lowerTerm))
	copy(lowerCopy, q.lowerTerm)
	upperCopy := make([]byte, len(q.upperTerm))
	copy(upperCopy, q.upperTerm)
	return &TermRangeQuery{
		BaseQuery:    &BaseQuery{},
		field:        q.field,
		lowerTerm:    lowerCopy,
		upperTerm:    upperCopy,
		includeLower: q.includeLower,
		includeUpper: q.includeUpper,
	}
}

// Equals checks if this query equals another.
func (q *TermRangeQuery) Equals(other Query) bool {
	if o, ok := other.(*TermRangeQuery); ok {
		if q.field != o.field || q.includeLower != o.includeLower || q.includeUpper != o.includeUpper {
			return false
		}
		if len(q.lowerTerm) != len(o.lowerTerm) || len(q.upperTerm) != len(o.upperTerm) {
			return false
		}
		for i := range q.lowerTerm {
			if q.lowerTerm[i] != o.lowerTerm[i] {
				return false
			}
		}
		for i := range q.upperTerm {
			if q.upperTerm[i] != o.upperTerm[i] {
				return false
			}
		}
		return true
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *TermRangeQuery) HashCode() int {
	hash := 0
	for _, b := range q.lowerTerm {
		hash = hash*31 + int(b)
	}
	hash = hash*31 + 37 // separator
	for _, b := range q.upperTerm {
		hash = hash*31 + int(b)
	}
	if q.includeLower {
		hash = hash*31 + 1
	}
	if q.includeUpper {
		hash = hash*31 + 1
	}
	return hash
}

// Rewrite rewrites the query to a simpler form.
func (q *TermRangeQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

func (q *TermRangeQuery) String() string {
	buffer := q.field + ":"
	if q.includeLower {
		buffer += "["
	} else {
		buffer += "{"
	}

	if q.lowerTerm == nil {
		buffer += "*"
	} else {
		buffer += string(q.lowerTerm)
	}

	buffer += " TO "

	if q.upperTerm == nil {
		buffer += "*"
	} else {
		buffer += string(q.upperTerm)
	}

	if q.includeUpper {
		buffer += "]"
	} else {
		buffer += "}"
	}
	return buffer
}

// CreateWeight creates a Weight for this query.
func (q *TermRangeQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return newTermRangeWeight(q, boost), nil
}

// ─── TermRangeWeight ────────────────────────────────────────────────────────

// termRangeWeight is the Weight for TermRangeQuery. It iterates the field's
// terms, accepts those that fall inside [lowerTerm, upperTerm], collects all
// matching document IDs (deduplicating across terms), and hands them to a
// RegexpScorer (which already handles sorted int-slice iteration).
type termRangeWeight struct {
	*BaseWeight
	query *TermRangeQuery
	score float32
}

func newTermRangeWeight(q *TermRangeQuery, boost float32) *termRangeWeight {
	return &termRangeWeight{
		BaseWeight: NewBaseWeight(q),
		query:      q,
		score:      boost,
	}
}

func (w *termRangeWeight) Scorer(ctx *index.LeafReaderContext) (Scorer, error) {
	leaf := ctx.LeafReader()
	if leaf == nil {
		return nil, nil
	}
	terms, err := leaf.Terms(w.query.field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	termsEnum, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}

	seen := make(map[int]struct{})
	for {
		term, err := termsEnum.Next()
		if err != nil {
			return nil, err
		}
		if term == nil {
			break
		}
		tb := []byte(term.Text())
		if !w.inRange(tb) {
			continue
		}
		postings, err := termsEnum.Postings(0)
		if err != nil {
			return nil, err
		}
		if postings == nil {
			continue
		}
		for {
			doc, err := postings.NextDoc()
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
	docs := make([]int, 0, len(seen))
	for d := range seen {
		docs = append(docs, d)
	}
	sortInts(docs)
	return NewRegexpScorer(w, docs), nil
}

// inRange reports whether term is within the query's [lower, upper] bounds
// using lexicographic (byte) comparison, honouring the inclusive/exclusive flags.
func (w *termRangeWeight) inRange(term []byte) bool {
	q := w.query
	if q.lowerTerm != nil {
		cmp := bytesCompare(term, q.lowerTerm)
		if cmp < 0 || (cmp == 0 && !q.includeLower) {
			return false
		}
	}
	if q.upperTerm != nil {
		cmp := bytesCompare(term, q.upperTerm)
		if cmp > 0 || (cmp == 0 && !q.includeUpper) {
			return false
		}
	}
	return true
}

func (w *termRangeWeight) ScorerSupplier(ctx *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return NewScorerSupplierAdapter(scorer), nil
}

func (w *termRangeWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return NewDefaultBulkScorer(scorer), nil
}

func (w *termRangeWeight) Explain(ctx *index.LeafReaderContext, doc int) (Explanation, error) {
	return NewExplanation(false, 0, "TermRangeWeight explanation"), nil
}

func (w *termRangeWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }

func (w *termRangeWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

func (w *termRangeWeight) Matches(_ *index.LeafReaderContext, _ int) (Matches, error) {
	return nil, nil
}

// bytesCompare returns negative/zero/positive for a<b/a==b/a>b.
func bytesCompare(a, b []byte) int {
	la, lb := len(a), len(b)
	n := la
	if lb < n {
		n = lb
	}
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if la < lb {
		return -1
	}
	if la > lb {
		return 1
	}
	return 0
}

// Compile-time check.
var _ Weight = (*termRangeWeight)(nil)

// NewTermRangeQueryWithStrings creates a new TermRangeQuery using strings.
func NewTermRangeQueryWithStrings(field string, lowerTerm, upperTerm string, includeLower, includeUpper bool) *TermRangeQuery {
	var lower, upper []byte
	if lowerTerm != "" {
		lower = []byte(lowerTerm)
	}
	if upperTerm != "" {
		upper = []byte(upperTerm)
	}
	return NewTermRangeQuery(field, lower, upper, includeLower, includeUpper)
}
