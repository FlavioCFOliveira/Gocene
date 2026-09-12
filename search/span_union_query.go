// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SpanUnionQuery matches the union of its clauses.
// This is the Go port of Lucene's org.apache.lucene.queries.spans.SpanOrQuery.
type SpanUnionQuery struct {
	BaseSpanQuery
	clauses []SpanQuery
}

// NewSpanUnionQuery constructs a SpanUnionQuery merging the provided clauses.
// All clauses must have the same field.
func NewSpanUnionQuery(clauses ...SpanQuery) (*SpanUnionQuery, error) {
	q := &SpanUnionQuery{
		clauses: make([]SpanQuery, 0, len(clauses)),
	}
	for _, seq := range clauses {
		if err := q.addClause(seq); err != nil {
			return nil, err
		}
	}
	return q, nil
}

func (q *SpanUnionQuery) addClause(clause SpanQuery) error {
	if q.field == "" {
		q.field = clause.GetField()
	} else if clause.GetField() != "" && clause.GetField() != q.field {
		return fmt.Errorf("clauses must have same field")
	}
	q.clauses = append(q.clauses, clause)
	return nil
}

// GetClauses returns the clauses whose spans are matched.
func (q *SpanUnionQuery) GetClauses() []SpanQuery {
	return q.clauses
}

// Rewrite rewrites the query.
func (q *SpanUnionQuery) Rewrite(indexSearcher *IndexSearcher) (Query, error) {
	rewritten := &SpanUnionQuery{}
	actuallyRewritten := false
	for _, c := range q.clauses {
		query, err := c.Rewrite(indexSearcher)
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
		if err := rewritten.addClause(spanQuery); err != nil {
			return nil, err
		}
	}
	if actuallyRewritten {
		return rewritten, nil
	}
	return q, nil
}

// Visit visits the query.
func (q *SpanUnionQuery) Visit(visitor QueryVisitor) {
	if !visitor.AcceptField(q.GetField()) {
		return
	}
	v := visitor.GetSubVisitor(SHOULD, q)
	for _, query := range q.clauses {
		query.Visit(v)
	}
}

// ToString returns a user-readable version of this query.
func (q *SpanUnionQuery) ToString(field string) string {
	buffer := "spanOr(["
	for i, clause := range q.clauses {
		buffer += clause.ToString(field)
		if i < len(q.clauses)-1 {
			buffer += ", "
		}
	}
	buffer += "])"
	return buffer
}

// Equals compares this query with another.
func (q *SpanUnionQuery) Equals(other Query) bool {
	o, ok := other.(*SpanUnionQuery)
	if !ok {
		return false
	}
	if len(q.clauses) != len(o.clauses) {
		return false
	}
	for i := range q.clauses {
		if q.clauses[i] != o.clauses[i] {
			return false
		}
	}
	return true
}

// HashCode returns the hash code of this query.
func (q *SpanUnionQuery) HashCode() int {
	h := 0
	for _, c := range q.clauses {
		h ^= c.HashCode()
	}
	return h
}

