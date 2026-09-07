package prefixtree

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/golang/geo/s2"
)

// S2ShapeFactory is the contract that creates S2 shapes for the tree.
// Mirrors org.apache.lucene.spatial.prefix.tree.S2ShapeFactory.
type S2ShapeFactory interface {
	GetS2CellShape(cellID s2.CellID) interface{}
	NewCircle(lat, lon, radius float64) interface{}
	NewRectangle(minX, minY, maxX, maxY float64) interface{}
}

// S2PrefixTree is the Google S2-based spatial prefix tree.
//
// Port of org.apache.lucene.spatial.prefix.tree.S2PrefixTree.
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

// S2GetMaxLevels returns the maximum tree depth for the given arity.
func S2GetMaxLevels(arity int) int {
	return 30/arity + 1
}

// GetLevelForDistance returns the tree level whose cell diagonal is ≤ dist
// degrees.
func (t *S2PrefixTree) GetLevelForDistance(dist float64) int {
	if dist == 0 {
		return t.MaxLevels
	}

	// Faithful approximation of S2Projections.MAX_WIDTH.getMinLevel.
	// Width ≈ 360 / 2^L.
	// L = ceil(log2(360/dist)).
	level := int(math.Ceil(math.Log2(360.0 / dist)))

	// Arity adjustment
	roundLevel := 0
	if level%t.arity != 0 {
		roundLevel = 1
	}
	level = level/t.arity + roundLevel

	if level > t.MaxLevels {
		return t.MaxLevels
	}
	return level + 1
}

// GetDistanceForLevel returns the approximate cell diagonal in degrees for
// the given level.
func (t *S2PrefixTree) GetDistanceForLevel(level int) float64 {
	if level == 0 {
		return 180.0
	}
	// Faithful approximation of S2Projections.MAX_WIDTH.getValue.
	// Value ≈ 360 / 2^(arity * (level-1)).
	return 360.0 / math.Pow(2, float64(t.arity*(level-1)))
}

// GetWorldCell returns the level-0 cell.
func (t *S2PrefixTree) GetWorldCell() Cell {
	return NewS2PrefixTreeCell(t, s2.CellID(0))
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
		cell = NewS2PrefixTreeCell(t, s2.CellID(0))
	}
	cell.ReadCellFromTerm(t, term)
	return cell
}

// GetTreeCellIterator returns a TreeCellIterator.
func (t *S2PrefixTree) GetTreeCellIterator(shape interface{}, detailLevel int) CellIterator {
	// In a real port, we'd check if shape is a Point.
	// Since we don't have the full spatial4j Point type, we use a generic check or
	// defer to BaseSpatialPrefixTree.
	// If we can detect a Point, we use the optimized S2 path.

	// Assuming shape could be a custom Point type for now.
	// For the translation, we implement the logic:
	// S2CellId id = S2CellId.fromLatLng(S2LatLng.fromDegrees(p.getY(), p.getX())).parent(arity * (detailLevel - 1));

	// Since we don't know the Point type, we call the base iterator unless we can verify it's a point.
	return t.BaseSpatialPrefixTree.GetTreeCellIterator(t, shape, detailLevel)
}

// GetSpatialContext returns the spatial context.
func (t *S2PrefixTree) GetSpatialContext() interface{} { return t.Ctx }

// GetMaxLevels returns the maximum depth.
func (t *S2PrefixTree) GetMaxLevels() int { return t.MaxLevels }

var _ SpatialPrefixTree = (*S2PrefixTree)(nil)
