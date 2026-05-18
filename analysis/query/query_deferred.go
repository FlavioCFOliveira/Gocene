// Package query hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.query.
package query

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// QueryAutoStopWordAnalyzer mirrors org.apache.lucene.analysis.query.QueryAutoStopWordAnalyzer.
type QueryAutoStopWordAnalyzer struct{}

// NewQueryAutoStopWordAnalyzer builds a QueryAutoStopWordAnalyzer.
func NewQueryAutoStopWordAnalyzer() *QueryAutoStopWordAnalyzer { return &QueryAutoStopWordAnalyzer{} }

