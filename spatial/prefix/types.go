// Package prefix implements org.apache.lucene.spatial.prefix support types.
package prefix

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/spatial/prefixtree"
)

// BytesRefIteratorTokenStream wraps a BytesRefIterator into a TokenStream.
// Mirrors org.apache.lucene.spatial.prefix.BytesRefIteratorTokenStream.
type BytesRefIteratorTokenStream struct {
	source    func() ([]byte, bool)
	exhausted bool
	current   []byte
}

// NewBytesRefIteratorTokenStream wraps a BytesRef iterator (next function).
func NewBytesRefIteratorTokenStream(next func() ([]byte, bool)) *BytesRefIteratorTokenStream {
	return &BytesRefIteratorTokenStream{source: next}
}

// IncrementToken advances the wrapped iterator.
func (s *BytesRefIteratorTokenStream) IncrementToken() (bool, error) {
	if s.exhausted {
		return false, nil
	}
	tok, ok := s.source()
	if !ok {
		s.exhausted = true
		return false, nil
	}
	s.current = tok
	return true, nil
}

// CurrentToken returns the current bytes.
func (s *BytesRefIteratorTokenStream) CurrentToken() []byte { return s.current }

// End is a no-op.
func (s *BytesRefIteratorTokenStream) End() error { return nil }

// Close is a no-op.
func (s *BytesRefIteratorTokenStream) Close() error { return nil }

var _ analysis.TokenStream = (*BytesRefIteratorTokenStream)(nil)

// CellToBytesRefIterator converts a CellIterator into a function-style byte
// iterator. Mirrors
// org.apache.lucene.spatial.prefix.CellToBytesRefIterator.
func CellToBytesRefIterator(iter prefixtree.CellIterator) func() ([]byte, bool) {
	return func() ([]byte, bool) {
		if !iter.HasNext() {
			return nil, false
		}
		return iter.Next().TokenBytes(), true
	}
}

// NumberRangePrefixTreeStrategy is the strategy that indexes 1-D number
// ranges via a NumberRangePrefixTree. Mirrors
// org.apache.lucene.spatial.prefix.NumberRangePrefixTreeStrategy.
type NumberRangePrefixTreeStrategy struct {
	Tree  *prefixtree.NumberRangePrefixTree
	Field string
}

// NewNumberRangePrefixTreeStrategy builds the strategy.
func NewNumberRangePrefixTreeStrategy(tree *prefixtree.NumberRangePrefixTree, field string) *NumberRangePrefixTreeStrategy {
	return &NumberRangePrefixTreeStrategy{Tree: tree, Field: field}
}

// PointPrefixTreeFieldCacheProvider is the FieldCacheProvider tied to a
// point spatial-prefix-tree strategy. Mirrors
// org.apache.lucene.spatial.prefix.PointPrefixTreeFieldCacheProvider.
type PointPrefixTreeFieldCacheProvider struct {
	Field string
}

// NewPointPrefixTreeFieldCacheProvider builds the provider.
func NewPointPrefixTreeFieldCacheProvider(field string) *PointPrefixTreeFieldCacheProvider {
	return &PointPrefixTreeFieldCacheProvider{Field: field}
}

// PrefixTreeFacetCounter counts how many documents fall inside each cell of a
// spatial prefix tree. Mirrors
// org.apache.lucene.spatial.prefix.PrefixTreeFacetCounter.
type PrefixTreeFacetCounter struct {
	counts map[string]int
}

// NewPrefixTreeFacetCounter builds the counter.
func NewPrefixTreeFacetCounter() *PrefixTreeFacetCounter {
	return &PrefixTreeFacetCounter{counts: make(map[string]int)}
}

// Increment registers a hit for the cell token.
func (c *PrefixTreeFacetCounter) Increment(token []byte) {
	c.counts[string(token)]++
}

