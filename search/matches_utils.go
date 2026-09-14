// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "github.com/FlavioCFOliveira/Gocene/util"

// matchesUtils carries the static functions that aid the implementation of the
// [Matches] and [MatchesIterator] interfaces.
//
// Mirrors the static-only final class org.apache.lucene.search.MatchesUtils of
// Apache Lucene 10.5.0
// (lucene/core/src/java/org/apache/lucene/search/MatchesUtils.java); Go has no
// static members, so the class is rendered as the [MatchesUtils] singleton.
type matchesUtils struct{}

// MatchesUtils is the singleton through which the MatchesUtils static functions
// are reached.
var MatchesUtils = matchesUtils{}

// noTermsMatches renders the anonymous Matches assigned to
// MatchesUtils.MATCH_WITH_NO_TERMS in Java.
type noTermsMatches struct{}

// GetMatches always returns nil: this match carries no term positions.
func (noTermsMatches) GetMatches(string) (MatchesIterator, error) { return nil, nil }

// GetSubMatches returns the empty list, mirroring Collections.emptyList().
func (noTermsMatches) GetSubMatches() []Matches { return nil }

// Iterator returns the empty iterator, mirroring Collections.emptyIterator().
func (noTermsMatches) Iterator() util.Iterator[string] {
	return util.NewSliceIteratorG[string](nil)
}

// MatchWithNoTerms indicates a match with no term positions, for example on a
// Point or DocValues field, or a field indexed as docs and freqs only.
//
// Mirrors MatchesUtils.MATCH_WITH_NO_TERMS. It is a pointer so that the
// identity comparison Java performs in fromSubMatches is preserved exactly.
var MatchWithNoTerms Matches = &noTermsMatches{}

// compositeMatches renders the anonymous Matches returned by
// MatchesUtils.fromSubMatches when more than one sub-match survives the filter.
type compositeMatches struct {
	// sm holds the sub-matches that are not MATCH_WITH_NO_TERMS; it drives
	// getMatches and iterator.
	sm []Matches
	// subMatches holds the unfiltered argument; Java's getSubMatches returns
	// this list, not the filtered one.
	subMatches []Matches
}

// GetMatches amalgamates the sub-matches for the field into one disjunction.
func (c *compositeMatches) GetMatches(field string) (MatchesIterator, error) {
	subIterators := make([]MatchesIterator, 0, len(c.sm))
	for _, m := range c.sm {
		it, err := m.GetMatches(field)
		if err != nil {
			return nil, err
		}
		if it != nil {
			subIterators = append(subIterators, it)
		}
	}
	return disjunctionFromSubIterators(subIterators)
}

// Iterator returns the distinct field names of the sub-matches, in encounter
// order. Mirrors the flatMap/distinct stream Java builds over sm.
func (c *compositeMatches) Iterator() util.Iterator[string] {
	seen := make(map[string]struct{})
	fields := make([]string, 0, len(c.sm))
	for _, m := range c.sm {
		it := m.Iterator()
		for it.HasNext() {
			f := it.Next()
			if _, dup := seen[f]; dup {
				continue
			}
			seen[f] = struct{}{}
			fields = append(fields, f)
		}
	}
	return util.NewSliceIteratorG(fields)
}

// GetSubMatches returns the unfiltered sub-match list handed to FromSubMatches,
// exactly as Java's anonymous class does.
func (c *compositeMatches) GetSubMatches() []Matches { return c.subMatches }

// FromSubMatches amalgamates a collection of [Matches] into a single object.
//
// Mirrors MatchesUtils.fromSubMatches(List<Matches>).
func (matchesUtils) FromSubMatches(subMatches []Matches) Matches {
	if len(subMatches) == 0 {
		return nil
	}
	sm := make([]Matches, 0, len(subMatches))
	for _, m := range subMatches {
		if m != MatchWithNoTerms {
			sm = append(sm, m)
		}
	}
	if len(sm) == 0 {
		return MatchWithNoTerms
	}
	if len(sm) == 1 {
		return sm[0]
	}
	return &compositeMatches{sm: sm, subMatches: subMatches}
}

// singleFieldMatches renders the anonymous Matches returned by
// MatchesUtils.forField.
type singleFieldMatches struct {
	field  string
	mis    util.IOSupplier[MatchesIterator]
	mi     MatchesIterator
	cached bool
}

// GetMatches returns the eagerly produced iterator on the first call for the
// matching field, and a freshly supplied one on every later call.
func (s *singleFieldMatches) GetMatches(field string) (MatchesIterator, error) {
	if s.field != field {
		return nil, nil
	}
	if !s.cached {
		return s.mis()
	}
	s.cached = false
	return s.mi, nil
}

// Iterator returns the single field name, mirroring Collections.singleton.
func (s *singleFieldMatches) Iterator() util.Iterator[string] {
	return util.NewSliceIteratorG([]string{s.field})
}

// GetSubMatches returns the empty list: this is not a composite.
func (s *singleFieldMatches) GetSubMatches() []Matches { return nil }

// ForField creates a [Matches] for a single field.
//
// Mirrors MatchesUtils.forField(String, IOSupplier<MatchesIterator>). The
// indirection through a supplier rather than a MatchesIterator directly is what
// allows multiple calls to GetMatches to return new iterators; the supplier is
// still invoked eagerly here to work out whether there is a hit at all.
func (matchesUtils) ForField(field string, mis util.IOSupplier[MatchesIterator]) (Matches, error) {
	mi, err := mis()
	if err != nil {
		return nil, err
	}
	if mi == nil {
		return nil, nil
	}
	return &singleFieldMatches{field: field, mis: mis, mi: mi, cached: true}, nil
}

// Disjunction creates a [MatchesIterator] that iterates in order over all
// matches in a set of sub-iterators.
//
// Mirrors MatchesUtils.disjunction(List<MatchesIterator>), which delegates to
// DisjunctionMatchesIterator.fromSubIterators.
func (matchesUtils) Disjunction(subMatches []MatchesIterator) (MatchesIterator, error) {
	return disjunctionFromSubIterators(subMatches)
}

var (
	_ Matches = (*noTermsMatches)(nil)
	_ Matches = (*compositeMatches)(nil)
	_ Matches = (*singleFieldMatches)(nil)
)
