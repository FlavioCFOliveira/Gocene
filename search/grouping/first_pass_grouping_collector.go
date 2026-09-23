// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FirstPassGroupingCollector is the first of two passes necessary to collect
// grouped hits. This pass gathers the top N sorted groups. Groups are defined
// by a GroupSelector.
//
// See the package documentation for more details including a full code
// example.
//
// Mirrors org.apache.lucene.search.grouping.FirstPassGroupingCollector<T>,
// which extends SimpleCollector.
//
// lucene.experimental
type FirstPassGroupingCollector[T any] struct {
	search.BaseSimpleCollector
	search.BaseLeafCollector

	groupSelector               GroupSelector[T]
	ignoreDocsWithoutGroupField bool

	comparators     []search.FieldComparator
	leafComparators []search.LeafFieldComparator
	reversed        []int
	topNGroups      int
	needsScores     bool
	groupMap        *groupMap[T, *CollectedSearchGroup[T]]
	compIDXEnd      int

	// orderedGroups is set once we reach topNGroups unique groups. It renders
	// the protected field `TreeSet<CollectedSearchGroup<T>> orderedGroups`;
	// the Go rendering of java.util.TreeSet is unexported, so the field is
	// too.
	//
	// lucene.internal
	orderedGroups *treeSet[*CollectedSearchGroup[T]]

	docBase   int
	spareSlot int
}

// NewFirstPassGroupingCollector creates the first pass collector.
//
// groupSelector is a GroupSelector used to define groups. groupSort is the
// Sort used to sort the groups: the top sorted document within each group
// according to groupSort determines how that group sorts against other
// groups; it must be non-nil, i.e. if you want to groupSort by relevance use
// search.RELEVANCE. topNGroups says how many top groups to keep.
//
// Mirrors FirstPassGroupingCollector(GroupSelector, Sort, int).
func NewFirstPassGroupingCollector[T any](
	groupSelector GroupSelector[T],
	groupSort *search.Sort,
	topNGroups int,
) (*FirstPassGroupingCollector[T], error) {
	return NewFirstPassGroupingCollectorIgnoringDocsWithoutGroupField(
		groupSelector, groupSort, topNGroups, false)
}

// NewFirstPassGroupingCollectorIgnoringDocsWithoutGroupField creates the
// first pass collector with ignoreDocsWithoutGroupField: when true, ignore
// documents that don't have the group field instead of putting them in a null
// group.
//
// Mirrors FirstPassGroupingCollector(GroupSelector, Sort, int, boolean).
func NewFirstPassGroupingCollectorIgnoringDocsWithoutGroupField[T any](
	groupSelector GroupSelector[T],
	groupSort *search.Sort,
	topNGroups int,
	ignoreDocsWithoutGroupField bool,
) (*FirstPassGroupingCollector[T], error) {
	c := &FirstPassGroupingCollector[T]{
		groupSelector:               groupSelector,
		ignoreDocsWithoutGroupField: ignoreDocsWithoutGroupField,
	}
	c.Outer = c
	if topNGroups < 1 {
		return nil, fmt.Errorf("topNGroups must be >= 1 (got %d)", topNGroups)
	}

	// TODO: allow null groupSort to mean "by relevance",
	// and specialize it?

	c.topNGroups = topNGroups
	c.needsScores = groupSort.NeedsScores()
	sortFields := groupSort.GetSort()
	c.comparators = make([]search.FieldComparator, len(sortFields))
	c.leafComparators = make([]search.LeafFieldComparator, len(sortFields))
	c.compIDXEnd = len(c.comparators) - 1
	c.reversed = make([]int, len(sortFields))
	for i := 0; i < len(sortFields); i++ {
		sortField := sortFields[i]

		// use topNGroups + 1 so we have a spare slot to use for comparing (tracked by
		// this.spareSlot):
		c.comparators[i] = search.SortFieldGetComparator(sortField, topNGroups+1, search.PruningNone)
		if sortField.GetReverse() {
			c.reversed[i] = -1
		} else {
			c.reversed[i] = 1
		}
	}

	c.spareSlot = topNGroups
	c.groupMap = newGroupMapWithSize[T, *CollectedSearchGroup[T]](topNGroups)
	return c, nil
}

// ScoreMode mirrors FirstPassGroupingCollector.scoreMode().
func (c *FirstPassGroupingCollector[T]) ScoreMode() search.ScoreMode {
	if c.needsScores {
		return search.COMPLETE
	}
	return search.COMPLETE_NO_SCORES
}

