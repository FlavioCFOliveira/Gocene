package prefixtree

import "github.com/FlavioCFOliveira/Gocene/util"

// S2ShapeFactory is the contract that creates S2 shapes for the tree.
// Mirrors org.apache.lucene.spatial.prefix.tree.S2ShapeFactory.
type S2ShapeFactory interface {
	NewCircle(lat, lon, radius float64) interface{}
	NewRectangle(minX, minY, maxX, maxY float64) interface{}
}

// S2PrefixTree is the Google S2-based spatial prefix tree.
//
// Port of org.apache.lucene.spatial.prefix.tree.S2PrefixTree.
//
// Deviation: S2CellId, S2Projections, S2LatLng (Google S2 Java library) and
// spatial4j are not yet ported. Algorithmic bodies deferred to backlog #2693.
type S2PrefixTree struct {
	BaseSpatialPrefixTree
	s2ShapeFactory S2ShapeFactory
	arity          int
}

// NewS2PrefixTree builds the tree with arity 1.
func NewS2PrefixTree(ctx interface{}, maxLevels int) *S2PrefixTree {
	return NewS2PrefixTreeWithArity(ctx, maxLevels, 1)
}

// NewS2PrefixTreeWithArity builds the tree with the given arity (1, 2, or 3).
func NewS2PrefixTreeWithArity(ctx interface{}, maxLevels, arity int) *S2PrefixTree {
	if arity < 1 || arity > 3 {
		panic("invalid S2 tree arity: must be 1, 2, or 3")
	}
	if maxLevels < 1 {
		maxLevels = 1
	}
	t := &S2PrefixTree{arity: arity}
	t.MaxLevels = maxLevels
	t.Ctx = ctx
	if sf, ok := ctx.(S2ShapeFactory); ok {
		t.s2ShapeFactory = sf
	}
	return t
}

// GetMaxLevels returns the maximum tree depth for the given arity.
//
// Port of S2PrefixTree.getMaxLevels(arity).
// S2CellId.MAX_LEVEL = 30 in the S2 Java library.
func S2GetMaxLevels(arity int) int {
	return 30/arity + 1
}

// GetLevelForDistance returns the tree level whose cell side is ≤ dist degrees.
// Returns MaxLevels — deferred to #2693.
func (t *S2PrefixTree) GetLevelForDistance(_ float64) int { return t.MaxLevels }

// GetDistanceForLevel returns the hypotenuse distance for level.
// Returns 0 — deferred to #2693.
func (t *S2PrefixTree) GetDistanceForLevel(_ int) float64 { return 0 }

// GetWorldCell returns the level-0 cell.
func (t *S2PrefixTree) GetWorldCell() Cell {
	return NewS2PrefixTreeCell(t, nil)
}

// ReadCell initialises a cell from a BytesRef term.
func (t *S2PrefixTree) ReadCell(term *util.BytesRef, scratch Cell) Cell {
	var cell *S2PrefixTreeCell
	if scratch != nil {
		if sc, ok := scratch.(*S2PrefixTreeCell); ok {
			cell = sc
		}
	}
	if cell == nil {
		cell = NewS2PrefixTreeCell(t, nil)
	}
	cell.ReadCellFromTerm(t, term)
	return cell
}

// GetTreeCellIterator returns a TreeCellIterator.
func (t *S2PrefixTree) GetTreeCellIterator(shape interface{}, detailLevel int) CellIterator {
	return t.BaseSpatialPrefixTree.GetTreeCellIterator(t, shape, detailLevel)
}

// GetSpatialContext returns the spatial context.
func (t *S2PrefixTree) GetSpatialContext() interface{} { return t.Ctx }

// GetMaxLevels returns the maximum depth.
func (t *S2PrefixTree) GetMaxLevels() int { return t.MaxLevels }

var _ SpatialPrefixTree = (*S2PrefixTree)(nil)
