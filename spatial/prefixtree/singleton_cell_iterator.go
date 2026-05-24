package prefixtree

// SingletonCellIterator is a CellIterator over exactly one cell.
//
// Port of org.apache.lucene.spatial.prefix.tree.SingletonCellIterator.
type SingletonCellIterator struct {
	BaseCellIterator
}

// NewSingletonCellIterator creates an iterator pre-loaded with cell.
func NewSingletonCellIterator(cell Cell) *SingletonCellIterator {
	s := &SingletonCellIterator{}
	s.nextCell = cell
	return s
}

// HasNext returns true until the single cell has been consumed.
func (s *SingletonCellIterator) HasNext() bool {
	s.thisCell = nil
	return s.nextCell != nil
}

var _ CellIterator = (*SingletonCellIterator)(nil)
