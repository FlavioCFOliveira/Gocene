package prefixtree

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/golang/geo/s2"
)

// S2PrefixTreeCell represents a cell in an S2PrefixTree.
//
// Port of org.apache.lucene.spatial.prefix.tree.S2PrefixTreeCell.
type S2PrefixTreeCell struct {
	tree   *S2PrefixTree
	cellID s2.CellID
	level  int
	leaf   bool
	shape  interface{}
	shapeRel interface{} // spatial4j SpatialRelation
}

var faces = [6]s2.CellID{
	s2.CellIDFromFacePosLevel(0, 0, 0),
	s2.CellIDFromFacePosLevel(1, 0, 0),
	s2.CellIDFromFacePosLevel(2, 0, 0),
	s2.CellIDFromFacePosLevel(3, 0, 0),
	s2.CellIDFromFacePosLevel(4, 0, 0),
	s2.CellIDFromFacePosLevel(5, 0, 0),
}

var tokens = []byte{
	'.', '/', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', 'A', 'B', 'C', 'D', 'E', 'F', 'G',
	'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
	'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's',
	't', 'u', 'v', 'w', 'x', 'y', 'z',
}

var pixels map[byte]int

func init() {
	pixels = make(map[byte]int, len(tokens))
	for i, t := range tokens {
		pixels[t] = i
	}
}

const leafByte = '+'

// NewS2PrefixTreeCell creates a cell for the given tree and S2 cell ID.
func NewS2PrefixTreeCell(tree *S2PrefixTree, cellID s2.CellID) *S2PrefixTreeCell {
	c := &S2PrefixTreeCell{
		tree:   tree,
		cellID: cellID,
	}
	c.setLevel()
	if c.level == tree.GetMaxLevels() {
		c.SetLeaf()
	}
	return c
}

// ReadCellFromTerm resets the cell state from a BytesRef term.
func (c *S2PrefixTreeCell) ReadCellFromTerm(tree *S2PrefixTree, term *util.BytesRef) {
	c.leaf = false
	c.shape = nil
	c.shapeRel = nil
	c.tree = tree

	c.cellID = c.getS2CellIDFromBytesRef(term)
	c.setLevel()

	if c.isLeaf(term) || c.level == tree.GetMaxLevels() {
		c.SetLeaf()
	}
}

func (c *S2PrefixTreeCell) GetShapeRel() interface{} {
	return c.shapeRel
}

func (c *S2PrefixTreeCell) SetShapeRel(rel interface{}) {
	c.shapeRel = rel
}

func (c *S2PrefixTreeCell) IsLeaf() bool {
	return c.leaf
}

func (c *S2PrefixTreeCell) SetLeaf() {
	c.leaf = true
}

func (c *S2PrefixTreeCell) GetTokenBytesWithLeaf(result *util.BytesRef) *util.BytesRef {
	result = c.GetTokenBytesNoLeaf(result)
	if c.leaf && c.level != c.tree.GetMaxLevels() {
		result.Bytes = append(result.Bytes[:result.Offset+result.Length], leafByte)
		result.Length++
	}
	return result
}

func (c *S2PrefixTreeCell) GetTokenBytesNoLeaf(result *util.BytesRef) *util.BytesRef {
	if result == nil {
		result = util.NewBytesRef(nil)
	}
	return c.getBytesRefFromS2CellID(result)
}

func (c *S2PrefixTreeCell) GetLevel() int {
	return c.level
}

func (c *S2PrefixTreeCell) setLevel() {
	if c.cellID == s2.CellID(0) {
		c.level = 0
	} else {
		// Java: assert cellId.level() % tree.arity == 0;
		c.level = (c.cellID.Level() / c.tree.arity) + 1
	}
}

func (c *S2PrefixTreeCell) GetNextLevelCells(shapeFilter interface{}) CellIterator {
	var children []s2.CellID
	if c.cellID == s2.CellID(0) { // world cell
		children = faces[:]
	} else {
		nChildren := 1 << (2 * c.tree.arity)
		children = make([]s2.CellID, nChildren)
		// childBeginAtLevel is the correct method for the target level
		children[0] = c.cellID.ChildBeginAtLevel(c.cellID.Level() + c.tree.arity)
		for i := 1; i < nChildren; i++ {
			children[i] = children[i-1].Next()
		}
	}

	cells := make([]Cell, 0, len(children))
	for _, id := range children {
		cells = append(cells, NewS2PrefixTreeCell(c.tree, id))
	}
	return NewFilterCellIterator(newSliceCellIterator(cells), shapeFilter)
}