// Get returns the count for token.
func (c *PrefixTreeFacetCounter) Get(token []byte) int { return c.counts[string(token)] }

// HeatmapFacetCounter aggregates counts into a heatmap grid. Mirrors
// org.apache.lucene.spatial.prefix.HeatmapFacetCounter.
type HeatmapFacetCounter struct {
	Counts []int
	Cols   int
	Rows   int
}

// NewHeatmapFacetCounter builds an empty heatmap.
func NewHeatmapFacetCounter(cols, rows int) *HeatmapFacetCounter {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	return &HeatmapFacetCounter{Counts: make([]int, cols*rows), Cols: cols, Rows: rows}
}

// Increment increases the count at (col, row).
func (c *HeatmapFacetCounter) Increment(col, row int) {
	if col < 0 || col >= c.Cols || row < 0 || row >= c.Rows {
		return
	}
	c.Counts[row*c.Cols+col]++
}

// AbstractPrefixTreeQuery is the shared base of the SPT-based queries.
// Mirrors org.apache.lucene.spatial.prefix.AbstractPrefixTreeQuery.
type AbstractPrefixTreeQuery struct {
	Field  string
	Strategy any
}

// NewAbstractPrefixTreeQuery builds the base.
func NewAbstractPrefixTreeQuery(field string, strategy any) *AbstractPrefixTreeQuery {
	return &AbstractPrefixTreeQuery{Field: field, Strategy: strategy}
}

// AbstractVisitingPrefixTreeQuery extends AbstractPrefixTreeQuery with a
// visitor-style walk. Mirrors
// org.apache.lucene.spatial.prefix.AbstractVisitingPrefixTreeQuery.
type AbstractVisitingPrefixTreeQuery struct {
	*AbstractPrefixTreeQuery
}

// NewAbstractVisitingPrefixTreeQuery builds the visiting variant.
func NewAbstractVisitingPrefixTreeQuery(field string, strategy any) *AbstractVisitingPrefixTreeQuery {
	return &AbstractVisitingPrefixTreeQuery{AbstractPrefixTreeQuery: NewAbstractPrefixTreeQuery(field, strategy)}
}

// WithinPrefixTreeQuery matches documents whose geometry is fully contained
// in the query shape. Mirrors
// org.apache.lucene.spatial.prefix.WithinPrefixTreeQuery.
type WithinPrefixTreeQuery struct {
	*AbstractVisitingPrefixTreeQuery
	BufferDistance float64
}

// NewWithinPrefixTreeQuery builds the query.
func NewWithinPrefixTreeQuery(field string, strategy any, bufferDistance float64) *WithinPrefixTreeQuery {
	return &WithinPrefixTreeQuery{
		AbstractVisitingPrefixTreeQuery: NewAbstractVisitingPrefixTreeQuery(field, strategy),
		BufferDistance:                  bufferDistance,
	}
}

// RecursivePrefixTreeStrategy is the recursive SPT strategy. Mirrors
// org.apache.lucene.spatial.prefix.RecursivePrefixTreeStrategy.
type RecursivePrefixTreeStrategy struct {
	Tree  any
	Field string
}

// NewRecursivePrefixTreeStrategy builds the strategy.
func NewRecursivePrefixTreeStrategy(tree any, field string) *RecursivePrefixTreeStrategy {
	return &RecursivePrefixTreeStrategy{Tree: tree, Field: field}
}

// TermQueryPrefixTreeStrategy is the simpler term-only SPT strategy. Mirrors
// org.apache.lucene.spatial.prefix.TermQueryPrefixTreeStrategy.
type TermQueryPrefixTreeStrategy struct {
	Tree  any
	Field string
}

// NewTermQueryPrefixTreeStrategy builds the strategy.
func NewTermQueryPrefixTreeStrategy(tree any, field string) *TermQueryPrefixTreeStrategy {
	return &TermQueryPrefixTreeStrategy{Tree: tree, Field: field}
}
