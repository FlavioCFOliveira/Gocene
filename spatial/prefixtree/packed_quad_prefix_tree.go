package prefixtree

// PackedQuadPrefixTree is the bit-packed quad spatial prefix tree. Mirrors
// org.apache.lucene.spatial.prefix.tree.PackedQuadPrefixTree.
type PackedQuadPrefixTree struct {
	MaxLevels int
}

// NewPackedQuadPrefixTree builds the tree with the supplied maxLevels.
func NewPackedQuadPrefixTree(maxLevels int) *PackedQuadPrefixTree {
	if maxLevels < 1 {
		maxLevels = 1
	}
	return &PackedQuadPrefixTree{MaxLevels: maxLevels}
}
