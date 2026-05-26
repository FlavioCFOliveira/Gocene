// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanTermQuery.java

package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// constants for position cost estimation, mirroring Java.
const (
	phraseToSpanTermPositionsCost float32 = 4.0
	termPosnsSeekOpsPerDoc                = 128
	termOpsPerPos                         = 7
)

// SpanTermQuery matches spans containing a single term.
//
// Mirrors org.apache.lucene.queries.spans.SpanTermQuery.
//
// Deviations from Java:
//   - Java holds a TermStates that is pre-built; Gocene performs a live
//     SeekExact + Postings call in GetSpans because TermStates is a skeleton
//     (backlog #2709) without a Build helper.
//   - The inner SpanTermWeight is a package-level struct, not an inner class.
type SpanTermQuery struct {
	search.BaseQuery
	term *index.Term
}

// NewSpanTermQuery constructs a SpanTermQuery for the given term.
func NewSpanTermQuery(term *index.Term) *SpanTermQuery {
	return &SpanTermQuery{term: term}
}

// GetField returns the field targeted by this query.
func (q *SpanTermQuery) GetField() string { return q.term.Field }

// GetTerm returns the term this query matches.
func (q *SpanTermQuery) GetTerm() *index.Term { return q.term }

// Visit walks the query tree.
func (q *SpanTermQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.term.Field) {
		visitor.ConsumeTerms(q, q.term)
	}
}

// Clone returns a shallow copy.
func (q *SpanTermQuery) Clone() search.Query {
	cp := *q
	return &cp
}

// Equals reports structural equality.
func (q *SpanTermQuery) Equals(other search.Query) bool {
	o, ok := other.(*SpanTermQuery)
	if !ok {
		return false
	}
	return q.term.Field == o.term.Field && util.BytesRefEquals(q.term.Bytes, o.term.Bytes)
}

// HashCode returns a hash code.
func (q *SpanTermQuery) HashCode() int {
	h := 0
	for _, b := range q.term.Field {
		h = h*31 + int(b)
	}
	if q.term.Bytes != nil {
		for _, b := range q.term.Bytes.Bytes {
			h = h*31 + int(b)
		}
	}
	return h
}

// String returns the canonical Lucene rendering.
func (q *SpanTermQuery) String() string {
	text := ""
	if q.term.Bytes != nil {
		text = q.term.Bytes.String()
	}
	return fmt.Sprintf("%s:%s", q.term.Field, text)
}

// CreateWeight creates a Weight for this query (non-span path, used by IndexSearcher).
func (q *SpanTermQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return q.createSpanWeight(searcher, needsScores, boost)
}

// CreateSpanWeight creates a SpanWeight for this query.
func (q *SpanTermQuery) CreateSpanWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (*SpanWeight, error) {
	return q.createSpanWeight(searcher, needsScores, boost)
}

func (q *SpanTermQuery) createSpanWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (*SpanWeight, error) {
	term := q.term
	return NewSpanWeight(q, SpanWeightConfig{
		Field:     term.Field,
		SimScorer: nil, // scoring deferred; no SimScorer API on IndexSearcher yet
		GetSpans: func(ctx *index.LeafReaderContext, postings Postings) (Spans, error) {
			return getSpansForTerm(ctx, term, postings)
		},
		ExtractStates: func(terms map[string]*index.TermStates) {
			// TermStates skeleton — omit; full build deferred to backlog #2709.
		},
		IsCacheable: func(*index.LeafReaderContext) bool { return true },
	}), nil
}

// getSpansForTerm looks up the term in the leaf reader and returns a TermSpans.
// Returns nil if the term is absent or the field has no positions indexed.
func getSpansForTerm(ctx *index.LeafReaderContext, term *index.Term, postingsLevel Postings) (Spans, error) {
	if ctx == nil {
		return nil, nil
	}
	lr := ctx.LeafReader()
	if lr == nil {
		return nil, nil
	}
	terms, err := lr.Terms(term.Field)
	if err != nil {
		return nil, err
	}
	if terms == nil {
		return nil, nil
	}
	if !terms.HasPositions() {
		text := ""
		if term.Bytes != nil {
			text = term.Bytes.String()
		}
		return nil, fmt.Errorf("field %q was indexed without position data; cannot run SpanTermQuery (term=%q)",
			term.Field, text)
	}
	te, err := terms.GetIterator()
	if err != nil {
		return nil, err
	}
	found, err := te.SeekExact(term)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	positionsCost, err := termPositionsCost(te)
	if err != nil {
		return nil, err
	}
	pe, err := te.Postings(postingsLevel.GetRequiredPostings())
	if err != nil {
		return nil, err
	}
	// Store a copy of the term value for the collector.
	termCopy := *term
	return NewTermSpans(pe, termCopy, positionsCost*phraseToSpanTermPositionsCost), nil
}

// termPositionsCost estimates the per-document position iteration cost.
// Mirrors SpanTermQuery.termPositionsCost.
func termPositionsCost(te index.TermsEnum) (float32, error) {
	docFreq, err := te.DocFreq()
	if err != nil {
		return 0, err
	}
	if docFreq <= 0 {
		return 0, nil
	}
	totalTermFreq, err := te.TotalTermFreq()
	if err != nil {
		return 0, err
	}
	if totalTermFreq <= 0 {
		totalTermFreq = int64(docFreq)
	}
	expOccurrencesPerDoc := float32(totalTermFreq) / float32(docFreq)
	return termPosnsSeekOpsPerDoc + expOccurrencesPerDoc*termOpsPerPos, nil
}

var _ search.Query = (*SpanTermQuery)(nil)
