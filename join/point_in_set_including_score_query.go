// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// PointInSetIncludingScoreQueryStream is the source of (packed-point, score)
// pairs consumed by PointInSetIncludingScoreQuery's constructor. Mirrors the
// abstract inner class PointInSetIncludingScoreQuery.Stream.
//
// Callers must implement Next() which returns the next packed-point value or
// nil when exhausted; the Score field holds the score for that point.
type PointInSetIncludingScoreQueryStream interface {
	// Next returns the next packed point bytes, or nil when exhausted.
	Next() []byte
	// Score returns the score for the point most recently returned by Next.
	Score() float32
}

// PointInSetIncludingScoreQuery matches "to" documents whose point-values
// field contains one of the collected packed points, propagating per-point
// scores from a GlobalOrdinalsWithScoreCollector.
//
// Mirrors org.apache.lucene.search.join.PointInSetIncludingScoreQuery.
type PointInSetIncludingScoreQuery struct {
	scoreMode                 ScoreMode
	originalQuery             search.Query
	multipleValuesPerDocument bool
	field                     string
	bytesPerDim               int
	sortedPackedPoints        *index.PrefixCodedTerms
	aggregatedJoinScores      []float32
	// valueFormatter converts raw packed bytes to human-readable string.
	valueFormatter func([]byte) string
}

// NewPointInSetIncludingScoreQuery creates a PointInSetIncludingScoreQuery.
//
//   - scoreMode: how per-ordinal scores are aggregated
//   - originalQuery: the originating query (used in equality/hash)
//   - multipleValuesPerDocument: true when a doc may carry multiple points
//   - field: the point-values field in "to" documents
//   - bytesPerDim: bytes per dimension (must be 1..16)
//   - packedPoints: sorted stream of (point, score) pairs
//   - valueFormatter: optional; formats raw bytes for String(); pass nil for
//     default hex formatting
func NewPointInSetIncludingScoreQuery(
	scoreMode ScoreMode,
	originalQuery search.Query,
	multipleValuesPerDocument bool,
	field string,
	bytesPerDim int,
	packedPoints PointInSetIncludingScoreQueryStream,
	valueFormatter func([]byte) string,
) (*PointInSetIncludingScoreQuery, error) {
	if bytesPerDim < 1 || bytesPerDim > 16 {
		return nil, fmt.Errorf("bytesPerDim must be in [1, 16]; got %d", bytesPerDim)
	}
	q := &PointInSetIncludingScoreQuery{
		scoreMode:                 scoreMode,
		originalQuery:             originalQuery,
		multipleValuesPerDocument: multipleValuesPerDocument,
		field:                     field,
		bytesPerDim:               bytesPerDim,
	}
	if valueFormatter != nil {
		q.valueFormatter = valueFormatter
	} else {
		q.valueFormatter = defaultPointFormatter
	}

	builder := index.NewPrefixCodedTermsBuilder()
	var previous []byte
	for {
		current := packedPoints.Next()
		if current == nil {
			break
		}
		if len(current) != bytesPerDim {
			return nil, fmt.Errorf(
				"packed point length should be %d but got %d; field=%q bytesPerDim=%d",
				bytesPerDim, len(current), field, bytesPerDim,
			)
		}
		if previous != nil {
			cmp := bytes.Compare(previous, current)
			if cmp == 0 {
				return nil, fmt.Errorf("unexpected duplicated value: %x", current)
			}
			if cmp > 0 {
				return nil, fmt.Errorf(
					"values are out of order: saw %x before %x", previous, current)
			}
		}
		builder.AddFieldBytes(field, current)
		q.aggregatedJoinScores = append(q.aggregatedJoinScores, packedPoints.Score())
		cp := make([]byte, len(current))
		copy(cp, current)
		previous = cp
	}
	q.sortedPackedPoints = builder.Finish()
	return q, nil
}

// GetField returns the point-values field name.
func (q *PointInSetIncludingScoreQuery) GetField() string { return q.field }

// GetScoreMode returns the join ScoreMode.
func (q *PointInSetIncludingScoreQuery) GetScoreMode() ScoreMode { return q.scoreMode }

// String implements search.Query.
func (q *PointInSetIncludingScoreQuery) String() string {
	sb := &bytes.Buffer{}
	sb.WriteString(q.field)
	sb.WriteString(":{")
	it := q.sortedPackedPoints.Iterator()
	first := true
	for pt := it.Next(); pt != nil; pt = it.Next() {
		if !first {
			sb.WriteByte(' ')
		}
		first = false
		sb.WriteString(q.valueFormatter(pt))
	}
	sb.WriteByte('}')
	return sb.String()
}

