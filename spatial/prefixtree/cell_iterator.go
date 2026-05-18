package prefixtree

// CellIterator is the iteration primitive used by every SpatialPrefixTree to
// traverse cells lazily. Mirrors
// org.apache.lucene.spatial.prefix.tree.CellIterator.
type CellIterator interface {
	HasNext() bool
	Next() Cell
}

// Cell is the minimal contract every spatial prefix-tree cell exposes.
type Cell interface {
	Level() int
	TokenBytes() []byte
	IsLeaf() bool
}
