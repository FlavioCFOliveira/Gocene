// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// AllGroupHeadsCollector specializes in collecting the most relevant document
// (group head) for each group that matches the query.
//
// Clients should create new collectors by calling NewAllGroupHeadsCollector.
//
// Mirrors the abstract class
// org.apache.lucene.search.grouping.AllGroupHeadsCollector<T>, which extends
// SimpleCollector. Its single abstract member, newGroupHead, is supplied by
// the private subclass that built the instance.
//
// lucene.experimental
type AllGroupHeadsCollector[T any] struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector

	groupSelector GroupSelector[T]
	sort          *search.Sort

	reversed   []int
	compIDXEnd int

	heads *groupMap[T, GroupHead[T]]

	context *index.LeafReaderContext
	scorer  search.Scorable

	// newGroupHead renders the protected abstract method
	// `GroupHead<T> newGroupHead(int, T, LeafReaderContext, Scorable)`.
	newGroupHead func(doc int, value T, context *index.LeafReaderContext, scorer search.Scorable) (GroupHead[T], error)
}

// NewAllGroupHeadsCollector creates a new AllGroupHeadsCollector based on the
// type of within-group Sort required.
//
// selector is a GroupSelector to define the groups and sort is the
// within-group sort to use to choose the group head document.
//
// Mirrors the static method
// AllGroupHeadsCollector<T> newCollector(GroupSelector<T>, Sort).
func NewAllGroupHeadsCollector[T any](selector GroupSelector[T], sort *search.Sort) *AllGroupHeadsCollector[T] {
	if sort.Equals(search.RELEVANCE) {
		return newScoringGroupHeadsCollector(selector, sort).AllGroupHeadsCollector
	}
	return newSortingGroupHeadsCollector(selector, sort).AllGroupHeadsCollector
}

// newAllGroupHeadsCollector mirrors the private constructor
// AllGroupHeadsCollector(GroupSelector<T>, Sort).
func newAllGroupHeadsCollector[T any](selector GroupSelector[T], sort *search.Sort) *AllGroupHeadsCollector[T] {
	c := &AllGroupHeadsCollector[T]{
		groupSelector: selector,
		sort:          sort,
		heads:         newGroupMap[T, GroupHead[T]](),
	}
	c.Outer = c
	c.reversed = make([]int, len(sort.GetSort()))
	sortFields := sort.GetSort()
	for i := 0; i < len(sortFields); i++ {
		if sortFields[i].GetReverse() {
			c.reversed[i] = -1
		} else {
			c.reversed[i] = 1
		}
	}
	c.compIDXEnd = len(c.reversed) - 1
	return c
}

// RetrieveGroupHeadsBitSet returns a util.FixedBitSet containing all group
// heads, where maxDoc is the maxDoc of the top level index reader.
//
// Mirrors FixedBitSet retrieveGroupHeads(int maxDoc).
func (c *AllGroupHeadsCollector[T]) RetrieveGroupHeadsBitSet(maxDoc int) (*util.FixedBitSet, error) {
	bitSet, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}

	groupHeads := c.GetCollectedGroupHeads()
	for _, groupHead := range groupHeads {
		bitSet.Set(groupHead.Doc())
	}

	return bitSet, nil
}

// RetrieveGroupHeads returns an int slice containing all group heads. Its
// length is equal to the number of collected unique groups.
//
// Mirrors int[] retrieveGroupHeads().
func (c *AllGroupHeadsCollector[T]) RetrieveGroupHeads() []int {
	groupHeads := c.GetCollectedGroupHeads()
	docHeads := make([]int, len(groupHeads))

	i := 0
	for _, groupHead := range groupHeads {
		docHeads[i] = groupHead.Doc()
		i++
	}

	return docHeads
}

// GroupHeadsSize returns the number of group heads found for a query.
//
// Mirrors int groupHeadsSize().
func (c *AllGroupHeadsCollector[T]) GroupHeadsSize() int {
	return len(c.GetCollectedGroupHeads())
}

// GetCollectedGroupHeads returns the collected group heads. Subsequent calls
// return the same group heads.
//
// Mirrors protected Collection<? extends GroupHead<T>> getCollectedGroupHeads().
func (c *AllGroupHeadsCollector[T]) GetCollectedGroupHeads() []GroupHead[T] {
	return c.heads.values()
}

