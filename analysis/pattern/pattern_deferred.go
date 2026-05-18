// Package pattern hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.pattern.
package pattern

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// PatternCaptureGroupFilterFactory mirrors org.apache.lucene.analysis.pattern.PatternCaptureGroupFilterFactory.
type PatternCaptureGroupFilterFactory struct{}

// NewPatternCaptureGroupFilterFactory builds a PatternCaptureGroupFilterFactory.
func NewPatternCaptureGroupFilterFactory() *PatternCaptureGroupFilterFactory {
	return &PatternCaptureGroupFilterFactory{}
}

// PatternCaptureGroupTokenFilter mirrors org.apache.lucene.analysis.pattern.PatternCaptureGroupTokenFilter.
type PatternCaptureGroupTokenFilter struct{}

// NewPatternCaptureGroupTokenFilter builds a PatternCaptureGroupTokenFilter.
func NewPatternCaptureGroupTokenFilter() *PatternCaptureGroupTokenFilter {
	return &PatternCaptureGroupTokenFilter{}
}

// PatternTypingFilter mirrors org.apache.lucene.analysis.pattern.PatternTypingFilter.
type PatternTypingFilter struct{}

// NewPatternTypingFilter builds a PatternTypingFilter.
func NewPatternTypingFilter() *PatternTypingFilter { return &PatternTypingFilter{} }

// SimplePatternSplitTokenizerFactory mirrors org.apache.lucene.analysis.pattern.SimplePatternSplitTokenizerFactory.
type SimplePatternSplitTokenizerFactory struct{}

// NewSimplePatternSplitTokenizerFactory builds a SimplePatternSplitTokenizerFactory.
func NewSimplePatternSplitTokenizerFactory() *SimplePatternSplitTokenizerFactory {
	return &SimplePatternSplitTokenizerFactory{}
}

// PatternTypingFilterFactory mirrors org.apache.lucene.analysis.pattern.PatternTypingFilterFactory.
type PatternTypingFilterFactory struct{}

// NewPatternTypingFilterFactory builds a PatternTypingFilterFactory.
func NewPatternTypingFilterFactory() *PatternTypingFilterFactory {
	return &PatternTypingFilterFactory{}
}
