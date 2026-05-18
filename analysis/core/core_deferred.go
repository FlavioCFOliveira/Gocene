// Package core hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.core.
package core

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// UnicodeWhitespaceAnalyzer mirrors org.apache.lucene.analysis.core.UnicodeWhitespaceAnalyzer.
type UnicodeWhitespaceAnalyzer struct{}

// NewUnicodeWhitespaceAnalyzer builds a UnicodeWhitespaceAnalyzer.
func NewUnicodeWhitespaceAnalyzer() *UnicodeWhitespaceAnalyzer { return &UnicodeWhitespaceAnalyzer{} }

// UnicodeWhitespaceTokenizer mirrors org.apache.lucene.analysis.core.UnicodeWhitespaceTokenizer.
type UnicodeWhitespaceTokenizer struct{}

// NewUnicodeWhitespaceTokenizer builds a UnicodeWhitespaceTokenizer.
func NewUnicodeWhitespaceTokenizer() *UnicodeWhitespaceTokenizer { return &UnicodeWhitespaceTokenizer{} }

