package prefixtree

// TreeCellIterator navigates a SpatialPrefixTree from a parent cell down to a
// maximum depth, optionally filtered by a shape.
//
// Port of org.apache.lucene.spatial.prefix.tree.TreeCellIterator.
//
// Deviation: Point-shape specialisation (depth-first encode) and spatial4j
// SpatialRelation.intersects() checks are stubbed — full implementation
// requires spatial4j Shape types (backlog #2693). The iterator correctly
// traverses the tree structure; shape filtering is conservatively pass-through.
type TreeCellIterator struct {
	BaseCellIterator
	shapeFilter interface{}    // spatial4j Shape; nil = no filter
	iterStack   []CellIterator // iterStack[i] = iterator at level i+1
	stackIdx    int            // -1 when exhausted
	descend     bool
}

// NewTreeCellIterator creates the iterator rooted at parentCell (level 0).
func NewTreeCellIterator(shapeFilter interface{}, detailLevel int, parentCell Cell) *TreeCellIterator {
	t := &TreeCellIterator{
		shapeFilter: shapeFilter,
		iterStack:   make([]CellIterator, detailLevel),
		stackIdx:    0,
		descend:     false,
	}
	t.iterStack[0] = parentCell.GetNextLevelCells(shapeFilter)
	return t
}

// HasNext advances through the tree, returning true when more cells exist.
func (t *TreeCellIterator) HasNext() bool {
	if t.nextCell != nil {
		return true
	}
	for {
		if t.stackIdx == -1 {
			return false
		}
		// Descend if possible.
		if t.descend &&
			t.stackIdx < len(t.iterStack)-1 &&
			t.iterStack[t.stackIdx].ThisCell() != nil &&
			!t.iterStack[t.stackIdx].ThisCell().IsLeaf() {
			nextIter := t.iterStack[t.stackIdx].ThisCell().GetNextLevelCells(t.shapeFilter)
			if nextIter != nil {
				t.stackIdx++
				t.iterStack[t.stackIdx] = nextIter
			}
		}
		// Get sibling.
		if t.iterStack[t.stackIdx].HasNext() {
			t.nextCell = t.iterStack[t.stackIdx].Next()
			// At detailLevel, mark as leaf.
			if t.stackIdx == len(t.iterStack)-1 {
				t.nextCell.SetLeaf()
			}
			break
		}
		// Go up.
		t.iterStack[t.stackIdx] = nil
		t.stackIdx--
		t.descend = false
	}
	t.descend = true
	return true
}

// Remove signals that sub-cells of the current cell should be skipped.
func (t *TreeCellIterator) Remove() {
	t.descend = false
}

var _ CellIterator = (*TreeCellIterator)(nil)
