// Package classic hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.classic.
package classic

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// ClassicFilter mirrors org.apache.lucene.analysis.classic.ClassicFilter.
type ClassicFilter struct{}

// NewClassicFilter builds a ClassicFilter.
func NewClassicFilter() *ClassicFilter { return &ClassicFilter{} }

// ClassicFilterFactory mirrors org.apache.lucene.analysis.classic.ClassicFilterFactory.
type ClassicFilterFactory struct{}

// NewClassicFilterFactory builds a ClassicFilterFactory.
func NewClassicFilterFactory() *ClassicFilterFactory { return &ClassicFilterFactory{} }

// ClassicTokenizer mirrors org.apache.lucene.analysis.classic.ClassicTokenizer.
type ClassicTokenizer struct{}

// NewClassicTokenizer builds a ClassicTokenizer.
func NewClassicTokenizer() *ClassicTokenizer { return &ClassicTokenizer{} }

// ClassicTokenizerFactory mirrors org.apache.lucene.analysis.classic.ClassicTokenizerFactory.
type ClassicTokenizerFactory struct{}

// NewClassicTokenizerFactory builds a ClassicTokenizerFactory.
func NewClassicTokenizerFactory() *ClassicTokenizerFactory { return &ClassicTokenizerFactory{} }

// ClassicAnalyzer mirrors org.apache.lucene.analysis.classic.ClassicAnalyzer.
type ClassicAnalyzer struct{}

// NewClassicAnalyzer builds a ClassicAnalyzer.
func NewClassicAnalyzer() *ClassicAnalyzer { return &ClassicAnalyzer{} }

