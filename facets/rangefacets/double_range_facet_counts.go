package rangefacets

// DoubleRangeFacetCounts counts how many observed float64 values fall into
// each of the configured DoubleRange buckets. Mirrors
// org.apache.lucene.facet.range.DoubleRangeFacetCounts.
type DoubleRangeFacetCounts struct {
	dim    string
	ranges []*DoubleRange
	counts []int
	total  int
}

// NewDoubleRangeFacetCounts builds an aggregator on the supplied dimension
// label and range list.
func NewDoubleRangeFacetCounts(dim string, ranges ...*DoubleRange) *DoubleRangeFacetCounts {
	return &DoubleRangeFacetCounts{
		dim:    dim,
		ranges: ranges,
		counts: make([]int, len(ranges)),
	}
}

// Accept observes value v and increments every range whose Accept returns
// true. Ranges may overlap; in that case multiple counters advance per call.
func (c *DoubleRangeFacetCounts) Accept(v float64) {
	c.total++
	for i, r := range c.ranges {
		if r.Accept(v) {
			c.counts[i]++
		}
	}
}

// GetTotalCount returns the number of Accept calls made.
func (c *DoubleRangeFacetCounts) GetTotalCount() int { return c.total }

// CountForRange returns the count for the range at index i.
func (c *DoubleRangeFacetCounts) CountForRange(i int) int {
	if i < 0 || i >= len(c.counts) {
		return 0
	}
	return c.counts[i]
}

// GetCounts returns a defensive copy of the per-range counts.
func (c *DoubleRangeFacetCounts) GetCounts() []int {
	out := make([]int, len(c.counts))
	copy(out, c.counts)
	return out
}

// GetDim returns the dimension label.
func (c *DoubleRangeFacetCounts) GetDim() string { return c.dim }
