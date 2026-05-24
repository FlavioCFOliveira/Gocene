package prefixtree

// CellCanPrune is the marker interface implemented by Cell-like types that
// expose a CanPrune() method so SPT search loops can skip subtrees. Mirrors
// org.apache.lucene.spatial.prefix.tree.CellCanPrune.
type CellCanPrune interface {
	CanPrune() bool
}