// GetTopGroups returns top groups, starting from offset. This may return nil,
// if no groups were collected, or if the number of unique groups collected is
// <= offset.
//
// Mirrors Collection<SearchGroup<T>> getTopGroups(int groupOffset).
func (c *FirstPassGroupingCollector[T]) GetTopGroups(groupOffset int) ([]*SearchGroup[T], error) {

	if groupOffset < 0 {
		return nil, fmt.Errorf("groupOffset must be >= 0 (got %d)", groupOffset)
	}

	if c.groupMap.size() <= groupOffset {
		return nil, nil
	}

	if c.orderedGroups == nil {
		c.buildSortedSet()
	}

	result := make([]*SearchGroup[T], 0, c.orderedGroups.size()-groupOffset)
	upto := 0
	sortFieldCount := len(c.comparators)
	for _, group := range c.orderedGroups.values() {
		if upto < groupOffset {
			upto++
			continue
		}
		upto++
		searchGroup := &SearchGroup[T]{}
		searchGroup.GroupValue = group.GroupValue
		searchGroup.SortValues = make([]any, sortFieldCount)
		for sortFieldIDX := 0; sortFieldIDX < sortFieldCount; sortFieldIDX++ {
			searchGroup.SortValues[sortFieldIDX] = c.comparators[sortFieldIDX].Value(group.comparatorSlot)
		}
		result = append(result, searchGroup)
	}
	return result, nil
}

// SetScorer mirrors FirstPassGroupingCollector.setScorer(Scorable).
func (c *FirstPassGroupingCollector[T]) SetScorer(scorer search.Scorable) error {
	if err := c.groupSelector.SetScorer(scorer); err != nil {
		return err
	}
	for _, comparator := range c.leafComparators {
		if err := comparator.SetScorer(scorer); err != nil {
			return err
		}
	}
	return nil
}

// isCompetitive mirrors the private boolean isCompetitive(int doc).
func (c *FirstPassGroupingCollector[T]) isCompetitive(doc int) (bool, error) {
	// If orderedGroups != null we already have collected N groups and
	// can short circuit by comparing this document to the bottom group,
	// without having to find what group this document belongs to.

	// Even if this document belongs to a group in the top N, we'll know that
	// we don't have to update that group.

	// Downside: if the number of unique groups is very low, this is
	// wasted effort as we will most likely be updating an existing group.
	if c.orderedGroups != nil {
		for compIDX := 0; ; compIDX++ {
			bottom, err := c.leafComparators[compIDX].CompareBottom(doc)
			if err != nil {
				return false, err
			}
			cmp := c.reversed[compIDX] * bottom
			if cmp < 0 {
				// Definitely not competitive. So don't even bother to continue
				return false, nil
			} else if cmp > 0 {
				// Definitely competitive.
				break
			} else if compIDX == c.compIDXEnd {
				// Here c=0. If we're at the last comparator, this doc is not
				// competitive, since docs are visited in doc Id order, which means
				// this doc cannot compete with any other document in the queue.
				return false, nil
			}
		}
	}
	return true, nil
}

// Collect mirrors FirstPassGroupingCollector.collect(int).
func (c *FirstPassGroupingCollector[T]) Collect(doc int) error {

	competitive, err := c.isCompetitive(doc)
	if err != nil {
		return err
	}
	if !competitive {
		return nil
	}

	state, err := c.groupSelector.AdvanceTo(doc)
	if err != nil {
		return err
	}
	groupValue, err := c.groupSelector.CurrentValue()
	if err != nil {
		return err
	}

	// Skip documents without group field if option is enabled
	if c.ignoreDocsWithoutGroupField && state == GroupSelectorStateSkip {
		return nil
	}

	group, found := c.groupMap.get(groupValue)

	if !found {
		return c.collectNewGroup(doc)
	}
	return c.collectExistingGroup(doc, group)
}

// CollectRange mirrors the default body of LeafCollector.collectRange(int, int).
func (c *FirstPassGroupingCollector[T]) CollectRange(min, max int) error {
	return search.DefaultCollectRange(c, min, max)
}

// CollectStream mirrors the default body of LeafCollector.collect(DocIdStream).
func (c *FirstPassGroupingCollector[T]) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(c, stream)
}

// collectNewGroup mirrors the private void collectNewGroup(int doc).
func (c *FirstPassGroupingCollector[T]) collectNewGroup(doc int) error {
	// First time we are seeing this group, or, we've seen
	// it before but it fell out of the top N and is now
	// coming back

	if !c.isGroupMapFull() {

		// Still in startup transient: we have not
		// seen enough unique groups to start pruning them;
		// just keep collecting them

		// Add a new CollectedSearchGroup:
		sg := &CollectedSearchGroup[T]{}
		value, err := c.groupSelector.CopyValue()
		if err != nil {
			return err
		}
		sg.GroupValue = value
		sg.comparatorSlot = c.groupMap.size()
		sg.topDoc = c.docBase + doc
		for _, fc := range c.leafComparators {
			if err := fc.Copy(sg.comparatorSlot, doc); err != nil {
				return err
			}
		}
		c.groupMap.put(sg.GroupValue, sg)

		if c.isGroupMapFull() {
			// End of startup transient: we now have max
			// number of groups; from here on we will drop
			// bottom group when we insert new one:
			c.buildSortedSet()
			lastComparatorSlot := c.orderedGroups.last().comparatorSlot
			for _, fc := range c.leafComparators {
				if err := fc.SetBottom(lastComparatorSlot); err != nil {
					return err
				}
			}
		}

		return nil
	}

	// We already tested that the document is competitive, so replace
	// the bottom group with this new group.

	bottomGroup, _ := c.orderedGroups.pollLast()

	c.groupMap.remove(bottomGroup.GroupValue)

	// reuse the removed CollectedSearchGroup
	value, err := c.groupSelector.CopyValue()
	if err != nil {
		return err
	}
	bottomGroup.GroupValue = value
	bottomGroup.topDoc = c.docBase + doc

	for _, fc := range c.leafComparators {
		if err := fc.Copy(bottomGroup.comparatorSlot, doc); err != nil {
			return err
		}
	}

	c.groupMap.put(bottomGroup.GroupValue, bottomGroup)
	c.orderedGroups.add(bottomGroup)

	lastComparatorSlot := c.orderedGroups.last().comparatorSlot
	for _, fc := range c.leafComparators {
		if err := fc.SetBottom(lastComparatorSlot); err != nil {
			return err
		}
	}
	return nil
}

