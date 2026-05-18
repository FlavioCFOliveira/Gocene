package prefixtree

// S2PrefixTree is the Google S2-based spatial prefix tree. Mirrors
// org.apache.lucene.spatial.prefix.tree.S2PrefixTree.
type S2PrefixTree struct {
	MaxLevels int
}

// NewS2PrefixTree builds the tree.
func NewS2PrefixTree(maxLevels int) *S2PrefixTree {
	if maxLevels < 1 {
		maxLevels = 1
	}
	return &S2PrefixTree{MaxLevels: maxLevels}
}

// S2ShapeFactory is the contract that creates S2 shapes for the tree.
// Mirrors org.apache.lucene.spatial.prefix.tree.S2ShapeFactory.
type S2ShapeFactory interface {
	NewCircle(lat, lon, radius float64) any
	NewRectangle(minX, minY, maxX, maxY float64) any
}
