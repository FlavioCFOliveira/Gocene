// Package in hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.in.
package in

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// IndicNormalizationFilter mirrors org.apache.lucene.analysis.in.IndicNormalizationFilter.
type IndicNormalizationFilter struct{}

// NewIndicNormalizationFilter builds a IndicNormalizationFilter.
func NewIndicNormalizationFilter() *IndicNormalizationFilter { return &IndicNormalizationFilter{} }

// IndicNormalizationFilterFactory mirrors org.apache.lucene.analysis.in.IndicNormalizationFilterFactory.
type IndicNormalizationFilterFactory struct{}

// NewIndicNormalizationFilterFactory builds a IndicNormalizationFilterFactory.
func NewIndicNormalizationFilterFactory() *IndicNormalizationFilterFactory {
	return &IndicNormalizationFilterFactory{}
}
