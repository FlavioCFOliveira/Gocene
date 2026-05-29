// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/TopFieldCollector.java
//   lucene/core/src/java/org/apache/lucene/search/FieldValueHitQueue.java
//
// TopFieldCollector collects the top-N documents ordered by one or more sort
// fields read from DocValues, rather than by relevance score. It maintains a
// fixed-capacity priority queue keyed by the sort comparators and runs the
// LeafFieldComparator lifecycle (setReader/copy/setBottom/compareBottom) per
// segment, exactly as Lucene does.
//
// Scope note: this port reproduces the value-correct collection path (the order
// of returned hits and their FieldDoc sort values). The two performance
// optimisations Lucene layers on top — competitive-document skipping via the
// comparators' CompetitiveIterator and early termination when the search sort is
// a prefix of the index sort — are intentionally omitted; with them disabled
// Lucene produces the identical ordering, so the omission is not observable.

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// fieldEntry is one slot in the field-value priority queue. slot indexes the
// per-slot value cache held by every comparator; doc is the global document id.
//
// Mirrors org.apache.lucene.search.FieldValueHitQueue.Entry.
type fieldEntry struct {
	slot int
	doc  int
}

// fieldValueHitQueue is a min-heap whose "smallest" element is the weakest hit
// (the one that should be evicted first). Ordering is delegated to the sort
// comparators via lessThan.
//
// Mirrors org.apache.lucene.search.FieldValueHitQueue.
type fieldValueHitQueue struct {
	heap        []*fieldEntry
	comparators []sortFieldComparator
	reverseMul  []int
}

func newFieldValueHitQueue(comparators []sortFieldComparator, reverseMul []int, capacity int) *fieldValueHitQueue {
	return &fieldValueHitQueue{
		heap:        make([]*fieldEntry, 0, capacity),
		comparators: comparators,
		reverseMul:  reverseMul,
	}
}

func (q *fieldValueHitQueue) size() int { return len(q.heap) }

// lessThan reports whether a should sort after b — i.e. a is weaker than b and
// belongs closer to the top of the min-heap. The first non-zero comparator
// result wins; ties break on higher doc id (so equal-value hits keep ascending
// docID order, matching Lucene bug #31241 fix).
//
// Mirrors FieldValueHitQueue.lessThan (single- and multi-comparator variants).
func (q *fieldValueHitQueue) lessThan(a, b *fieldEntry) bool {
	for i, cmp := range q.comparators {
		c := q.reverseMul[i] * cmp.compare(a.slot, b.slot)
		if c != 0 {
			return c > 0
		}
	}
	return a.doc > b.doc
}

func (q *fieldValueHitQueue) up(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if !q.lessThan(q.heap[i], q.heap[parent]) {
			break
		}
		q.heap[i], q.heap[parent] = q.heap[parent], q.heap[i]
		i = parent
	}
}

func (q *fieldValueHitQueue) down(i int) {
	n := len(q.heap)
	for {
		l, r := 2*i+1, 2*i+2
		smallest := i
		if l < n && q.lessThan(q.heap[l], q.heap[smallest]) {
			smallest = l
		}
		if r < n && q.lessThan(q.heap[r], q.heap[smallest]) {
			smallest = r
		}
		if smallest == i {
			break
		}
		q.heap[i], q.heap[smallest] = q.heap[smallest], q.heap[i]
		i = smallest
	}
}

// add pushes a new entry and returns the (possibly unchanged) top entry.
//
// Mirrors PriorityQueue.add followed by reading top().
func (q *fieldValueHitQueue) add(e *fieldEntry) *fieldEntry {
	q.heap = append(q.heap, e)
	q.up(len(q.heap) - 1)
	return q.heap[0]
}

// top returns the weakest entry without removing it.
func (q *fieldValueHitQueue) top() *fieldEntry {
	if len(q.heap) == 0 {
		return nil
	}
	return q.heap[0]
}

// updateTop re-sifts the root after its key changed in place and returns the new
// top. Mirrors PriorityQueue.updateTop.
func (q *fieldValueHitQueue) updateTop() *fieldEntry {
	q.down(0)
	return q.heap[0]
}

