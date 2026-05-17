// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// IndriQuery is the abstract base for IndriAndQuery and its peers. It holds a
// list of BooleanClauses and provides string/equality/visit helpers.
//
// Mirrors org.apache.lucene.search.IndriQuery. Concrete subclasses override
// CreateWeight to produce the IndriAnd/Indri scorers.
type IndriQuery struct {
	BaseQuery
	clauses []*BooleanClause
}

// NewIndriQuery builds an IndriQuery from clauses. The slice is copied so the
// caller cannot mutate the query after construction.
func NewIndriQuery(clauses []*BooleanClause) *IndriQuery {
	cp := append([]*BooleanClause(nil), clauses...)
	return &IndriQuery{clauses: cp}
}

// Clauses returns the list of clauses (do not mutate).
func (q *IndriQuery) Clauses() []*BooleanClause { return q.clauses }

// Iterator returns a callback-style iterator over clauses.
func (q *IndriQuery) Iterator(fn func(*BooleanClause) bool) {
	for _, c := range q.clauses {
		if !fn(c) {
			return
		}
	}
}

// String returns the canonical Indri "(clause...)" rendering.
func (q *IndriQuery) String() string {
	out := "("
	for i, c := range q.clauses {
		if i > 0 {
			out += " "
		}
		out += sprintQuery(c.Query)
	}
	out += ")"
	return out
}

// Equals checks structural equality across the clause list.
func (q *IndriQuery) Equals(other Query) bool {
	o, ok := other.(*IndriQuery)
	if !ok {
		return false
	}
	if len(q.clauses) != len(o.clauses) {
		return false
	}
	for i, c := range q.clauses {
		if !c.Query.Equals(o.clauses[i].Query) || c.Occur != o.clauses[i].Occur {
			return false
		}
	}
	return true
}

// HashCode hashes the clauses by query+occur.
func (q *IndriQuery) HashCode() int {
	h := 17
	for _, c := range q.clauses {
		h = 31*h + c.Query.HashCode() + int(c.Occur)
	}
	return h
}

// Clone returns a deep copy.
func (q *IndriQuery) Clone() Query {
	cp := make([]*BooleanClause, len(q.clauses))
	for i, c := range q.clauses {
		cp[i] = &BooleanClause{Query: c.Query.Clone(), Occur: c.Occur}
	}
	return &IndriQuery{clauses: cp}
}

// Rewrite rewrites each clause's inner query.
func (q *IndriQuery) Rewrite(reader IndexReader) (Query, error) {
	changed := false
	out := make([]*BooleanClause, len(q.clauses))
	for i, c := range q.clauses {
		rw, err := c.Query.Rewrite(reader)
		if err != nil {
			return nil, err
		}
		out[i] = &BooleanClause{Query: rw, Occur: c.Occur}
		if rw != c.Query {
			changed = true
		}
	}
	if !changed {
		return q, nil
	}
	return &IndriQuery{clauses: out}, nil
}

// CreateWeight returns nil by default; concrete subclasses (IndriAndQuery)
// override this to produce a real Weight.
func (q *IndriQuery) CreateWeight(searcher *IndexSearcher, needsScores bool, boost float32) (Weight, error) {
	return nil, nil
}
