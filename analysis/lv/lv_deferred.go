// Package lv hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.lv.
package lv

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// LatvianStemFilterFactory mirrors org.apache.lucene.analysis.lv.LatvianStemFilterFactory.
type LatvianStemFilterFactory struct{}

// NewLatvianStemFilterFactory builds a LatvianStemFilterFactory.
func NewLatvianStemFilterFactory() *LatvianStemFilterFactory { return &LatvianStemFilterFactory{} }

// LatvianStemFilter mirrors org.apache.lucene.analysis.lv.LatvianStemFilter.
type LatvianStemFilter struct{}

// NewLatvianStemFilter builds a LatvianStemFilter.
func NewLatvianStemFilter() *LatvianStemFilter { return &LatvianStemFilter{} }

