// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Ported from Apache Lucene 10.5.0:
//   lucene/grouping/src/java/org/apache/lucene/search/grouping/AllGroupHeadsCollector.java

// AllGroupHeadsCollector specializes in collecting the most relevant document
// (group head) for each group that matches the query.
//
// Clients should create new collectors by calling [NewAllGroupHeadsCollector].
//
// Mirrors org.apache.lucene.search.grouping.AllGroupHeadsCollector<T>, an
// abstract class with two private subclasses. Java's single abstract member,
// newGroupHead, is rendered as the newGroupHead field: the two subclasses
// differ in nothing else, so each factory supplies its own body.
type AllGroupHeadsCollector[T any] struct {
	search.BaseSimpleCollector

	groupSelector GroupSelector[T]
	sort          *search.Sort

	reversed   []int
	compIDXEnd int

	heads map[any]GroupHead[T]
	// headKeys records the order in which group values were first seen.
	// Java iterates heads.values() of a HashMap, whose order is unspecified but
	// stable for a given key set; Go randomises map iteration deliberately, so
	// the insertion order is kept to make RetrieveGroupHeads reproducible.
	headKeys []any

	context *index.LeafReaderContext
	scorer  search.Scorable

	newGroupHead func(doc int, value T, context *index.LeafReaderContext, scorer search.Scorable) (GroupHead[T], error)
}

// NewAllGroupHeadsCollector creates a new AllGroupHeadsCollector based on the
// type of within-group Sort required. selector defines the groups; sort is the
// within-group sort used to choose the group head document.
//
// Mirrors the static AllGroupHeadsCollector.newCollector(GroupSelector, Sort),
// which returns a ScoringGroupHeadsCollector when the sort equals
// Sort.RELEVANCE and a SortingGroupHeadsCollector otherwise.
func NewAllGroupHeadsCollector[T any](selector GroupSelector[T], sort *search.Sort) *AllGroupHeadsCollector[T] {
	c := newAllGroupHeadsCollector(selector, sort)
	if sort.Equals(search.RELEVANCE) {
		// ScoringGroupHeadsCollector.newGroupHead.
		c.newGroupHead = func(doc int, value T, context *index.LeafReaderContext, scorer search.Scorable) (GroupHead[T], error) {
			docBase := 0
			if context != nil {
				docBase = context.DocBase
			}
			return newScoringGroupHead(scorer, value, doc, docBase)
		}
		return c
	}
	// SortingGroupHeadsCollector.newGroupHead.
	c.newGroupHead = func(doc int, value T, context *index.LeafReaderContext, scorer search.Scorable) (GroupHead[T], error) {
		return newSortingGroupHead(sort, value, doc, context, scorer)
	}
	return c
}

// newAllGroupHeadsCollector mirrors the private
// AllGroupHeadsCollector(GroupSelector, Sort) constructor.
func newAllGroupHeadsCollector[T any](selector GroupSelector[T], sort *search.Sort) *AllGroupHeadsCollector[T] {
	sortFields := sort.GetSort()
	reversed := make([]int, len(sortFields))
	for i := range sortFields {
		if sortFields[i].GetReverse() {
			reversed[i] = -1
		} else {
			reversed[i] = 1
		}
	}
	c := &AllGroupHeadsCollector[T]{
		groupSelector: selector,
		sort:          sort,
		reversed:      reversed,
		compIDXEnd:    len(reversed) - 1,
		heads:         make(map[any]GroupHead[T]),
	}
	c.BaseSimpleCollector.Outer = c
	return c
}

// RetrieveGroupHeadsBitSet returns a FixedBitSet containing all group heads.
// maxDoc is the maxDoc of the top level IndexReader.
//
// Mirrors retrieveGroupHeads(int); Go cannot overload, so the bit-set form
// carries the longer name.
func (c *AllGroupHeadsCollector[T]) RetrieveGroupHeadsBitSet(maxDoc int) (*util.FixedBitSet, error) {
	bitSet, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, err
	}
	for _, groupHead := range c.GetCollectedGroupHeads() {
		bitSet.Set(groupHead.Doc())
	}
	return bitSet, nil
}

// RetrieveGroupHeads returns an int slice containing all group heads. Its
// length is equal to the number of collected unique groups.
//
// Mirrors retrieveGroupHeads().
func (c *AllGroupHeadsCollector[T]) RetrieveGroupHeads() []int {
	groupHeads := c.GetCollectedGroupHeads()
	docHeads := make([]int, len(groupHeads))
	for i, groupHead := range groupHeads {
		docHeads[i] = groupHead.Doc()
	}
	return docHeads
}

