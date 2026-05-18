// Package te hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.te.
package te

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// TeluguAnalyzer mirrors org.apache.lucene.analysis.te.TeluguAnalyzer.
type TeluguAnalyzer struct{}

// NewTeluguAnalyzer builds a TeluguAnalyzer.
func NewTeluguAnalyzer() *TeluguAnalyzer { return &TeluguAnalyzer{} }
