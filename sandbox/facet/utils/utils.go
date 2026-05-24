// Package utils implements org.apache.lucene.sandbox.facet.utils.
package utils

// ComparableUtils hosts the bundled helpers for ordering facet results.
type ComparableUtils struct{}

// CompareInt is the canonical int comparator (a - b sign).
func (ComparableUtils) CompareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// FacetBuilder is the contract every higher-level facet builder implements.
type FacetBuilder interface {
	Build() []FacetEntry
}

// FacetEntry is a (label, value) pair emitted by every builder.
type FacetEntry struct {
	Label string
	Value int64
}

// FacetOrchestrator coordinates a series of FacetBuilders.
type FacetOrchestrator struct {
	Builders []FacetBuilder
}

// NewFacetOrchestrator builds an empty orchestrator.
func NewFacetOrchestrator() *FacetOrchestrator { return &FacetOrchestrator{} }

// Add registers a builder.
func (o *FacetOrchestrator) Add(b FacetBuilder) { o.Builders = append(o.Builders, b) }

// BuildAll runs every registered builder and concatenates the results.
func (o *FacetOrchestrator) BuildAll() [][]FacetEntry {
	out := make([][]FacetEntry, len(o.Builders))
	for i, b := range o.Builders {
		out[i] = b.Build()
	}
	return out
}

// DrillSidewaysFacetOrchestrator is the drill-sideways-aware variant.
type DrillSidewaysFacetOrchestrator struct {
	*FacetOrchestrator
	DrillDownDimensions []string
}

// NewDrillSidewaysFacetOrchestrator builds the orchestrator.
func NewDrillSidewaysFacetOrchestrator(dims []string) *DrillSidewaysFacetOrchestrator {
	return &DrillSidewaysFacetOrchestrator{FacetOrchestrator: NewFacetOrchestrator(), DrillDownDimensions: append([]string(nil), dims...)}
}

// PostCollectionFaceting is the orchestrator used after collection completes.
type PostCollectionFaceting struct {
	*FacetOrchestrator
}

// NewPostCollectionFaceting builds the helper.
func NewPostCollectionFaceting() *PostCollectionFaceting {
	return &PostCollectionFaceting{FacetOrchestrator: NewFacetOrchestrator()}
}

// CommonFacetBuilder is the common label-and-count builder.
type CommonFacetBuilder struct {
	Counts map[string]int64
}

// NewCommonFacetBuilder builds the helper.
func NewCommonFacetBuilder() *CommonFacetBuilder {
	return &CommonFacetBuilder{Counts: make(map[string]int64)}
}

// Increment adds 1 to the count for label.
func (b *CommonFacetBuilder) Increment(label string) { b.Counts[label]++ }

// Build returns the (label, count) entries.
func (b *CommonFacetBuilder) Build() []FacetEntry {
	out := make([]FacetEntry, 0, len(b.Counts))
	for k, v := range b.Counts {
		out = append(out, FacetEntry{Label: k, Value: v})
	}
	return out
}

var _ FacetBuilder = (*CommonFacetBuilder)(nil)

// LongValueFacetBuilder is the variant that accumulates int64 values.
type LongValueFacetBuilder struct {
	Values map[string]int64
}

// NewLongValueFacetBuilder builds the helper.
func NewLongValueFacetBuilder() *LongValueFacetBuilder {
	return &LongValueFacetBuilder{Values: make(map[string]int64)}
}

// Add records value for label.
func (b *LongValueFacetBuilder) Add(label string, value int64) { b.Values[label] += value }

// Build returns the (label, value) entries.
func (b *LongValueFacetBuilder) Build() []FacetEntry {
	out := make([]FacetEntry, 0, len(b.Values))
	for k, v := range b.Values {
		out = append(out, FacetEntry{Label: k, Value: v})
	}
	return out
}

var _ FacetBuilder = (*LongValueFacetBuilder)(nil)

// RangeFacetBuilderFactory builds RangeFacetBuilder instances for the
// supplied ranges.
type RangeFacetBuilderFactory struct {
	Ranges []string
}

// NewRangeFacetBuilderFactory builds the factory.
func NewRangeFacetBuilderFactory(ranges []string) *RangeFacetBuilderFactory {
	return &RangeFacetBuilderFactory{Ranges: append([]string(nil), ranges...)}
}

// Create returns a fresh CommonFacetBuilder pre-populated with the range
// labels.
func (f *RangeFacetBuilderFactory) Create() *CommonFacetBuilder {
	b := NewCommonFacetBuilder()
	for _, r := range f.Ranges {
		b.Counts[r] = 0
	}
	return b
}

// TaxonomyFacetBuilder is the taxonomy-driven CommonFacetBuilder variant.
type TaxonomyFacetBuilder struct {
	*CommonFacetBuilder
	Dim string
}

// NewTaxonomyFacetBuilder builds the helper.
func NewTaxonomyFacetBuilder(dim string) *TaxonomyFacetBuilder {
	return &TaxonomyFacetBuilder{CommonFacetBuilder: NewCommonFacetBuilder(), Dim: dim}
}
