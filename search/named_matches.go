// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// NamedMatches wraps a Matches with a name, used to identify which sub-query
// inside a larger compound query produced the match.
//
// Mirrors org.apache.lucene.search.NamedMatches.
type NamedMatches struct {
	name  string
	inner Matches
}

// NewNamedMatches creates a named wrapper around an existing Matches.
func NewNamedMatches(name string, inner Matches) *NamedMatches {
	return &NamedMatches{name: name, inner: inner}
}

// Name returns the assigned name.
func (n *NamedMatches) Name() string { return n.name }

// GetQuery delegates to the wrapped Matches.
func (n *NamedMatches) GetQuery() Query {
	if n.inner == nil {
		return nil
	}
	return n.inner.GetQuery()
}

// GetDocID delegates to the wrapped Matches.
func (n *NamedMatches) GetDocID() int {
	if n.inner == nil {
		return -1
	}
	return n.inner.GetDocID()
}

// GetSubMatches returns the inner Matches wrapped in a slice so the caller can
// continue traversing.
func (n *NamedMatches) GetSubMatches() []Matches {
	if n.inner == nil {
		return nil
	}
	return []Matches{n.inner}
}

// WrapQuery wraps a Query so that any Matches it produces are tagged with the
// given name. Mirrors NamedMatches.wrapQuery.
func WrapQuery(name string, query Query) Query {
	return &namedQuery{name: name, inner: query}
}

type namedQuery struct {
	BaseQuery
	name  string
	inner Query
}

func (q *namedQuery) Name() string  { return q.name }
func (q *namedQuery) Inner() Query  { return q.inner }
func (q *namedQuery) String() string { return "NamedQuery(" + q.name + ")" }

func (q *namedQuery) Equals(other Query) bool {
	o, ok := other.(*namedQuery)
	if !ok {
		return false
	}
	if q.inner == nil || o.inner == nil {
		return q.name == o.name && q.inner == o.inner
	}
	return q.name == o.name && q.inner.Equals(o.inner)
}

func (q *namedQuery) HashCode() int {
	h := 17
	for _, b := range []byte(q.name) {
		h = 31*h + int(b)
	}
	if q.inner != nil {
		h = 31*h + q.inner.HashCode()
	}
	return h
}

func (q *namedQuery) Clone() Query {
	if q.inner == nil {
		return &namedQuery{name: q.name}
	}
	return &namedQuery{name: q.name, inner: q.inner.Clone()}
}

func (q *namedQuery) Rewrite(reader IndexReader) (Query, error) {
	if q.inner == nil {
		return q, nil
	}
	rw, err := q.inner.Rewrite(reader)
	if err != nil {
		return nil, err
	}
	if rw == q.inner {
		return q, nil
	}
	return &namedQuery{name: q.name, inner: rw}, nil
}

func (q *namedQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	if q.inner == nil {
		return nil, nil
	}
	return q.inner.CreateWeight(searcher, needsScores, boost)
}

// FindNamedMatches walks a Matches tree (using GetSubMatches) and returns
// every NamedMatches it finds via breadth-first traversal.
//
// Mirrors NamedMatches.findNamedMatches.
func FindNamedMatches(m Matches) []*NamedMatches {
	if m == nil {
		return nil
	}
	var out []*NamedMatches
	queue := []Matches{m}
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		if nm, ok := head.(*NamedMatches); ok {
			out = append(out, nm)
		}
		queue = append(queue, head.GetSubMatches()...)
	}
	return out
}