// Rewrite implements search.Query.
func (q *PointInSetIncludingScoreQuery) Rewrite(_ search.IndexReader) (search.Query, error) {
	return q, nil
}

// Clone implements search.Query.
func (q *PointInSetIncludingScoreQuery) Clone() search.Query {
	cp := *q
	return &cp
}

// Equals implements search.Query.
func (q *PointInSetIncludingScoreQuery) Equals(other search.Query) bool {
	o, ok := other.(*PointInSetIncludingScoreQuery)
	if !ok {
		return false
	}
	if q.scoreMode != o.scoreMode || q.field != o.field || q.bytesPerDim != o.bytesPerDim {
		return false
	}
	if q.originalQuery != nil && !q.originalQuery.Equals(o.originalQuery) {
		return false
	}
	if o.originalQuery != nil && !o.originalQuery.Equals(q.originalQuery) {
		return false
	}
	// Compare sorted packed points by iterating both.
	it1 := q.sortedPackedPoints.Iterator()
	it2 := o.sortedPackedPoints.Iterator()
	for {
		p1 := it1.Next()
		p2 := it2.Next()
		if p1 == nil && p2 == nil {
			break
		}
		if p1 == nil || p2 == nil {
			return false
		}
		if !bytes.Equal(p1, p2) {
			return false
		}
	}
	return true
}

// HashCode implements search.Query.
func (q *PointInSetIncludingScoreQuery) HashCode() int {
	h := 31
	for _, c := range q.field {
		h = 31*h + int(c)
	}
	h = 31*h + int(q.scoreMode) + q.bytesPerDim
	return h
}

// CreateWeight implements search.Query.
func (q *PointInSetIncludingScoreQuery) CreateWeight(_ *search.IndexSearcher, _ bool, boost float32) (search.Weight, error) {
	return &pointInSetIncludingScoreWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		boost:      boost,
	}, nil
}

// pointInSetIncludingScoreWeight is the Weight for PointInSetIncludingScoreQuery.
type pointInSetIncludingScoreWeight struct {
	*search.BaseWeight
	query *PointInSetIncludingScoreQuery
	boost float32
}

func (w *pointInSetIncludingScoreWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	if ctx == nil {
		return newPointInSetStubScorer(w.boost), nil
	}
	// Try to obtain a point-values reader that supports Intersect.
	// If the concrete reader does not support it we return nil (no match).
	type leafWithPointValues interface {
		GetPointValues(field string) (index.PointValues, error)
		GetFieldInfos() *index.FieldInfos
		MaxDoc() int
	}
	lr, ok := ctx.Reader().(leafWithPointValues)
	if !ok {
		return nil, nil
	}

	fi := lr.GetFieldInfos().GetByName(w.query.field)
	if fi == nil {
		return nil, nil
	}
	if fi.PointDimensionCount() != 1 {
		return nil, fmt.Errorf(
			"field %q was indexed with numDims=%d but this query has numDims=1",
			w.query.field, fi.PointDimensionCount(),
		)
	}
	if fi.PointNumBytes() != w.query.bytesPerDim {
		return nil, fmt.Errorf(
			"field %q was indexed with bytesPerDim=%d but this query has bytesPerDim=%d",
			w.query.field, fi.PointNumBytes(), w.query.bytesPerDim,
		)
	}

	raw, err := lr.GetPointValues(w.query.field)
	if err != nil || raw == nil {
		return nil, err
	}

	maxDoc := lr.MaxDoc()
	result, err2 := util.NewFixedBitSet(maxDoc)
	if err2 != nil {
		return nil, err2
	}
	scores := make([]float32, maxDoc)

	v := &mergePointVisitor{
		q:      w.query,
		result: result,
		scores: scores,
	}
	v.reset()

	// intersectPointValues delegates to codecs.PointValues.Intersect via
	// a type assertion; no-op if the reader does not expose that surface.
	if err := intersectPointValues(raw, v); err != nil {
		return nil, err
	}

	disi := util.NewBitSetIterator(result, int64(result.Cardinality()))
	return &pointInSetIncludingScoreScorer{
		disi:   disi,
		scores: scores,
	}, nil
}

