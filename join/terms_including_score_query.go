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

// termsReader is a local interface so the join package can call Terms() on
// a concrete reader without importing implementation types directly.
type termsReader interface {
	Terms(field string) (index.Terms, error)
}

// TermsIncludingScoreQuery matches documents in a "to" field whose term
// values were collected from matching "from" documents, propagating the
// per-term scores produced by the join collector.
//
// Mirrors org.apache.lucene.search.join.TermsIncludingScoreQuery.
type TermsIncludingScoreQuery struct {
	scoreMode                 ScoreMode
	toField                   string
	multipleValuesPerDocument bool
	terms                     *util.BytesRefHash
	scores                    []float32
	ords                      []int
	fromField                 string
	fromQuery                 search.Query
	topReaderContextID        interface{}
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
		scoreMode:                 scoreMode,
		toField:                   toField,
		multipleValuesPerDocument: multipleValuesPerDocument,
		terms:                     terms,
		scores:                    scores,
		ords:                      ords,
		fromField:                 fromField,
		fromQuery:                 fromQuery,
		topReaderContextID:        topReaderContextID,
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
	w := &termsIncludingScoreWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		boost:      boost,
	}
	return w, nil
}

// termsIncludingScoreWeight is the Weight for TermsIncludingScoreQuery.
type termsIncludingScoreWeight struct {
	*search.BaseWeight
	query *TermsIncludingScoreQuery
	boost float32
}

// Scorer builds the per-segment scorer by iterating over the collected terms
// via the segment's TermsEnum, building a FixedBitSet of matching docs with
// their per-doc scores, and returning a BitSetIterator-backed Scorer.
//
// Mirrors SVInOrderScorer / MVInOrderScorer from Lucene 10.4.0.
func (w *termsIncludingScoreWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	if ctx == nil {
		return nil, nil
	}
	r := ctx.LeafReader()
	if r == nil {
		return nil, nil
	}
	tr, ok := r.(termsReader)
	if !ok {
		return nil, nil
	}
	q := w.query
	if q == nil {
		return nil, nil
	}
	terms, err := tr.Terms(q.toField)
	if err != nil {
		return nil, fmt.Errorf("termsIncludingScoreQuery: terms(%s): %w", q.toField, err)
	}
	if terms == nil {
		return nil, nil
	}
	maxDoc := r.MaxDoc()
	cost := int64(maxDoc) * terms.Size()
	termsEnum, err := terms.GetIterator()
	if err != nil {
		return nil, fmt.Errorf("termsIncludingScoreQuery: get iterator: %w", err)
	}
	if q.multipleValuesPerDocument {
		return newMVInOrderScorer(q, termsEnum, maxDoc, cost, w.boost)
	}
	return newSVInOrderScorer(q, termsEnum, maxDoc, cost, w.boost)
}

func (w *termsIncludingScoreWeight) ScorerSupplier(_ *index.LeafReaderContext) (search.ScorerSupplier, error) {
	return nil, nil
}

func (w *termsIncludingScoreWeight) BulkScorer(_ *index.LeafReaderContext) (search.BulkScorer, error) {
	return nil, nil
}

func (w *termsIncludingScoreWeight) IsCacheable(_ *index.LeafReaderContext) bool { return true }

