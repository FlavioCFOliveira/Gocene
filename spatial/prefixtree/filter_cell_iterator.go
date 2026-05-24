package prefixtree

// FilterCellIterator filters cells from a base iterator, skipping those that
// are disjoint with a shape filter. If shapeFilter is nil, all cells pass.
//
// Port of org.apache.lucene.spatial.prefix.tree.FilterCellIterator.
//
// Deviation: spatial4j Shape/SpatialRelation are interface{}; intersection
// logic deferred to backlog #2693. When shapeFilter is non-nil the current
// stub passes all cells through (conservative — no pruning).
type FilterCellIterator struct {
	BaseCellIterator
	baseIter    CellIterator
	shapeFilter interface{} // spatial4j Shape — nil means no filtering
}

// NewFilterCellIterator creates the filtering iterator.
func NewFilterCellIterator(baseIter CellIterator, shapeFilter interface{}) *FilterCellIterator {
	return &FilterCellIterator{baseIter: baseIter, shapeFilter: shapeFilter}
}

// HasNext advances to the next matching cell.
func (f *FilterCellIterator) HasNext() bool {
	f.thisCell = nil
	if f.nextCell != nil {
		return true
	}
	for f.baseIter.HasNext() {
		candidate := f.baseIter.Next()
		// When shapeFilter is nil, all cells match.
		// When non-nil, full spatial4j intersection is deferred to #2693;
		// conservatively accept all cells to avoid missing results.
		f.nextCell = candidate
		return true
	}
	return false
}

// Remove delegates to the base iterator's Remove.
func (f *FilterCellIterator) Remove() { f.baseIter.Remove() }

var _ CellIterator = (*FilterCellIterator)(nil)