// collectExistingGroup mirrors the private void collectExistingGroup(int,
// CollectedSearchGroup).
func (c *FirstPassGroupingCollector[T]) collectExistingGroup(doc int, group *CollectedSearchGroup[T]) error {
	// Update existing group:
	for compIDX := 0; ; compIDX++ {
		if err := c.leafComparators[compIDX].Copy(c.spareSlot, doc); err != nil {
			return err
		}

		cmp := c.reversed[compIDX] * c.comparators[compIDX].Compare(group.comparatorSlot, c.spareSlot)
		if cmp < 0 {
			// Definitely not competitive.
			return nil
		} else if cmp > 0 {
			// Definitely competitive; set remaining comparators:
			for compIDX2 := compIDX + 1; compIDX2 < len(c.comparators); compIDX2++ {
				if err := c.leafComparators[compIDX2].Copy(c.spareSlot, doc); err != nil {
					return err
				}
			}
			break
		} else if compIDX == c.compIDXEnd {
			// Here c=0. If we're at the last comparator, this doc is not
			// competitive, since docs are visited in doc Id order, which means
			// this doc cannot compete with any other document in the queue.
			return nil
		}
	}

	var prevLast *CollectedSearchGroup[T]
	skipHeavyOps := false

	if c.orderedGroups != nil {
		prevLast = c.orderedGroups.last()

		// Skip remove/add for first group
		if group == c.orderedGroups.first() {
			skipHeavyOps = true
		} else {
			c.orderedGroups.remove(group)
		}
	} else {
		prevLast = nil
	}

	group.topDoc = c.docBase + doc

	// Swap slots
	tmp := c.spareSlot
	c.spareSlot = group.comparatorSlot
	group.comparatorSlot = tmp

	// Re-add only if we removed it
	if c.orderedGroups != nil {
		if !skipHeavyOps {
			c.orderedGroups.add(group)
		}
		newLast := c.orderedGroups.last()
		// If we changed the value of the last group, or changed which group was last, then update
		// bottom:
		if group == newLast || prevLast != newLast {
			for _, fc := range c.leafComparators {
				if err := fc.SetBottom(newLast.comparatorSlot); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// buildSortedSet mirrors the private void buildSortedSet().
func (c *FirstPassGroupingCollector[T]) buildSortedSet() {
	comparator := func(o1, o2 *CollectedSearchGroup[T]) int {
		for compIDX := 0; ; compIDX++ {
			fc := c.comparators[compIDX]
			cmp := c.reversed[compIDX] * fc.Compare(o1.comparatorSlot, o2.comparatorSlot)
			if cmp != 0 {
				return cmp
			} else if compIDX == c.compIDXEnd {
				return o1.topDoc - o2.topDoc
			}
		}
	}

	c.orderedGroups = newTreeSet(comparator)
	c.orderedGroups.addAll(c.groupMap.values())
}

// DoSetNextReader mirrors
// FirstPassGroupingCollector.doSetNextReader(LeafReaderContext).
func (c *FirstPassGroupingCollector[T]) DoSetNextReader(readerContext *index.LeafReaderContext) error {
	c.docBase = readerContext.DocBase
	for i := 0; i < len(c.comparators); i++ {
		leaf, err := c.comparators[i].GetLeafComparator(readerContext)
		if err != nil {
			return err
		}
		c.leafComparators[i] = leaf
	}
	return c.groupSelector.SetNextReader(readerContext)
}

// GetGroupSelector returns the GroupSelector used for this Collector.
func (c *FirstPassGroupingCollector[T]) GetGroupSelector() GroupSelector[T] {
	return c.groupSelector
}

// isGroupMapFull mirrors the private boolean isGroupMapFull().
func (c *FirstPassGroupingCollector[T]) isGroupMapFull() bool {
	return c.groupMap.size() >= c.topNGroups
}
