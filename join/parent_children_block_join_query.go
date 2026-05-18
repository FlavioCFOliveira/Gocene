// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import "github.com/FlavioCFOliveira/Gocene/search"

// ParentChildrenBlockJoinQuery is a join query that returns each parent
// together with the union of its children matching a child-side query.
// Mirrors org.apache.lucene.search.join.ParentChildrenBlockJoinQuery.
type ParentChildrenBlockJoinQuery struct {
	parentsFilter BitSetProducer
	childQuery    search.Query
	parentDocID   int
}

// NewParentChildrenBlockJoinQuery builds the query for the supplied parent
// filter, child query, and pinned parent document.
func NewParentChildrenBlockJoinQuery(parents BitSetProducer, child search.Query, parentDocID int) *ParentChildrenBlockJoinQuery {
	return &ParentChildrenBlockJoinQuery{
		parentsFilter: parents,
		childQuery:    child,
		parentDocID:   parentDocID,
	}
}

// GetParentsFilter returns the BitSetProducer that identifies parent docs.
func (q *ParentChildrenBlockJoinQuery) GetParentsFilter() BitSetProducer {
	return q.parentsFilter
}

// GetChildQuery returns the child-side query.
func (q *ParentChildrenBlockJoinQuery) GetChildQuery() search.Query { return q.childQuery }

// GetParentDocID returns the parent doc whose children should be returned.
func (q *ParentChildrenBlockJoinQuery) GetParentDocID() int { return q.parentDocID }
