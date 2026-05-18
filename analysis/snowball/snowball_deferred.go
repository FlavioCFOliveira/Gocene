// Package snowball hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.snowball.
package snowball

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// SnowballPorterFilterFactory mirrors org.apache.lucene.analysis.snowball.SnowballPorterFilterFactory.
type SnowballPorterFilterFactory struct{}

// NewSnowballPorterFilterFactory builds a SnowballPorterFilterFactory.
func NewSnowballPorterFilterFactory() *SnowballPorterFilterFactory { return &SnowballPorterFilterFactory{} }

// SnowballFilter mirrors org.apache.lucene.analysis.snowball.SnowballFilter.
type SnowballFilter struct{}

// NewSnowballFilter builds a SnowballFilter.
func NewSnowballFilter() *SnowballFilter { return &SnowballFilter{} }