// Collect mirrors AllGroupHeadsCollector.collect(int).
func (c *AllGroupHeadsCollector[T]) Collect(doc int) error {
	if _, err := c.groupSelector.AdvanceTo(doc); err != nil {
		return err
	}
	groupValue, err := c.groupSelector.CurrentValue()
	if err != nil {
		return err
	}
	groupHead, found := c.heads.get(groupValue)
	if !found {
		groupValue, err = c.groupSelector.CopyValue()
		if err != nil {
			return err
		}
		head, err := c.newGroupHead(doc, groupValue, c.context, c.scorer)
		if err != nil {
			return err
		}
		c.heads.put(groupValue, head)
		return nil
	}

	// Ok now we need to check if the current doc is more relevant than top doc for this group
	for compIDX := 0; ; compIDX++ {
		cmp, err := groupHead.Compare(compIDX, doc)
		if err != nil {
			return err
		}
		cmp = c.reversed[compIDX] * cmp
		if cmp < 0 {
			// Definitely not competitive. So don't even bother to continue
			return nil
		} else if cmp > 0 {
			// Definitely competitive.
			break
		} else if compIDX == c.compIDXEnd {
			// Here c=0. If we're at the last comparator, this doc is not
			// competitive, since docs are visited in doc Id order, which means
			// this doc cannot compete with any other document in the queue.
			return nil
		}
	}
	return groupHead.UpdateDocHead(doc)
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (c *AllGroupHeadsCollector[T]) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (c *AllGroupHeadsCollector[T]) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// ScoreMode mirrors AllGroupHeadsCollector.scoreMode().
func (c *AllGroupHeadsCollector[T]) ScoreMode() search.ScoreMode {
	if c.sort.NeedsScores() {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

// DoSetNextReader mirrors
// AllGroupHeadsCollector.doSetNextReader(LeafReaderContext).
func (c *AllGroupHeadsCollector[T]) DoSetNextReader(context *index.LeafReaderContext) error {
	if err := c.groupSelector.SetNextReader(context); err != nil {
		return err
	}
	c.context = context
	for _, head := range c.heads.values() {
		if err := head.SetNextReader(context); err != nil {
			return err
		}
	}
	return nil
}

// SetScorer mirrors AllGroupHeadsCollector.setScorer(Scorable).
func (c *AllGroupHeadsCollector[T]) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	for _, head := range c.heads.values() {
		if err := head.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// GroupHead represents a group head. A group head is the most relevant
// document for a particular group. The relevancy is usually based on the
// sort.
//
// The group head contains a group value with its associated most relevant
// document id.
//
// Mirrors the nested abstract class AllGroupHeadsCollector.GroupHead<T>.
// Java's public fields groupValue and doc are reachable through an interface
// in Go only as methods, so they are rendered as the accessors GroupValue and
// Doc.
type GroupHead[T any] interface {
	// GroupValue renders the public final field T groupValue.
	GroupValue() T

	// Doc renders the public field int doc.
	Doc() int

	// SetNextReader is called for each segment.
	SetNextReader(ctx *index.LeafReaderContext) error

	// SetScorer is called for each segment.
	SetScorer(scorer search.Scorable) error

	// Compare compares the specified document for a specified comparator
	// against the current most relevant document. compIDX is the comparator
	// index of the specified comparator and doc the specified document. It
	// returns -1 if the specified document wasn't competitive against the
	// current most relevant document, 1 if it was, and otherwise 0.
	Compare(compIDX, doc int) (int, error)

	// UpdateDocHead updates the current most relevant document with the
	// specified document.
	UpdateDocHead(doc int) error

	// GetSortValues returns the sort values for this group head, or nil if
	// not stored.
	GetSortValues() []any

	// GetComparators returns the field comparators used to determine the
	// group head ordering, one per sort field.
	GetComparators() []search.FieldComparator
}

// BaseGroupHead carries the state and the concrete member of the abstract
// class AllGroupHeadsCollector.GroupHead<T>.
type BaseGroupHead[T any] struct {
	groupValue T
	doc        int

	docBase int
}

// newBaseGroupHead mirrors protected GroupHead(T groupValue, int doc, int docBase).
func newBaseGroupHead[T any](groupValue T, doc, docBase int) BaseGroupHead[T] {
	return BaseGroupHead[T]{groupValue: groupValue, doc: doc + docBase, docBase: docBase}
}

// GroupValue renders the public final field groupValue.
func (h *BaseGroupHead[T]) GroupValue() T { return h.groupValue }

// Doc renders the public field doc.
func (h *BaseGroupHead[T]) Doc() int { return h.doc }

// SetNextReader is called for each segment.
//
// Mirrors protected void setNextReader(LeafReaderContext ctx).
func (h *BaseGroupHead[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	h.docBase = ctx.DocBase
	return nil
}

// sortingGroupHeadsCollector mirrors the private static class
// AllGroupHeadsCollector.SortingGroupHeadsCollector<T>: the general
// implementation using a search.FieldComparator to select the group head.
type sortingGroupHeadsCollector[T any] struct {
	*AllGroupHeadsCollector[T]
}

// newSortingGroupHeadsCollector mirrors
// SortingGroupHeadsCollector(GroupSelector<T>, Sort).
func newSortingGroupHeadsCollector[T any](selector GroupSelector[T], sort *search.Sort) *sortingGroupHeadsCollector[T] {
	base := newAllGroupHeadsCollector(selector, sort)
	c := &sortingGroupHeadsCollector[T]{AllGroupHeadsCollector: base}
	base.newGroupHead = c.newGroupHeadImpl
	return c
}

// newGroupHeadImpl mirrors SortingGroupHeadsCollector.newGroupHead.
func (c *sortingGroupHeadsCollector[T]) newGroupHeadImpl(
	doc int, value T, ctx *index.LeafReaderContext, scorer search.Scorable,
) (GroupHead[T], error) {
	return newSortingGroupHead(c.sort, value, doc, ctx, scorer)
}

// sortingGroupHead mirrors the private static class
// AllGroupHeadsCollector.SortingGroupHead<T>.
type sortingGroupHead[T any] struct {
	BaseGroupHead[T]

	comparators     []search.FieldComparator
	leafComparators []search.LeafFieldComparator
	sortValues      []any
}

// newSortingGroupHead mirrors
// SortingGroupHead(Sort, T, int, LeafReaderContext, Scorable).
func newSortingGroupHead[T any](
	sort *search.Sort, groupValue T, doc int, context *index.LeafReaderContext, scorer search.Scorable,
) (*sortingGroupHead[T], error) {
	h := &sortingGroupHead[T]{BaseGroupHead: newBaseGroupHead(groupValue, doc, context.DocBase)}
	sortFields := sort.GetSort()
	h.comparators = make([]search.FieldComparator, len(sortFields))
	h.leafComparators = make([]search.LeafFieldComparator, len(sortFields))
	h.sortValues = make([]any, len(sortFields))
	for i := 0; i < len(sortFields); i++ {
		h.comparators[i] = search.SortFieldGetComparator(sortFields[i], 1, search.PruningNone)
		leaf, err := h.comparators[i].GetLeafComparator(context)
		if err != nil {
			return nil, err
		}
		h.leafComparators[i] = leaf
		if err := h.leafComparators[i].SetScorer(scorer); err != nil {
			return nil, err
		}
		if err := h.leafComparators[i].Copy(0, doc); err != nil {
			return nil, err
		}
		if err := h.leafComparators[i].SetBottom(0); err != nil {
			return nil, err
		}
		h.sortValues[i] = h.comparators[i].Value(0)
	}
	return h, nil
}

// SetNextReader mirrors SortingGroupHead.setNextReader(LeafReaderContext).
func (h *sortingGroupHead[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	if err := h.BaseGroupHead.SetNextReader(ctx); err != nil {
		return err
	}
	for i := 0; i < len(h.comparators); i++ {
		leaf, err := h.comparators[i].GetLeafComparator(ctx)
		if err != nil {
			return err
		}
		h.leafComparators[i] = leaf
	}
	return nil
}

// SetScorer mirrors SortingGroupHead.setScorer(Scorable).
func (h *sortingGroupHead[T]) SetScorer(scorer search.Scorable) error {
	for _, c := range h.leafComparators {
		if err := c.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// Compare mirrors SortingGroupHead.compare(int, int).
func (h *sortingGroupHead[T]) Compare(compIDX, doc int) (int, error) {
	return h.leafComparators[compIDX].CompareBottom(doc)
}

// UpdateDocHead mirrors SortingGroupHead.updateDocHead(int).
func (h *sortingGroupHead[T]) UpdateDocHead(doc int) error {
	for i := 0; i < len(h.leafComparators); i++ {
		if err := h.leafComparators[i].Copy(0, doc); err != nil {
			return err
		}
		if err := h.leafComparators[i].SetBottom(0); err != nil {
			return err
		}
		h.sortValues[i] = h.comparators[i].Value(0)
	}
	h.doc = doc + h.docBase
	return nil
}

// GetSortValues mirrors SortingGroupHead.getSortValues().
func (h *sortingGroupHead[T]) GetSortValues() []any { return h.sortValues }

// GetComparators mirrors SortingGroupHead.getComparators().
func (h *sortingGroupHead[T]) GetComparators() []search.FieldComparator { return h.comparators }

// scoringGroupHeadsCollector mirrors the private static class
// AllGroupHeadsCollector.ScoringGroupHeadsCollector<T>: the specialized
// implementation for sorting by score.
type scoringGroupHeadsCollector[T any] struct {
	*AllGroupHeadsCollector[T]
}

// newScoringGroupHeadsCollector mirrors
// ScoringGroupHeadsCollector(GroupSelector<T>, Sort).
func newScoringGroupHeadsCollector[T any](selector GroupSelector[T], sort *search.Sort) *scoringGroupHeadsCollector[T] {
	base := newAllGroupHeadsCollector(selector, sort)
	c := &scoringGroupHeadsCollector[T]{AllGroupHeadsCollector: base}
	base.newGroupHead = c.newGroupHeadImpl
	return c
}

// newGroupHeadImpl mirrors ScoringGroupHeadsCollector.newGroupHead.
func (c *scoringGroupHeadsCollector[T]) newGroupHeadImpl(
	doc int, value T, context *index.LeafReaderContext, scorer search.Scorable,
) (GroupHead[T], error) {
	return newScoringGroupHead(scorer, value, doc, context.DocBase)
}

// scoringGroupHead mirrors the private static class
// AllGroupHeadsCollector.ScoringGroupHead<T>.
type scoringGroupHead[T any] struct {
	BaseGroupHead[T]

	scorer     search.Scorable
	topScore   float32
	sortValues []any
}

// newScoringGroupHead mirrors ScoringGroupHead(Scorable, T, int, int).
func newScoringGroupHead[T any](scorer search.Scorable, groupValue T, doc, docBase int) (*scoringGroupHead[T], error) {
	h := &scoringGroupHead[T]{BaseGroupHead: newBaseGroupHead(groupValue, doc, docBase)}
	h.scorer = scorer
	topScore, err := scorer.Score()
	if err != nil {
		return nil, err
	}
	h.topScore = topScore
	h.sortValues = []any{h.topScore}
	return h, nil
}

// SetScorer mirrors ScoringGroupHead.setScorer(Scorable).
func (h *scoringGroupHead[T]) SetScorer(scorer search.Scorable) error {
	h.scorer = scorer
	return nil
}

// Compare mirrors ScoringGroupHead.compare(int, int). Java asserts compIDX == 0.
func (h *scoringGroupHead[T]) Compare(compIDX, doc int) (int, error) {
	score, err := h.scorer.Score()
	if err != nil {
		return 0, err
	}
	c := javaFloatCompare(score, h.topScore)
	if c > 0 {
		h.topScore = score
	}
	return c, nil
}

// UpdateDocHead mirrors ScoringGroupHead.updateDocHead(int).
func (h *scoringGroupHead[T]) UpdateDocHead(doc int) error {
	h.doc = doc + h.docBase
	h.sortValues[0] = h.topScore
	return nil
}

// GetSortValues mirrors ScoringGroupHead.getSortValues().
func (h *scoringGroupHead[T]) GetSortValues() []any { return h.sortValues }

// GetComparators mirrors ScoringGroupHead.getComparators(), which returns null.
func (h *scoringGroupHead[T]) GetComparators() []search.FieldComparator { return nil }