// intersectPointValues attempts to run the merge visitor against the raw
// index.PointValues by checking whether it implements a richer interface
// with an Intersect method. If not, it falls through silently (no-op).
func intersectPointValues(raw index.PointValues, v *mergePointVisitor) error {
	// index.PointValues in Gocene (doc_values_interfaces.go) is read-only
	// metadata; the codecs layer provides the richer surface with Intersect.
	// Type-assert to a locally declared interface that matches
	// codecs.PointValues.Intersect's signature, so we do not import codecs
	// from join (would create a cycle).
	type intersectable interface {
		Intersect(visitor intersectVisitorShim) error
	}
	if iv, ok := raw.(intersectable); ok {
		return iv.Intersect(v)
	}
	// No Intersect available — no matches (graceful degradation).
	return nil
}

// intersectVisitorShim mirrors the contract of codecs.IntersectVisitor so
// that join can call Intersect without importing codecs.
type intersectVisitorShim interface {
	Visit(docID int) error
	VisitByPackedValue(docID int, packedValue []byte) error
	Compare(minPackedValue, maxPackedValue []byte) int
	Grow(count int)
}

// ── mergePointVisitor ────────────────────────────────────────────────────────

// mergePointVisitor implements intersectVisitorShim and drives the merge
// between the sorted query-points list and the BKD leaf. Mirrors
// PointInSetIncludingScoreQuery.MergePointVisitor.
type mergePointVisitor struct {
	q              *PointInSetIncludingScoreQuery
	result         util.BitSet
	scores         []float32
	iterator       *index.PrefixCodedTermIterator
	scoreIdx       int
	nextQueryPoint []byte
	nextScore      float32
	scratch        []byte
}

func (v *mergePointVisitor) reset() {
	v.iterator = v.q.sortedPackedPoints.Iterator()
	v.scoreIdx = 0
	v.nextQueryPoint = v.iterator.Next()
	if v.scoreIdx < len(v.q.aggregatedJoinScores) {
		v.nextScore = v.q.aggregatedJoinScores[v.scoreIdx]
	}
}

// Visit is called for CELL_INSIDE_QUERY (not emitted in single-dimension
// intersection — required by the intersectVisitorShim interface).
func (v *mergePointVisitor) Visit(_ int) error { return nil }

// VisitByPackedValue is called for each document in a leaf cell.
func (v *mergePointVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	v.scratch = packedValue
	for v.nextQueryPoint != nil {
		cmp := bytes.Compare(v.nextQueryPoint, v.scratch)
		if cmp == 0 {
			// Exact match.
			if v.q.multipleValuesPerDocument {
				if !v.result.GetAndSet(docID) {
					v.scores[docID] = v.nextScore
				}
			} else {
				v.result.Set(docID)
				v.scores[docID] = v.nextScore
			}
			return nil
		} else if cmp < 0 {
			// Query point is before index point — advance query cursor.
			v.nextQueryPoint = v.iterator.Next()
			v.scoreIdx++
			if v.scoreIdx < len(v.q.aggregatedJoinScores) {
				v.nextScore = v.q.aggregatedJoinScores[v.scoreIdx]
			}
		} else {
			// Query point is after index point — no match.
			return nil
		}
	}
	return nil
}

// Compare classifies a BKD cell against the current query-point cursor.
// Returns: 0=OUTSIDE, 1=INSIDE, 2=CROSSES.
func (v *mergePointVisitor) Compare(minPackedValue, maxPackedValue []byte) int {
	for v.nextQueryPoint != nil {
		cmpMin := bytes.Compare(v.nextQueryPoint, minPackedValue)
		if cmpMin < 0 {
			// Query point before cell start — advance.
			v.nextQueryPoint = v.iterator.Next()
			v.scoreIdx++
			if v.scoreIdx < len(v.q.aggregatedJoinScores) {
				v.nextScore = v.q.aggregatedJoinScores[v.scoreIdx]
			}
			continue
		}
		cmpMax := bytes.Compare(v.nextQueryPoint, maxPackedValue)
		if cmpMax > 0 {
			// Query point after cell end — outside.
			return 0 // CELL_OUTSIDE_QUERY
		}
		return 2 // CELL_CROSSES_QUERY
	}
	return 0 // CELL_OUTSIDE_QUERY — no more query points
}

// Grow is a hint; no-op here.
func (v *mergePointVisitor) Grow(_ int) {}

// ── Scorer ───────────────────────────────────────────────────────────────────

// pointInSetIncludingScoreScorer returns matched docs in docID order with
// their collected scores.
type pointInSetIncludingScoreScorer struct {
	disi   search.DocIdSetIterator
	scores []float32
}

