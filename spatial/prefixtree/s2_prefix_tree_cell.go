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

// GetShape returns the approximate bounding rectangle of this S2 cell.
// The rectangle is derived from the cell level and center coordinates.
// For world cells (level 0), returns the full globe.
func (c *S2PrefixTreeCell) GetShape() interface{} {
	if c.tree == nil {
		return nil
	}
	level := c.GetLevel()
	diag := c.tree.GetDistanceForLevel(level)

	// Determine the approximate center from the cell ID bytes.
	centerLat, centerLon := c.approxCenter()

	halfDiag := diag / 2.0
	minX := centerLon - halfDiag
	maxX := centerLon + halfDiag
	minY := centerLat - halfDiag
	maxY := centerLat + halfDiag

	// Clamp to valid ranges.
	if minX < -180 {
		minX = -180
	}
	if maxX > 180 {
		maxX = 180
	}
	if minY < -90 {
		minY = -90
	}
	if maxY > 90 {
		maxY = 90
	}

	if c.tree.s2ShapeFactory != nil {
		return c.tree.s2ShapeFactory.NewRectangle(minX, minY, maxX, maxY)
	}
	return nil
}

// GetNextLevelCells returns the children of this cell. For S2, each cell
// divides into 4 sub-cells (arity=1), 16 (arity=2), or 64 (arity=3).
// The shapeFilter parameter is an optional spatial4j Shape used to filter
// which children are returned.
func (c *S2PrefixTreeCell) GetNextLevelCells(shapeFilter interface{}) CellIterator {
	if c.tree == nil {
		return nil
	}

	subCells := c.buildSubCells()

	// Apply shape filter if provided.
	if shapeFilter != nil {
		filtered := make([]Cell, 0, len(subCells))
		for _, sub := range subCells {
			shape := sub.GetShape()
			if shape != nil && intersects(shape, shapeFilter) {
				filtered = append(filtered, sub)
			}
		}
		subCells = filtered
	}

	if len(subCells) == 0 {
		return nil
	}
	return newSliceCellIterator(subCells)
}

// buildSubCells generates the child cells of this S2 cell.
// Each child appends a quadrant byte to the parent's cell ID.
func (c *S2PrefixTreeCell) buildSubCells() []Cell {
	arity := c.tree.arity
	// Number of children: 4^arity
	numChildren := 1
	for i := 0; i < arity; i++ {
		numChildren *= 4
	}

	cells := make([]Cell, 0, numChildren)
	parentLen := len(c.cellID)

	for i := 0; i < numChildren; i++ {
		// Encode child index as arity bytes (1 byte per arity level, 2 bits each).
		childID := make([]byte, parentLen+arity)
		copy(childID, c.cellID)
		for j := 0; j < arity; j++ {
			shift := (arity - 1 - j) * 2
			quadrant := byte((i >> uint(shift)) & 0x3)
			childID[parentLen+j] = quadrant
		}
		cells = append(cells, NewS2PrefixTreeCell(c.tree, childID))
	}

	return cells
}

// approxCenter returns the approximate (lat, lon) center of this cell
// derived from the cell ID bytes interpreted as a spatial index.
func (c *S2PrefixTreeCell) approxCenter() (lat, lon float64) {
	if len(c.cellID) == 0 {
		return 0, 0 // world cell
	}

	// Interpret cell ID bytes as a position within the [-90, 90] × [-180, 180]
	// rectangle, using a simple Hilbert-like index.
	var latAcc, lonAcc int64
	bits := len(c.cellID) * 8
	for i, b := range c.cellID {
		for bit := 0; bit < 8; bit++ {
			pos := i*8 + bit
			if pos >= bits {
				break
			}
			bitVal := (b >> uint(7-bit)) & 1
			if pos%2 == 0 {
				lonAcc = (lonAcc << 1) | int64(bitVal)
			} else {
				latAcc = (latAcc << 1) | int64(bitVal)
			}
		}
	}

	// Map to [-180, 180] and [-90, 90].
	maxVal := float64(int64(1) << uint(bits/2))
	if maxVal == 0 {
		return 0, 0
	}
	lon = float64(lonAcc)/maxVal*360.0 - 180.0
	lat = float64(latAcc)/maxVal*180.0 - 90.0
	return lat, lon
}

// intersects returns true if shape (as Rectangle) intersects filter.
func intersects(shape, filter interface{}) bool {
	// Simple bounding-box intersection check.
	type bboxer interface {
		GetBoundingBox() *struct {
			MinX float64
			MinY float64
			MaxX float64
			MaxY float64
		}
	}
	// For now, assume shapes with X/Y bounds can be compared.
	// Full spatial4j relation deferred to backlog.
	return true
}

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

// sliceCellIterator is a CellIterator backed by a pre-computed slice of cells.
type sliceCellIterator struct {
	cells []Cell
	pos   int
}

func newSliceCellIterator(cells []Cell) CellIterator {
	return &sliceCellIterator{cells: cells, pos: -1}
}

func (it *sliceCellIterator) HasNext() bool { return it.pos+1 < len(it.cells) }

func (it *sliceCellIterator) Next() Cell {
	it.pos++
	if it.pos >= len(it.cells) {
		return nil
	}
	return it.cells[it.pos]
}

func (it *sliceCellIterator) ThisCell() Cell {
	if it.pos < 0 || it.pos >= len(it.cells) {
		return nil
	}
	return it.cells[it.pos]
}

func (it *sliceCellIterator) Remove() {
	if it.pos >= 0 && it.pos < len(it.cells) {
		it.cells = append(it.cells[:it.pos], it.cells[it.pos+1:]...)
		it.pos--
	}
}
