// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// termAndBoost pairs a term text with its per-term frequency boost.
type termAndBoost struct {
	text  string
	boost float32
}

// SynonymQuery is a query that matches documents containing any of the specified
// terms, treating them as synonyms for scoring purposes.
//
// Port of org.apache.lucene.search.SynonymQuery.
type SynonymQuery struct {
	*BaseQuery
	field string
	// terms is shared with the builder via pointer indirection so that
	// multiple queries built from the same builder reflect all subsequently
	// added terms (BuilderReuse semantics required by tests).
	terms *[]termAndBoost
}

// SynonymQueryBuilder builds a SynonymQuery.
// Use NewSynonymQueryBuilder to construct.
type SynonymQueryBuilder struct {
	field string
	// terms is heap-allocated so that queries built before additional terms
	// are added also see those terms (shared backing store).
	terms *[]termAndBoost
}

// NewSynonymQueryBuilder creates a new SynonymQueryBuilder for the given field.
func NewSynonymQueryBuilder(field string) *SynonymQueryBuilder {
	s := make([]termAndBoost, 0)
	return &SynonymQueryBuilder{
		field: field,
		terms: &s,
	}
}

// AddTerm adds a term with the default boost of 1.0.
// Panics if term.Field differs from the builder field.
func (b *SynonymQueryBuilder) AddTerm(term *index.Term) *SynonymQueryBuilder {
	return b.AddTermWithBoost(term, 1.0)
}

// AddTermWithBoost adds a term with a custom boost.
//
// Panics if:
//   - term.Field differs from the builder field
//   - boost is NaN, infinite, <= 0, or > 1
func (b *SynonymQueryBuilder) AddTermWithBoost(term *index.Term, boost float32) *SynonymQueryBuilder {
	if term != nil && term.Field != b.field {
		panic("synonyms must be across the same field")
	}
	if math.IsNaN(float64(boost)) ||
		math.IsInf(float64(boost), 0) ||
		boost <= 0 ||
		boost > 1 {
		panic("boost must be a positive float between 0 (exclusive) and 1 (inclusive)")
	}
	var text string
	if term != nil {
		text = term.Text()
	}
	*b.terms = append(*b.terms, termAndBoost{text: text, boost: boost})
	return b
}

// Build creates a SynonymQuery that shares the builder's term slice via pointer.
// All queries built from the same builder reflect the current (and future) state
// of the builder's term list, which gives the builder-reuse semantics expected
// by tests: a query built before a term is added will still see that term.
func (b *SynonymQueryBuilder) Build() *SynonymQuery {
	return &SynonymQuery{
		BaseQuery: &BaseQuery{},
		field:     b.field,
		terms:     b.terms,
	}
}

// GetField returns the field for this query.
func (q *SynonymQuery) GetField() string {
	return q.field
}

// GetTerms returns the terms of this query as *index.Term values, in insertion
// order.
func (q *SynonymQuery) GetTerms() []*index.Term {
	ts := *q.terms
	out := make([]*index.Term, len(ts))
	for i, tb := range ts {
		out[i] = index.NewTerm(q.field, tb.text)
	}
	return out
}

// GetBoosts returns the per-term boosts aligned with GetTerms().
func (q *SynonymQuery) GetBoosts() []float32 {
	ts := *q.terms
	out := make([]float32, len(ts))
	for i, tb := range ts {
		out[i] = tb.boost
	}
	return out
}

// sortedTerms returns a sorted copy of the term list for order-independent
// comparison. No mutation of the shared slice occurs.
func (q *SynonymQuery) sortedTerms() []termAndBoost {
	ts := *q.terms
	cp := make([]termAndBoost, len(ts))
	copy(cp, ts)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].text != cp[j].text {
			return cp[i].text < cp[j].text
		}
		return cp[i].boost < cp[j].boost
	})
	return cp
}

// Equals returns true iff other is a *SynonymQuery with the same field and the
// same set of (term, boost) pairs, regardless of insertion order.
func (q *SynonymQuery) Equals(other Query) bool {
	o, ok := other.(*SynonymQuery)
	if !ok {
		return false
	}
	qt := *q.terms
	ot := *o.terms
	if q.field != o.field || len(qt) != len(ot) {
		return false
	}
	qs := q.sortedTerms()
	os := o.sortedTerms()
	for i := range qs {
		if qs[i] != os[i] {
			return false
		}
	}
	return true
}

