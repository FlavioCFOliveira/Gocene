// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// FilterMatchesIterator is a MatchesIterator that delegates all calls to another MatchesIterator.
//
// Mirrors org.apache.lucene.search.FilterMatchesIterator.
type FilterMatchesIterator struct {
	in MatchesIterator
}

// NewFilterMatchesIterator creates a new FilterMatchesIterator.
func NewFilterMatchesIterator(in MatchesIterator) *FilterMatchesIterator {
	return &FilterMatchesIterator{in: in}
}

func (f *FilterMatchesIterator) Next() (bool, error) {
	return f.in.Next()
}

func (f *FilterMatchesIterator) StartPosition() int {
	return f.in.StartPosition()
}

func (f *FilterMatchesIterator) EndPosition() int {
	return f.in.EndPosition()
}

func (f *FilterMatchesIterator) StartOffset() (int, error) {
	return f.in.StartOffset()
}

func (f *FilterMatchesIterator) EndOffset() (int, error) {
	return f.in.EndOffset()
}

func (f *FilterMatchesIterator) GetSubMatches() (MatchesIterator, error) {
	return f.in.GetSubMatches()
}

func (f *FilterMatchesIterator) GetQuery() Query {
	return f.in.GetQuery()
}