func (c *S2PrefixTreeCell) GetShape() interface{} {
	if c.shape == nil {
		if c.cellID == s2.CellID(0) {
			// World cell: return full globe bounds (roughly)
			// Use tree.Ctx to get world bounds if available
			if c.tree.Ctx != nil {
				// In a real port, this would use the SpatialContext.
				// For now, return a dummy or use the factory.
			}
		} else {
			if c.tree.s2ShapeFactory != nil {
				c.shape = c.tree.s2ShapeFactory.GetS2CellShape(c.cellID)
			}
		}
	}
	return c.shape
}

func (c *S2PrefixTreeCell) IsPrefixOf(other Cell) bool {
	if c.cellID == s2.CellID(0) {
		return true
	}
	otherCell, ok := other.(*S2PrefixTreeCell)
	if !ok {
		return false
	}
	return c.cellID.Contains(otherCell.cellID)
}

func (c *S2PrefixTreeCell) CompareToNoLeaf(fromCell Cell) int {
	if c.cellID == s2.CellID(0) {
		return 1
	}
	otherCell, ok := fromCell.(*S2PrefixTreeCell)
	if !ok {
		return 0
	}
	// S2CellID is uint64, use standard comparison
	if c.cellID < otherCell.cellID {
		return -1
	}
	if c.cellID > otherCell.cellID {
		return 1
	}
	return 0
}

func (c *S2PrefixTreeCell) isLeaf(ref *util.BytesRef) bool {
	if ref == nil || ref.Length == 0 {
		return false
	}
	return ref.Bytes[ref.Offset+ref.Length-1] == leafByte
}

func (c *S2PrefixTreeCell) getS2CellIDFromBytesRef(ref *util.BytesRef) s2.CellID {
	length := ref.Length
	if c.isLeaf(ref) {
		length--
	}
	if length == 0 {
		return s2.CellID(0) // world cell
	}

	faceIdx := pixels[ref.Bytes[ref.Offset]]
	cellID := faces[faceIdx]
	id := uint64(cellID)

	for i := ref.Offset + 1; i < ref.Offset+length; i++ {
		thisLevel := i - ref.Offset
		pos := pixels[ref.Bytes[i]]
		// id = id - (id & -id) + (1L << (2 * (S2CellId.MAX_LEVEL - thisLevel * tree.arity)));
		// In Go, we can use s2.CellID properties or translate the bit math.
		// Java's id & -id is the lowest set bit.
		lowBit := id & -id
		id = id - lowBit + (1 << uint64(2*(30-thisLevel*c.tree.arity)))
		id = id + uint64(pos)* (lowBit << 1)
	}
	return s2.CellID(id)
}

func (c *S2PrefixTreeCell) getBytesRefFromS2CellID(result *util.BytesRef) *util.BytesRef {
	if c.cellID == s2.CellID(0) {
		result.Length = 0
		return result
	}

	level := c.GetLevel()
	length := level + 1
	b := make([]byte, length)

	b[0] = tokens[c.cellID.Face()]
	for i := 1; i < level; i++ {
		offset := 0
		levelVal := c.tree.arity * i
		for j := 1; j < c.tree.arity; j++ {
			offset = 4*offset + int(c.cellID.ChildPosition(levelVal-c.tree.arity+j))
		}
		b[i] = tokens[4*offset+int(c.cellID.ChildPosition(levelVal))]
	}

	result.Bytes = b
	result.Length = level
	result.Offset = 0
	return result
}

func (c *S2PrefixTreeCell) GetSubCellsSize() int {
	if c.cellID == s2.CellID(0) {
		return 6
	}
	return 1 << (2 * c.tree.arity)
}

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

var _ Cell = (*S2PrefixTreeCell)(nil)
