package prefixtree

import "github.com/FlavioCFOliveira/Gocene/util"

// LegacyCell is the abstract base used by the pre-PackedQuad spatial
// prefix-tree cells (Geohash, Quad). Mirrors
// org.apache.lucene.spatial.prefix.tree.LegacyCell.
//
// Deviation: spatial4j Shape/SpatialRelation are interface{}; full shape
// geometry deferred to backlog #2693.
type LegacyCell struct {
	level    int
	token    []byte
	leaf     bool
	shapeRel interface{} // spatial4j SpatialRelation
}

// NewLegacyCell builds the base cell.
func NewLegacyCell(level int, token []byte, leaf bool) *LegacyCell {
	return &LegacyCell{level: level, token: append([]byte(nil), token...), leaf: leaf}
}

// GetLevel returns the cell's depth in the tree.
func (c *LegacyCell) GetLevel() int { return c.level }

// GetTokenBytesNoLeaf returns a copy of the raw token bytes (no leaf marker).
func (c *LegacyCell) GetTokenBytesNoLeaf(result *util.BytesRef) *util.BytesRef {
	if result == nil {
		result = util.NewBytesRef(nil)
	}
	dst := make([]byte, len(c.token))
	copy(dst, c.token)
	result.Bytes = dst
	result.Length = len(dst)
	result.Offset = 0
	return result
}

// GetTokenBytesWithLeaf returns the token bytes, appending a leaf marker byte
// (0xFF) when this cell is a leaf.
func (c *LegacyCell) GetTokenBytesWithLeaf(result *util.BytesRef) *util.BytesRef {
	result = c.GetTokenBytesNoLeaf(result)
	if c.leaf {
		result.Bytes = append(result.Bytes[:result.Offset+result.Length], 0xFF)
		result.Length++
	}
	return result
}

// IsLeaf reports whether the cell is a leaf.
func (c *LegacyCell) IsLeaf() bool { return c.leaf }

// SetLeaf marks this cell as a leaf.
func (c *LegacyCell) SetLeaf() { c.leaf = true }

// GetShapeRel returns the stored spatial relation (interface{} = SpatialRelation).
func (c *LegacyCell) GetShapeRel() interface{} { return c.shapeRel }

// SetShapeRel stores the spatial relation.
func (c *LegacyCell) SetShapeRel(rel interface{}) { c.shapeRel = rel }

// GetShape returns nil — deferred to #2693.
func (c *LegacyCell) GetShape() interface{} { return nil }

// GetNextLevelCells returns nil — deferred to #2693.
func (c *LegacyCell) GetNextLevelCells(_ interface{}) CellIterator { return nil }

// IsPrefixOf reports whether this cell is a prefix of c.
func (c *LegacyCell) IsPrefixOf(other Cell) bool {
	if other == nil {
		return false
	}
	a := c.token
	bRef := other.GetTokenBytesNoLeaf(nil)
	b := bRef.Bytes[bRef.Offset : bRef.Offset+bRef.Length]
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
func (c *LegacyCell) CompareToNoLeaf(fromCell Cell) int {
	other := fromCell.GetTokenBytesNoLeaf(nil)
	b := other.Bytes[other.Offset : other.Offset+other.Length]
	a := c.token
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

// ReadCell resets this cell from a BytesRef term (drops leaf flag).
func (c *LegacyCell) ReadCell(term *util.BytesRef) {
	if term == nil || term.Length == 0 {
		c.level = 0
		c.token = nil
		c.leaf = false
		return
	}
	raw := term.Bytes[term.Offset : term.Offset+term.Length]
	if raw[len(raw)-1] == 0xFF {
		c.leaf = true
		raw = raw[:len(raw)-1]
	} else {
		c.leaf = false
	}
	c.token = append(c.token[:0], raw...)
	c.level = len(c.token)
}

var _ Cell = (*LegacyCell)(nil)
