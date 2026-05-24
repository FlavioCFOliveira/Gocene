package prefixtree

import "github.com/FlavioCFOliveira/Gocene/util"

// S2PrefixTreeCell represents a cell in an S2PrefixTree.
//
// Port of org.apache.lucene.spatial.prefix.tree.S2PrefixTreeCell.
//
// Deviation: S2CellId token encoding and shape computation are deferred to
// backlog #2693. Cell ID is stored as raw bytes; geometric methods return nil.
type S2PrefixTreeCell struct {
	tree     *S2PrefixTree
	cellID   []byte // raw S2 cell-id bytes — nil = world cell
	leaf     bool
	shapeRel interface{} // spatial4j SpatialRelation
}

// NewS2PrefixTreeCell creates a cell for the given tree and raw S2 cell ID
// bytes (nil = world cell).
func NewS2PrefixTreeCell(tree *S2PrefixTree, cellIDBytes []byte) *S2PrefixTreeCell {
	c := &S2PrefixTreeCell{tree: tree}
	if cellIDBytes != nil {
		c.cellID = append([]byte(nil), cellIDBytes...)
	}
	return c
}

// ReadCellFromTerm resets the cell state from a BytesRef term.
func (c *S2PrefixTreeCell) ReadCellFromTerm(_ *S2PrefixTree, term *util.BytesRef) {
	if term == nil || term.Length == 0 {
		c.cellID = nil
		c.leaf = false
		return
	}
	raw := term.Bytes[term.Offset : term.Offset+term.Length]
	if len(raw) > 0 && raw[len(raw)-1] == 0xFF {
		c.leaf = true
		raw = raw[:len(raw)-1]
	} else {
		c.leaf = false
	}
	c.cellID = append(c.cellID[:0], raw...)
}

// GetLevel returns the depth of the cell; 0 = world cell.
func (c *S2PrefixTreeCell) GetLevel() int {
	// Each S2CellId encoded as 8 bytes; level = number of 8-byte blocks.
	return len(c.cellID) / 8
}

// IsLeaf reports whether this cell is a leaf.
func (c *S2PrefixTreeCell) IsLeaf() bool { return c.leaf }

// SetLeaf marks this cell as a leaf.
func (c *S2PrefixTreeCell) SetLeaf() { c.leaf = true }

// GetShapeRel returns the stored spatial relation.
func (c *S2PrefixTreeCell) GetShapeRel() interface{} { return c.shapeRel }

// SetShapeRel stores the spatial relation.
func (c *S2PrefixTreeCell) SetShapeRel(rel interface{}) { c.shapeRel = rel }

// GetTokenBytesNoLeaf returns a BytesRef of the raw cell-ID bytes.
func (c *S2PrefixTreeCell) GetTokenBytesNoLeaf(result *util.BytesRef) *util.BytesRef {
	if result == nil {
		result = util.NewBytesRef(nil)
	}
	dst := make([]byte, len(c.cellID))
	copy(dst, c.cellID)
	result.Bytes = dst
	result.Length = len(dst)
	result.Offset = 0
	return result
}

// GetTokenBytesWithLeaf returns the token bytes appending 0xFF when leaf.
func (c *S2PrefixTreeCell) GetTokenBytesWithLeaf(result *util.BytesRef) *util.BytesRef {
	result = c.GetTokenBytesNoLeaf(result)
	if c.leaf {
		result.Bytes = append(result.Bytes[:result.Offset+result.Length], 0xFF)
		result.Length++
	}
	return result
}

// GetShape returns nil — deferred to #2693.
func (c *S2PrefixTreeCell) GetShape() interface{} { return nil }

// GetNextLevelCells returns nil — deferred to #2693.
func (c *S2PrefixTreeCell) GetNextLevelCells(_ interface{}) CellIterator { return nil }

// IsPrefixOf reports whether this cell is an ancestor of other.
func (c *S2PrefixTreeCell) IsPrefixOf(other Cell) bool {
	if other == nil {
		return false
	}
	bRef := other.GetTokenBytesNoLeaf(nil)
	b := bRef.Bytes[bRef.Offset : bRef.Offset+bRef.Length]
	a := c.cellID
	if len(a) > len(b) {
		return false
	}
	for i, ab := range a {
		if b[i] != ab {
			return false
		}
	}
	return true
}

// CompareToNoLeaf compares this cell's token bytes to fromCell's.
func (c *S2PrefixTreeCell) CompareToNoLeaf(fromCell Cell) int {
	other := fromCell.GetTokenBytesNoLeaf(nil)
	b := other.Bytes[other.Offset : other.Offset+other.Length]
	a := c.cellID
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

var _ Cell = (*S2PrefixTreeCell)(nil)
