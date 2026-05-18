package rangeonrange

// QueryRelation matches Lucene's org.apache.lucene.document.RangeFieldQuery.QueryRelation
// — the policy used by rangeonrange counters to decide whether a document's
// [min, max] interval matches a configured DoubleRange.
type QueryRelation int

const (
	// WithinRelation: doc range must be contained inside the bucket range.
	WithinRelation QueryRelation = iota
	// ContainsRelation: doc range must contain the bucket range.
	ContainsRelation
	// IntersectsRelation: doc range must overlap the bucket range.
	IntersectsRelation
)

// DoubleRangeOnRangeFacetCounts counts how many document intervals match
// each configured DoubleRange under the supplied QueryRelation. Mirrors
// org.apache.lucene.facet.rangeonrange.DoubleRangeOnRangeFacetCounts.
type DoubleRangeOnRangeFacetCounts struct {
	dim       string
	relation  QueryRelation
	ranges    []*DoubleRange
	counts    []int
	total     int
}

// NewDoubleRangeOnRangeFacetCounts builds an aggregator on the supplied
// dimension label, relation, and range list.
func NewDoubleRangeOnRangeFacetCounts(dim string, relation QueryRelation, ranges ...*DoubleRange) *DoubleRangeOnRangeFacetCounts {
	return &DoubleRangeOnRangeFacetCounts{
		dim:      dim,
		relation: relation,
		ranges:   ranges,
		counts:   make([]int, len(ranges)),
	}
}

// Accept observes a document interval and increments the counter for every
// configured range that matches under the active relation.
func (c *DoubleRangeOnRangeFacetCounts) Accept(docMin, docMax float64) {
	c.total++
	for i, r := range c.ranges {
		if c.match(r, docMin, docMax) {
			c.counts[i]++
		}
	}
}

func (c *DoubleRangeOnRangeFacetCounts) match(r *DoubleRange, docMin, docMax float64) bool {
	switch c.relation {
	case WithinRelation:
		// The doc's interval must sit inside the bucket range.
		return r.Contains(docMin, docMax)
	case ContainsRelation:
		// The bucket range must sit inside the doc's interval.
		return r.Within(docMin, docMax)
	case IntersectsRelation:
		return r.Overlaps(docMin, docMax)
	}
	return false
}

// GetTotalCount returns the number of Accept calls made.
func (c *DoubleRangeOnRangeFacetCounts) GetTotalCount() int { return c.total }

// GetCounts returns a copy of the per-range counters.
func (c *DoubleRangeOnRangeFacetCounts) GetCounts() []int {
	out := make([]int, len(c.counts))
	copy(out, c.counts)
	return out
}

// GetDim returns the dimension label.
func (c *DoubleRangeOnRangeFacetCounts) GetDim() string { return c.dim }
