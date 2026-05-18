// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import "github.com/FlavioCFOliveira/Gocene/search"

// ParentsChildrenBlockJoinQuery is the multi-parent variant of
// ParentChildrenBlockJoinQuery: it returns the union of every child matching
// the child-side query whose parent matches the parents-side query. Mirrors
// org.apache.lucene.search.join.ParentsChildrenBlockJoinQuery.
type ParentsChildrenBlockJoinQuery struct {
	parentsFilter BitSetProducer
	parentQuery   search.Query
	childQuery    search.Query
}

// NewParentsChildrenBlockJoinQuery builds the query.
func NewParentsChildrenBlockJoinQuery(parents BitSetProducer, parentQuery, childQuery search.Query) *ParentsChildrenBlockJoinQuery {
	return &ParentsChildrenBlockJoinQuery{
		parentsFilter: parents,
		parentQuery:   parentQuery,
		childQuery:    childQuery,
	}
}

// GetParentsFilter returns the parent-bit-set producer.
func (q *ParentsChildrenBlockJoinQuery) GetParentsFilter() BitSetProducer {
	return q.parentsFilter
}

// GetParentQuery returns the parent-side query.
func (q *ParentsChildrenBlockJoinQuery) GetParentQuery() search.Query { return q.parentQuery }

// GetChildQuery returns the child-side query.
func (q *ParentsChildrenBlockJoinQuery) GetChildQuery() search.Query { return q.childQuery }
