// Package miscellaneous hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.miscellaneous.
package miscellaneous

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// ConcatenateGraphFilter mirrors org.apache.lucene.analysis.miscellaneous.ConcatenateGraphFilter.
type ConcatenateGraphFilter struct{}

// NewConcatenateGraphFilter builds a ConcatenateGraphFilter.
func NewConcatenateGraphFilter() *ConcatenateGraphFilter { return &ConcatenateGraphFilter{} }

// ConcatenateGraphFilterFactory mirrors org.apache.lucene.analysis.miscellaneous.ConcatenateGraphFilterFactory.
type ConcatenateGraphFilterFactory struct{}

// NewConcatenateGraphFilterFactory builds a ConcatenateGraphFilterFactory.
func NewConcatenateGraphFilterFactory() *ConcatenateGraphFilterFactory { return &ConcatenateGraphFilterFactory{} }

// ConcatenatingTokenStream mirrors org.apache.lucene.analysis.miscellaneous.ConcatenatingTokenStream.
type ConcatenatingTokenStream struct{}

// NewConcatenatingTokenStream builds a ConcatenatingTokenStream.
func NewConcatenatingTokenStream() *ConcatenatingTokenStream { return &ConcatenatingTokenStream{} }

// ProtectedTermFilter mirrors org.apache.lucene.analysis.miscellaneous.ProtectedTermFilter.
type ProtectedTermFilter struct{}

// NewProtectedTermFilter builds a ProtectedTermFilter.
func NewProtectedTermFilter() *ProtectedTermFilter { return &ProtectedTermFilter{} }

// ProtectedTermFilterFactory mirrors org.apache.lucene.analysis.miscellaneous.ProtectedTermFilterFactory.
type ProtectedTermFilterFactory struct{}

// NewProtectedTermFilterFactory builds a ProtectedTermFilterFactory.
func NewProtectedTermFilterFactory() *ProtectedTermFilterFactory { return &ProtectedTermFilterFactory{} }

// StemmerOverrideFilter mirrors org.apache.lucene.analysis.miscellaneous.StemmerOverrideFilter.
type StemmerOverrideFilter struct{}

// NewStemmerOverrideFilter builds a StemmerOverrideFilter.
func NewStemmerOverrideFilter() *StemmerOverrideFilter { return &StemmerOverrideFilter{} }

// StemmerOverrideFilterFactory mirrors org.apache.lucene.analysis.miscellaneous.StemmerOverrideFilterFactory.
type StemmerOverrideFilterFactory struct{}

// NewStemmerOverrideFilterFactory builds a StemmerOverrideFilterFactory.
func NewStemmerOverrideFilterFactory() *StemmerOverrideFilterFactory { return &StemmerOverrideFilterFactory{} }

