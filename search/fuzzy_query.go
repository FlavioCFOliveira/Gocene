// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// FuzzyQuery matches documents containing terms similar to the given term.
// It uses the Damerau-Levenshtein algorithm to calculate edit distance.
type FuzzyQuery struct {
	*BaseQuery
	term           *index.Term
	maxEdits       int
	prefixLength   int
	maxExpansions  int
	transpositions bool
}

// NewFuzzyQuery creates a new FuzzyQuery with default parameters.
func NewFuzzyQuery(term *index.Term) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           term,
		maxEdits:       2,
		prefixLength:   0,
		maxExpansions:  50,
		transpositions: true,
	}
}

// NewFuzzyQueryWithMaxEdits creates a FuzzyQuery with specified max edits.
func NewFuzzyQueryWithMaxEdits(term *index.Term, maxEdits int) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           term,
		maxEdits:       maxEdits,
		prefixLength:   0,
		maxExpansions:  50,
		transpositions: true,
	}
}

// NewFuzzyQueryWithParams creates a FuzzyQuery with all parameters.
func NewFuzzyQueryWithParams(term *index.Term, maxEdits, prefixLength, maxExpansions int) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           term,
		maxEdits:       maxEdits,
		prefixLength:   prefixLength,
		maxExpansions:  maxExpansions,
		transpositions: true,
	}
}

// NewFuzzyQueryFull creates a FuzzyQuery with all parameters including transpositions.
func NewFuzzyQueryFull(term *index.Term, maxEdits, prefixLength, maxExpansions int, transpositions bool) *FuzzyQuery {
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           term,
		maxEdits:       maxEdits,
		prefixLength:   prefixLength,
		maxExpansions:  maxExpansions,
		transpositions: transpositions,
	}
}

// Term returns the fuzzy term.
func (q *FuzzyQuery) Term() *index.Term {
	return q.term
}

// MaxEdits returns the maximum edit distance.
func (q *FuzzyQuery) MaxEdits() int {
	return q.maxEdits
}

// PrefixLength returns the prefix length.
func (q *FuzzyQuery) PrefixLength() int {
	return q.prefixLength
}

// MaxExpansions returns the maximum number of expansions.
func (q *FuzzyQuery) MaxExpansions() int {
	return q.maxExpansions
}

// TranspositionsAllowed returns true if transpositions are allowed.
func (q *FuzzyQuery) TranspositionsAllowed() bool {
	return q.transpositions
}

// Clone creates a copy of this query.
func (q *FuzzyQuery) Clone() Query {
	if q.term == nil {
		return &FuzzyQuery{
			BaseQuery:      &BaseQuery{},
			term:           nil,
			maxEdits:       q.maxEdits,
			prefixLength:   q.prefixLength,
			maxExpansions:  q.maxExpansions,
			transpositions: q.transpositions,
		}
	}
	return &FuzzyQuery{
		BaseQuery:      &BaseQuery{},
		term:           q.term.Clone(),
		maxEdits:       q.maxEdits,
		prefixLength:   q.prefixLength,
		maxExpansions:  q.maxExpansions,
		transpositions: q.transpositions,
	}
}