func (s *pointInSetIncludingScoreScorer) Score() float32 {
	doc := s.disi.DocID()
	if doc >= 0 && doc < len(s.scores) {
		return s.scores[doc]
	}
	return 0
}
func (s *pointInSetIncludingScoreScorer) GetMaxScore(_ int) float32  { return float32(math.Inf(1)) }
func (s *pointInSetIncludingScoreScorer) DocID() int                 { return s.disi.DocID() }
func (s *pointInSetIncludingScoreScorer) NextDoc() (int, error)      { return s.disi.NextDoc() }
func (s *pointInSetIncludingScoreScorer) Advance(t int) (int, error) { return s.disi.Advance(t) }
func (s *pointInSetIncludingScoreScorer) Cost() int64                { return s.disi.Cost() }
func (s *pointInSetIncludingScoreScorer) DocIDRunEnd() int           { return s.disi.DocIDRunEnd() }

// ── weight methods ──────────────────────────────────────────────────────────

// ScorerSupplier returns a ScorerSupplier that lazily creates the Scorer.
// Mirrors the default Weight.scorerSupplier in Java which wraps scorer().
func (w *pointInSetIncludingScoreWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return &pointInSetScorerSupplier{scorer: scorer}, nil
}

// BulkScorer delegates to the default BulkScorer implementation that uses
// the Scorer for iteration. Returns nil if no matches are possible.
func (w *pointInSetIncludingScoreWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return nil, err
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

func (w *pointInSetIncludingScoreWeight) IsCacheable(_ *index.LeafReaderContext) bool {
	return true
}

func (w *pointInSetIncludingScoreWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil || scorer == nil {
		return search.NewExplanation(false, 0, "No match"), nil
	}
	target, err2 := scorer.Advance(doc)
	if err2 != nil || target != doc {
		return search.NewExplanation(false, 0, "Not a match"), nil
	}
	return search.NewExplanation(true, scorer.Score(), "A match"), nil
}

func (w *pointInSetIncludingScoreWeight) Count(_ *index.LeafReaderContext) (int, error) {
	return -1, nil
}

func (w *pointInSetIncludingScoreWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

// pointInSetScorerSupplier adapts a Scorer into a ScorerSupplier.
// Mirrors the default Weight.scorerSupplier(LeafReaderContext) wrapper in Java.
type pointInSetScorerSupplier struct {
	scorer search.Scorer
}

func (s *pointInSetScorerSupplier) Get(_ int64) (search.Scorer, error) { return s.scorer, nil }
func (s *pointInSetScorerSupplier) Cost() int64                        { return s.scorer.Cost() }
func (s *pointInSetScorerSupplier) SetTopLevelScoringClause()          {}

// ── stub scorer ──────────────────────────────────────────────────────────────

type pointInSetStubScorer struct {
	*search.BaseDocIdSetIterator
	boost float32
}

func newPointInSetStubScorer(boost float32) *pointInSetStubScorer {
	return &pointInSetStubScorer{
		BaseDocIdSetIterator: &search.BaseDocIdSetIterator{},
		boost:                boost,
	}
}

func (s *pointInSetStubScorer) Score() float32            { return 0 }
func (s *pointInSetStubScorer) GetMaxScore(_ int) float32 { return float32(math.Inf(1)) }
func (s *pointInSetStubScorer) DocID() int                { return search.NO_MORE_DOCS }
func (s *pointInSetStubScorer) NextDoc() (int, error)     { return search.NO_MORE_DOCS, nil }
func (s *pointInSetStubScorer) Advance(_ int) (int, error) {
	return search.NO_MORE_DOCS, nil
}
func (s *pointInSetStubScorer) Cost() int64      { return 0 }
func (s *pointInSetStubScorer) DocIDRunEnd() int { return search.NO_MORE_DOCS }

// ── helpers ──────────────────────────────────────────────────────────────────

// defaultPointFormatter formats packed bytes as big-endian int64 decimal when
// bytesPerDim==8, otherwise as hex.
func defaultPointFormatter(value []byte) string {
	if len(value) == 8 {
		v := int64(binary.BigEndian.Uint64(value))
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("%x", value)
}

// interface compliance
var _ search.Query = (*PointInSetIncludingScoreQuery)(nil)
var _ search.Scorer = (*pointInSetIncludingScoreScorer)(nil)
var _ search.Scorer = (*pointInSetStubScorer)(nil)
