// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.5.0:
//   lucene/queries/src/java/org/apache/lucene/queries/spans/SpanOrQuery.java

package spans

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// classHashSpanOrQuery seeds SpanOrQuery.hashCode() in place of Java's
// classHash().
const classHashSpanOrQuery = 0x5370_4f72 // "SpOr"

// SpanOrQuery matches the union of its clauses.
//
// Mirrors org.apache.lucene.queries.spans.SpanOrQuery (final).
type SpanOrQuery struct {
	search.BaseQuery
	clauses []SpanQuery
	field   string
}

// NewSpanOrQuery constructs a SpanOrQuery merging the provided clauses.
// All clauses must have the same field.
//
// Mirrors SpanOrQuery(SpanQuery...).
func NewSpanOrQuery(clauses ...SpanQuery) (*SpanOrQuery, error) {
	q := &SpanOrQuery{clauses: make([]SpanQuery, 0, len(clauses))}
	for _, seq := range clauses {
		if err := q.addClause(seq); err != nil {
			return nil, err
		}
	}
	return q, nil
}

// addClause adds a clause to this query.
//
// Mirrors the private SpanOrQuery.addClause(SpanQuery).
func (q *SpanOrQuery) addClause(clause SpanQuery) error {
	if q.field == "" {
		q.field = clause.GetField()
	} else if clause.GetField() != "" && clause.GetField() != q.field {
		return fmt.Errorf("Clauses must have same field.")
	}
	q.clauses = append(q.clauses, clause)
	return nil
}

// GetClauses returns the clauses whose spans are matched.
//
// Mirrors SpanOrQuery.getClauses().
func (q *SpanOrQuery) GetClauses() []SpanQuery {
	out := make([]SpanQuery, len(q.clauses))
	copy(out, q.clauses)
	return out
}

// GetField returns the field shared by every clause.
func (q *SpanOrQuery) GetField() string { return q.field }

// Rewrite mirrors SpanOrQuery.rewrite(IndexSearcher).
func (q *SpanOrQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewritten := &SpanOrQuery{clauses: make([]SpanQuery, 0, len(q.clauses))}
	actuallyRewritten := false
	for _, c := range q.clauses {
		rewrittenQuery, err := c.Rewrite(searcher)
		if err != nil {
			return nil, err
		}
		query, ok := rewrittenQuery.(SpanQuery)
		if !ok {
			return nil, fmt.Errorf("SpanOrQuery.Rewrite: clause rewrote to non-SpanQuery %T", rewrittenQuery)
		}
		actuallyRewritten = actuallyRewritten || query != c
		if err := rewritten.addClause(query); err != nil {
			return nil, err
		}
	}
	if actuallyRewritten {
		return rewritten, nil
	}
	return q, nil
}

// Visit mirrors SpanOrQuery.visit(QueryVisitor).
func (q *SpanOrQuery) Visit(visitor search.QueryVisitor) {
	if !visitor.AcceptField(q.GetField()) {
		return
	}
	v := visitor.GetSubVisitor(search.SHOULD, q)
	for _, c := range q.clauses {
		visitSpanQuery(c, v)
	}
}

// ToString mirrors SpanOrQuery.toString(String field).
func (q *SpanOrQuery) ToString(field string) string {
	parts := make([]string, len(q.clauses))
	for i, clause := range q.clauses {
		parts[i] = spanQueryToString(clause, field)
	}
	return "spanOr([" + strings.Join(parts, ", ") + "])"
}

// Equals mirrors SpanOrQuery.equals(Object).
func (q *SpanOrQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SpanOrQuery)
	if !ok {
		return false
	}
	if len(q.clauses) != len(o.clauses) {
		return false
	}
	for i := range q.clauses {
		if !q.clauses[i].Equals(o.clauses[i]) {
			return false
		}
	}
	return true
}

// HashCode mirrors SpanOrQuery.hashCode(): classHash() ^ clauses.hashCode(),
// where clauses.hashCode() is java.util.List's 31-fold.
func (q *SpanOrQuery) HashCode() int {
	return classHashSpanOrQuery ^ javaListHashCode(q.clauses)
}