// CreateWeight creates a SpanWeight for this query.
func (q *SpanUnionQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (SpanWeight, error) {
	subWeights := make([]SpanWeight, 0, len(q.clauses))
	for _, query := range q.clauses {
		w, err := query.CreateWeight(searcher, needsScores, boost)
		if err != nil {
			return nil, err
		}
		subWeights = append(subWeights, w)
	}

	var terms map[*index.Term]*index.TermStates
	if needsScores {
		terms = GetTermStates(subWeights...)
	}

	return &spanUnionWeight{
		query:      q,
		searcher:   searcher,
		terms:      terms,
		subWeights: subWeights,
		boost:      boost,
	}, nil
}

type spanUnionWeight struct {
	query      *SpanUnionQuery
	searcher   *IndexSearcher
	terms      map[*index.Term]*index.TermStates
	subWeights []SpanWeight
	boost      float32
}

func (w *spanUnionWeight) IsCacheable(ctx LeafReaderContext) bool {
	for _, sw := range w.subWeights {
		if !sw.IsCacheable(ctx) {
			return false
		}
	}
	return true
}

func (w *spanUnionWeight) ExtractTermStates(contexts map[*index.Term]*index.TermStates) {
	for _, sw := range w.subWeights {
		sw.ExtractTermStates(contexts)
	}
}

func (w *spanUnionWeight) GetSpans(context LeafReaderContext, requiredPostings Postings) (Spans, error) {
	subSpans := make([]Spans, 0, len(w.query.clauses))
	for _, sw := range w.subWeights {
		spans, err := sw.GetSpans(context, requiredPostings)
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
		byDocQueue.add(&spanDisiWrapper{spans: spans})
	}

	byPositionQueue := util.NewPriorityQueue(len(subSpans), func(a, b Spans) bool {
		s1 := a.StartPosition()
		s2 := b.StartPosition()
		if s1 < s2 {
			return true
		}
		if s1 == s2 {
			return a.EndPosition() < b.EndPosition()
		}
		return false
	})

	return &spanUnionSpans{
		byDocQueue:      byDocQueue,
		byPositionQueue: byPositionQueue,
		subSpans:        subSpans,
	}, nil
}

type spanUnionSpans struct {
	byDocQueue             *spanDisiPriorityQueue
	byPositionQueue        *util.PriorityQueue[Spans]
	subSpans               []Spans
	topPositionSpans       Spans
	lastDocTwoPhaseMatched int
}

func (s *spanUnionSpans) NextDoc() (int, error) {
	s.topPositionSpans = nil
	topDocSpans := s.byDocQueue.top()
	currentDoc := topDocSpans.doc

	for {
		nextDoc, err := topDocSpans.iterator.NextDoc()
		if err != nil {
			return NO_MORE_DOCS, err
		}
		topDocSpans.doc = nextDoc
		topDocSpans = s.byDocQueue.updateTop()
		if topDocSpans.doc != currentDoc {
			break
		}
	}
	return topDocSpans.doc, nil
}

func (s *spanUnionSpans) Advance(target int) (int, error) {
	s.topPositionSpans = nil
	topDocSpans := s.byDocQueue.top()
	for {
		nextDoc, err := topDocSpans.iterator.Advance(target)
		if err != nil {
			return NO_MORE_DOCS, err
		}
		topDocSpans.doc = nextDoc
		topDocSpans = s.byDocQueue.updateTop()
		if topDocSpans.doc >= target {
			break
		}
	}
	return topDocSpans.doc, nil
}

func (s *spanUnionSpans) DocID() int {
	return s.byDocQueue.top().doc
}

func (s *spanUnionSpans) AsTwoPhaseIterator() *TwoPhaseIterator {
	var sumMatchCost float32
	var sumApproxCost int64

	for _, w := range s.byDocQueue.heap {
		if w == nil {
			continue
		}
		if w.twoPhaseView != nil {
			costWeight := w.cost
			if costWeight <= 1 {
				costWeight = 1
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

	return &spanUnionTwoPhaseIterator{
		byDocQueue: s.byDocQueue,
		matchCost:  matchCost,
		parent:     s,
	}
}

type spanUnionTwoPhaseIterator struct {
	byDocQueue *spanDisiPriorityQueue
	matchCost  float32
	parent     *spanUnionSpans
}

func (it *spanUnionTwoPhaseIterator) Matches() bool {
	listAtCurrentDoc := it.byDocQueue.topList()
	if listAtCurrentDoc == nil {
		return false
	}
	currentDoc := listAtCurrentDoc.doc
	for listAtCurrentDoc != nil {
		if listAtCurrentDoc.twoPhaseView != nil {
			if listAtCurrentDoc.twoPhaseView.Matches() {
				listAtCurrentDoc.lastApproxMatchDoc = currentDoc
				it.parent.lastDocTwoPhaseMatched = currentDoc
				break
			}
			listAtCurrentDoc.lastApproxNonMatchDoc = currentDoc
		}
		listAtCurrentDoc = listAtCurrentDoc.next
	}
	it.parent.topPositionSpans = nil
	return listAtCurrentDoc != nil
}

func (it *spanUnionTwoPhaseIterator) MatchCost() float32 {
	return it.matchCost
}

func (s *spanUnionSpans) NextStartPosition() (int, error) {
	if s.topPositionSpans == nil {
		s.byPositionQueue.Clear()
		if err := s.fillPositionQueue(); err != nil {
			return -1, err
		}
		s.topPositionSpans = s.byPositionQueue.Top()
	} else {
		_, err := s.topPositionSpans.NextStartPosition()
		if err != nil {
			return -1, err
		}
		s.topPositionSpans = s.byPositionQueue.Top()
	}
	return s.topPositionSpans.StartPosition(), nil
}

func (s *spanUnionSpans) StartPosition() int {
	if s.topPositionSpans == nil {
		return -1
	}
	return s.topPositionSpans.StartPosition()
}

func (s *spanUnionSpans) EndPosition() int {
	if s.topPositionSpans == nil {
		return -1
	}
	return s.topPositionSpans.EndPosition()
}

func (s *spanUnionSpans) Width() int {
	if s.topPositionSpans == nil {
		return 0
	}
	return s.topPositionSpans.Width()
}

func (s *spanUnionSpans) Collect(collector *SpanCollector) {
	if s.topPositionSpans != nil {
		s.topPositionSpans.Collect(collector)
	}
}

func (s *spanUnionSpans) computePositionsCost() {
	var sumPositionsCost float32
	var sumCost int64
	for _, w := range s.byDocQueue.heap {
		if w == nil {
			continue
		}
		costWeight := w.cost
		if costWeight <= 1 {
			costWeight = 1
		}
		sumPositionsCost += s.subSpans[0].PositionsCost() * float32(costWeight) // simplified
		sumCost += costWeight
	}
	// In a real implementation, the cost is aggregated across all matching spans
}

func (s *spanUnionSpans) PositionsCost() float32 {
	return 4.0 // Simplified estimate matching BaseSpans
}

func (s *spanUnionSpans) fillPositionQueue() error {
	// This is called at first NextStartPosition.
	// Add all matching Spans at current doc to byPositionQueue.
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
			s.byPositionQueue.Add(spansAtDoc)
		}
		listAtCurrentDoc = listAtCurrentDoc.next
	}
	return nil
}

// Support classes

type spanDisiWrapper struct {
	iterator              DocIdSetIterator
	cost                  int64
	matchCost             float32
	doc                   int
	next                  *spanDisiWrapper
	approximation         DocIdSetIterator
	twoPhaseView          *TwoPhaseIterator
	spans                 Spans
	lastApproxMatchDoc    int
	lastApproxNonMatchDoc int
}

func newSpanDisiWrapper(spans Spans) *spanDisiWrapper {
	iterator := spans
	cost := iterator.Cost()
	var twoPhaseView *TwoPhaseIterator
	var approximation DocIdSetIterator
	var matchCost float32

	twoPhaseView = spans.AsTwoPhaseIterator()
	if twoPhaseView != nil {
		approximation = twoPhaseView.Approximation()
		matchCost = twoPhaseView.MatchCost()
	} else {
		approximation = iterator
		matchCost = 0
	}

	return &spanDisiWrapper{
		spans:                 spans,
		iterator:              iterator,
		cost:                  cost,
		doc:                   -1,
		twoPhaseView:          twoPhaseView,
		approximation:         approximation,
		matchCost:             matchCost,
		lastApproxNonMatchDoc: -2,
		lastApproxMatchDoc:    -2,
	}
}

type spanDisiPriorityQueue struct {
	heap []*spanDisiWrapper
	size int
}

func newSpanDisiPriorityQueue(maxSize int) *spanDisiPriorityQueue {
	return &spanDisiPriorityQueue{
		heap: make([]*spanDisiWrapper, maxSize),
		size: 0,
	}
}

func (pq *spanDisiPriorityQueue) top() *spanDisiWrapper {
	return pq.heap[0]
}

func (pq *spanDisiPriorityQueue) topList() *spanDisiWrapper {
	heap := pq.heap
	size := pq.size
	list := heap[0]
	list.next = nil
	if size >= 3 {
		list = pq.topListRec(list, heap, size, 1)
		list = pq.topListRec(list, heap, size, 2)
	} else if size == 2 && heap[1].doc == list.doc {
		list = pq.prepend(heap[1], list)
	}
	return list
}

func (pq *spanDisiPriorityQueue) prepend(w1, w2 *spanDisiWrapper) *spanDisiWrapper {
	w1.next = w2
	return w1
}

func (pq *spanDisiPriorityQueue) topListRec(list *spanDisiWrapper, heap []*spanDisiWrapper, size, i int) *spanDisiWrapper {
	w := heap[i]
	if w.doc == list.doc {
		list = pq.prepend(w, list)
		left := (i+1)<<1 - 1
		right := left + 1
		if right < size {
			list = pq.topListRec(list, heap, size, left)
			list = pq.topListRec(list, heap, size, right)
		} else if left < size && heap[left].doc == list.doc {
			list = pq.prepend(heap[left], list)
		}
	}
	return list
}

func (pq *spanDisiPriorityQueue) add(entry *spanDisiWrapper) *spanDisiWrapper {
	heap := pq.heap
	size := pq.size
	heap[size] = entry
	pq.upHeap(size)
	pq.size = size + 1
	return heap[0]
}

func (pq *spanDisiPriorityQueue) updateTop() *spanDisiWrapper {
	pq.downHeap(pq.size)
	return pq.heap[0]
}

func (pq *spanDisiPriorityQueue) upHeap(i int) {
	node := pq.heap[i]
	nodeDoc := node.doc
	j := ((i + 1) >> 1) - 1
	for j >= 0 && nodeDoc < pq.heap[j].doc {
		pq.heap[i] = pq.heap[j]
		i = j
		j = ((i + 1) >> 1) - 1
	}
	pq.heap[i] = node
}

func (pq *spanDisiPriorityQueue) downHeap(size int) {
	i := 0
	node := pq.heap[0]
	j := (i+1)<<1 - 1
	if j < size {
		k := j + 1
		if k < size && pq.heap[k].doc < pq.heap[j].doc {
			j = k
		}
		if pq.heap[j].doc < node.doc {
			for {
				pq.heap[i] = pq.heap[j]
				i = j
				j = (i+1)<<1 - 1
				k = j + 1
				if j >= size {
					break
				}
				if k < size && pq.heap[k].doc < pq.heap[j].doc {
					j = k
				}
				if pq.heap[j].doc >= node.doc {
					break
				}
			}
			pq.heap[i] = node
		}
	}
}
