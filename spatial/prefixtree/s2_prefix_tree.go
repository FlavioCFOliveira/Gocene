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

// GetLevelForDistance returns the tree level whose cell diagonal is ≤ dist
// degrees. Uses the approximate formula for S2 cell size at the given arity:
// cell_diagonal ≈ 360 / (2^level). The smallest level whose diagonal ≤ dist
// is returned, clamped to [0, MaxLevels].
func (t *S2PrefixTree) GetLevelForDistance(dist float64) int {
	if dist <= 0 {
		return t.MaxLevels
	}
	// Solve: 360 / (2^level) / sqrt(arity) ≈ dist
	// level = ceil(log2(360 / dist))
	level := 0
	cellSize := 360.0
	arityFactor := 1.0
	if t.arity > 1 {
		arityFactor = float64(t.arity)
	}
	for cellSize/arityFactor > dist && level < t.MaxLevels {
		level++
		cellSize /= 2.0
	}
	if level > t.MaxLevels {
		level = t.MaxLevels
	}
	return level
}

// GetDistanceForLevel returns the approximate cell diagonal in degrees for
// the given level.
func (t *S2PrefixTree) GetDistanceForLevel(level int) float64 {
	if level <= 0 {
		return 360.0
	}
	arityFactor := 1.0
	if t.arity > 1 {
		arityFactor = float64(t.arity)
	}
	return 360.0 / float64(int(1)<<uint(level)) / arityFactor
}

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