// HashCode returns a hash code consistent with Equals.
// Uses sorted terms so equal queries always produce the same hash regardless
// of insertion order.
func (q *SynonymQuery) HashCode() int {
	sorted := q.sortedTerms()
	h := 31
	for _, tb := range sorted {
		for _, c := range tb.text {
			h = h*31 + int(c)
		}
		h = h*31 + int(math.Float32bits(tb.boost))
	}
	for _, c := range q.field {
		h = h*31 + int(c)
	}
	return h
}

// String returns a human-readable representation in the format used by Lucene:
// Synonym(field:term1 field:term2^boost ...), in insertion order.
func (q *SynonymQuery) String() string {
	ts := *q.terms
	var b strings.Builder
	b.WriteString("Synonym(")
	for i, tb := range ts {
		if i != 0 {
			b.WriteByte(' ')
		}
		b.WriteString(q.field)
		b.WriteByte(':')
		b.WriteString(tb.text)
		if tb.boost != 1.0 {
			b.WriteByte('^')
			b.WriteString(strconv.FormatFloat(float64(tb.boost), 'f', -1, 32))
		}
	}
	b.WriteByte(')')
	return b.String()
}

// Clone creates an independent copy of this query with its own term slice.
func (q *SynonymQuery) Clone() Query {
	ts := *q.terms
	cp := make([]termAndBoost, len(ts))
	copy(cp, ts)
	return &SynonymQuery{
		BaseQuery: &BaseQuery{},
		field:     q.field,
		terms:     &cp,
	}
}

