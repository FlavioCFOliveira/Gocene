// Package commongrams hosts the deferred Sprint 28 ports for
// org.apache.lucene.analysis.commongrams.
package commongrams

// The Sprint 28 analysis-common deferral surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// CommonGramsFilter mirrors org.apache.lucene.analysis.commongrams.CommonGramsFilter.
type CommonGramsFilter struct{}

// NewCommonGramsFilter builds a CommonGramsFilter.
func NewCommonGramsFilter() *CommonGramsFilter { return &CommonGramsFilter{} }

// CommonGramsQueryFilterFactory mirrors org.apache.lucene.analysis.commongrams.CommonGramsQueryFilterFactory.
type CommonGramsQueryFilterFactory struct{}

// NewCommonGramsQueryFilterFactory builds a CommonGramsQueryFilterFactory.
func NewCommonGramsQueryFilterFactory() *CommonGramsQueryFilterFactory {
	return &CommonGramsQueryFilterFactory{}
}

// CommonGramsQueryFilter mirrors org.apache.lucene.analysis.commongrams.CommonGramsQueryFilter.
type CommonGramsQueryFilter struct{}

// NewCommonGramsQueryFilter builds a CommonGramsQueryFilter.
func NewCommonGramsQueryFilter() *CommonGramsQueryFilter { return &CommonGramsQueryFilter{} }

// CommonGramsFilterFactory mirrors org.apache.lucene.analysis.commongrams.CommonGramsFilterFactory.
type CommonGramsFilterFactory struct{}

// NewCommonGramsFilterFactory builds a CommonGramsFilterFactory.
func NewCommonGramsFilterFactory() *CommonGramsFilterFactory { return &CommonGramsFilterFactory{} }