// CreateWeight mirrors SpanOrQuery.createWeight(IndexSearcher, ScoreMode,
// float), whose covariant return is a SpanWeight.
func (q *SpanOrQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	return q.CreateSpanWeight(searcher, scoreMode, boost)
}

// CreateSpanWeight carries the covariant SpanWeight return of
// SpanOrQuery.createWeight.
//
// Java passes `scoreMode.needsScores() ? getTermStates(subWeights) : null` as
// the weight's termStates argument, which feeds SpanWeight.buildSimWeight and
// nothing else. This port carries a nil SimScorer here, exactly as
// SpanTermQuery and SpanNearQuery do, because buildSimWeight needs the Term
// behind each TermStates and the package's term-states map is keyed by
// "field:text" (see span_weight.go:57-59).
func (q *SpanOrQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*SpanWeight, error) {
	subWeights := make([]*SpanWeight, 0, len(q.clauses))
	for _, c := range q.clauses {
		sw, err := c.CreateSpanWeight(searcher, scoreMode, boost)
		if err != nil {
			return nil, err
		}
		subWeights = append(subWeights, sw)
	}
	return newSpanOrWeight(q, subWeights), nil
}

// newSpanOrWeight mirrors the inner class SpanOrQuery.SpanOrWeight.
func newSpanOrWeight(query *SpanOrQuery, subWeights []*SpanWeight) *SpanWeight {
	return NewSpanWeight(query, SpanWeightConfig{
		Field:     query.GetField(),
		SimScorer: nil,
		GetSpans: func(ctx *index.LeafReaderContext, requiredPostings Postings) (Spans, error) {
			return spanOrGetSpans(query, subWeights, ctx, requiredPostings)
		},
		ExtractStates: func(contexts map[string]*index.TermStates) {
			for _, w := range subWeights {
				w.ExtractTermStates(contexts)
			}
		},
		IsCacheable: func(ctx *index.LeafReaderContext) bool {
			for _, w := range subWeights {
				if !w.IsCacheable(ctx) {
					return false
				}
			}
			return true
		},
	})
}

// spanOrGetSpans mirrors SpanOrWeight.getSpans(LeafReaderContext, Postings).
func spanOrGetSpans(query *SpanOrQuery, subWeights []*SpanWeight, ctx *index.LeafReaderContext, requiredPostings Postings) (Spans, error) {
	subSpans := make([]Spans, 0, len(query.clauses))
	for _, w := range subWeights {
		spans, err := w.GetSpans(ctx, requiredPostings)
		if err != nil {
			return nil, err
		}
		if spans != nil {
			subSpans = append(subSpans, spans)
		}
	}

	if len(subSpans) == 0 {
		return nil, nil
	} else if len(subSpans) == 1 {
		return subSpans[0], nil
	}

	byDocQueue := NewSpanDisiPriorityQueue(len(subSpans))
	for _, spans := range subSpans {
		byDocQueue.Add(NewSpanDisiWrapper(spans))
	}

	byPositionQueue := NewSpanPositionQueue(len(subSpans)) // when empty use -1

	return &spanOrSpans{
		query:                  query,
		subSpans:               subSpans,
		byDocQueue:             byDocQueue,
		byPositionQueue:        byPositionQueue,
		positionsCost:          -1,
		lastDocTwoPhaseMatched: -1,
		cost:                   -1,
	}, nil
}

// spanOrSpans is the anonymous Spans subclass returned by
// SpanOrQuery.SpanOrWeight.getSpans: a disjunction over the sub-spans.
type spanOrSpans struct {
	BaseSpans

	query           *SpanOrQuery
	subSpans        []Spans
	byDocQueue      *SpanDisiPriorityQueue
	byPositionQueue *SpanPositionQueue

	topPositionSpans Spans

	positionsCost          float32
	lastDocTwoPhaseMatched int
	cost                   int64
}

