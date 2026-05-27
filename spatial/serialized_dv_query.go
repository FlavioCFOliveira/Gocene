// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// serializedDVQuery is the Go port of the per-doc, post-filtering query
// produced by SerializedDVStrategy.makeQueryDistanceScore in Lucene
// 10.4.0. It pairs a SpatialOperation with a reference query Shape, and
// at scoring time iterates every document in the leaf, reads the binary
// doc-values payload via the configured strategy, deserialises it back
// into a Shape, and applies the operation predicate.
//
// The query is identity-preserving (Equals / HashCode / String reflect
// strategy + operation + shape) so consumers can reason about query
// caching, rewriting and explain output without touching the scorer
// internals.
//
// # Foundation gap
//
// At the time of porting, the Gocene LeafReader returns (nil, nil) from
// GetBinaryDocValues — the codec wiring for binary doc values still
// lands incrementally over Sprints 118+. When the per-leaf reader does
// not yet expose binary doc values, the scorer reports zero matches
// rather than fabricating a MatchAllDocs result; this keeps the
// integration-test surface honest until the foundation gap closes.
// The algorithmic substance — the per-document predicate evaluation —
// is implemented and unit-tested independently via
// SerializedDVStrategy.matchShape.
type serializedDVQuery struct {
	*search.BaseQuery
	strategy   *SerializedDVStrategy
	operation  SpatialOperation
	queryShape Shape
}

// newSerializedDVQuery constructs a serializedDVQuery for the given
// strategy, operation and reference shape. Callers must enter through
// SerializedDVStrategy.MakeQuery; the constructor stays package-private
// because the contract on shape and strategy is enforced upstream.
func newSerializedDVQuery(strategy *SerializedDVStrategy, op SpatialOperation, shape Shape) *serializedDVQuery {
	return &serializedDVQuery{
		BaseQuery:  &search.BaseQuery{},
		strategy:   strategy,
		operation:  op,
		queryShape: shape,
	}
}

// Operation returns the spatial predicate this query evaluates.
func (q *serializedDVQuery) Operation() SpatialOperation { return q.operation }

// QueryShape returns the reference shape the predicate is applied
// against.
func (q *serializedDVQuery) QueryShape() Shape { return q.queryShape }

// Strategy returns the SerializedDVStrategy that produced this query.
func (q *serializedDVQuery) Strategy() *SerializedDVStrategy { return q.strategy }

// String returns a debug representation in the same shape as Lucene's
// SerializedDVStrategy.MakeQuery toString.
func (q *serializedDVQuery) String() string {
	return fmt.Sprintf("SerializedDVQuery(field=%s,op=%s,shape=%v)",
		q.strategy.dvFieldName, q.operation, q.queryShape)
}

// Clone produces an independent copy of this query.
func (q *serializedDVQuery) Clone() search.Query {
	return newSerializedDVQuery(q.strategy, q.operation, q.queryShape)
}

// Equals tests identity against another query.
func (q *serializedDVQuery) Equals(other search.Query) bool {
	o, ok := other.(*serializedDVQuery)
	if !ok {
		return false
	}
	if q.strategy != o.strategy {
		return false
	}
	if q.operation != o.operation {
		return false
	}
	return q.queryShape == o.queryShape
}

// HashCode returns a stable hash for this query.
func (q *serializedDVQuery) HashCode() int {
	h := 17
	h = 31*h + hashCode(q.strategy.dvFieldName)
	h = 31*h + int(q.operation)
	return h
}

// Rewrite returns the query itself; the doc-values predicate cannot be
// reduced to a primitive Term query.
func (q *serializedDVQuery) Rewrite(reader search.IndexReader) (search.Query, error) {
	return q, nil
}

// CreateWeight produces a Weight that, when supplied with a leaf
// reader exposing BinaryDocValues, runs the per-doc predicate
// implemented by SerializedDVStrategy.matchShape against every
// document in the leaf. Without a BinaryDocValues path the weight
// scores zero documents — see the type doc for the foundation-gap
// rationale.
func (q *serializedDVQuery) CreateWeight(searcher *search.IndexSearcher, needsScores bool, boost float32) (search.Weight, error) {
	return &serializedDVWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		boost:      boost,
	}, nil
}

// serializedDVWeight is the per-search Weight produced by
// serializedDVQuery.CreateWeight.
type serializedDVWeight struct {
	*search.BaseWeight
	query *serializedDVQuery
	boost float32
}

