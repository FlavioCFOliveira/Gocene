package grouping

import (
	"fmt"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FirstPassGroupingCollector is the first of two passes necessary to collect grouped hits.
// This pass gathers the top N sorted groups.
type FirstPassGroupingCollector[T any] struct {
	search.BaseSimpleCollector
	groupSelector               GroupSelector[T]
	ignoreDocsWithoutGroupField bool
	comparators                  []search.FieldComparator
	leafComparators              []search.LeafFieldComparator
	reversed                    []int
	topNGroups                  int
	needsScores                 bool
	groupMap                    map[any]*CollectedSearchGroup[T]
	compIDXEnd                  int
	orderedGroups               []*CollectedSearchGroup[T]
	docBase                     int
	spareSlot                   int
}

func NewFirstPassGroupingCollector[T any](groupSelector GroupSelector[T], groupSort *search.Sort, topNGroups int, ignoreDocsWithoutGroupField bool) *FirstPassGroupingCollector[T] {
	if topNGroups < 1 {
		panic("topNGroups must be >= 1")
	}

	sortFields := groupSort.Fields
	comparators := make([]search.FieldComparator, len(sortFields))
	reversed := make([]int, len(sortFields))
	for i, sf := range sortFields {
		// use topNGroups + 1 so we have a spare slot to use for comparing:
		comparators[i] = sf.GetComparator(topNGroups+1, search.PruningNone)
		if sf.Reverse {
			reversed[i] = -1
		} else {
			reversed[i] = 1
		}
	}

	return &FirstPassGroupingCollector[T]{
		groupSelector:               groupSelector,
		ignoreDocsWithoutGroupField:   ignoreDocsWithoutGroupField,
		comparators:                  comparators,
		leafComparators:              make([]search.LeafFieldComparator, len(sortFields)),
		reversed:                    reversed,
		topNGroups:                  topNGroups,
		needsScores:                 groupSort.NeedsScores(),
		groupMap:                    make(map[any]*CollectedSearchGroup[T]),
		compIDXEnd:                  len(comparators) - 1,
		spareSlot:                   topNGroups,
	}
}

func (c *FirstPassGroupingCollector[T]) ScoreMode() search.ScoreMode {
	if c.needsScores {
		return search.ScoreModeComplete
	}
	return search.ScoreModeCompleteNoScores
}

func (c *FirstPassGroupingCollector[T]) GetTopGroups(groupOffset int) ([]SearchGroup[T], error) {
	if groupOffset < 0 {
		return nil, fmt.Errorf("groupOffset must be >= 0 (got %d)", groupOffset)
	}

	if len(c.groupMap) <= groupOffset {
		return nil, nil
	}

	if c.orderedGroups == nil {
		c.buildSortedSet()
	}

	if groupOffset >= len(c.orderedGroups) {
		return nil, nil
	}

	result := make([]SearchGroup[T], 0, len(c.orderedGroups)-groupOffset)
	for i := groupOffset; i < len(c.orderedGroups); i++ {
		group := c.orderedGroups[i]
		searchGroup := SearchGroup[T]{
			GroupValue: group.GroupValue,
			SortValues: make([]any, len(c.comparators)),
		}
		for sortFieldIDX := 0; sortFieldIDX < len(c.comparators); sortFieldIDX++ {
			searchGroup.SortValues[sortFieldIDX] = c.comparators[sortFieldIDX].Value(group.ComparatorSlot)
		}
		result = append(result, searchGroup)
	}
	return result, nil
}

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

func (c *FirstPassGroupingCollector[T]) isCompetitive(doc int) bool {
	if c.orderedGroups != nil {
		for compIDX := 0; ; compIDX++ {
			cmp := c.leafComparators[compIDX].CompareBottom(doc)
			res := c.reversed[compIDX] * cmp
			if res < 0 {
				return false
			} else if res > 0 {
				break
			} else if compIDX == c.compIDXEnd {
				return false
			}
		}
	}
	return true
}

func (c *FirstPassGroupingCollector[T]) Collect(doc int) error {
	if !c.isCompetitive(doc) {
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

	if c.ignoreDocsWithoutGroupField && state == StateSkip {
		return nil
	}

	group, ok := c.groupMap[any(groupValue)]
	if !ok {
		return c.collectNewGroup(doc)
	}
	return c.collectExistingGroup(doc, group)
}

func (c *FirstPassGroupingCollector[T]) collectNewGroup(doc int) error {
	if len(c.groupMap) < c.topNGroups {
		sg := &CollectedSearchGroup[T]{}
		val, err := c.groupSelector.CopyValue()
		if err != nil {
			return err
		}
		sg.GroupValue = &val
		sg.ComparatorSlot = len(c.groupMap)
		sg.TopDoc = c.docBase + doc
		for _, fc := range c.leafComparators {
			fc.Copy(sg.ComparatorSlot, doc)
		}
		c.groupMap[any(val)] = sg

		if len(c.groupMap) == c.topNGroups {
			c.buildSortedSet()
			lastComparatorSlot := c.orderedGroups[len(c.orderedGroups)-1].ComparatorSlot
			for _, fc := range c.leafComparators {
				fc.SetBottom(lastComparatorSlot)
			}
		}
		return nil
	}

	bottomGroup := c.orderedGroups[len(c.orderedGroups)-1]
	// Need to remove it from the sorted set and the map
	// But since we're using a slice, we just remove the last element
	c.orderedGroups = c.orderedGroups[:len(c.orderedGroups)-1]
	delete(c.groupMap, any(*bottomGroup.GroupValue))

	val, err := c.groupSelector.CopyValue()
	if err != nil {
		return err
	}
	bottomGroup.GroupValue = &val
	bottomGroup.TopDoc = c.docBase + doc
	for _, fc := range c.leafComparators {
		fc.Copy(bottomGroup.ComparatorSlot, doc)
	}
	c.groupMap[any(val)] = bottomGroup

	// We need to re-insert it into the sorted slice
	c.insertSorted(bottomGroup)

	lastComparatorSlot := c.orderedGroups[len(c.orderedGroups)-1].ComparatorSlot
	for _, fc := range c.leafComparators {
		fc.SetBottom(lastComparatorSlot)
	}
	return nil
}

func (c *FirstPassGroupingCollector[T]) collectExistingGroup(doc int, group *CollectedSearchGroup[T]) error {
	for compIDX := 0; ; compIDX++ {
		c.leafComparators[compIDX].Copy(c.spareSlot, doc)
		cmp := c.comparators[compIDX].Compare(group.ComparatorSlot, c.spareSlot)
		res := c.reversed[compIDX] * cmp
		if res < 0 {
			return nil
		} else if res > 0 {
			for compIDX2 := compIDX + 1; compIDX2 < len(c.comparators); compIDX2++ {
				c.leafComparators[compIDX2].Copy(c.spareSlot, doc)
			}
			break
		} else if compIDX == c.compIDXEnd {
			return nil
		}
	}

	var prevLast *CollectedSearchGroup[T]
	skipHeavyOps := false

	if c.orderedGroups != nil {
		prevLast = c.orderedGroups[len(c.orderedGroups)-1]
		if group == c.orderedGroups[0] {
			skipHeavyOps = true
		} else {
			c.removeSorted(group)
		}
	}

	group.TopDoc = c.docBase + doc
	tmp := c.spareSlot
	c.spareSlot = group.ComparatorSlot
	group.ComparatorSlot = tmp

	if c.orderedGroups != nil {
		if !skipHeavyOps {
			c.insertSorted(group)
		}
		newLast := c.orderedGroups[len(c.orderedGroups)-1]
		if group == newLast || prevLast != newLast {
			for _, fc := range c.leafComparators {
				fc.SetBottom(newLast.ComparatorSlot)
			}
		}
	}
	return nil
}

func (c *FirstPassGroupingCollector[T]) buildSortedSet() {
	c.orderedGroups = make([]*CollectedSearchGroup[T], 0, len(c.groupMap))
	for _, g := range c.groupMap {
		c.orderedGroups = append(c.orderedGroups, g)
	}

	sort.Slice(c.orderedGroups, func(i, j int) bool {
		o1 := c.orderedGroups[i]
		o2 := c.orderedGroups[j]
		for compIDX := 0; ; compIDX++ {
			cmp := c.comparators[compIDX].Compare(o1.ComparatorSlot, o2.ComparatorSlot)
			res := c.reversed[compIDX] * cmp
			if res != 0 {
				return res < 0
			} else if compIDX == c.compIDXEnd {
				return o1.TopDoc < o2.TopDoc
			}
		}
	})
}

func (c *FirstPassGroupingCollector[T]) insertSorted(group *CollectedSearchGroup[T]) {
	idx := sort.Search(len(c.orderedGroups), func(i int) bool {
		o2 := c.orderedGroups[i]
		for compIDX := 0; ; compIDX++ {
			cmp := c.comparators[compIDX].Compare(group.ComparatorSlot, o2.ComparatorSlot)
			res := c.reversed[compIDX] * cmp
			if res != 0 {
				return res < 0
			} else if compIDX == c.compIDXEnd {
				return group.TopDoc < o2.TopDoc
			}
		}
	})
	c.orderedGroups = append(c.orderedGroups, nil)
	copy(c.orderedGroups[idx+1:], c.orderedGroups[idx:])
	c.orderedGroups[idx] = group
}

func (c *FirstPassGroupingCollector[T]) removeSorted(group *CollectedSearchGroup[T]) {
	for i, g := range c.orderedGroups {
		if g == group {
			c.orderedGroups = append(c.orderedGroups[:i], c.orderedGroups[i+1:]...)
			return
		}
	}
}

func (c *FirstPassGroupingCollector[T]) DoSetNextReader(readerContext *index.LeafReaderContext) error {
	c.docBase = readerContext.docBase
	for i := 0; i < len(c.comparators); i++ {
		c.leafComparators[i] = c.comparators[i].GetLeafComparator(readerContext)
	}
	if err := c.groupSelector.SetNextReader(readerContext); err != nil {
		return err
	}
	return nil
}

func (c *FirstPassGroupingCollector[T]) Finish() error {
	return nil
}
