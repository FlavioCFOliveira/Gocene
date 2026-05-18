// Package gl hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.gl.
package gl

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// GalicianMinimalStemFilterFactory mirrors org.apache.lucene.analysis.gl.GalicianMinimalStemFilterFactory.
type GalicianMinimalStemFilterFactory struct{}

// NewGalicianMinimalStemFilterFactory builds a GalicianMinimalStemFilterFactory.
func NewGalicianMinimalStemFilterFactory() *GalicianMinimalStemFilterFactory { return &GalicianMinimalStemFilterFactory{} }

// GalicianMinimalStemFilter mirrors org.apache.lucene.analysis.gl.GalicianMinimalStemFilter.
type GalicianMinimalStemFilter struct{}

// NewGalicianMinimalStemFilter builds a GalicianMinimalStemFilter.
func NewGalicianMinimalStemFilter() *GalicianMinimalStemFilter { return &GalicianMinimalStemFilter{} }

