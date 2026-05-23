// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermsIncludingScoreQuery matches documents in a "to" field whose term
// values were collected from matching "from" documents, propagating the
// per-term scores produced by the join collector.
//
// Mirrors org.apache.lucene.search.join.TermsIncludingScoreQuery.
type TermsIncludingScoreQuery struct {
	scoreMode                ScoreMode
	toField                  string
	multipleValuesPerDocument bool
	terms                    *util.BytesRefHash
	scores                   []float32
	ords                     []int
	fromField                string
	fromQuery                search.Query
	topReaderContextID       interface{}
}

// NewTermsIncludingScoreQuery creates a TermsIncludingScoreQuery.
//   - scoreMode: join score-mode used when collecting
//   - toField: the field in the "to" side whose terms to match
//   - multipleValuesPerDocument: true when a doc may carry multiple values
//   - terms: collected terms (from BytesRefHash)
//   - scores: per-term scores indexed by BytesRefHash ordinal
//   - fromField: originating field name (for equality/hash only)
//   - fromQuery: originating query (for equality/hash only)
//   - topReaderContextID: identity of the top-level reader context at
//     collection time (for equality/hash only)
func NewTermsIncludingScoreQuery(
	scoreMode ScoreMode,
	toField string,
	multipleValuesPerDocument bool,
	terms *util.BytesRefHash,
	scores []float32,
	fromField string,
	fromQuery search.Query,
	topReaderContextID interface{},
) *TermsIncludingScoreQuery {
	ords := terms.Sort()
	return &TermsIncludingScoreQuery{
		scoreMode:                scoreMode,
		toField:                  toField,
		multipleValuesPerDocument: multipleValuesPerDocument,
		terms:                    terms,
		scores:                   scores,
		ords:                     ords,
		fromField:                fromField,
		fromQuery:                fromQuery,
		topReaderContextID:       topReaderContextID,
	}
}

// GetToField returns the "to" field name.
func (q *TermsIncludingScoreQuery) GetToField() string { return q.toField }

// GetScoreMode returns the join ScoreMode.
func (q *TermsIncludingScoreQuery) GetScoreMode() ScoreMode { return q.scoreMode }

// GetTerms returns the collected terms hash.
func (q *TermsIncludingScoreQuery) GetTerms() *util.BytesRefHash { return q.terms }

// GetScores returns the per-term scores slice (indexed by BytesRefHash ordinal).
func (q *TermsIncludingScoreQuery) GetScores() []float32 { return q.scores }

// IsMultipleValuesPerDocument reports whether a document may carry multiple
// values for the "to" field.
func (q *TermsIncludingScoreQuery) IsMultipleValuesPerDocument() bool {
	return q.multipleValuesPerDocument
}

// String implements search.Query.
func (q *TermsIncludingScoreQuery) String() string {
	return fmt.Sprintf("TermsIncludingScoreQuery{field=%s;fromQuery=%v}", q.toField, q.fromQuery)
}

// Rewrite implements search.Query.
func (q *TermsIncludingScoreQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	return q, nil
}

// Clone implements search.Query.
func (q *TermsIncludingScoreQuery) Clone() search.Query {
	cp := *q
	return &cp
}

// Equals implements search.Query.
func (q *TermsIncludingScoreQuery) Equals(other search.Query) bool {
	o, ok := other.(*TermsIncludingScoreQuery)
	if !ok {
		return false
	}
	return q.scoreMode == o.scoreMode &&
		q.toField == o.toField &&
		q.fromField == o.fromField &&
		q.topReaderContextID == o.topReaderContextID
}

// HashCode implements search.Query.
func (q *TermsIncludingScoreQuery) HashCode() int {
	h := 31
	for _, c := range q.toField {
		h = 31*h + int(c)
	}
	for _, c := range q.fromField {
		h = 31*h + int(c)
	}
	h = 31*h + int(q.scoreMode)
	return h
}

// CreateWeight implements search.Query.
func (q *TermsIncludingScoreQuery) CreateWeight(_ *search.IndexSearcher, _ bool, boost float32) (search.Weight, error) {
	return &termsIncludingScoreWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		boost:      boost,
	}, nil
}

// termsIncludingScoreWeight is the Weight for TermsIncludingScoreQuery.
type termsIncludingScoreWeight struct {
	*search.BaseWeight
	query *TermsIncludingScoreQuery
	boost float32
}

func (w *termsIncludingScoreWeight) Scorer(_ *index.LeafReaderContext) (search.Scorer, error) {
	// Gocene deviation: full Terms/PostingsEnum integration requires the codec
	// layer to be fully wired to the search engine. We return a no-match stub
	// until that integration is complete.
	return newTermsIncludingScoreStubScorer(), nil
}

func (w *termsIncludingScoreWeight) ScorerSupplier(_ *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return nil, nil
}

func (w *termsIncludingScoreWeight) BulkScorer(_ *index.LeafReaderContext) (search.BulkScorer, error) {
	return nil, nil
}

func (w *termsIncludingScoreWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }

func (w *termsIncludingScoreWeight) Explain(_ *index.LeafReaderContext, _ int) (search.Explanation, error) {
	return search.NewExplanation(false, 0, "TermsIncludingScoreWeight stub"), nil
}

func (w *termsIncludingScoreWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

func (w *termsIncludingScoreWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

// termsIncludingScoreStubScorer is a no-match placeholder scorer.
type termsIncludingScoreStubScorer struct {
	*search.BaseDocIdSetIterator
}

func newTermsIncludingScoreStubScorer() *termsIncludingScoreStubScorer {
	return &termsIncludingScoreStubScorer{
		BaseDocIdSetIterator: &search.BaseDocIdSetIterator{},
	}
}

func (s *termsIncludingScoreStubScorer) Score() float32        { return 0 }
func (s *termsIncludingScoreStubScorer) GetMaxScore(_ int) float32 { return float32(math.Inf(1)) }
func (s *termsIncludingScoreStubScorer) DocID() int               { return search.NO_MORE_DOCS }
func (s *termsIncludingScoreStubScorer) NextDoc() (int, error)    { return search.NO_MORE_DOCS, nil }
func (s *termsIncludingScoreStubScorer) Advance(_ int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *termsIncludingScoreStubScorer) Cost() int64      { return 0 }
func (s *termsIncludingScoreStubScorer) DocIDRunEnd() int { return search.NO_MORE_DOCS }

// interface compliance
var _ search.Query = (*TermsIncludingScoreQuery)(nil)
var _ search.Scorer = (*termsIncludingScoreStubScorer)(nil)