// Equals checks if this query equals another.
func (q *FuzzyQuery) Equals(other Query) bool {
	if o, ok := other.(*FuzzyQuery); ok {
		if q.maxEdits != o.maxEdits || q.prefixLength != o.prefixLength ||
			q.maxExpansions != o.maxExpansions || q.transpositions != o.transpositions {
			return false
		}
		if q.term == nil || o.term == nil {
			return q.term == nil && o.term == nil
		}
		return q.term.Equals(o.term)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *FuzzyQuery) HashCode() int {
	hash := 0
	if q.term != nil {
		hash = q.term.HashCode()
	}
	hash = hash*31 + q.maxEdits
	hash = hash*31 + q.prefixLength
	hash = hash*31 + q.maxExpansions
	if q.transpositions {
		hash = hash*31 + 1
	}
	return hash
}

// Rewrite expands this FuzzyQuery against the terms dictionary into a
// scoring BooleanQuery of the field terms within maxEdits Damerau-
// Levenshtein distance of the target (respecting prefixLength and
// transpositions), each boosted by closeness so nearer terms score
// higher — the Go analogue of Lucene's MultiTermQuery rewrite driven by
// FuzzyTermsEnum/LevenshteinAutomata.
//
// Behaviour:
//   - nil term -> MatchNoDocsQuery.
//   - maxEdits == 0 -> the exact TermQuery.
//   - a Terms-capable reader is required to enumerate candidates. When the
//     reader cannot expose Terms (e.g. a nil reader in a unit test), the
//     query is returned unchanged so expansion is deferred to a later
//     rewrite that has a real terms dictionary.
//   - enumerated but nothing matched -> MatchNoDocsQuery.
//   - matches found -> SHOULD BooleanQuery (terms beyond maxExpansions are
//     dropped lowest-boost-first).
func (q *FuzzyQuery) Rewrite(reader IndexReader) (Query, error) {
	if q.term == nil {
		return NewMatchNoDocsQuery(), nil
	}
	if q.maxEdits == 0 {
		return NewTermQuery(q.term), nil
	}

	builder, err := NewFuzzyAutomatonBuilder(q.term.Text(), q.maxEdits, q.prefixLength, q.transpositions)
	if err != nil {
		return nil, err
	}
	levels := builder.BuildAutomatonSet() // levels[i] accepts terms within edit distance i

	matched, enumerated, err := q.enumerateFuzzyTerms(reader, levels)
	if err != nil {
		return nil, err
	}
	if !enumerated {
		// No terms dictionary available; defer expansion.
		return q, nil
	}
	if len(matched) == 0 {
		return NewMatchNoDocsQuery(), nil
	}

	// Cap to maxExpansions, keeping the closest (highest-boost) terms, then
	// term-ascending for determinism — mirroring TopTermsRewrite's selection.
	if q.maxExpansions > 0 && len(matched) > q.maxExpansions {
		sort.Slice(matched, func(i, j int) bool {
			if matched[i].boost != matched[j].boost {
				return matched[i].boost > matched[j].boost
			}
			return bytes.Compare(matched[i].bytes, matched[j].bytes) < 0
		})
		matched = matched[:q.maxExpansions]
	}

	bq := NewBooleanQuery()
	for _, m := range matched {
		var clause Query = NewTermQuery(index.NewTermFromBytes(q.term.Field, m.bytes))
		if m.boost != 1.0 {
			clause = NewBoostQuery(clause, m.boost)
		}
		bq.Add(clause, SHOULD)
	}
	return bq, nil
}

// fuzzyTermMatch is a field term accepted by the fuzzy automaton together
// with the closeness boost derived from its edit distance.
type fuzzyTermMatch struct {
	bytes []byte
	boost float32
}

// enumerateFuzzyTerms walks every term in the field and returns those
// accepted by the maximum-edit automaton, with a per-term closeness boost.
// The second result reports whether enumeration was possible at all (false
// when the reader exposes no Terms accessor for the field).
func (q *FuzzyQuery) enumerateFuzzyTerms(reader IndexReader, levels []*automaton.CompiledAutomaton) ([]fuzzyTermMatch, bool, error) {
	if reader == nil || len(levels) == 0 {
		return nil, false, nil
	}
	// LeafReader / SegmentReader / DirectoryReader expose Terms(field); use a
	// narrow interface so we do not depend on the concrete reader type.
	type schemaTermsProvider interface {
		Terms(field string) (index.Terms, error)
	}
	stp, ok := interface{}(reader).(schemaTermsProvider)
	if !ok {
		return nil, false, nil
	}
	terms, err := stp.Terms(q.term.Field)
	if err != nil {
		return nil, false, err
	}
	if terms == nil {
		// Field absent: enumeration was possible, it simply matched nothing.
		return nil, true, nil
	}
	it, err := terms.GetIterator()
	if err != nil || it == nil {
		return nil, err == nil, err
	}

	maxEdit := levels[len(levels)-1]
	out := make([]fuzzyTermMatch, 0, 16)
	for {
		t, err := it.Next()
		if err != nil {
			return nil, true, err
		}
		if t == nil {
			break
		}
		bv := t.BytesValue()
		if bv == nil {
			continue
		}
		b := bv.ValidBytes()
		if !maxEdit.Run(b) {
			continue
		}
		// Minimal edit distance: the first level whose automaton accepts the term.
		dist := q.maxEdits
		for i := 0; i < len(levels); i++ {
			if levels[i].Run(b) {
				dist = i
				break
			}
		}
		dup := make([]byte, len(b))
		copy(dup, b)
		out = append(out, fuzzyTermMatch{bytes: dup, boost: fuzzyBoost(dist, q.maxEdits)})
	}
	return out, true, nil
}

// fuzzyBoost maps an edit distance to a closeness boost in (0,1]: an exact
// match (distance 0) scores 1.0 and each further edit lowers the boost
// linearly, so nearer terms contribute more — the spirit of Lucene's
// blended fuzzy scoring (the exact blend constant is not reproduced).
func fuzzyBoost(dist, maxEdits int) float32 {
	if maxEdits <= 0 {
		return 1.0
	}
	return 1.0 - float32(dist)/float32(maxEdits+1)
}

// CreateWeight creates a Weight for this query.
func (q *FuzzyQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return NewTermWeight(q, q.term, searcher, needsScores), nil
}

// String returns the Lucene-canonical representation "field:text~maxEdits".
// For a nil term the output is "<nil>~maxEdits".
func (q *FuzzyQuery) String(field string) string {
	if q.term == nil {
		return fmt.Sprintf("<nil>~%d", q.maxEdits)
	}
	return fmt.Sprintf("%s:%s~%d", q.term.Field, q.term.Text(), q.maxEdits)
}

// NewFuzzyQueryWithStrings creates a new FuzzyQuery using strings.
func NewFuzzyQueryWithStrings(field string, text string) *FuzzyQuery {
	return NewFuzzyQuery(index.NewTerm(field, text))
}

// NewFuzzyQueryWithStringsMaxEdits creates a new FuzzyQuery with max edits using strings.
func NewFuzzyQueryWithStringsMaxEdits(field string, text string, maxEdits int) *FuzzyQuery {
	return NewFuzzyQueryWithMaxEdits(index.NewTerm(field, text), maxEdits)
}

// CalculateDamerauLevenshteinDistance calculates the Damerau-Levenshtein distance between two strings.
// This includes transpositions as a single edit.
func CalculateDamerauLevenshteinDistance(s1, s2 string) float64 {
	if s1 == s2 {
		return 0
	}
	if len(s1) == 0 {
		return float64(len(s2))
	}
	if len(s2) == 0 {
		return float64(len(s1))
	}

	// Create matrix
	rows := len(s1) + 1
	cols := len(s2) + 1
	matrix := make([][]int, rows)
	for i := range matrix {
		matrix[i] = make([]int, cols)
	}

	// Initialize first row and column
	for i := 0; i < rows; i++ {
		matrix[i][0] = i
	}
	for j := 0; j < cols; j++ {
		matrix[0][j] = j
	}

	// Fill the matrix
	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			deletion := matrix[i-1][j] + 1
			insertion := matrix[i][j-1] + 1
			substitution := matrix[i-1][j-1] + cost
			min := deletion
			if insertion < min {
				min = insertion
			}
			if substitution < min {
				min = substitution
			}

			// Check for transposition
			if i > 1 && j > 1 && s1[i-1] == s2[j-2] && s1[i-2] == s2[j-1] {
				transposition := matrix[i-2][j-2] + cost
				if transposition < min {
					min = transposition
				}
			}

			matrix[i][j] = min
		}
	}

	return float64(matrix[rows-1][cols-1])
}

