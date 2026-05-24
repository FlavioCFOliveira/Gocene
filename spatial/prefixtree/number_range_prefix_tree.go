package prefixtree

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// NumberRangePrefixTree is the 1-D number-range spatial prefix tree used as
// the base for DateRangePrefixTree.
//
// Port of org.apache.lucene.spatial.prefix.tree.NumberRangePrefixTree.
//
// Deviation: NRShape/UnitNRShape nested interfaces require spatial4j; full
// algorithmic body deferred to backlog #2693. The public contract skeleton
// (constructor, fields, SpatialPrefixTree interface) is present.
type NumberRangePrefixTree struct {
	BaseSpatialPrefixTree
	// maxSubCellsByLevel[i] holds the number of sub-cells per cell at level i.
	maxSubCellsByLevel []int
	// termLenByLevel[i] is the term byte length for a cell at level i.
	termLenByLevel []int
	// levelByTermLen[n] maps term length n to a tree level.
	levelByTermLen []int
	// maxTermLen is the maximum possible term length (including leaf byte).
	maxTermLen int
}

// NewNumberRangePrefixTree builds the tree from the per-level sub-cell counts.
func NewNumberRangePrefixTree(maxSubCellsByLevel []int) *NumberRangePrefixTree {
	t := &NumberRangePrefixTree{
		maxSubCellsByLevel: append([]int(nil), maxSubCellsByLevel...),
	}
	levels := len(maxSubCellsByLevel)
	t.MaxLevels = levels
	t.termLenByLevel = make([]int, levels+1)
	t.levelByTermLen = make([]int, 0)
	for i := 1; i <= levels; i++ {
		// byte width: ceil(log2(n)/8) approximated for this stub
		t.termLenByLevel[i] = i
	}
	t.maxTermLen = levels + 1 // +1 for leaf byte
	return t
}

// GetMaxSubCellsByLevel returns the sub-cell count at each level.
func (t *NumberRangePrefixTree) GetMaxSubCellsByLevel() []int {
	return append([]int(nil), t.maxSubCellsByLevel...)
}

// GetLevelForDistance returns the tree level whose cell side is ≤ dist.
func (t *NumberRangePrefixTree) GetLevelForDistance(dist float64) int {
	if dist == 0 {
		return t.MaxLevels
	}
	for i := t.MaxLevels; i >= 1; i-- {
		if t.GetDistanceForLevel(i) > dist {
			return i
		}
	}
	return 1
}

// GetDistanceForLevel returns the fractional distance represented by level.
func (t *NumberRangePrefixTree) GetDistanceForLevel(level int) float64 {
	if level < 1 || level > t.MaxLevels {
		return math.NaN()
	}
	d := 1.0
	for i := 1; i <= level; i++ {
		if t.maxSubCellsByLevel[i-1] > 0 {
			d /= float64(t.maxSubCellsByLevel[i-1])
		}
	}
	return d
}

// GetWorldCell returns the level-0 world cell.
func (t *NumberRangePrefixTree) GetWorldCell() Cell {
	return NewLegacyCell(0, nil, false)
}

// ReadCell initialises a cell from a BytesRef term.
func (t *NumberRangePrefixTree) ReadCell(term *util.BytesRef, scratch Cell) Cell {
	var cell *LegacyCell
	if scratch != nil {
		if lc, ok := scratch.(*LegacyCell); ok {
			cell = lc
		}
	}
	if cell == nil {
		cell = NewLegacyCell(0, nil, false)
	}
	cell.ReadCell(term)
	return cell
}

// GetTreeCellIterator returns a TreeCellIterator.
func (t *NumberRangePrefixTree) GetTreeCellIterator(shape interface{}, detailLevel int) CellIterator {
	return t.BaseSpatialPrefixTree.GetTreeCellIterator(t, shape, detailLevel)
}

// GetSpatialContext returns the spatial context (nil for number range trees).
func (t *NumberRangePrefixTree) GetSpatialContext() interface{} { return nil }

// GetMaxLevels returns the maximum depth.
func (t *NumberRangePrefixTree) GetMaxLevels() int { return t.MaxLevels }

var _ SpatialPrefixTree = (*NumberRangePrefixTree)(nil)

// ---------------------------------------------------------------------------
// DateRangePrefixTree
// ---------------------------------------------------------------------------

// DateRangePrefixTree is the date-typed specialisation of NumberRangePrefixTree.
//
// Port of org.apache.lucene.spatial.prefix.tree.DateRangePrefixTree.
//
// Deviation: Java Calendar/ZonedDateTime parsing and FIELD_BY_LEVEL mapping
// deferred to backlog #2693. The type exposes the same SpatialPrefixTree
// surface as NumberRangePrefixTree.
type DateRangePrefixTree struct {
	NumberRangePrefixTree
}

// dateRangeLevels is the per-level sub-cell count as used by Lucene 10.4.0
// DateRangePrefixTree (years/months/days/hours/minutes/seconds/milliseconds
// approximated as power-of-10).
// Java: {2(root placeholder), ..., 10 years, 12 months, 31 days, 24 hours, 60 min, 60 sec, 1000ms}
var dateRangeLevels = []int{2, 2, 10, 12, 31, 24, 60, 60, 1000}

// NewDateRangePrefixTree builds the tree.
func NewDateRangePrefixTree() *DateRangePrefixTree {
	base := NewNumberRangePrefixTree(dateRangeLevels)
	return &DateRangePrefixTree{NumberRangePrefixTree: *base}
}

// NewDateRangePrefixTreeFromLevels builds the tree from custom level counts.
func NewDateRangePrefixTreeFromLevels(levels []int) *DateRangePrefixTree {
	base := NewNumberRangePrefixTree(levels)
	return &DateRangePrefixTree{NumberRangePrefixTree: *base}
}