// GroupHeadsSize returns the number of group heads found for a query.
//
// Mirrors groupHeadsSize().
func (c *AllGroupHeadsCollector[T]) GroupHeadsSize() int {
	return len(c.GetCollectedGroupHeads())
}

// GetCollectedGroupHeads returns the collected group heads. Subsequent calls
// return the same group heads.
//
// Mirrors the protected getCollectedGroupHeads(), whose body is heads.values().
func (c *AllGroupHeadsCollector[T]) GetCollectedGroupHeads() []GroupHead[T] {
	groupHeads := make([]GroupHead[T], 0, len(c.headKeys))
	for _, key := range c.headKeys {
		groupHeads = append(groupHeads, c.heads[key])
	}
	return groupHeads
}

// Collect records doc as the group head of its group when it is more relevant
// than the group's current head.
//
// Mirrors collect(int).
func (c *AllGroupHeadsCollector[T]) Collect(doc int) error {
	if _, err := c.groupSelector.AdvanceTo(doc); err != nil {
		return err
	}
	groupValue, err := c.groupSelector.CurrentValue()
	if err != nil {
		return err
	}
	key := getComparableKey(groupValue)
	groupHead, ok := c.heads[key]
	if !ok {
		groupValue, err = c.groupSelector.CopyValue()
		if err != nil {
			return err
		}
		key = getComparableKey(groupValue)
		head, err := c.newGroupHead(doc, groupValue, c.context, c.scorer)
		if err != nil {
			return err
		}
		if _, exists := c.heads[key]; !exists {
			c.headKeys = append(c.headKeys, key)
		}
		c.heads[key] = head
		return nil
	}

	// Ok now we need to check if the current doc is more relevant than top doc
	// for this group.
	for compIDX := 0; ; compIDX++ {
		cmp, err := groupHead.Compare(compIDX, doc)
		if err != nil {
			return err
		}
		cmp *= c.reversed[compIDX]
		if cmp < 0 {
			// Definitely not competitive. So don't even bother to continue.
			return nil
		} else if cmp > 0 {
			// Definitely competitive.
			break
		} else if compIDX == c.compIDXEnd {
			// Here cmp = 0. If we're at the last comparator, this doc is not
			// competitive, since docs are visited in doc Id order, which means
			// this doc cannot compete with any other document in the queue.
			return nil
		}
	}
	return groupHead.UpdateDocHead(doc)
}

