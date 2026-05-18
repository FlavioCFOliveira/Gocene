// Package mlt hosts the Sprint 29 overflow ports for
// org.apache.lucene.queries.mlt.
package mlt

// The Sprint 29 queries-module overflow surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// land progressively.

// MoreLikeThis mirrors org.apache.lucene.queries.mlt.MoreLikeThis.
type MoreLikeThis struct{}

// NewMoreLikeThis builds a MoreLikeThis.
func NewMoreLikeThis() *MoreLikeThis { return &MoreLikeThis{} }

// MoreLikeThisQuery mirrors org.apache.lucene.queries.mlt.MoreLikeThisQuery.
type MoreLikeThisQuery struct{}

// NewMoreLikeThisQuery builds a MoreLikeThisQuery.
func NewMoreLikeThisQuery() *MoreLikeThisQuery { return &MoreLikeThisQuery{} }

