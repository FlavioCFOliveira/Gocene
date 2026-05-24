package prefixtree

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// LegacyPrefixTree is the abstract base for the original Geohash and Quad
// spatial prefix trees.
//
// Port of org.apache.lucene.spatial.prefix.tree.LegacyPrefixTree.
//
// Deviation: SpatialContext geometry (getCell, getShape, getBoundingBox, etc.)
// deferred to backlog #2693. GetDistanceForLevel returns NaN until spatial4j
// geometry is available.
type LegacyPrefixTree struct {
	BaseSpatialPrefixTree
}

// GetDistanceForLevel returns the hypotenuse length for a cell at level.
//
// Requires spatial4j shape geometry; returns NaN until #2693 is resolved.
func (t *LegacyPrefixTree) GetDistanceForLevel(level int) float64 {
	if level < 1 || level > t.MaxLevels {
		panic("level out of range [1, maxLevels]")
	}
	// Full computation requires getCell(worldCenter, level).getShape().getBoundingBox();
	// deferred to #2693.
	return math.NaN()
}

// ReadCell initialises a cell from a BytesRef. Concrete subclasses that
// provide a LegacyCell implementation may override.
func (t *LegacyPrefixTree) ReadCell(term *util.BytesRef, scratch Cell) Cell {
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
