// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermAndBoost pairs a term text with its per-term frequency boost.
type TermAndBoost struct {
	term  string
	boost float32
}

// SynonymQuery is a query that treats multiple terms as synonyms.
//
// For scoring purposes, this query tries to score the terms as if you had indexed them as one
// term: it will match any of the terms but only invoke the similarity a single time, scoring the
// sum of all term frequencies for the document.
//
// Port of org.apache.lucene.search.SynonymQuery.
type SynonymQuery struct {
	*BaseQuery
	terms []TermAndBoost
	field string
}

// SynonymQueryBuilder is a builder for SynonymQuery.
type SynonymQueryBuilder struct {
	field string
	terms []TermAndBoost
}

// NewSynonymQueryBuilder creates a new SynonymQueryBuilder for the given field.
func NewSynonymQueryBuilder(field string) *SynonymQueryBuilder {
	return &SynonymQueryBuilder{
		field: field,
		terms: make([]TermAndBoost, 0),
	}
}

// AddTerm adds the provided term as a synonym with the default boost of 1.0.
func (b *SynonymQueryBuilder) AddTerm(term *index.Term) *SynonymQueryBuilder {
	return b.AddTermWithBoost(term, 1.0)
}

// AddTermWithBoost adds the provided term as a synonym. Document frequencies of this term
// will be boosted by boost.
//
// Panics if the term field differs from the builder field.
func (b *SynonymQueryBuilder) AddTermWithBoost(term *index.Term, boost float32) *SynonymQueryBuilder {
	if term != nil && term.Field != b.field {
		panic("synonyms must be across the same field")
	}
	if math.IsNaN(float64(boost)) || boost <= 0 || boost > 1 {
		panic("boost must be a positive float between 0 (exclusive) and 1 (inclusive)")
	}

	var text string
	if term != nil {
		text = term.Text()
	}

	b.terms = append(b.terms, TermAndBoost{term: text, boost: boost})
	return b
}

// Build constructs the SynonymQuery. The terms are sorted before construction.
func (b *SynonymQueryBuilder) Build() *SynonymQuery {
	cp := make([]TermAndBoost, len(b.terms))
	copy(cp, b.terms)

	sort.Slice(cp, func(i, j int) bool {
		return cp[i].term < cp[j].term
	})

	return &SynonymQuery{
		BaseQuery: &BaseQuery{},
		terms:     cp,
		field:     b.field,
	}
}

// GetTerms returns the terms of this SynonymQuery.
func (q *SynonymQuery) GetTerms() []*index.Term {
	out := make([]*index.Term, len(q.terms))
	for i, tb := range q.terms {
		out[i] = index.NewTerm(q.field, tb.term)
	}
	return out
}

// GetField returns the field name of this SynonymQuery.
func (q *SynonymQuery) GetField() string {
	return q.field
}

// String returns a human-readable representation of the query.
// Mimics Lucene's toString(String field).
func (q *SynonymQuery) String() string {
	var b strings.Builder
	b.WriteString("Synonym(")
	for i, tb := range q.terms {
		if i != 0 {
			b.WriteByte(' ')
		}
		// In Lucene: new TermQuery(new Term(this.field, terms[i].term)).toString(field)
		// TermQuery.toString(field) typically returns "field:term"
		b.WriteString(q.field)
		b.WriteByte(':')
		b.WriteString(tb.term)
		if tb.boost != 1.0 {
			b.WriteByte('^')
			b.WriteString(strconv.FormatFloat(float64(tb.boost), 'f', -1, 32))
		}
	}
	b.WriteByte(')')
	return b.String()
}

// HashCode returns a hash code consistent with Equals.
func (q *SynonymQuery) HashCode() int {
	h := 31 * 0 // classHash() is usually a constant for the class
	for _, tb := range q.terms {
		// Arrays.hashCode(terms) for TermAndBoost
		// In Java, records implement hashCode. Here we simulate it.
		th := 0
		for _, c := range tb.term {
			th = 31*th + int(c)
		}
		th = 31*th + int(math.Float32bits(tb.boost))
		h = 31*h + th
	}
	for _, c := range q.field {
		h = 31*h + int(c)
	}
	return h
}

// Equals checks if this query is equal to another.
func (q *SynonymQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SynonymQuery)
	if !ok {
		return false
	}
	if q.field != o.field || len(q.terms) != len(o.terms) {
		return false
	}
	for i := range q.terms {
		if q.terms[i] != o.terms[i] {
			return false
		}
	}
	return true
}

// Rewrite rewrites the query to a simpler form.
func (q *SynonymQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	if len(q.terms) == 0 {
		// return new BooleanQuery.Builder().build();
		return NewBooleanQueryBuilder().Build(), nil
	}
	if len(q.terms) == 1 && q.terms[0].boost == 1.0 {
		return NewTermQuery(index.NewTerm(q.field, q.terms[0].term)), nil
	}
	return q, nil
}

// Visit implements the QueryVisitor pattern.
func (q *SynonymQuery) Visit(visitor QueryVisitor) {
	if !visitor.AcceptField(q.field) {
		return
	}
	v := visitor.GetSubVisitor(SHOULD, q)
	ts := make([]*index.Term, len(q.terms))
	for i, tb := range q.terms {
		ts[i] = index.NewTerm(q.field, tb.term)
	}
	v.ConsumeTerms(q, ts...)
}

