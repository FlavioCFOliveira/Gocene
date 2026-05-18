// Package ta hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.ta.
package ta

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// TamilAnalyzer mirrors org.apache.lucene.analysis.ta.TamilAnalyzer.
type TamilAnalyzer struct{}

// NewTamilAnalyzer builds a TamilAnalyzer.
func NewTamilAnalyzer() *TamilAnalyzer { return &TamilAnalyzer{} }

