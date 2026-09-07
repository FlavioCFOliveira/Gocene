// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// SpanOrQuery matches the union of its clauses.
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanOrQuery.
type SpanOrQuery struct {
	*BaseSpanQuery
	clauses []SpanQuery
}

// NewSpanOrQuery constructs a SpanOrQuery merging the provided clauses.
// All clauses must have the same field.
func NewSpanOrQuery(clauses ...SpanQuery) (*SpanOrQuery, error) {
	q := &SpanOrQuery{
		BaseSpanQuery: &BaseSpanQuery{},
		clauses:       make([]SpanQuery, 0, len(clauses)),
	}
	for _, c := range clauses {
		if err := q.addClause(c); err != nil {
			return nil, err
		}
	}
	return q, nil
}

func (q *SpanOrQuery) addClause(clause SpanQuery) error {
	if q.field == "" {
		q.field = clause.GetField()
	} else if clause.GetField() != "" && clause.GetField() != q.field {
		return fmt.Errorf("clauses must have same field")
	}
	q.clauses = append(q.clauses, clause)
	return nil
}

// GetClauses returns the clauses whose spans are matched.
func (q *SpanOrQuery) GetClauses() []SpanQuery {
	return q.clauses
}

// Rewrite rewrites the query to a simpler form.
func (q *SpanOrQuery) Rewrite(reader IndexReader) (Query, error) {
	rewritten, _ := NewSpanOrQuery()
	actuallyRewritten := false
	for _, c := range q.clauses {
		query, err := c.Rewrite(reader)
		if err != nil {
			return nil, err
		}
		spanQuery, ok := query.(SpanQuery)
		if !ok {
			return nil, fmt.Errorf("rewritten query must be a SpanQuery")
		}
		if spanQuery != c {
			actuallyRewritten = true
		}
		rewritten.addClause(spanQuery)
	}
	if actuallyRewritten {
		return rewritten, nil
	}
	return q.BaseSpanQuery.Rewrite(reader)
}

// Visit visits the clauses.
func (q *SpanOrQuery) Visit(visitor QueryVisitor) {
	if !visitor.AcceptField(q.GetField()) {
		return
	}
	v := visitor.GetSubVisitor(BooleanClauseOccurShould, q)
	for _, cq := range q.clauses {
		cq.Visit(v)
	}
}

