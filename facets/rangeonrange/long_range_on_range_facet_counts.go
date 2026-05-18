package rangeonrange

// LongRangeOnRangeFacetCounts is the int64 counterpart of
// DoubleRangeOnRangeFacetCounts. Mirrors
// org.apache.lucene.facet.rangeonrange.LongRangeOnRangeFacetCounts.
type LongRangeOnRangeFacetCounts struct {
	dim      string
	relation QueryRelation
	ranges   []*LongRange
	counts   []int
	total    int
}

// NewLongRangeOnRangeFacetCounts builds an aggregator.
func NewLongRangeOnRangeFacetCounts(dim string, relation QueryRelation, ranges ...*LongRange) *LongRangeOnRangeFacetCounts {
	return &LongRangeOnRangeFacetCounts{
		dim:      dim,
		relation: relation,
		ranges:   ranges,
		counts:   make([]int, len(ranges)),
	}
}

// Accept observes a document interval and updates counters.
func (c *LongRangeOnRangeFacetCounts) Accept(docMin, docMax int64) {
	c.total++
	for i, r := range c.ranges {
		if c.match(r, docMin, docMax) {
			c.counts[i]++
		}
	}
}

func (c *LongRangeOnRangeFacetCounts) match(r *LongRange, docMin, docMax int64) bool {
	switch c.relation {
	case WithinRelation:
		return r.Contains(docMin, docMax)
	case ContainsRelation:
		return r.Within(docMin, docMax)
	case IntersectsRelation:
		return r.Overlaps(docMin, docMax)
	}
	return false
}

// GetTotalCount returns the number of Accept calls made.
func (c *LongRangeOnRangeFacetCounts) GetTotalCount() int { return c.total }

// GetCounts returns a copy of the per-range counters.
func (c *LongRangeOnRangeFacetCounts) GetCounts() []int {
	out := make([]int, len(c.counts))
	copy(out, c.counts)
	return out
}

// GetDim returns the dimension label.
func (c *LongRangeOnRangeFacetCounts) GetDim() string { return c.dim }