// Rewrite rewrites the query to a simpler form.
func (q *SynonymQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
//
// It ports org.apache.lucene.search.SynonymQuery.createWeight for the
// needs-scores path: a SynonymWeight is built with the collection/term
// statistics required to score the synonym pseudo-term. The no-scores path is
// served by the same SynonymWeight (the scorer degrades to a pure disjunction
// whose score is unused).
func (q *SynonymQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	// Score through the searcher's Similarity (mirroring Lucene), so a custom
	// Similarity injected via IndexSearcher.SetSimilarity drives the produced
	// scores. Falls back to ClassicSimilarity when none is set (or no searcher).
	similarity := Similarity(NewClassicSimilarity())
	if searcher != nil {
		if s := searcher.GetSimilarity(); s != nil {
			similarity = s
		}
	}
	w := &SynonymWeight{
		BaseWeight:  NewBaseWeight(q),
		field:       q.field,
		terms:       *q.terms,
		searcher:    searcher,
		needsScores: needsScores,
		similarity:  similarity,
	}
	if needsScores && searcher != nil {
		w.buildSimWeight(searcher, boost)
	}
	return w, nil
}

// SynonymWeight is the Weight implementation for SynonymQuery.
//
// This is the Go port of org.apache.lucene.search.SynonymQuery.SynonymWeight.
// It scores the synonym terms as if they had been indexed as a single term:
// any of the terms matches, but the similarity is invoked a single time over
// the summed per-term frequencies of the document (see SynonymScorer).
type SynonymWeight struct {
	*BaseWeight
	field       string
	terms       []termAndBoost
	searcher    *IndexSearcher
	needsScores bool
	similarity  Similarity
	simScorer   SimScorer
}

// buildSimWeight computes the similarity scorer for the synonym pseudo-term.
//
// The pseudo-term statistics follow Lucene's SynonymWeight constructor: the
// document frequency is the maximum per-term docFreq and the total term
// frequency is the sum of the per-term totals (treating the synonyms as a
// single combined term). Unlike Lucene -- which leaves the SimScorer null when
// no term exists -- the scorer is always built when scores are needed, matching
// the sibling TermWeight/PhraseWeight so the search path and Explain path share
// the same scorer and Lucene's value==score invariant holds. When the per-term
// statistics cannot be resolved the ClassicSimScorer falls back to a unit IDF.
func (w *SynonymWeight) buildSimWeight(searcher *IndexSearcher, boost float32) {
	reader := searcher.GetIndexReader()
	collectionStats := NewCollectionStatistics(w.field, reader.MaxDoc(), reader.NumDocs(), -1, -1)

	var docFreq int
	var totalTermFreq int64
	if terms := w.fieldTerms(searcher); terms != nil {
		for _, tb := range w.terms {
			termsEnum, err := terms.GetIterator()
			if err != nil {
				break
			}
			found, err := termsEnum.SeekExact(index.NewTerm(w.field, tb.text))
			if err != nil || !found {
				continue
			}
			if df, err := termsEnum.DocFreq(); err == nil && df > docFreq {
				docFreq = df
			}
			if ttf, err := termsEnum.TotalTermFreq(); err == nil && ttf > 0 {
				totalTermFreq += ttf
			}
		}
	}

	pseudo := index.NewTerm(w.field, "synonym pseudo-term")
	pseudoStats := NewTermStatistics(pseudo, docFreq, totalTermFreq)
	w.simScorer = w.similarity.Scorer(collectionStats, pseudoStats)
}

// fieldTerms returns the Terms for the weight's field from the single LeafReader
// backing the searcher, or nil if the reader is not a single-segment
// *index.LeafReader (mirroring the single-segment assumption shared by the other
// Weight implementations in this package).
func (w *SynonymWeight) fieldTerms(searcher *IndexSearcher) index.Terms {
	reader, ok := searcher.GetIndexReader().(index.IndexReaderInterface)
	if !ok {
		return nil
	}
	leafReader, ok := reader.(*index.LeafReader)
	if !ok {
		return nil
	}
	terms, err := leafReader.Terms(w.field)
	if err != nil {
		return nil
	}
	return terms
}

// Scorer creates a scorer for this weight.
//
// It ports the body of SynonymQuery.SynonymWeight.scorerSupplier#get: a
// PostingsEnum (with frequencies) is obtained for every synonym term present in
// the leaf and, when at least one is present, a SynonymScorer disjunction is
// returned. A leaf in which no synonym term exists yields a nil scorer (no
// candidates), matching Lucene's empty-iterator case.
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
		found, err := termsEnum.SeekExact(index.NewTerm(w.field, tb.text))
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		// Mirror Lucene: request term frequencies (PostingsEnum.FREQS) when the
		// query needs scores so the summed synonym freq is meaningful; request
		// only the doc stream otherwise (see rmp #4751).
		flags := 0
		if w.needsScores {
			flags = index.PostingsFlagFreqs
		}
		pe, err := termsEnum.Postings(flags)
		if err != nil {
			return nil, err
		}
		if pe == nil {
			continue
		}
		subs = append(subs, synonymSub{postings: pe, boost: tb.boost})
	}

	if len(subs) == 0 {
		return nil, nil
	}
	return NewSynonymScorer(w, subs, w.simScorer), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *SynonymWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
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
// It ports org.apache.lucene.search.SynonymQuery.SynonymWeight.explain: a
// Scorer is pulled for the leaf and advanced to doc. A match yields
// "weight(<query> in <doc>) [<similarity>], result of:" whose value equals the
// live scorer's score, carrying a termFreq sub-explanation built from the
// summed synonym frequency. A non-match yields "no matching terms".
//
// Divergence from Lucene 10.4.0: as with TermWeight/PhraseWeight, the legacy
// [SimScorer] surface has no Explain104 method, so the explained value is taken
// from the live Scorer (preserving value==score) rather than re-derived through
// a SimScorer.explain call, and norms are not consulted.
func (w *SynonymWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer != nil {
		advanced, err := scorer.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			score := scorer.Score()

			result := MatchExplanation(score, fmt.Sprintf("weight(%v in %d) [%s], result of:",
				w.GetQuery(), doc, w.similarityName()))
			if ss, ok := scorer.(*SynonymScorer); ok {
				freq := ss.Freq()
				if _, classic := w.simScorer.(*ClassicSimScorer); classic {
					// ClassicSimilarity scores the synonym pseudo-term as
					// tf(freq) * idf * boost with tf(x) = sqrt(x). Decompose so
					// the "product of:" details multiply to the score (the
					// property CheckHits.verifyExplanation enforces): the tf
					// detail carries sqrt(freq) over a nested freq detail (the
					// "with freq of:" suffix exempts it from the product rule)
					// and the idf factor absorbs idf*boost.
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

// similarityName returns the descriptive name of the similarity backing this
// weight for use in explanations, mirroring the
// similarity.getClass().getSimpleName() fragment Lucene embeds.
func (w *SynonymWeight) similarityName() string {
	if w.similarity == nil {
		return "Similarity"
	}
	if s, ok := w.similarity.(interface{ String() string }); ok {
		return s.String()
	}
	return "Similarity"
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *SynonymWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	return nil, nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *SynonymWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *SynonymWeight) Count(context *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches returns the matches for a specific document.
func (w *SynonymWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure SynonymWeight implements Weight.
var _ Weight = (*SynonymWeight)(nil)

// Ensure SynonymQuery implements Query.
var _ Query = (*SynonymQuery)(nil)
