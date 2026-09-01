package grouping

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BlockGroupingCollectorManager is a CollectorManager implementation for BlockGroupingCollector.
type BlockGroupingCollectorManager struct {
	groupSort       *search.Sort
	topNGroups      int
	needsScores     bool
	lastDocPerGroup search.Weight
}

func NewBlockGroupingCollectorManager(groupSort *search.Sort, topNGroups int, needsScores bool, lastDocPerGroup search.Weight) *BlockGroupingCollectorManager {
	return &BlockGroupingCollectorManager{
		groupSort:       groupSort,
		topNGroups:      topNGroups,
		needsScores:     needsScores,
		lastDocPerGroup: lastDocPerGroup,
	}
}

func (m *BlockGroupingCollectorManager) NewCollector() (search.Collector, error) {
	return NewBlockGroupingCollector(m.groupSort, m.topNGroups, m.needsScores, m.lastDocPerGroup), nil
}

func (m *BlockGroupingCollectorManager) Reduce(collectors []search.Collector) (*TopGroups[any], error) {
	// Reduction of BlockGroupingCollector is handled by the collector's GetTopGroups method.
	// Since we are reducing multiple segments, we'll need to merge the TopGroups.
	var shardGroups []*TopGroups[any]
	for _, c := range collectors {
		if collector, ok := c.(*BlockGroupingCollector); ok {
			// We need withinGroupSort, groupOffset, withinGroupOffset, maxDocsPerGroup
			// In Java, these are passed to getTopGroups.
			// For now, we'll use default values or assume they are provided.
			groups := collector.GetTopGroups(search.Sort{}, 0, 0, 10)
			shardGroups = append(shardGroups, groups)
		}
	}

	return Merge(shardGroups, m.groupSort, search.Sort{}, 0, 10, 0), nil
}
