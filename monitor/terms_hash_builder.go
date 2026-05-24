// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/util"

// QueryTermFilter is a (field, term) filter backed by an in-memory hash of all
// terms in a query index reader.
//
// Port of org.apache.lucene.monitor.QueryIndex.QueryTermFilter (inner class).
//
// Deviation: the Gocene port is a package-level type (not an inner class) with
// a map[string]*util.BytesRefHash; BytesRefHash integration is a stub.
type QueryTermFilter struct {
	// terms maps field → set of term bytes as strings (simplified until BytesRefHash lands).
	terms map[string]map[string]struct{}
}

// NewQueryTermFilter creates an empty QueryTermFilter.
func NewQueryTermFilter() *QueryTermFilter {
	return &QueryTermFilter{terms: make(map[string]map[string]struct{})}
}

// Add records a (field, term) pair.
func (f *QueryTermFilter) Add(field string, term *util.BytesRef) {
	if _, ok := f.terms[field]; !ok {
		f.terms[field] = make(map[string]struct{})
	}
	if term != nil {
		f.terms[field][string(term.ValidBytes())] = struct{}{}
	}
}

// Test returns true when the (field, term) pair is present in the filter.
func (f *QueryTermFilter) Test(field string, term *util.BytesRef) bool {
	fieldTerms, ok := f.terms[field]
	if !ok {
		return false
	}
	if term == nil {
		return false
	}
	_, found := fieldTerms[string(term.ValidBytes())]
	return found
}

// TermsHashBuilder builds a QueryTermFilter from a reader.
//
// Port of org.apache.lucene.monitor.TermsHashBuilder (package-private in Java).
//
// Deviation: TermsHashBuilder in Java extends SearcherFactory and intercepts
// newSearcher to populate a per-reader filter.  In Gocene, without a live
// index reader, it is a stand-alone builder.  Wiring to a real IndexReader is
// deferred to backlog #2693.
type TermsHashBuilder struct {
	filters map[interface{}]*QueryTermFilter // cacheKey → filter
}

// NewTermsHashBuilder creates a TermsHashBuilder.
func NewTermsHashBuilder() *TermsHashBuilder {
	return &TermsHashBuilder{filters: make(map[interface{}]*QueryTermFilter)}
}

// Register adds a QueryTermFilter for the given cache key.
func (b *TermsHashBuilder) Register(cacheKey interface{}, filter *QueryTermFilter) {
	b.filters[cacheKey] = filter
}

// Get returns the QueryTermFilter for the given cache key, or nil.
func (b *TermsHashBuilder) Get(cacheKey interface{}) *QueryTermFilter {
	return b.filters[cacheKey]
}
