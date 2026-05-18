package prefixtree

// LegacyCell is the abstract base used by the pre-PackedQuad spatial
// prefix-tree cells (Geohash, Quad). Mirrors
// org.apache.lucene.spatial.prefix.tree.LegacyCell.
type LegacyCell struct {
	level int
	token []byte
	leaf  bool
}

// NewLegacyCell builds the base.
func NewLegacyCell(level int, token []byte, leaf bool) *LegacyCell {
	return &LegacyCell{level: level, token: append([]byte(nil), token...), leaf: leaf}
}

// Level returns the cell's depth.
func (c *LegacyCell) Level() int { return c.level }

// TokenBytes returns the raw token bytes.
func (c *LegacyCell) TokenBytes() []byte {
	out := make([]byte, len(c.token))
	copy(out, c.token)
	return out
}

// IsLeaf reports whether the cell is a leaf in its tree.
func (c *LegacyCell) IsLeaf() bool { return c.leaf }

var _ Cell = (*LegacyCell)(nil)
