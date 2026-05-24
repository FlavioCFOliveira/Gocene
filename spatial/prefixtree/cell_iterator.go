package prefixtree

import "github.com/FlavioCFOliveira/Gocene/util"

// CellIterator lazily traverses spatial prefix-tree cells.
//
// Port of org.apache.lucene.spatial.prefix.tree.CellIterator.
//
// Deviation: Java uses an abstract class with nextCell/thisCell state; Go
// uses a CellIterator interface + BaseCellIterator composition struct.
type CellIterator interface {
	// HasNext reports whether more cells are available.
	HasNext() bool
	// Next advances and returns the next cell; must only be called after HasNext returns true.
	Next() Cell
	// ThisCell returns the most-recently returned cell (after Next), or nil.
	ThisCell() Cell
	// Remove signals that sub-cells of the current cell should not be visited.
	Remove()
}

// BaseCellIterator provides the shared thisCell/nextCell state used by all
// concrete CellIterator implementations.
type BaseCellIterator struct {
	nextCell Cell // candidate for the next Next() return
	thisCell Cell // the last cell returned by Next()
}

// Next advances the iterator.
func (b *BaseCellIterator) Next() Cell {
	if b.nextCell == nil {
		panic("Next called when HasNext is false")
	}
	b.thisCell = b.nextCell
	b.nextCell = nil
	return b.thisCell
}

// ThisCell returns the cell most recently returned by Next.
func (b *BaseCellIterator) ThisCell() Cell { return b.thisCell }

// Remove is a no-op at this level; override in TreeCellIterator.
func (b *BaseCellIterator) Remove() {}

// Cell is the contract every spatial prefix-tree cell exposes.
//
// Port of org.apache.lucene.spatial.prefix.tree.Cell.
//
// Deviation: spatial4j Shape/SpatialRelation/Point types are represented as
// interface{} until the spatial4j Go port is available (backlog #2693).
type Cell interface {
	// GetShapeRel returns the spatial relation of this cell with the shape from
	// which it was filtered.
	GetShapeRel() interface{} // spatial4j SpatialRelation

	// SetShapeRel sets the spatial relation.
	SetShapeRel(rel interface{})

	// IsLeaf reports whether the cell is flagged as a leaf.
	IsLeaf() bool

	// SetLeaf marks this cell as a leaf.
	SetLeaf()

	// GetTokenBytesWithLeaf returns the token bytes for this cell, including the
	// leaf byte if this cell is a leaf.
	GetTokenBytesWithLeaf(result *util.BytesRef) *util.BytesRef

	// GetTokenBytesNoLeaf returns the token bytes without the leaf flag.
	GetTokenBytesNoLeaf(result *util.BytesRef) *util.BytesRef

	// GetLevel returns the depth of this cell in the tree (0 = world).
	GetLevel() int

	// GetNextLevelCells returns the children of this cell optionally filtered by
	// shapeFilter (interface{} = spatial4j Shape).
	GetNextLevelCells(shapeFilter interface{}) CellIterator

	// GetShape returns the spatial shape for this cell.
	GetShape() interface{} // spatial4j Shape

	// IsPrefixOf reports whether the target term is within/underneath this cell.
	IsPrefixOf(c Cell) bool

	// CompareToNoLeaf is equivalent to
	// getTokenBytesNoLeaf(nil).compareTo(fromCell.getTokenBytesNoLeaf(nil)).
	CompareToNoLeaf(fromCell Cell) int
}
