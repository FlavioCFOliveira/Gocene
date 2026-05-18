// Package cjk hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.cjk.
package cjk

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// CJKBigramFilter mirrors org.apache.lucene.analysis.cjk.CJKBigramFilter.
type CJKBigramFilter struct{}

// NewCJKBigramFilter builds a CJKBigramFilter.
func NewCJKBigramFilter() *CJKBigramFilter { return &CJKBigramFilter{} }

// CJKBigramFilterFactory mirrors org.apache.lucene.analysis.cjk.CJKBigramFilterFactory.
type CJKBigramFilterFactory struct{}

// NewCJKBigramFilterFactory builds a CJKBigramFilterFactory.
func NewCJKBigramFilterFactory() *CJKBigramFilterFactory { return &CJKBigramFilterFactory{} }

// CJKWidthCharFilterFactory mirrors org.apache.lucene.analysis.cjk.CJKWidthCharFilterFactory.
type CJKWidthCharFilterFactory struct{}

// NewCJKWidthCharFilterFactory builds a CJKWidthCharFilterFactory.
func NewCJKWidthCharFilterFactory() *CJKWidthCharFilterFactory { return &CJKWidthCharFilterFactory{} }

// CJKWidthFilterFactory mirrors org.apache.lucene.analysis.cjk.CJKWidthFilterFactory.
type CJKWidthFilterFactory struct{}

// NewCJKWidthFilterFactory builds a CJKWidthFilterFactory.
func NewCJKWidthFilterFactory() *CJKWidthFilterFactory { return &CJKWidthFilterFactory{} }

// CJKWidthCharFilter mirrors org.apache.lucene.analysis.cjk.CJKWidthCharFilter.
type CJKWidthCharFilter struct{}

// NewCJKWidthCharFilter builds a CJKWidthCharFilter.
func NewCJKWidthCharFilter() *CJKWidthCharFilter { return &CJKWidthCharFilter{} }

// CJKWidthFilter mirrors org.apache.lucene.analysis.cjk.CJKWidthFilter.
type CJKWidthFilter struct{}

// NewCJKWidthFilter builds a CJKWidthFilter.
func NewCJKWidthFilter() *CJKWidthFilter { return &CJKWidthFilter{} }
