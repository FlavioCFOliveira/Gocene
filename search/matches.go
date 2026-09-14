// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/util"

// Matches reports the positions and optionally offsets of all matching terms in
// a query for a single document.
//
// To obtain a [MatchesIterator] for a particular field, call GetMatches. Note
// that you can call GetMatches multiple times to retrieve new iterators, but it
// is not thread-safe.
//
// Mirrors org.apache.lucene.search.Matches of Apache Lucene 10.5.0
// (lucene/core/src/java/org/apache/lucene/search/Matches.java). Java declares
// the type as "interface Matches extends Iterable<String>"; Go has no Iterable,
// so the inherited iterator() is rendered here as the Iterator method below,
// returning util.Iterator[string] — this port's established spelling of
// java.util.Iterator<T> (util/merged_iterator.go).
type Matches interface {
	// Iterator returns an iterator over the names of the fields that carry
	// matches. Mirrors the iterator() inherited from Iterable<String>.
	Iterator() util.Iterator[string]

	// GetMatches returns a [MatchesIterator] over the matches for a single
	// field, or nil if there are no matches in that field.
	GetMatches(field string) (MatchesIterator, error)

	// GetSubMatches returns the collection of Matches that make up this
	// instance; if it is not a composite, this returns an empty list.
	GetSubMatches() []Matches
}
