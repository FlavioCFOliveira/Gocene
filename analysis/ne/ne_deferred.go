// Package ne hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.ne.
package ne

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// NepaliAnalyzer mirrors org.apache.lucene.analysis.ne.NepaliAnalyzer.
type NepaliAnalyzer struct{}

// NewNepaliAnalyzer builds a NepaliAnalyzer.
func NewNepaliAnalyzer() *NepaliAnalyzer { return &NepaliAnalyzer{} }