// CreateWeight creates a Weight for this query.
func (q *SynonymQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	if scoreMode.NeedsScores() {
		return NewSynonymWeight(q, searcher, scoreMode.NeedsScores(), boost)
	}

	// if scores are not needed, let BooleanWeight deal with optimizing that case.
	bq := NewBooleanQueryBuilder()
	for _, tb := range q.terms {
		bq.Add(NewTermQuery(index.NewTerm(q.field, tb.term)), SHOULD)
	}

	rewritten, err := searcher.Rewrite(bq.Build())
	if err != nil {
		return nil, err
	}
	return rewritten.CreateWeight(searcher, scoreMode, boost)
}

// SynonymWeight is the Weight implementation for SynonymQuery.
type SynonymWeight struct {
	*BaseWeight
	field       string
	terms       []TermAndBoost
	searcher    *IndexSearcher
	needsScores bool
	similarity  Similarity
	simScorer   SimScorer
}

// NewSynonymWeight creates a new SynonymWeight.
func NewSynonymWeight(q *SynonymQuery, searcher *IndexSearcher, needsScores bool, boost float32) (*SynonymWeight, error) {
	w := &SynonymWeight{
		BaseWeight:  NewBaseWeight(q),
		field:       q.field,
		terms:       q.terms,
		searcher:    searcher,
		needsScores: needsScores,
	}

	// Collection statistics
	reader := searcher.GetIndexReader()
	collectionStats := NewCollectionStatistics(w.field, reader.MaxDoc(), reader.NumDocs(), -1, -1)

	var docFreq int
	var totalTermFreq int64
	for _, tb := range w.terms {
		term := index.NewTerm(w.field, tb.term)
		ts, err := index.BuildTermStates(searcher, term, true)
		if err != nil {
			return nil, err
		}
		if ts.DocFreq() > 0 {
			stats := searcher.TermStatistics(term, ts.DocFreq(), ts.TotalTermFreq())
			if stats.DocFreq() > docFreq {
				docFreq = stats.DocFreq()
			}
			totalTermFreq += stats.TotalTermFreq()
		}
	}

	w.similarity = searcher.GetSimilarity()
	if docFreq > 0 {
		pseudoStats := NewTermStatistics(index.NewTerm(w.field, "synonym pseudo-term"), docFreq, totalTermFreq)
		w.simScorer = w.similarity.Scorer104(boost, collectionStats, pseudoStats)
	}

	return w, nil
}

// Scorer creates a scorer for this weight.
func (w *SynonymWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	leafReader := context.LeafReader()
	if leafReader == nil {
		return nil, nil
	}
	terms, err := leafReader.Terms(w.field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}

	subs := make([]synonymSub, 0, len(w.terms))
	for _, tb := range w.terms {
		termsEnum, err := terms.GetIterator()
		if err != nil {
			return nil, err
		}
		found, err := termsEnum.SeekExact(index.NewTerm(w.field, tb.term))
		if err != nil || !found {
			continue
		}

		flags := 0
		if w.needsScores {
			flags = index.PostingsFlagFreqs
		}
		pe, err := termsEnum.Postings(flags)
		if err != nil || pe == nil {
			continue
		}
		subs = append(subs, synonymSub{postings: pe, boost: tb.boost})
	}

	if len(subs) == 0 {
		return nil, nil
	}
	return NewSynonymScorer(w, subs, w.simScorer), nil
}

// ScorerSupplier returns a ScorerSupplier for the given leaf reader context.
func (w *SynonymWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultScorerSupplier(scorer), nil
}

// Explain returns an explanation of the score for the given document.
func (w *SynonymWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer != nil {
		advanced, err := scorer.Iterator().Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			score, err := scorer.Score()
			if err != nil {
				return nil, err
			}

			result := MatchExplanation(score, fmt.Sprintf("weight(%v in %d) [%s], result of:",
				w.GetQuery(), doc, w.similarityName()))

			if ss, ok := scorer.(*SynonymScorer); ok {
				freq := ss.Freq()
				if _, classic := w.simScorer.(*ClassicSimScorer); classic {
					tfValue := float32(tf(float64(freq)))
					idfFactor := float32(1)
					if tfValue != 0 {
						idfFactor = score / tfValue
					}
					scoreExpl := MatchExplanation(score, "score(freq), product of:")
					scoreExpl.AddDetail(MatchExplanation(
						idfFactor, "idf, computed as log(maxDocs/docFreq)"))
					scoreExpl.AddDetail(MatchExplanationWithDetails(
						tfValue,
						fmt.Sprintf("tf(freq=%v), with freq of:", freq),
						MatchExplanation(freq, fmt.Sprintf("termFreq=%v", freq))))
					result.AddDetail(scoreExpl)
				} else {
					result.AddDetail(MatchExplanation(freq, fmt.Sprintf("termFreq=%v", freq)))
				}
			}
			return result, nil
		}
	}
	return NoMatchExplanation("no matching terms"), nil
}

func (w *SynonymWeight) similarityName() string {
	if w.similarity == nil {
		return "Similarity"
	}
	if s, ok := w.similarity.(interface{ String() string }); ok {
		return s.String()
	}
	return "Similarity"
}

// BulkScorer returns a BulkScorer for efficient bulk scoring.
func (w *SynonymWeight) BulkScorer(ctx *index.LeafReaderContext) (BulkScorer, error) {
	return nil, nil
}

// Count returns the count of matching documents in sub-linear time.
func (w *SynonymWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns Matches for a specific document.
func (w *SynonymWeight) Matches(ctx *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *SynonymWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Ensure SynonymWeight implements Weight.
var _ Weight = (*SynonymWeight)(nil)

// Ensure SynonymQuery implements Query.
var _ Query = (*SynonymQuery)(nil)
