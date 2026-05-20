// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/TermMatchesIterator.java

import "github.com/FlavioCFOliveira/Gocene/index"

// termMatchesIterator is a MatchesIterator over a single term's postings list.
//
// It iterates term positions within a single document by counting down from
// the term frequency returned by PostingsEnum.Freq at construction time.
//
// The Java original is package-private; the Go port follows the same
// visibility model (unexported).
//
// Ported from org.apache.lucene.search.TermMatchesIterator.
type termMatchesIterator struct {
	pe    index.PostingsEnum
	query Query
	upto  int
	pos   int
}

// newTermMatchesIterator creates a new termMatchesIterator for the given query
// and postings enumeration.  The caller must have already positioned pe on the
// current document; Freq() is called immediately to capture the term frequency
// for the current document.
//
// Mirrors TermMatchesIterator(Query, PostingsEnum) in the Java reference.
func newTermMatchesIterator(query Query, pe index.PostingsEnum) (*termMatchesIterator, error) {
	freq, err := pe.Freq()
	if err != nil {
		return nil, err
	}
	return &termMatchesIterator{
		pe:    pe,
		query: query,
		upto:  freq,
		pos:   -1,
	}, nil
}

// Next advances to the next position match within the current document.
// Returns false when all term positions have been consumed.
func (it *termMatchesIterator) Next() (bool, error) {
	if it.upto <= 0 {
		return false, nil
	}
	it.upto--
	pos, err := it.pe.NextPosition()
	if err != nil {
		return false, err
	}
	it.pos = pos
	return true, nil
}

// StartPosition returns the current term position.
func (it *termMatchesIterator) StartPosition() int { return it.pos }

// EndPosition returns the current term position (term matches are single-token).
func (it *termMatchesIterator) EndPosition() int { return it.pos }

// StartOffset returns the start character offset of the current occurrence.
func (it *termMatchesIterator) StartOffset() (int, error) { return it.pe.StartOffset() }

// EndOffset returns the end character offset of the current occurrence.
func (it *termMatchesIterator) EndOffset() (int, error) { return it.pe.EndOffset() }

// GetSubMatches always returns nil: term matches are atomic and have no
// sub-matches.
func (it *termMatchesIterator) GetSubMatches() (MatchesIterator, error) { return nil, nil }

// GetQuery returns the Query that produced this iterator.
func (it *termMatchesIterator) GetQuery() Query { return it.query }

// Ensure termMatchesIterator implements MatchesIterator.
var _ MatchesIterator = (*termMatchesIterator)(nil)