// TopFieldCollector collects the top-N documents sorted by the sort fields'
// DocValues. It is the Go port of org.apache.lucene.search.TopFieldCollector.
type TopFieldCollector struct {
	*SimpleCollector

	numHits int
	sort    *Sort

	comparators []sortFieldComparator
	reverseMul  []int
	queue       *fieldValueHitQueue

	totalHits int
	maxScore  float32
	collected int
	queueFull bool
	bottom    *fieldEntry
}

// NewTopFieldCollector creates a TopFieldCollector for the given sort. numHits
// must be > 0 and sort must contain at least one field; callers that build it
// directly (rather than via the manager) are responsible for those invariants.
func NewTopFieldCollector(numHits int, sort *Sort) *TopFieldCollector {
	scoreMode := COMPLETE_NO_SCORES
	if sort.NeedsScores() {
		scoreMode = COMPLETE
	}

	comparators := make([]sortFieldComparator, 0, len(sort.Fields))
	reverseMuls := make([]int, 0, len(sort.Fields))
	for _, sf := range sort.Fields {
		cmp, err := newSortFieldComparator(sf, numHits)
		if err != nil {
			// SCORE/DOC sort fields have no DocValues comparator; substitute a
			// score/doc comparator so the collector still orders correctly.
			cmp = newBuiltinComparator(numHits, sf)
		}
		comparators = append(comparators, cmp)
		reverseMuls = append(reverseMuls, reverseMul(sf))
	}

	return &TopFieldCollector{
		SimpleCollector: NewSimpleCollector(scoreMode),
		numHits:         numHits,
		sort:            sort,
		comparators:     comparators,
		reverseMul:      reverseMuls,
		queue:           newFieldValueHitQueue(comparators, reverseMuls, numHits),
	}
}

// GetLeafCollector binds every comparator to the new leaf and returns a
// LeafCollector. The reader carries the segment's DocValues, which the
// comparators resolve in setReader.
func (c *TopFieldCollector) GetLeafCollector(reader IndexReader) (LeafCollector, error) {
	for _, cmp := range c.comparators {
		if err := cmp.setReader(reader); err != nil {
			return nil, err
		}
	}
	return NewTopFieldLeafCollector(c, 0), nil
}

// TopDocs returns the collected hits as a TopFieldDocs, ordered best-first, with
// each ScoreDoc upgraded to a FieldDoc carrying its per-field sort values.
func (c *TopFieldCollector) TopDocs() *TopDocs {
	return c.topFieldDocs().TopDocs
}

// topFieldDocs is the typed accessor used by the manager's Reduce so the per-hit
// FieldDoc sort values survive the merge.
func (c *TopFieldCollector) topFieldDocs() *TopFieldDocs {
	n := c.queue.size()
	entries := make([]*fieldEntry, n)
	copy(entries, c.queue.heap)

	// Sort entries best-first: an entry is "better" when it is NOT lessThan the
	// other (lessThan marks the weaker hit). Reusing the queue ordering keeps the
	// tie-break (ascending docID) identical to collection.
	sort.SliceStable(entries, func(i, j int) bool {
		return c.queue.lessThan(entries[j], entries[i])
	})

	fieldDocs := make([]*FieldDoc, n)
	for i, e := range entries {
		fields := make([]any, len(c.comparators))
		for k, cmp := range c.comparators {
			fields[k] = cmp.value(e.slot)
		}
		fieldDocs[i] = NewFieldDocWithFields(e.doc, float32(0), fields)
	}

	return NewTopFieldDocsWithFieldDocs(
		NewTotalHits(int64(c.totalHits), EQUAL_TO),
		fieldDocs,
		c.sort.Fields,
	)
}

// GetTotalHits returns the total number of hits collected.
func (c *TopFieldCollector) GetTotalHits() int { return c.totalHits }

// GetMaxScore returns the maximum score seen (0 unless the sort needs scores).
func (c *TopFieldCollector) GetMaxScore() float32 { return c.maxScore }

// TopFieldLeafCollector drives one segment. It composes the per-leaf comparators
// into a single multiLeafFieldComparator (so a multi-key sort short-circuits on
// the first differing key) and runs the copy/add/setBottom lifecycle.
type TopFieldLeafCollector struct {
	*BaseLeafCollector
	collector  *TopFieldCollector
	comparator LeafFieldComparator
	scorer     Scorer
	docBase    int
}

