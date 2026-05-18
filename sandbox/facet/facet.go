// Package facet implements org.apache.lucene.sandbox.facet collector entry
// points.
package facet

// FacetFieldCollector is the collector that drives the sandbox facet
// pipeline. Mirrors
// org.apache.lucene.sandbox.facet.FacetFieldCollector.
type FacetFieldCollector struct {
	Field string
}

// NewFacetFieldCollector builds the collector.
func NewFacetFieldCollector(field string) *FacetFieldCollector {
	return &FacetFieldCollector{Field: field}
}

// FacetFieldCollectorManager builds per-segment collectors. Mirrors
// org.apache.lucene.sandbox.facet.FacetFieldCollectorManager.
type FacetFieldCollectorManager struct {
	Field string
}

// NewFacetFieldCollectorManager builds the manager.
func NewFacetFieldCollectorManager(field string) *FacetFieldCollectorManager {
	return &FacetFieldCollectorManager{Field: field}
}

// NewCollector returns a fresh FacetFieldCollector for a new leaf.
func (m *FacetFieldCollectorManager) NewCollector() *FacetFieldCollector {
	return NewFacetFieldCollector(m.Field)
}
