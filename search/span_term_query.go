// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanTermQuery matches documents containing a specific term at specific positions.
// This is the Go port of Lucene's org.apache.lucene.search.spans.SpanTermQuery.
type SpanTermQuery struct {
	BaseSpanQuery
	term *index.Term
}

// NewSpanTermQuery creates a new SpanTermQuery.
func NewSpanTermQuery(term *index.Term) *SpanTermQuery {
	return &SpanTermQuery{
		BaseSpanQuery: *NewBaseSpanQuery(term.Field),
		term:          term,
	}
}

// Term returns the term.
func (q *SpanTermQuery) Term() *index.Term {
	return q.term
}

// Rewrite rewrites this query to a more primitive form.
func (q *SpanTermQuery) Rewrite(reader IndexReader) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
//
// It ports org.apache.lucene.queries.spans.SpanTermQuery.createWeight: a
// SpanTermWeight is returned, carrying the similarity used to score the term's
// spans when scores are required.
func (q *SpanTermQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	var similarity Similarity
	if needsScores {
		similarity = NewClassicSimilarity()
	}
	w := &SpanTermWeight{
		SpanWeight:  NewSpanWeight(q, similarity),
		term:        q.term,
		searcher:    searcher,
		needsScores: needsScores,
	}
	if needsScores && searcher != nil {
		w.simScorer = w.buildSimScorer(searcher)
	}
	return w, nil
}

// SpanTermWeight is the Weight implementation for SpanTermQuery.
//
// This is the Go port of
// org.apache.lucene.queries.spans.SpanTermQuery.SpanTermWeight. It builds a
// postings-backed Spans (TermSpans) over the term's positions so that
// SpanScorer can produce matches and similarity-based scores.
type SpanTermWeight struct {
	*SpanWeight
	term        *index.Term
	searcher    *IndexSearcher
	needsScores bool
	simScorer   SimScorer
}

// buildSimScorer computes the similarity scorer for the term: collection
// statistics for the field combined with the term's own statistics.
//
// It mirrors the always-build behaviour of the sibling TermWeight/PhraseWeight
// (rather than Lucene SpanWeight.buildSimWeight, which returns null when no term
// statistics are available). Those weights construct the ClassicSimScorer
// unconditionally so that the search path and Explain path share the same
// scorer, preserving Lucene's value==score invariant; SpanTermWeight follows the
// same contract. The per-term docFreq is looked up best-effort; when it is
// unavailable the ClassicSimScorer falls back to a unit IDF.
func (w *SpanTermWeight) buildSimScorer(searcher *IndexSearcher) SimScorer {
	if w.Similarity == nil {
		return nil
	}
	reader := searcher.GetIndexReader()
	collectionStats := NewCollectionStatistics(w.term.Field, reader.MaxDoc(), reader.NumDocs(), -1, -1)

	docFreq := 0
	var totalTermFreq int64 = -1
	if r, ok := searcher.GetIndexReader().(index.IndexReaderInterface); ok {
		if leafReader, ok := r.(*index.LeafReader); ok {
			terms, err := leafReader.Terms(w.term.Field)
			if err == nil && terms != nil {
				if termsEnum, err := terms.GetIterator(); err == nil {
					if found, err := termsEnum.SeekExact(w.term); err == nil && found {
						if df, err := termsEnum.DocFreq(); err == nil {
							docFreq = df
						}
						if ttf, err := termsEnum.TotalTermFreq(); err == nil {
							totalTermFreq = ttf
						}
					}
				}
			}
		}
	}
	termStats := NewTermStatistics(w.term, docFreq, totalTermFreq)
	return w.Similarity.Scorer(collectionStats, termStats)
}

