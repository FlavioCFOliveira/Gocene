package prefixtree

// NumberRangePrefixTree is the 1-D number-range prefix tree used as the base
// for DateRangePrefixTree. Mirrors
// org.apache.lucene.spatial.prefix.tree.NumberRangePrefixTree.
type NumberRangePrefixTree struct {
	MaxLevels int
}

// NewNumberRangePrefixTree builds the tree.
func NewNumberRangePrefixTree(maxLevels int) *NumberRangePrefixTree {
	if maxLevels < 1 {
		maxLevels = 1
	}
	return &NumberRangePrefixTree{MaxLevels: maxLevels}
}

// DateRangePrefixTree is the date-typed specialisation. Mirrors
// org.apache.lucene.spatial.prefix.tree.DateRangePrefixTree.
type DateRangePrefixTree struct {
	*NumberRangePrefixTree
}

// NewDateRangePrefixTree builds the tree.
func NewDateRangePrefixTree(maxLevels int) *DateRangePrefixTree {
	return &DateRangePrefixTree{NumberRangePrefixTree: NewNumberRangePrefixTree(maxLevels)}
}