// NewTopFieldLeafCollector creates a leaf collector. The composite leaf
// comparator is built from the collector's comparators with their reverse
// multipliers.
func NewTopFieldLeafCollector(collector *TopFieldCollector, docBase int) *TopFieldLeafCollector {
	leafComparators := make([]LeafFieldComparator, len(collector.comparators))
	for i, cmp := range collector.comparators {
		leafComparators[i] = cmp
	}
	var comparator LeafFieldComparator
	if len(leafComparators) == 1 {
		comparator = leafComparators[0]
	} else {
		comparator, _ = newMultiLeafFieldComparator(leafComparators, collector.reverseMul)
	}
	return &TopFieldLeafCollector{
		BaseLeafCollector: NewBaseLeafCollector(),
		collector:         collector,
		comparator:        comparator,
		docBase:           docBase,
	}
}

// SetScorer records the scorer and forwards it to the comparators (only a
// score-typed comparator consumes it).
func (c *TopFieldLeafCollector) SetScorer(scorer Scorer) error {
	c.scorer = scorer
	if sc, ok := scorerAsScorable(scorer); ok {
		return c.comparator.SetScorer(sc)
	}
	return nil
}

// SetDocBase sets the document base offset for the segment and propagates it to
// any DOC comparator, whose values are global (docBase-rebased) document ids.
func (c *TopFieldLeafCollector) SetDocBase(docBase int) {
	c.docBase = docBase
	for _, cmp := range c.collector.comparators {
		if dc, ok := cmp.(*docComparator); ok {
			dc.docBase = docBase
		}
	}
}

// Collect adds the leaf-local doc to the queue if it is competitive.
//
// Mirrors TopFieldCollector.TopFieldLeafCollector.collect (the non-paging path).
func (c *TopFieldLeafCollector) Collect(doc int) error {
	col := c.collector
	col.totalHits++
	if c.scorer != nil {
		if s := c.scorer.Score(); s > col.maxScore {
			col.maxScore = s
		}
	}

	if col.queueFull {
		// Competitive check: reverseMul * compareBottom(doc) > 0 means doc is
		// better than the current bottom. Multi-key sorts already fold the
		// reverse multipliers inside the composite comparator, so use +1 there.
		rm := 1
		if len(col.comparators) == 1 {
			rm = col.reverseMul[0]
		}
		cb, err := c.comparator.CompareBottom(doc)
		if err != nil {
			return err
		}
		if rm*cb <= 0 {
			// Not competitive.
			return nil
		}
		// Replace the bottom element.
		if err := c.comparator.Copy(col.bottom.slot, doc); err != nil {
			return err
		}
		col.bottom.doc = c.docBase + doc
		col.bottom = col.queue.updateTop()
		return c.comparator.SetBottom(col.bottom.slot)
	}

	// Queue not yet full: take the next free slot.
	slot := col.collected
	col.collected++
	if err := c.comparator.Copy(slot, doc); err != nil {
		return err
	}
	col.bottom = col.queue.add(&fieldEntry{slot: slot, doc: c.docBase + doc})
	if col.collected == col.numHits {
		col.queueFull = true
		return c.comparator.SetBottom(col.bottom.slot)
	}
	return nil
}

// scorerAsScorable adapts a Scorer (Score() float32) to a Scorable
// (Score() (float32, error)) so it can be handed to the comparators' SetScorer.
// DocValues comparators ignore the scorer, so the adapter is only exercised by a
// score-typed comparator.
func scorerAsScorable(s Scorer) (Scorable, bool) {
	if s == nil {
		return nil, false
	}
	return &scorerScorableAdapter{s: s}, true
}

type scorerScorableAdapter struct {
	BaseScorable
	s Scorer
}

func (a *scorerScorableAdapter) Score() (float32, error) { return a.s.Score(), nil }

// Ensure TopFieldCollector implements Collector and the leaf type satisfies
// LeafCollector.
var (
	_ Collector     = (*TopFieldCollector)(nil)
	_ LeafCollector = (*TopFieldLeafCollector)(nil)
)

// reader-type assertion helper: the search loop hands GetLeafCollector the
// concrete leaf reader; the comparators type-assert it to the DocValues views.
var _ = index.NumericDocValues(nil)
