// Package ckb hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.ckb.
package ckb

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// SoraniAnalyzer mirrors org.apache.lucene.analysis.ckb.SoraniAnalyzer.
type SoraniAnalyzer struct{}

// NewSoraniAnalyzer builds a SoraniAnalyzer.
func NewSoraniAnalyzer() *SoraniAnalyzer { return &SoraniAnalyzer{} }