func (w *termsIncludingScoreWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	if ctx == nil {
		return search.NewExplanation(false, 0, "no leaf context"), nil
	}
	r := ctx.LeafReader()
	if r == nil {
		return search.NewExplanation(false, 0, "Not a match"), nil
	}
	tr, ok := r.(termsReader)
	if !ok {
		return search.NewExplanation(false, 0, "Not a match"), nil
	}
	q := w.query
	if q == nil {
		return search.NewExplanation(false, 0, "Not a match"), nil
	}
	terms, err := tr.Terms(q.toField)
	if err != nil || terms == nil {
		return search.NewExplanation(false, 0, "Not a match"), nil
	}
	termsEnum, err := terms.GetIterator()
	if err != nil || termsEnum == nil {
		return search.NewExplanation(false, 0, "Not a match"), nil
	}
	spare := util.NewBytesRefEmpty()
	for i := 0; i < q.terms.Size(); i++ {
		q.terms.Get(q.ords[i], spare)
		seekTerm := index.NewTermFromBytesRef(q.toField, spare)
		found, err := termsEnum.SeekExact(seekTerm)
		if err != nil {
			return search.NewExplanation(false, 0, "Not a match"), nil
		}
		if !found {
			continue
		}
		pe, err := termsEnum.Postings(0)
		if err != nil || pe == nil {
			continue
		}
		got, err := pe.Advance(doc)
		if err != nil || got != doc {
			continue
		}
		sc := q.scores[q.ords[i]] * w.boost
		return search.NewExplanation(true, sc,
			fmt.Sprintf("Score based on join value %s", string(spare.Bytes[:spare.Length]))), nil
	}
	return search.NewExplanation(false, 0, "Not a match"), nil
}

func (w *termsIncludingScoreWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

func (w *termsIncludingScoreWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

// --- SVInOrderScorer ----------------------------------------------------------

// svInOrderScorer is the single-value scorer: for each matching doc the score
// from the last matching term is used (overwrites previous).  Mirrors
// SVInOrderScorer in Lucene 10.4.0.
type svInOrderScorer struct {
	matchingDocsIter search.DocIdSetIterator
	docScores        []float32
	cost             int64
	boost            float32
	currentDoc       int
}

func newSVInOrderScorer(
	q *TermsIncludingScoreQuery,
	termsEnum index.TermsEnum,
	maxDoc int,
	cost int64,
	boost float32,
) (*svInOrderScorer, error) {
	matchingDocs, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}
	docScores := make([]float32, maxDoc)
	if err := fillDocsAndScoresSV(matchingDocs, docScores, q, termsEnum); err != nil {
		return nil, err
	}
	iter := util.NewBitSetIterator(matchingDocs.AsReadOnlyBits(), cost)
	return &svInOrderScorer{
		matchingDocsIter: iter,
		docScores:        docScores,
		cost:             cost,
		boost:            boost,
		currentDoc:       -1,
	}, nil
}

// fillDocsAndScoresSV populates matchingDocs and docScores for the SV case.
// When a document matches multiple terms the last score wins (Java SV behaviour).
func fillDocsAndScoresSV(
	matchingDocs *util.FixedBitSet,
	docScores []float32,
	q *TermsIncludingScoreQuery,
	termsEnum index.TermsEnum,
) error {
	spare := util.NewBytesRefEmpty()
	var pe index.PostingsEnum
	for i := 0; i < q.terms.Size(); i++ {
		q.terms.Get(q.ords[i], spare)
		seekTerm := index.NewTermFromBytesRef(q.toField, spare)
		found, err := termsEnum.SeekExact(seekTerm)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		pe, err = termsEnum.Postings(0) // NONE flag
		if err != nil {
			return err
		}
		score := q.scores[q.ords[i]]
		for {
			doc, err := pe.NextDoc()
			if err != nil {
				return err
			}
			if doc == index.NO_MORE_DOCS {
				break
			}
			matchingDocs.Set(doc)
			docScores[doc] = score // last-wins for SV case
		}
	}
	return nil
}

func (s *svInOrderScorer) Score() float32            { return s.docScores[s.currentDoc] * s.boost }
func (s *svInOrderScorer) GetMaxScore(_ int) float32 { return float32(math.Inf(1)) }
func (s *svInOrderScorer) DocID() int                { return s.currentDoc }

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. This scorer does not expose
// per-block impact information.
func (s *svInOrderScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *svInOrderScorer) NextDoc() (int, error) {
	doc, err := s.matchingDocsIter.NextDoc()
	s.currentDoc = doc
	return doc, err
}
func (s *svInOrderScorer) Advance(target int) (int, error) {
	doc, err := s.matchingDocsIter.Advance(target)
	s.currentDoc = doc
	return doc, err
}
func (s *svInOrderScorer) Cost() int64      { return s.cost }
func (s *svInOrderScorer) DocIDRunEnd() int { return s.currentDoc + 1 }

var _ search.Scorer = (*svInOrderScorer)(nil)

// --- MVInOrderScorer ----------------------------------------------------------

// mvInOrderScorer is the multi-value scorer: the first-seen score for a doc
// is kept (subsequent scores for the same doc are discarded).  Mirrors
// MVInOrderScorer / MVInnerScorer in Lucene 10.4.0.
type mvInOrderScorer struct {
	matchingDocsIter search.DocIdSetIterator
	docScores        []float32
	cost             int64
	boost            float32
	currentDoc       int
}

func newMVInOrderScorer(
	q *TermsIncludingScoreQuery,
	termsEnum index.TermsEnum,
	maxDoc int,
	cost int64,
	boost float32,
) (*mvInOrderScorer, error) {
	matchingDocs, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}
	docScores := make([]float32, maxDoc)
	if err := fillDocsAndScoresMV(matchingDocs, docScores, q, termsEnum); err != nil {
		return nil, err
	}
	iter := util.NewBitSetIterator(matchingDocs.AsReadOnlyBits(), cost)
	return &mvInOrderScorer{
		matchingDocsIter: iter,
		docScores:        docScores,
		cost:             cost,
		boost:            boost,
		currentDoc:       -1,
	}, nil
}