// CalculateLevenshteinDistance calculates the Levenshtein distance between two strings.
// Does not include transpositions.
func CalculateLevenshteinDistance(s1, s2 string) float64 {
	if s1 == s2 {
		return 0
	}
	if len(s1) == 0 {
		return float64(len(s2))
	}
	if len(s2) == 0 {
		return float64(len(s1))
	}

	// Make s1 the shorter string for memory efficiency
	if len(s1) > len(s2) {
		s1, s2 = s2, s1
	}

	// Use only two rows for space efficiency
	previous := make([]int, len(s1)+1)
	current := make([]int, len(s1)+1)

	// Initialize previous row
	for i := 0; i <= len(s1); i++ {
		previous[i] = i
	}

	// Fill current row
	for j := 1; j <= len(s2); j++ {
		current[0] = j
		for i := 1; i <= len(s1); i++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			deletion := previous[i] + 1
			insertion := current[i-1] + 1
			substitution := previous[i-1] + cost
			min := deletion
			if insertion < min {
				min = insertion
			}
			if substitution < min {
				min = substitution
			}
			current[i] = min
		}
		// Swap rows
		previous, current = current, previous
	}

	return float64(previous[len(s1)])
}

// GetTerm returns the term for this query.
func (q *FuzzyQuery) GetTerm() *index.Term {
	return q.term
}

// GetMaxEdits returns the maximum edit distance (for API compatibility).
func (q *FuzzyQuery) GetMaxEdits() int {
	return q.maxEdits
}

// GetPrefixLength returns the prefix length (for API compatibility).
func (q *FuzzyQuery) GetPrefixLength() int {
	return q.prefixLength
}

// GetMaxExpansions returns the maximum number of expansions (for API compatibility).
func (q *FuzzyQuery) GetMaxExpansions() int {
	return q.maxExpansions
}

// IsTranspositionsAllowed returns true if transpositions are allowed (for API compatibility).
func (q *FuzzyQuery) IsTranspositionsAllowed() bool {
	return q.transpositions
}

// Ensure FuzzyQuery implements Query
var _ Query = (*FuzzyQuery)(nil)
