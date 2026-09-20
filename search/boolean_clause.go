// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
)

// Occur specifies how clauses are to occur in matching documents.
type Occur int

const (
	// MUST: Use this operator for clauses that must appear in the matching documents.
	MUST Occur = iota
	// FILTER: Like MUST except that these clauses do not participate in scoring.
	FILTER
	// SHOULD: Use this operator for clauses that should appear in the matching documents.
	// For a BooleanQuery with no MUST clauses one or more SHOULD clauses must
	// match a document for the BooleanQuery to match.
	SHOULD
	// MUST_NOT: Use this operator for clauses that must not appear in the matching documents.
	// These clauses do not contribute to the score of documents.
	MUST_NOT
)

// String returns the string representation of the Occur operator.
func (o Occur) String() string {
	switch o {
	case MUST:
		return "+"
	case FILTER:
		return "#"
	case SHOULD:
		return ""
	case MUST_NOT:
		return "-"
	default:
		return fmt.Sprintf("Occur(%d)", o)
	}
}

// BooleanClause is a clause in a BooleanQuery.
type BooleanClause struct {
	query Query
	occur Occur
}

// NewBooleanClause constructs a BooleanClause.
func NewBooleanClause(query Query, occur Occur) *BooleanClause {
	if query == nil {
		panic("Query must not be null")
	}
	// Occur is a value type (int), so it cannot be null.
	return &BooleanClause{
		query: query,
		occur: occur,
	}
}

// Query returns the query associated with this clause.
func (bc *BooleanClause) Query() Query {
	return bc.query
}

// Occur returns the occurrence operator for this clause.
func (bc *BooleanClause) Occur() Occur {
	return bc.occur
}

// IsProhibited returns true if the clause is a MUST_NOT clause.
func (bc *BooleanClause) IsProhibited() bool {
	return bc.occur == MUST_NOT
}

// IsRequired returns true if the clause is either a MUST or a FILTER clause.
func (bc *BooleanClause) IsRequired() bool {
	return bc.occur == MUST || bc.occur == FILTER
}

// IsScoring returns true if the clause is either a MUST or a SHOULD clause.
func (bc *BooleanClause) IsScoring() bool {
	return bc.occur == MUST || bc.occur == SHOULD
}
