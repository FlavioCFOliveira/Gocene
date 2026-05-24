// Package prefixtree implements
// org.apache.lucene.spatial.prefix.tree: the spatial-prefix-tree primitives.
package prefixtree

import "github.com/FlavioCFOliveira/Gocene/util"

// SpatialPrefixTree is the abstract base for grid-based spatial index trees.
//
// Port of org.apache.lucene.spatial.prefix.tree.SpatialPrefixTree.
//
// Deviation: SpatialContext is interface{}; full spatial4j integration
// deferred to backlog #2693.
type SpatialPrefixTree interface {
	// GetSpatialContext returns the spatial context (interface{} = SpatialContext).
	GetSpatialContext() interface{}

	// GetMaxLevels returns the maximum depth of the tree.
	GetMaxLevels() int

	// GetLevelForDistance returns the grid level whose cell side is ≤ dist degrees.
	GetLevelForDistance(dist float64) int

	// GetDistanceForLevel returns the hypotenuse distance (degrees) for a cell at
	// the given level.
	GetDistanceForLevel(level int) float64

	// GetWorldCell returns the level-0 cell encompassing all data.
	GetWorldCell() Cell

	// ReadCell initialises (or re-initialises) a cell from a BytesRef term.
	ReadCell(term *util.BytesRef, scratch Cell) Cell

	// GetTreeCellIterator returns cells that intersect shape at up to detailLevel.
	GetTreeCellIterator(shape interface{}, detailLevel int) CellIterator
}

// BaseSpatialPrefixTree provides the shared maxLevels/ctx state and the
// default GetTreeCellIterator implementation.
type BaseSpatialPrefixTree struct {
	MaxLevels int
	Ctx       interface{} // spatial4j SpatialContext
}

// GetSpatialContext returns the spatial context.
func (t *BaseSpatialPrefixTree) GetSpatialContext() interface{} { return t.Ctx }

// GetMaxLevels returns the maximum depth of the tree.
func (t *BaseSpatialPrefixTree) GetMaxLevels() int { return t.MaxLevels }

// GetTreeCellIterator returns a TreeCellIterator rooted at the world cell.
// Implementations must have GetWorldCell() available.
func (t *BaseSpatialPrefixTree) GetTreeCellIterator(
	tree SpatialPrefixTree,
	shape interface{},
	detailLevel int,
) CellIterator {
	if detailLevel > t.MaxLevels {
		panic("detailLevel > maxLevels")
	}
	return NewTreeCellIterator(shape, detailLevel, tree.GetWorldCell())
}
