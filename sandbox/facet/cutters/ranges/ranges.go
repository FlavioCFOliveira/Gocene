// Package ranges implements org.apache.lucene.sandbox.facet.cutters.ranges.
package ranges

// DoubleRangeFacetCutter slices facets into float64 ranges. Mirrors
// org.apache.lucene.sandbox.facet.cutters.ranges.DoubleRangeFacetCutter.
type DoubleRangeFacetCutter struct {
	Ranges  [][2]float64
	ValueFn func(docID int) (float64, bool)
}

// NewDoubleRangeFacetCutter builds the cutter.
func NewDoubleRangeFacetCutter(ranges [][2]float64, fn func(docID int) (float64, bool)) *DoubleRangeFacetCutter {
	clone := make([][2]float64, len(ranges))
	copy(clone, ranges)
	return &DoubleRangeFacetCutter{Ranges: clone, ValueFn: fn}
}

// Ordinals returns the range indices the doc's value falls in.
func (c *DoubleRangeFacetCutter) Ordinals(docID int) []int {
	if c.ValueFn == nil {
		return nil
	}
	v, ok := c.ValueFn(docID)
	if !ok {
		return nil
	}
	var out []int
	for i, r := range c.Ranges {
		if v >= r[0] && v < r[1] {
			out = append(out, i)
		}
	}
	return out
}

// LongRangeFacetCutter is the int64 counterpart.
type LongRangeFacetCutter struct {
	Ranges  [][2]int64
	ValueFn func(docID int) (int64, bool)
}

// NewLongRangeFacetCutter builds the cutter.
func NewLongRangeFacetCutter(ranges [][2]int64, fn func(docID int) (int64, bool)) *LongRangeFacetCutter {
	clone := make([][2]int64, len(ranges))
	copy(clone, ranges)
	return &LongRangeFacetCutter{Ranges: clone, ValueFn: fn}
}

// Ordinals returns the range indices the doc's value falls in.
func (c *LongRangeFacetCutter) Ordinals(docID int) []int {
	if c.ValueFn == nil {
		return nil
	}
	v, ok := c.ValueFn(docID)
	if !ok {
		return nil
	}
	var out []int
	for i, r := range c.Ranges {
		if v >= r[0] && v < r[1] {
			out = append(out, i)
		}
	}
	return out
}
