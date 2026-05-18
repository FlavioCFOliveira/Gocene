package rangefacets

// LongRangeFacetCounts is the int64 counterpart of DoubleRangeFacetCounts.
// Mirrors org.apache.lucene.facet.range.LongRangeFacetCounts.
type LongRangeFacetCounts struct {
	dim    string
	ranges []*LongRange
	counts []int
	total  int
}

// NewLongRangeFacetCounts builds an aggregator on the supplied dimension
// label and range list.
func NewLongRangeFacetCounts(dim string, ranges ...*LongRange) *LongRangeFacetCounts {
	return &LongRangeFacetCounts{
		dim:    dim,
		ranges: ranges,
		counts: make([]int, len(ranges)),
	}
}

// Accept observes value v and increments every range whose Accept returns
// true.
func (c *LongRangeFacetCounts) Accept(v int64) {
	c.total++
	for i, r := range c.ranges {
		if r.Accept(v) {
			c.counts[i]++
		}
	}
}

// GetTotalCount returns the number of Accept calls made.
func (c *LongRangeFacetCounts) GetTotalCount() int { return c.total }

// CountForRange returns the count for the range at index i.
func (c *LongRangeFacetCounts) CountForRange(i int) int {
	if i < 0 || i >= len(c.counts) {
		return 0
	}
	return c.counts[i]
}

// GetCounts returns a defensive copy.
func (c *LongRangeFacetCounts) GetCounts() []int {
	out := make([]int, len(c.counts))
	copy(out, c.counts)
	return out
}

// GetDim returns the dimension label.
func (c *LongRangeFacetCounts) GetDim() string { return c.dim }
