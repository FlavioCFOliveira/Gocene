// Package charfilter hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.charfilter.
package charfilter

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// BaseCharFilter mirrors org.apache.lucene.analysis.charfilter.BaseCharFilter.
type BaseCharFilter struct{}

// NewBaseCharFilter builds a BaseCharFilter.
func NewBaseCharFilter() *BaseCharFilter { return &BaseCharFilter{} }
