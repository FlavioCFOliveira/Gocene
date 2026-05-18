// Package shingle hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.shingle.
package shingle

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// FixedShingleFilter mirrors org.apache.lucene.analysis.shingle.FixedShingleFilter.
type FixedShingleFilter struct{}

// NewFixedShingleFilter builds a FixedShingleFilter.
func NewFixedShingleFilter() *FixedShingleFilter { return &FixedShingleFilter{} }

// FixedShingleFilterFactory mirrors org.apache.lucene.analysis.shingle.FixedShingleFilterFactory.
type FixedShingleFilterFactory struct{}

// NewFixedShingleFilterFactory builds a FixedShingleFilterFactory.
func NewFixedShingleFilterFactory() *FixedShingleFilterFactory { return &FixedShingleFilterFactory{} }

// ShingleAnalyzerWrapper mirrors org.apache.lucene.analysis.shingle.ShingleAnalyzerWrapper.
type ShingleAnalyzerWrapper struct{}

// NewShingleAnalyzerWrapper builds a ShingleAnalyzerWrapper.
func NewShingleAnalyzerWrapper() *ShingleAnalyzerWrapper { return &ShingleAnalyzerWrapper{} }