// NextDoc mirrors the anonymous subclass's nextDoc().
func (s *spanOrSpans) NextDoc() (int, error) {
	s.topPositionSpans = nil
	topDocSpans := s.byDocQueue.Top()
	currentDoc := topDocSpans.Doc
	for {
		doc, err := topDocSpans.Iterator.NextDoc()
		if err != nil {
			return 0, err
		}
		topDocSpans.Doc = doc
		topDocSpans = s.byDocQueue.UpdateTop()
		if topDocSpans.Doc != currentDoc {
			break
		}
	}
	return topDocSpans.Doc, nil
}

// Advance mirrors the anonymous subclass's advance(int).
func (s *spanOrSpans) Advance(target int) (int, error) {
	s.topPositionSpans = nil
	topDocSpans := s.byDocQueue.Top()
	for {
		doc, err := topDocSpans.Iterator.Advance(target)
		if err != nil {
			return 0, err
		}
		topDocSpans.Doc = doc
		topDocSpans = s.byDocQueue.UpdateTop()
		if topDocSpans.Doc >= target {
			break
		}
	}
	return topDocSpans.Doc, nil
}

// DocID mirrors the anonymous subclass's docID().
func (s *spanOrSpans) DocID() int {
	return s.byDocQueue.Top().Doc
}

// AsTwoPhaseIterator mirrors the anonymous subclass's asTwoPhaseIterator().
func (s *spanOrSpans) AsTwoPhaseIterator() *search.TwoPhaseIterator {
	var sumMatchCost float32 // See also DisjunctionScorer.asTwoPhaseIterator()
	var sumApproxCost int64

	for _, w := range s.byDocQueue.All() {
		if w.TwoPhaseView != nil {
			costWeight := w.Cost
			if costWeight <= 1 {
				costWeight = 1
			}
			sumMatchCost += w.TwoPhaseView.MatchCost() * float32(costWeight)
			sumApproxCost += costWeight
		}
	}

	if sumApproxCost == 0 { // no sub spans supports approximations
		s.computePositionsCost()
		return nil
	}

	matchCost := sumMatchCost / float32(sumApproxCost)

	return search.NewTwoPhaseIteratorWithMatchCost(
		NewSpanDisjunctionDISIApproximation(s.byDocQueue),
		func() (bool, error) { return s.twoPhaseCurrentDocMatches() },
		matchCost,
	)
}

// computePositionsCost mirrors the anonymous subclass's computePositionsCost().
func (s *spanOrSpans) computePositionsCost() {
	var sumPositionsCost float32
	var sumCost int64
	for _, w := range s.byDocQueue.All() {
		costWeight := w.Cost
		if costWeight <= 1 {
			costWeight = 1
		}
		sumPositionsCost += w.Spans.PositionsCost() * float32(costWeight)
		sumCost += costWeight
	}
	s.positionsCost = sumPositionsCost / float32(sumCost)
}

// PositionsCost mirrors the anonymous subclass's positionsCost(). It may be
// called only when AsTwoPhaseIterator returned nil, which happens when none of
// the sub spans supports approximations.
func (s *spanOrSpans) PositionsCost() float32 {
	return s.positionsCost
}

// twoPhaseCurrentDocMatches mirrors the anonymous subclass's
// twoPhaseCurrentDocMatches().
func (s *spanOrSpans) twoPhaseCurrentDocMatches() (bool, error) {
	listAtCurrentDoc := s.byDocQueue.TopList()
	// remove the head of the list as long as it does not match
	currentDoc := listAtCurrentDoc.Doc
	for listAtCurrentDoc.TwoPhaseView != nil {
		matches, err := listAtCurrentDoc.TwoPhaseView.Matches()
		if err != nil {
			return false, err
		}
		if matches {
			// use this spans for positions at current doc:
			listAtCurrentDoc.LastApproxMatchDoc = currentDoc
			break
		}
		// do not use this spans for positions at current doc:
		listAtCurrentDoc.LastApproxNonMatchDoc = currentDoc
		listAtCurrentDoc = listAtCurrentDoc.Next
		if listAtCurrentDoc == nil {
			return false, nil
		}
	}
	s.lastDocTwoPhaseMatched = currentDoc
	s.topPositionSpans = nil
	return true, nil
}

