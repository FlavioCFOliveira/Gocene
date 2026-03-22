// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"context"
	"fmt"
)

// CompletionQuery is a query for completion suggestions.
// It is used to retrieve suggestions from a suggester.
type CompletionQuery struct {
	prefix string
	field  string
	num    int
}

// NewCompletionQuery creates a new CompletionQuery.
func NewCompletionQuery(prefix, field string, num int) *CompletionQuery {
	return &CompletionQuery{
		prefix: prefix,
		field:  field,
		num:    num,
	}
}

// GetPrefix returns the completion prefix.
func (q *CompletionQuery) GetPrefix() string {
	return q.prefix
}

// GetField returns the field to complete on.
func (q *CompletionQuery) GetField() string {
	return q.field
}

// GetNum returns the number of suggestions to return.
func (q *CompletionQuery) GetNum() int {
	return q.num
}

// Execute executes this completion query against a suggester.
func (q *CompletionQuery) Execute(ctx context.Context, suggester Suggester) ([]*Suggestion, error) {
	return suggester.Lookup(ctx, q.prefix, q.num)
}

// String returns a string representation of this query.
func (q *CompletionQuery) String() string {
	return fmt.Sprintf("CompletionQuery{prefix=%s, field=%s, num=%d}", q.prefix, q.field, q.num)
}

// Ensure CompletionQuery implements Stringer
var _ fmt.Stringer = (*CompletionQuery)(nil)
