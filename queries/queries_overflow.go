// Package queries hosts the Sprint 29 overflow ports for
// org.apache.lucene.queries.
package queries

// The Sprint 29 queries-module overflow surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// CommonTermsQuery mirrors org.apache.lucene.queries.CommonTermsQuery.
type CommonTermsQuery struct{}

// NewCommonTermsQuery builds a CommonTermsQuery.
func NewCommonTermsQuery() *CommonTermsQuery { return &CommonTermsQuery{} }

