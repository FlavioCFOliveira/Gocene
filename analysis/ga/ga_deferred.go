// Package ga hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.ga.
package ga

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// IrishAnalyzer mirrors org.apache.lucene.analysis.ga.IrishAnalyzer.
type IrishAnalyzer struct{}

// NewIrishAnalyzer builds a IrishAnalyzer.
func NewIrishAnalyzer() *IrishAnalyzer { return &IrishAnalyzer{} }
