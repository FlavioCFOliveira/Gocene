// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// MatchWithNoTerms is the canonical Matches value used for queries that match
// the document but expose no term positions (e.g. Point or DocValues fields,
// or fields indexed with docs/freqs only).
//
// Mirrors MatchesUtils.MATCH_WITH_NO_TERMS in Lucene.
var MatchWithNoTerms Matches = noTermMatches{}

type noTermMatches struct{}

func (noTermMatches) GetQuery() Query          { return nil }
func (noTermMatches) GetDocID() int            { return -1 }
func (noTermMatches) GetSubMatches() []Matches { return nil }

// FromSubMatches consolidates a list of Matches into a single Matches.
// Nil entries are filtered out. If the resulting list is empty, nil is
// returned. If exactly one match remains, it is returned unchanged.
//
// Mirrors MatchesUtils.fromSubMatches.
func FromSubMatches(subs []Matches) Matches {
	out := make([]Matches, 0, len(subs))
	for _, m := range subs {
		if m == nil {
			continue
		}
		out = append(out, m)
	}
	switch len(out) {
	case 0:
		return nil
	case 1:
		return out[0]
	default:
		return &compositeMatches{subs: out}
	}
}

type compositeMatches struct {
	subs []Matches
}

func (c *compositeMatches) GetQuery() Query {
	if len(c.subs) == 0 {
		return nil
	}
	return c.subs[0].GetQuery()
}

func (c *compositeMatches) GetDocID() int {
	if len(c.subs) == 0 {
		return -1
	}
	return c.subs[0].GetDocID()
}

func (c *compositeMatches) GetSubMatches() []Matches {
	return append([]Matches(nil), c.subs...)
}

// DisjunctionMatchesIterator returns a MatchesIterator over the union of the
// given iterators. Matches are produced in the order each sub iterator yields
// them.
//
// Mirrors MatchesUtils.disjunction (best-effort port — Lucene's variant uses a
// priority queue; this implementation walks subs sequentially which preserves
// correctness if downstream code only needs match existence/count).
func DisjunctionMatchesIterator(iters []MatchesIterator) MatchesIterator {
	if len(iters) == 0 {
		return nil
	}
	if len(iters) == 1 {
		return iters[0]
	}
	return &disjunctionMatchesIter{subs: iters, current: -1}
}

type disjunctionMatchesIter struct {
	subs    []MatchesIterator
	current int
}

func (d *disjunctionMatchesIter) Next() (bool, error) {
	if d.current >= 0 {
		ok, err := d.subs[d.current].Next()
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	for i := d.current + 1; i < len(d.subs); i++ {
		ok, err := d.subs[i].Next()
		if err != nil {
			return false, err
		}
		if ok {
			d.current = i
			return true, nil
		}
	}
	return false, nil
}

func (d *disjunctionMatchesIter) StartPosition() int {
	if d.current < 0 {
		return -1
	}
	return d.subs[d.current].StartPosition()
}

func (d *disjunctionMatchesIter) EndPosition() int {
	if d.current < 0 {
		return -1
	}
	return d.subs[d.current].EndPosition()
}

func (d *disjunctionMatchesIter) StartOffset() (int, error) {
	if d.current < 0 {
		return -1, nil
	}
	return d.subs[d.current].StartOffset()
}

func (d *disjunctionMatchesIter) EndOffset() (int, error) {
	if d.current < 0 {
		return -1, nil
	}
	return d.subs[d.current].EndOffset()
}

func (d *disjunctionMatchesIter) GetSubMatches() (MatchesIterator, error) {
	if d.current < 0 {
		return nil, nil
	}
	return d.subs[d.current].GetSubMatches()
}

func (d *disjunctionMatchesIter) GetQuery() Query {
	if d.current < 0 {
		return nil
	}
	return d.subs[d.current].GetQuery()
}