// ScoreMode mirrors scoreMode().
func (c *AllGroupHeadsCollector[T]) ScoreMode() search.ScoreMode {
	if c.sort.NeedsScores() {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

// DoSetNextReader mirrors doSetNextReader(LeafReaderContext).
func (c *AllGroupHeadsCollector[T]) DoSetNextReader(context *index.LeafReaderContext) error {
	if err := c.groupSelector.SetNextReader(context); err != nil {
		return err
	}
	c.context = context
	for _, head := range c.GetCollectedGroupHeads() {
		if err := head.SetNextReader(context); err != nil {
			return err
		}
	}
	return nil
}

// SetScorer mirrors setScorer(Scorable).
func (c *AllGroupHeadsCollector[T]) SetScorer(scorer search.Scorable) error {
	c.scorer = scorer
	for _, head := range c.GetCollectedGroupHeads() {
		if err := head.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// CollectRange collects a range of doc IDs.
//
// Mirrors the LeafCollector default collectRange(int, int), whose body is
// collect(new RangeDocIdStream(min, max)); the free function carries the
// concrete collector because Go embedding cannot dispatch back to it.
func (c *AllGroupHeadsCollector[T]) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream bulk-collects doc IDs from a DocIdStream.
//
// Mirrors the LeafCollector default collect(DocIdStream), whose body is
// stream.forEach(this::collect).
func (c *AllGroupHeadsCollector[T]) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// CompetitiveIterator returns nil: AllGroupHeadsCollector does not override the
// LeafCollector default, which returns null.
func (c *AllGroupHeadsCollector[T]) CompetitiveIterator() (search.DocIdSetIterator, error) {
	return nil, nil
}

// Finish is empty: AllGroupHeadsCollector does not override the LeafCollector
// default, whose body is empty.
func (c *AllGroupHeadsCollector[T]) Finish() error { return nil }

var _ search.SimpleCollector = (*AllGroupHeadsCollector[any])(nil)

// GroupHead represents a group head: the most relevant document for a
// particular group, whose relevancy is usually based on the sort. The group
// head contains a group value with its associated most relevant document id.
//
// Mirrors the public abstract static nested class
// AllGroupHeadsCollector.GroupHead<T>. Java's two public fields, groupValue
// and doc, are rendered as the accessors GroupValue and Doc, because a Go
// interface cannot declare fields; [BaseGroupHead] holds the state and
// supplies them.
type GroupHead[T any] interface {
	// GroupValue returns the group value this head belongs to.
	GroupValue() T

	// Doc returns the top-level document id of the group head.
	Doc() int

	// SetNextReader is called for each segment.
	SetNextReader(ctx *index.LeafReaderContext) error

	// SetScorer is called for each segment.
	SetScorer(scorer search.Scorable) error

	// Compare compares the specified document for a specified comparator
	// against the current most relevant document. It returns -1 if the
	// specified document was not competitive against the current most relevant
	// document, 1 if it was competitive, and otherwise 0.
	Compare(compIDX, doc int) (int, error)

	// UpdateDocHead updates the current most relevant document with doc.
	UpdateDocHead(doc int) error

	// GetSortValues returns the sort values for this group head, or nil if not
	// stored.
	GetSortValues() []any

	// GetComparators returns the field comparators used to determine the group
	// head ordering, one per sort field.
	GetComparators() []search.FieldComparator
}

// BaseGroupHead carries the state and the two concrete members of
// AllGroupHeadsCollector.GroupHead: the groupValue/doc/docBase fields and
// setNextReader.
type BaseGroupHead[T any] struct {
	groupValue T
	doc        int
	docBase    int
}

// newBaseGroupHead mirrors the protected GroupHead(T, int, int) constructor,
// which stores doc + docBase.
func newBaseGroupHead[T any](groupValue T, doc, docBase int) BaseGroupHead[T] {
	return BaseGroupHead[T]{groupValue: groupValue, doc: doc + docBase, docBase: docBase}
}

// GroupValue returns the group value of this head.
func (g *BaseGroupHead[T]) GroupValue() T { return g.groupValue }

// Doc returns the top-level document id of this head.
func (g *BaseGroupHead[T]) Doc() int { return g.doc }

// SetNextReader records the segment's docBase.
//
// Mirrors the protected GroupHead.setNextReader(LeafReaderContext).
func (g *BaseGroupHead[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	if ctx != nil {
		g.docBase = ctx.DocBase
	}
	return nil
}

// sortingGroupHead renders the private static nested class
// AllGroupHeadsCollector.SortingGroupHead<T>: the general implementation, which
// uses a FieldComparator per sort field to select the group head.
type sortingGroupHead[T any] struct {
	BaseGroupHead[T]

	comparators     []search.FieldComparator
	leafComparators []search.LeafFieldComparator
	sortValues      []any
}

// newSortingGroupHead mirrors the SortingGroupHead(Sort, T, int,
// LeafReaderContext, Scorable) constructor.
func newSortingGroupHead[T any](sort *search.Sort, groupValue T, doc int, context *index.LeafReaderContext, scorer search.Scorable) (GroupHead[T], error) {
	docBase := 0
	if context != nil {
		docBase = context.DocBase
	}
	sortFields := sort.GetSort()
	head := &sortingGroupHead[T]{
		BaseGroupHead:   newBaseGroupHead(groupValue, doc, docBase),
		comparators:     make([]search.FieldComparator, len(sortFields)),
		leafComparators: make([]search.LeafFieldComparator, len(sortFields)),
		sortValues:      make([]any, len(sortFields)),
	}
	for i := range sortFields {
		head.comparators[i] = search.SortFieldGetComparator(sortFields[i], 1, search.PruningNone)
		leaf, err := head.comparators[i].GetLeafComparator(context)
		if err != nil {
			return nil, err
		}
		head.leafComparators[i] = leaf
		if err := leaf.SetScorer(scorer); err != nil {
			return nil, err
		}
		if err := leaf.Copy(0, doc); err != nil {
			return nil, err
		}
		if err := leaf.SetBottom(0); err != nil {
			return nil, err
		}
		head.sortValues[i] = head.comparators[i].Value(0)
	}
	return head, nil
}

// SetNextReader rebinds every comparator to the new segment.
//
// Mirrors SortingGroupHead.setNextReader(LeafReaderContext).
func (g *sortingGroupHead[T]) SetNextReader(ctx *index.LeafReaderContext) error {
	if err := g.BaseGroupHead.SetNextReader(ctx); err != nil {
		return err
	}
	for i := range g.comparators {
		leaf, err := g.comparators[i].GetLeafComparator(ctx)
		if err != nil {
			return err
		}
		g.leafComparators[i] = leaf
	}
	return nil
}

// SetScorer mirrors SortingGroupHead.setScorer(Scorable).
func (g *sortingGroupHead[T]) SetScorer(scorer search.Scorable) error {
	for _, c := range g.leafComparators {
		if err := c.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// Compare mirrors SortingGroupHead.compare(int, int).
func (g *sortingGroupHead[T]) Compare(compIDX, doc int) (int, error) {
	return g.leafComparators[compIDX].CompareBottom(doc)
}

// UpdateDocHead mirrors SortingGroupHead.updateDocHead(int).
func (g *sortingGroupHead[T]) UpdateDocHead(doc int) error {
	for i := range g.leafComparators {
		if err := g.leafComparators[i].Copy(0, doc); err != nil {
			return err
		}
		if err := g.leafComparators[i].SetBottom(0); err != nil {
			return err
		}
		g.sortValues[i] = g.comparators[i].Value(0)
	}
	g.doc = doc + g.docBase
	return nil
}

// GetSortValues mirrors SortingGroupHead.getSortValues().
func (g *sortingGroupHead[T]) GetSortValues() []any { return g.sortValues }

// GetComparators mirrors SortingGroupHead.getComparators().
func (g *sortingGroupHead[T]) GetComparators() []search.FieldComparator { return g.comparators }

// scoringGroupHead renders the private static nested class
// AllGroupHeadsCollector.ScoringGroupHead<T>: the specialized implementation
// for sorting by score.
type scoringGroupHead[T any] struct {
	BaseGroupHead[T]

	scorer     search.Scorable
	topScore   float32
	sortValues []any
}

// newScoringGroupHead mirrors the ScoringGroupHead(Scorable, T, int, int)
// constructor, which declares `throws IOException` because it reads
// scorer.score().
func newScoringGroupHead[T any](scorer search.Scorable, groupValue T, doc, docBase int) (GroupHead[T], error) {
	topScore, err := scorer.Score()
	if err != nil {
		return nil, err
	}
	return &scoringGroupHead[T]{
		BaseGroupHead: newBaseGroupHead(groupValue, doc, docBase),
		scorer:        scorer,
		topScore:      topScore,
		sortValues:    []any{topScore},
	}, nil
}

// SetScorer mirrors ScoringGroupHead.setScorer(Scorable).
func (g *scoringGroupHead[T]) SetScorer(scorer search.Scorable) error {
	g.scorer = scorer
	return nil
}

// Compare mirrors ScoringGroupHead.compare(int, int), which asserts compIDX is
// 0 and promotes the top score when the current document scores higher.
func (g *scoringGroupHead[T]) Compare(compIDX, doc int) (int, error) {
	score, err := g.scorer.Score()
	if err != nil {
		return 0, err
	}
	c := compareFloat32(score, g.topScore)
	if c > 0 {
		g.topScore = score
	}
	return c, nil
}

// UpdateDocHead mirrors ScoringGroupHead.updateDocHead(int).
func (g *scoringGroupHead[T]) UpdateDocHead(doc int) error {
	g.doc = doc + g.docBase
	g.sortValues[0] = g.topScore
	return nil
}

// GetSortValues mirrors ScoringGroupHead.getSortValues().
func (g *scoringGroupHead[T]) GetSortValues() []any { return g.sortValues }

// GetComparators mirrors ScoringGroupHead.getComparators(), which returns null.
func (g *scoringGroupHead[T]) GetComparators() []search.FieldComparator { return nil }

// compareFloat32 mirrors java.lang.Float.compare: -0.0 sorts before 0.0 and
// NaN is the greatest value.
func compareFloat32(a, b float32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	ab := int32(math.Float32bits(a))
	bb := int32(math.Float32bits(b))
	switch {
	case ab < bb:
		return -1
	case ab > bb:
		return 1
	default:
		return 0
	}
}
