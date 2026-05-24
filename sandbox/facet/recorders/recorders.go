// Package recorders implements org.apache.lucene.sandbox.facet.recorders.
package recorders

// FacetRecorder is the contract every facet aggregator implements.
type FacetRecorder interface {
	Record(ordinal int)
}

// LeafFacetRecorder is the per-segment variant.
type LeafFacetRecorder interface {
	FacetRecorder
}

// CountFacetRecorder is the basic counter.
type CountFacetRecorder struct {
	Counts map[int]int
}

// NewCountFacetRecorder builds the recorder.
func NewCountFacetRecorder() *CountFacetRecorder {
	return &CountFacetRecorder{Counts: make(map[int]int)}
}

// Record increments the count for ordinal.
func (r *CountFacetRecorder) Record(ordinal int) { r.Counts[ordinal]++ }

var _ FacetRecorder = (*CountFacetRecorder)(nil)

// LongAggregationsFacetRecorder accumulates an int64 aggregate per ordinal.
type LongAggregationsFacetRecorder struct {
	Sums       map[int]int64
	ValueFn    func(docID int) (int64, bool)
	currentDoc int
}

// NewLongAggregationsFacetRecorder builds the recorder.
func NewLongAggregationsFacetRecorder(fn func(docID int) (int64, bool)) *LongAggregationsFacetRecorder {
	return &LongAggregationsFacetRecorder{Sums: make(map[int]int64), ValueFn: fn}
}

// SetDoc switches the active doc.
func (r *LongAggregationsFacetRecorder) SetDoc(docID int) { r.currentDoc = docID }

// Record adds the doc's value to ordinal's sum.
func (r *LongAggregationsFacetRecorder) Record(ordinal int) {
	if r.ValueFn == nil {
		return
	}
	if v, ok := r.ValueFn(r.currentDoc); ok {
		r.Sums[ordinal] += v
	}
}

var _ FacetRecorder = (*LongAggregationsFacetRecorder)(nil)

// MultiFacetsRecorder fans recording out to multiple inner recorders.
type MultiFacetsRecorder struct {
	Recorders []FacetRecorder
}

// NewMultiFacetsRecorder builds the recorder.
func NewMultiFacetsRecorder(recorders ...FacetRecorder) *MultiFacetsRecorder {
	return &MultiFacetsRecorder{Recorders: append([]FacetRecorder(nil), recorders...)}
}

// Record forwards to every recorder.
func (r *MultiFacetsRecorder) Record(ordinal int) {
	for _, rec := range r.Recorders {
		rec.Record(ordinal)
	}
}

var _ FacetRecorder = (*MultiFacetsRecorder)(nil)

// Reducer reduces the per-ordinal counts/aggregates emitted by a recorder.
type Reducer interface {
	Reduce(values []int64) int64
}

// SumReducer returns the sum.
type SumReducer struct{}

// Reduce returns the sum of values.
func (SumReducer) Reduce(values []int64) int64 {
	var s int64
	for _, v := range values {
		s += v
	}
	return s
}

var _ Reducer = SumReducer{}