// fillDocsAndScoresMV populates matchingDocs and docScores for the MV case.
// The first-seen score for a document wins (consistent with Lucene's MVInnerScorer).
func fillDocsAndScoresMV(
	matchingDocs *util.FixedBitSet,
	docScores []float32,
	q *TermsIncludingScoreQuery,
	termsEnum index.TermsEnum,
) error {
	spare := util.NewBytesRefEmpty()
	var pe index.PostingsEnum
	for i := 0; i < q.terms.Size(); i++ {
		q.terms.Get(q.ords[i], spare)
		seekTerm := index.NewTermFromBytesRef(q.toField, spare)
		found, err := termsEnum.SeekExact(seekTerm)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		pe, err = termsEnum.Postings(0) // NONE flag
		if err != nil {
			return err
		}
		score := q.scores[q.ords[i]]
		for {
			doc, err := pe.NextDoc()
			if err != nil {
				return err
			}
			if doc == index.NO_MORE_DOCS {
				break
			}
			// First-wins: only set the score if the bit was not already set.
			if !matchingDocs.Get(doc) {
				matchingDocs.Set(doc)
				docScores[doc] = score
			}
		}
	}
	return nil
}

func (s *mvInOrderScorer) Score() float32            { return s.docScores[s.currentDoc] * s.boost }
func (s *mvInOrderScorer) GetMaxScore(_ int) float32 { return float32(math.Inf(1)) }
func (s *mvInOrderScorer) DocID() int                { return s.currentDoc }

// AdvanceShallow returns search.NO_MORE_DOCS, the default defined by
// org.apache.lucene.search.Scorer#advanceShallow. This scorer does not expose
// per-block impact information.
func (s *mvInOrderScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *mvInOrderScorer) NextDoc() (int, error) {
	doc, err := s.matchingDocsIter.NextDoc()
	s.currentDoc = doc
	return doc, err
}
func (s *mvInOrderScorer) Advance(target int) (int, error) {
	doc, err := s.matchingDocsIter.Advance(target)
	s.currentDoc = doc
	return doc, err
}
func (s *mvInOrderScorer) Cost() int64      { return s.cost }
func (s *mvInOrderScorer) DocIDRunEnd() int { return s.currentDoc + 1 }

var _ search.Scorer = (*mvInOrderScorer)(nil)

// interface compliance
var _ search.Query = (*TermsIncludingScoreQuery)(nil)
