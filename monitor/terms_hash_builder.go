// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/util"

// QueryTermFilter is a (field, term) filter backed by per-field BytesRefHash
// tables for efficient O(1) term lookup without string allocation.
//
// Port of org.apache.lucene.monitor.QueryIndex.QueryTermFilter (inner class).
type QueryTermFilter struct {
	// terms maps field → BytesRefHash of term bytes. Uses BytesRefHash
	// instead of map[string]struct{} for compact storage and zero-alloc
	// lookups matching Lucene's BytesRefHash-backed implementation.
	terms map[string]*util.BytesRefHash
}

// NewQueryTermFilter creates an empty QueryTermFilter.
func NewQueryTermFilter() *QueryTermFilter {
	return &QueryTermFilter{terms: make(map[string]*util.BytesRefHash)}
}

// Add records a (field, term) pair in the field's BytesRefHash.
func (f *QueryTermFilter) Add(field string, term *util.BytesRef) {
	if term == nil {
		return
	}
	hash, ok := f.terms[field]
	if !ok {
		hash = util.NewBytesRefHash()
		f.terms[field] = hash
	}
	hash.Add(term)
}

// Test returns true when the (field, term) pair is present in the filter.
// Uses BytesRefHash.Find for O(1) lookup without string conversion.
func (f *QueryTermFilter) Test(field string, term *util.BytesRef) bool {
	if term == nil {
		return false
	}
	hash, ok := f.terms[field]
	if !ok {
		return false
	}
	return hash.Find(term) >= 0
}

// Size returns the total number of unique terms across all fields.
func (f *QueryTermFilter) Size() int {
	total := 0
	for _, hash := range f.terms {
		total += hash.Size()
	}
	return total
}

// Fields returns the number of distinct fields in the filter.
func (f *QueryTermFilter) Fields() int {
	return len(f.terms)
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