// GetSpans returns a postings-backed Spans over the term's positions for the
// given leaf, or nil if the term is absent or the field lacks positions.
//
// It ports SpanTermQuery.SpanTermWeight.getSpans: seek the term, open a
// PostingsEnum with positions, and wrap it in a TermSpans. The requiredPostings
// argument is accepted for signature parity with the base SpanWeight; positions
// are always requested since a Spans is meaningless without them.
func (w *SpanTermWeight) GetSpans(ctx *index.LeafReaderContext, requiredPostings int) (*Spans, error) {
	leafReader := ctx.LeafReader()
	if leafReader == nil {
		return nil, nil
	}
	terms, err := leafReader.Terms(w.term.Field)
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
	found, err := termsEnum.SeekExact(w.term)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	postings, err := termsEnum.Postings(index.PostingsFlagPositions)
	if err != nil {
		return nil, err
	}
	if postings == nil {
		return nil, nil
	}
	return NewTermSpans(postings), nil
}

// Scorer creates a scorer for this weight by wrapping the term's TermSpans in a
// SpanScorer. It returns nil when the term is absent from the leaf (no spans).
func (w *SpanTermWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	spans, err := w.GetSpans(context, index.PostingsFlagPositions)
	if err != nil {
		return nil, err
	}
	if spans == nil {
		return nil, nil
	}
	return NewSpanScorerWithSimilarity(spans, 1.0, w.simScorer), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *SpanTermWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewScorerSupplierAdapter(scorer), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *SpanTermWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// Explain returns an explanation of the score for the given document.
//
// It ports org.apache.lucene.queries.spans.SpanWeight.explain: a SpanScorer is
// pulled for the leaf and advanced to doc. A match yields "weight(<query> in
// <doc>) [<similarity>], result of:" whose value equals the live scorer's score,
// carrying a phraseFreq sub-explanation built from the span's sloppy frequency.
// When scores are not needed it yields a zero-valued "match ... without score"
// explanation. A non-match yields "no matching term".
//
// Divergence from Lucene 10.4.0: as with the other ported weights, the value is
// taken from the live SpanScorer (preserving value==score) rather than
// re-derived through SimScorer.explain, and norms are not consulted.
func (w *SpanTermWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if ss, ok := scorer.(*SpanScorer); ok && ss != nil {
		advanced, err := ss.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advanced == doc {
			if w.simScorer != nil {
				score := ss.Score()
				freq := ss.SloppyFreq()
				result := MatchExplanation(score, fmt.Sprintf("weight(%s in %d) [%s], result of:",
					w.SpanQuery.String(""), doc, w.similarityName()))
				result.AddDetail(MatchExplanation(freq, fmt.Sprintf("phraseFreq=%v", freq)))
				return result, nil
			}
			return MatchExplanation(0, fmt.Sprintf("match %s in %d without score", w.SpanQuery.String(""), doc)), nil
		}
	}
	return NoMatchExplanation("no matching term"), nil
}

// similarityName returns the descriptive name of the similarity backing this
// weight for use in explanations.
func (w *SpanTermWeight) similarityName() string {
	if w.Similarity == nil {
		return "Similarity"
	}
	if s, ok := w.Similarity.(interface{ String() string }); ok {
		return s.String()
	}
	return "Similarity"
}

// Ensure SpanTermWeight implements Weight.
var _ Weight = (*SpanTermWeight)(nil)

// Clone creates a copy of this query.
func (q *SpanTermQuery) Clone() Query {
	return NewSpanTermQuery(index.NewTerm(q.term.Field, q.term.Text()))
}

// Equals checks if this query equals another.
func (q *SpanTermQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
	if o, ok := other.(*SpanTermQuery); ok {
		return q.term.Equals(o.term)
	}
	return false
}

// HashCode returns a hash code for this query.
func (q *SpanTermQuery) HashCode() int {
	return q.term.HashCode()
}

// String returns a string representation of the query.
func (q *SpanTermQuery) String(field string) string {
	if field == "" || field != q.field {
		return fmt.Sprintf("SpanTermQuery(field=%s, term=%s)", q.field, q.term.Text())
	}
	return fmt.Sprintf("SpanTermQuery(term=%s)", q.term.Text())
}

// Ensure SpanTermQuery implements SpanQuery
var _ SpanQuery = (*SpanTermQuery)(nil)
