// Package th hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.th.
package th

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// ThaiTokenizerFactory mirrors org.apache.lucene.analysis.th.ThaiTokenizerFactory.
type ThaiTokenizerFactory struct{}

// NewThaiTokenizerFactory builds a ThaiTokenizerFactory.
func NewThaiTokenizerFactory() *ThaiTokenizerFactory { return &ThaiTokenizerFactory{} }

// ThaiTokenizer mirrors org.apache.lucene.analysis.th.ThaiTokenizer.
type ThaiTokenizer struct{}

// NewThaiTokenizer builds a ThaiTokenizer.
func NewThaiTokenizer() *ThaiTokenizer { return &ThaiTokenizer{} }