// fillPositionQueue mirrors the anonymous subclass's fillPositionQueue(), which
// is called at the first nextStartPosition of a document.
func (s *spanOrSpans) fillPositionQueue() error {
	// add all matching Spans at current doc to byPositionQueue
	listAtCurrentDoc := s.byDocQueue.TopList()
	for listAtCurrentDoc != nil {
		spansAtDoc := listAtCurrentDoc.Spans
		if s.lastDocTwoPhaseMatched == listAtCurrentDoc.Doc { // matched by DisjunctionDisiApproximation
			if listAtCurrentDoc.TwoPhaseView != nil { // matched by approximation
				if listAtCurrentDoc.LastApproxNonMatchDoc == listAtCurrentDoc.Doc { // matches() returned false
					spansAtDoc = nil
				} else {
					if listAtCurrentDoc.LastApproxMatchDoc != listAtCurrentDoc.Doc {
						matches, err := listAtCurrentDoc.TwoPhaseView.Matches()
						if err != nil {
							return err
						}
						if !matches {
							spansAtDoc = nil
						}
					}
				}
			}
		}

		if spansAtDoc != nil {
			if _, err := spansAtDoc.NextStartPosition(); err != nil {
				return err
			}
			s.byPositionQueue.Add(spansAtDoc)
		}
		listAtCurrentDoc = listAtCurrentDoc.Next
	}
	return nil
}

// NextStartPosition mirrors the anonymous subclass's nextStartPosition().
func (s *spanOrSpans) NextStartPosition() (int, error) {
	if s.topPositionSpans == nil {
		s.byPositionQueue.Clear()
		if err := s.fillPositionQueue(); err != nil { // fills byPositionQueue at first position
			return 0, err
		}
		s.topPositionSpans = s.byPositionQueue.Top()
	} else {
		if _, err := s.topPositionSpans.NextStartPosition(); err != nil {
			return 0, err
		}
		s.topPositionSpans = s.byPositionQueue.UpdateTop()
	}
	return s.topPositionSpans.StartPosition(), nil
}

// StartPosition mirrors the anonymous subclass's startPosition().
func (s *spanOrSpans) StartPosition() int {
	if s.topPositionSpans == nil {
		return -1
	}
	return s.topPositionSpans.StartPosition()
}

// EndPosition mirrors the anonymous subclass's endPosition().
func (s *spanOrSpans) EndPosition() int {
	if s.topPositionSpans == nil {
		return -1
	}
	return s.topPositionSpans.EndPosition()
}

// Width mirrors the anonymous subclass's width().
func (s *spanOrSpans) Width() int {
	return s.topPositionSpans.Width()
}

// Collect mirrors the anonymous subclass's collect(SpanCollector).
func (s *spanOrSpans) Collect(collector SpanCollector) error {
	if s.topPositionSpans != nil {
		return s.topPositionSpans.Collect(collector)
	}
	return nil
}

// String mirrors the anonymous subclass's toString().
func (s *spanOrSpans) String() string {
	return fmt.Sprintf("spanOr(%s)@%d: %d - %d",
		spanQueryToString(s.query, ""), s.DocID(), s.StartPosition(), s.EndPosition())
}

// Cost mirrors the anonymous subclass's cost().
func (s *spanOrSpans) Cost() int64 {
	if s.cost == -1 {
		s.cost = 0
		for _, spans := range s.subSpans {
			s.cost += spans.Cost()
		}
	}
	return s.cost
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd() in
// Apache Lucene 10.5.0, which the anonymous Spans inherits without overriding.
func (s *spanOrSpans) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(s)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0,
// which the anonymous Spans inherits without overriding.
func (s *spanOrSpans) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

var (
	_ SpanQuery = (*SpanOrQuery)(nil)
	_ Spans     = (*spanOrSpans)(nil)
)

// String renders Query.toString(), whose Java body is toString("").
func (q *SpanOrQuery) String() string { return q.ToString("") }
