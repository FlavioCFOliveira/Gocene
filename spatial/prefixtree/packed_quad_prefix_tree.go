package prefixtree

import "github.com/FlavioCFOliveira/Gocene/util"

// PackedQuadPrefixTreeMaxLevelsPossible is the hard maximum depth for
// PackedQuadPrefixTree (29 quad-cell levels in a 64-bit long).
const PackedQuadPrefixTreeMaxLevelsPossible = 29

// PackedQuadPrefixTree is the bit-packed quad spatial prefix tree.
//
// Port of org.apache.lucene.spatial.prefix.tree.PackedQuadPrefixTree.
//
// Deviation: QuadPrefixTree parent is not yet ported; full bit-packing and
// shape traversal deferred to backlog #2693.
type PackedQuadPrefixTree struct {
	BaseSpatialPrefixTree
	leafyPrune bool
}

// NewPackedQuadPrefixTree builds the tree.
func NewPackedQuadPrefixTree(ctx interface{}, maxLevels int) *PackedQuadPrefixTree {
	if maxLevels < 1 {
		maxLevels = 1
	}
	if maxLevels > PackedQuadPrefixTreeMaxLevelsPossible {
		maxLevels = PackedQuadPrefixTreeMaxLevelsPossible
	}
	t := &PackedQuadPrefixTree{leafyPrune: true}
	t.MaxLevels = maxLevels
	t.Ctx = ctx
	return t
}

// SetLeafyPrune enables or disables pruning of leafy branches at detail level.
func (t *PackedQuadPrefixTree) SetLeafyPrune(v bool) { t.leafyPrune = v }

// IsLeafyPrune returns whether leafy-branch pruning is active.
func (t *PackedQuadPrefixTree) IsLeafyPrune() bool { return t.leafyPrune }

// GetLevelForDistance returns the tree level whose cell side is ≤ dist.
// Returns MaxLevels — deferred to #2693.
func (t *PackedQuadPrefixTree) GetLevelForDistance(_ float64) int { return t.MaxLevels }

// GetDistanceForLevel returns 0 — deferred to #2693.
func (t *PackedQuadPrefixTree) GetDistanceForLevel(_ int) float64 { return 0 }

// GetWorldCell returns the level-0 world cell.
func (t *PackedQuadPrefixTree) GetWorldCell() Cell {
	return NewLegacyCell(0, nil, false)
}

// ReadCell initialises a cell from a BytesRef term.
func (t *PackedQuadPrefixTree) ReadCell(term *util.BytesRef, scratch Cell) Cell {
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
func (t *PackedQuadPrefixTree) GetTreeCellIterator(shape interface{}, detailLevel int) CellIterator {
	return t.BaseSpatialPrefixTree.GetTreeCellIterator(t, shape, detailLevel)
}

// GetSpatialContext returns the spatial context.
func (t *PackedQuadPrefixTree) GetSpatialContext() interface{} { return t.Ctx }

// GetMaxLevels returns the maximum depth.
func (t *PackedQuadPrefixTree) GetMaxLevels() int { return t.MaxLevels }

var _ SpatialPrefixTree = (*PackedQuadPrefixTree)(nil)