// Scorer produces a scorer that filters the leaf's documents by the
// per-doc binary doc-values predicate. Falls back to a no-doc scorer
// when the leaf reader does not yet expose binary doc values.
func (w *serializedDVWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	if ctx == nil {
		return search.NewMatchNoDocsScorer(w), nil
	}
	reader := ctx.LeafReader()
	if reader == nil {
		return search.NewMatchNoDocsScorer(w), nil
	}

	type binaryDVReader interface {
		GetBinaryDocValues(field string) (index.BinaryDocValues, error)
	}
	r, ok := reader.(binaryDVReader)
	if !ok {
		return search.NewMatchNoDocsScorer(w), nil
	}
	bdv, err := r.GetBinaryDocValues(w.query.strategy.dvFieldName)
	if err != nil {
		return nil, err
	}
	if bdv == nil {
		return search.NewMatchNoDocsScorer(w), nil
	}

	maxDocReader, ok := reader.(interface{ MaxDoc() int })
	if !ok {
		return search.NewMatchNoDocsScorer(w), nil
	}

	return &serializedDVScorer{
		BaseScorer: search.NewBaseScorer(w),
		weight:     w,
		bdv:        bdv,
		maxDoc:     maxDocReader.MaxDoc(),
		doc:        -1,
		score:      w.boost,
	}, nil
}

// ScorerSupplier wraps Scorer to satisfy the Weight contract.
func (w *serializedDVWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewScorerSupplierAdapter(scorer), nil
}

// Explain reports a coarse explanation: matched or not matched.
func (w *serializedDVWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return search.NewExplanation(false, 0, w.query.String()+", no match"), nil
	}
	target, err := scorer.Advance(doc)
	if err != nil {
		return nil, err
	}
	if target == doc {
		return search.NewExplanation(true, w.boost, w.query.String()+", match"), nil
	}
	return search.NewExplanation(false, 0, w.query.String()+", no match"), nil
}

// BulkScorer adapts the scorer to a default bulk scorer.
func (w *serializedDVWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

// IsCacheable reports that the doc-values predicate is deterministic
// over a stable leaf, so its result is safe to cache.
func (w *serializedDVWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count cannot answer sub-linearly without iterating the scorer.
func (w *serializedDVWeight) Count(ctx *index.LeafReaderContext) (int, error) {
	return -1, nil
}

// Matches is not yet wired for the doc-values strategy.
func (w *serializedDVWeight) Matches(ctx *index.LeafReaderContext, doc int) (search.Matches, error) {
	return nil, nil
}

var _ search.Weight = (*serializedDVWeight)(nil)

// serializedDVScorer iterates documents in a leaf and reports those
// whose binary doc-values payload satisfies the query's spatial
// predicate.
type serializedDVScorer struct {
	*search.BaseScorer
	weight *serializedDVWeight
	bdv    index.BinaryDocValues
	maxDoc int
	doc    int
	score  float32
}

func (s *serializedDVScorer) DocID() int { return s.doc }

// NextDoc advances to the next matching document by walking the
// per-leaf BinaryDocValues iterator and applying the predicate.
func (s *serializedDVScorer) NextDoc() (int, error) {
	for {
		next, err := s.bdv.NextDoc()
		if err != nil {
			return 0, err
		}
		if next < 0 || next >= s.maxDoc {
			s.doc = search.NO_MORE_DOCS
			return s.doc, nil
		}
		matched, err := s.predicate(next)
		if err != nil {
			return 0, err
		}
		if matched {
			s.doc = next
			return s.doc, nil
		}
	}
}

// Advance positions the scorer at or after target.
func (s *serializedDVScorer) Advance(target int) (int, error) {
	if target >= s.maxDoc {
		s.doc = search.NO_MORE_DOCS
		return s.doc, nil
	}
	next, err := s.bdv.Advance(target)
	if err != nil {
		return 0, err
	}
	if next < 0 || next >= s.maxDoc {
		s.doc = search.NO_MORE_DOCS
		return s.doc, nil
	}
	matched, err := s.predicate(next)
	if err != nil {
		return 0, err
	}
	if matched {
		s.doc = next
		return s.doc, nil
	}
	return s.NextDoc()
}

func (s *serializedDVScorer) Score() float32   { return s.score }
func (s *serializedDVScorer) Cost() int64      { return int64(s.maxDoc) }
func (s *serializedDVScorer) DocIDRunEnd() int { return s.maxDoc }

// predicate decodes the BinaryDocValues payload for doc and applies
// the configured spatial operation against the query shape. Errors
// from decoding bubble up so callers see corrupt-payload failures
// instead of silent misses.
func (s *serializedDVScorer) predicate(doc int) (bool, error) {
	data, err := s.bdv.Get(doc)
	if err != nil {
		return false, err
	}
	if len(data) == 0 {
		return false, nil
	}
	return s.weight.query.strategy.matchShape(s.weight.query.operation, s.weight.query.queryShape, data)
}
