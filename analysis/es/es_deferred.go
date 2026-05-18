// Package es hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.es.
package es

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// SpanishPluralStemFilterFactory mirrors org.apache.lucene.analysis.es.SpanishPluralStemFilterFactory.
type SpanishPluralStemFilterFactory struct{}

// NewSpanishPluralStemFilterFactory builds a SpanishPluralStemFilterFactory.
func NewSpanishPluralStemFilterFactory() *SpanishPluralStemFilterFactory {
	return &SpanishPluralStemFilterFactory{}
}

// SpanishPluralStemFilter mirrors org.apache.lucene.analysis.es.SpanishPluralStemFilter.
type SpanishPluralStemFilter struct{}

// NewSpanishPluralStemFilter builds a SpanishPluralStemFilter.
func NewSpanishPluralStemFilter() *SpanishPluralStemFilter { return &SpanishPluralStemFilter{} }