// String returns a string representation of the query.
func (q *SpanOrQuery) String(field string) string {
	var sb strings.Builder
	sb.WriteString("spanOr([")
	for i, clause := range q.clauses {
		sb.WriteString(clause.String(field))
		if i < len(q.clauses)-1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("])")
	return sb.String()
}

// Equals checks if this query equals another.
func (q *SpanOrQuery) Equals(other Query) bool {
	if other == nil {
		return false
	}
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

// HashCode returns a hash code for this query.
func (q *SpanOrQuery) HashCode() int {
	h := 17
	for _, c := range q.clauses {
		h = 31*h + c.HashCode()
	}
	return h
}

// CreateWeight creates a SpanOrWeight for this query.
func (q *SpanOrQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (SpanWeight, error) {
	subWeights := make([]SpanWeight, 0, len(q.clauses))
	for _, cq := range q.clauses {
		sw, err := cq.CreateWeight(searcher, needsScores, boost)
		if err != nil {
			return nil, err
		}
		subWeights = append(subWeights, sw)
	}

	var terms map[*index.Term]*index.TermStates
	if needsScores {
		terms = GetTermStates(subWeights...)
	}

	return &SpanOrWeight{
		SpanWeight:  NewSpanWeight(q, nil), // Similarity is handled in the base
		subWeights:   subWeights,
		termStates:   terms,
		boost:        boost,
		searcher:     searcher,
	}, nil
}

// SpanOrWeight is the weight implementation for SpanOrQuery.
type SpanOrWeight struct {
	*SpanWeight
	subWeights []SpanWeight
	termStates map[*index.Term]*index.TermStates
	boost      float32
	searcher   *IndexSearcher
}

func (sw *SpanOrWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	for _, w := range sw.subWeights {
		if !w.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

func (sw *SpanOrWeight) ExtractTermStates(contexts map[*index.Term]*index.TermStates) {
	for _, w := range sw.subWeights {
		w.ExtractTermStates(contexts)
	}
}

func (sw *SpanOrWeight) GetSpans(context *index.LeafReaderContext, requiredPostings int) (Spans, error) {
	subSpans := make([]Spans, 0, len(sw.subWeights))

	for _, w := range sw.subWeights {
		spans, err := w.GetSpans(context, requiredPostings)
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

	byDocQueue := newSpanDisiPriorityQueue(len(subSpans))
	for _, spans := range subSpans {
		byDocQueue.add(newSpanDisiWrapper(spans))
	}

	byPositionQueue := newSpanPositionQueue(len(subSpans))

	return &spanOrSpans{
		byDocQueue:      byDocQueue,
		byPositionQueue: byPositionQueue,
		subSpans:        subSpans,
	}, nil
}

type spanOrSpans struct {
	byDocQueue      *spanDisiPriorityQueuePtr
	byPositionQueue *spanPositionQueue
	subSpans        []Spans
	topPositionSpans Spans
	lastDocTwoPhaseMatched int
	positionsCost    float32
}

func (s *spanOrSpans) NextDoc() (int, error) {
	s.topPositionSpans = nil
	topDocSpans := s.byDocQueue.top()
	currentDoc := topDocSpans.doc
	for {
		nextDoc, err := topDocSpans.iterator.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		topDocSpans.doc = nextDoc
		topDocSpans = s.byDocQueue.updateTop()
		if topDocSpans.doc != currentDoc {
			break
		}
	}
	return topDocSpans.doc, nil
}

func (s *spanOrSpans) Advance(target int) (int, error) {
	s.topPositionSpans = nil
	topDocSpans := s.byDocQueue.top()
	for {
		nextDoc, err := topDocSpans.iterator.Advance(target)
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		topDocSpans.doc = nextDoc
		topDocSpans = s.byDocQueue.updateTop()
		if topDocSpans.doc >= target {
			break
		}
	}
	return topDocSpans.doc, nil
}

func (s *spanOrSpans) DocID() int {
	return s.byDocQueue.top().doc
}

func (s *spanOrSpans) AsTwoPhaseIterator() *TwoPhaseIterator {
	var sumMatchCost float32
	var sumApproxCost int64

	for _, w := range s.byDocQueue.heap {
		if w == nil {
			continue
		}
		if w.twoPhaseView != nil {
			costWeight := int64(1)
			if w.cost > 1 {
				costWeight = w.cost
			}
			sumMatchCost += w.twoPhaseView.MatchCost() * float32(costWeight)
			sumApproxCost += costWeight
		}
	}

	if sumApproxCost == 0 {
		s.computePositionsCost()
		return nil
	}

	matchCost := sumMatchCost / float32(sumApproxCost)

	return &TwoPhaseIterator{
		approximation: &spanDisjunctionDISIApproximation{
			subIterators: s.byDocQueue,
			cost:         sumApproxCost,
		},
		matches: func() bool {
			return s.twoPhaseCurrentDocMatches()
		},
		matchCost: matchCost,
	}
}

func (s *spanOrSpans) computePositionsCost() {
	var sumPositionsCost float32
	var sumCost int64
	for _, w := range s.byDocQueue.heap {
		if w == nil {
			continue
		}
		costWeight := int64(1)
		if w.cost > 1 {
			costWeight = w.cost
		}
		sumPositionsCost += s.subSpans[0].PositionsCost() * float32(costWeight)
		sumCost += costWeight
	}
	s.positionsCost = sumPositionsCost / float32(sumCost)
}

func (s *spanOrSpans) PositionsCost() float32 {
	return s.positionsCost
}

func (s *spanOrSpans) twoPhaseCurrentDocMatches() bool {
	listAtCurrentDoc := s.byDocQueue.topList()
	if listAtCurrentDoc == nil {
		return false
	}
	currentDoc := listAtCurrentDoc.doc
	for listAtCurrentDoc != nil {
		if listAtCurrentDoc.twoPhaseView != nil {
			if listAtCurrentDoc.twoPhaseView.Matches() {
				listAtCurrentDoc.lastApproxMatchDoc = currentDoc
				break
			}
			listAtCurrentDoc.lastApproxNonMatchDoc = currentDoc
			listAtCurrentDoc = listAtCurrentDoc.next
			if listAtCurrentDoc == nil {
				return false
			}
		} else {
			break
		}
	}
	s.lastDocTwoPhaseMatched = currentDoc
	s.topPositionSpans = nil
	return true
}

func (s *spanOrSpans) fillPositionQueue() error {
	s.byPositionQueue.clear()
	listAtCurrentDoc := s.byDocQueue.topList()
	for listAtCurrentDoc != nil {
		spansAtDoc := listAtCurrentDoc.spans
		if s.lastDocTwoPhaseMatched == listAtCurrentDoc.doc {
			if listAtCurrentDoc.twoPhaseView != nil {
				if listAtCurrentDoc.lastApproxNonMatchDoc == listAtCurrentDoc.doc {
					spansAtDoc = nil
				} else {
					if listAtCurrentDoc.lastApproxMatchDoc != listAtCurrentDoc.doc {
						if !listAtCurrentDoc.twoPhaseView.Matches() {
							spansAtDoc = nil
						}
					}
				}
			}
		}

		if spansAtDoc != nil {
			spansAtDoc.NextStartPosition()
			s.byPositionQueue.add(spansAtDoc)
		}
		listAtCurrentDoc = listAtCurrentDoc.next
	}
	return nil
}

func (s *spanOrSpans) NextStartPosition() (int, error) {
	if s.topPositionSpans == nil {
		if err := s.fillPositionQueue(); err != nil {
			return -1, err
		}
		s.topPositionSpans = s.byPositionQueue.top()
	} else {
		s.topPositionSpans.NextStartPosition()
		s.topPositionSpans = s.byPositionQueue.updateTop()
	}
	if s.topPositionSpans == nil {
		return -1, nil
	}
	return s.topPositionSpans.StartPosition(), nil
}

func (s *spanOrSpans) StartPosition() int {
	if s.topPositionSpans == nil {
		return -1
	}
	return s.topPositionSpans.StartPosition()
}

func (s *spanOrSpans) EndPosition() int {
	if s.topPositionSpans == nil {
		return -1
	}
	return s.topPositionSpans.EndPosition()
}

func (s *spanOrSpans) Width() int {
	if s.topPositionSpans == nil {
		return 0
	}
	return s.topPositionSpans.Width()
}

func (s *spanOrSpans) Freq() int {
	return 0
}

func (s *spanOrSpans) DocIDRunEnd() int {
	return s.DocID() + 1
}

func (s *spanOrSpans) Collect(collector *SpanCollector) {
	if s.topPositionSpans != nil {
		s.topPositionSpans.Collect(collector)
	}
}

func (s *spanOrSpans) Cost() int64 {
	var cost int64
	for _, spans := range s.subSpans {
		cost += spans.Cost()
	}
	return cost
}

// --- Helpers ---

type spanDisiWrapper struct {
	spans             Spans
	iterator          DocIdSetIterator
	cost              int64
	matchCost         float32
	doc               int
	next              *spanDisiWrapper
	approximation     DocIdSetIterator
	twoPhaseView      *TwoPhaseIterator
	lastApproxMatchDoc int
	lastApproxNonMatchDoc int
}

func newSpanDisiWrapper(spans Spans) *spanDisiWrapper {
	sw := &spanDisiWrapper{
		spans: spans,
	}
	if it, ok := spans.(DocIdSetIterator); ok {
		sw.iterator = it
		sw.cost = it.Cost()
	}
	sw.doc = -1
	sw.twoPhaseView = spans.AsTwoPhaseIterator()
	if sw.twoPhaseView != nil {
		sw.approximation = sw.twoPhaseView.Approximation()
		sw.matchCost = sw.twoPhaseView.MatchCost()
	} else {
		sw.approximation = sw.iterator
		sw.matchCost = 0
	}
	sw.lastApproxNonMatchDoc = -2
	sw.lastApproxMatchDoc = -2
	return sw
}

type spanDisiPriorityQueuePtr struct {
	heap []*spanDisiWrapper
	size int
}

func newSpanDisiPriorityQueue(maxSize int) *spanDisiPriorityQueuePtr {
	return &spanDisiPriorityQueuePtr{
		heap: make([]*spanDisiWrapper, maxSize),
		size: 0,
	}
}

func (pq *spanDisiPriorityQueuePtr) top() *spanDisiWrapper {
	return pq.heap[0]
}

func (pq *spanDisiPriorityQueuePtr) topList() *spanDisiWrapper {
	list := pq.heap[0]
	list.next = nil
	if pq.size >= 3 {
		list = pq.topListRecursive(list, pq.heap, pq.size, 1)
		list = pq.topListRecursive(list, pq.heap, pq.size, 2)
	} else if pq.size == 2 && pq.heap[1].doc == list.doc {
		pq.heap[1].next = list
		list = pq.heap[1]
	}
	return list
}

func (pq *spanDisiPriorityQueuePtr) topListRecursive(list *spanDisiWrapper, heap []*spanDisiWrapper, size int, i int) *spanDisiWrapper {
	w := heap[i]
	if w.doc == list.doc {
		w.next = list
		list = w
		left := (i + 1) << 1 - 1
		right := left + 1
		if right < size {
			list = pq.topListRecursive(list, heap, size, left)
			list = pq.topListRecursive(list, heap, size, right)
		} else if left < size && heap[left].doc == list.doc {
			heap[left].next = list
			list = heap[left]
		}
	}
	return list
}

func (pq *spanDisiPriorityQueuePtr) add(entry *spanDisiWrapper) *spanDisiWrapper {
	pq.heap[pq.size] = entry
	pq.upHeap(pq.size)
	pq.size++
	return pq.heap[0]
}

func (pq *spanDisiPriorityQueuePtr) updateTop() *spanDisiWrapper {
	pq.downHeap(pq.size)
	return pq.heap[0]
}

func (pq *spanDisiPriorityQueuePtr) upHeap(i int) {
	node := pq.heap[i]
	j := (i + 1) >> 1 - 1
	for j >= 0 && node.doc < pq.heap[j].doc {
		pq.heap[i] = pq.heap[j]
		i = j
		j = (i + 1) >> 1 - 1
	}
	pq.heap[i] = node
}

func (pq *spanDisiPriorityQueuePtr) downHeap(size int) {
	i := 0
	node := pq.heap[0]
	j := (i + 1) << 1 - 1
	if j < size {
		k := j + 1
		if k < size && pq.heap[k].doc < pq.heap[j].doc {
			j = k
		}
		if pq.heap[j].doc < node.doc {
			for {
				pq.heap[i] = pq.heap[j]
				i = j
				j = (i + 1) << 1 - 1
				k = j + 1
				if k < size && pq.heap[k].doc < pq.heap[j].doc {
					j = k
				}
				if j >= size || pq.heap[j].doc >= node.doc {
					break
				}
			}
			pq.heap[i] = node
		}
	}
}

type spanPositionQueue struct {
	heap []Spans
	size int
}

func newSpanPositionQueue(maxSize int) *spanPositionQueue {
	return &spanPositionQueue{
		heap: make([]Spans, maxSize),
		size: 0,
	}
}

func (pq *spanPositionQueue) add(s Spans) {
	pq.heap[pq.size] = s
	pq.upHeap(pq.size)
	pq.size++
}

func (pq *spanPositionQueue) top() Spans {
	return pq.heap[0]
}

func (pq *spanPositionQueue) updateTop() Spans {
	pq.downHeap(pq.size)
	return pq.heap[0]
}

func (pq *spanPositionQueue) clear() {
	pq.size = 0
}

func (pq *spanPositionQueue) upHeap(i int) {
	node := pq.heap[i]
	j := (i + 1) >> 1 - 1
	for j >= 0 && pq.lessThan(node, pq.heap[j]) {
		pq.heap[i] = pq.heap[j]
		i = j
		j = (i + 1) >> 1 - 1
	}
	pq.heap[i] = node
}

func (pq *spanPositionQueue) downHeap(size int) {
	i := 0
	node := pq.heap[0]
	j := (i + 1) << 1 - 1
	if j < size {
		k := j + 1
		if k < size && pq.lessThan(pq.heap[k], pq.heap[j]) {
			j = k
		}
		if pq.lessThan(pq.heap[j], node) {
			for {
				pq.heap[i] = pq.heap[j]
				i = j
				j = (i + 1) << 1 - 1
				k = j + 1
				if k < size && pq.lessThan(pq.heap[k], pq.heap[j]) {
					j = k
				}
				if j >= size || !pq.lessThan(pq.heap[j], node) {
					break
				}
			}
			pq.heap[i] = node
		}
	}
}

func (pq *spanPositionQueue) lessThan(s1, s2 Spans) bool {
	start1 := s1.StartPosition()
	start2 := s2.StartPosition()
	if start1 < start2 {
		return true
	}
	if start1 == start2 {
		return s1.EndPosition() < s2.EndPosition()
	}
	return false
}

type spanDisjunctionDISIApproximation struct {
	subIterators *spanDisiPriorityQueuePtr
	cost         int64
}

func (a *spanDisjunctionDISIApproximation) NextDoc() (int, error) {
	top := a.subIterators.top()
	doc := top.doc
	for {
		nextDoc, err := top.approximation.NextDoc()
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		top.doc = nextDoc
		top = a.subIterators.updateTop()
		if top.doc != doc {
			break
		}
	}
	return top.doc, nil
}

func (a *spanDisjunctionDISIApproximation) Advance(target int) (int, error) {
	top := a.subIterators.top()
	for {
		nextDoc, err := top.approximation.Advance(target)
		if err != nil {
			return index.NO_MORE_DOCS, err
		}
		top.doc = nextDoc
		top = a.subIterators.updateTop()
		if top.doc >= target {
			break
		}
	}
	return top.doc, nil
}

func (a *spanDisjunctionDISIApproximation) DocID() int {
	return a.subIterators.top().doc
}

func (a *spanDisjunctionDISIApproximation) Cost() int64 {
	return a.cost
}
