// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// DocValuesRewriteMethod rewrites multi-term queries using DocValues.
// This is the Go port of Lucene's rewrite method for DocValues.
type DocValuesRewriteMethod struct {
	// Field to use for DocValues
	field string
}

// NewDocValuesRewriteMethod creates a new DocValuesRewriteMethod.
func NewDocValuesRewriteMethod(field string) *DocValuesRewriteMethod {
	return &DocValuesRewriteMethod{field: field}
}

// Rewrite rewrites the query using DocValues.
func (m *DocValuesRewriteMethod) Rewrite(query *MultiTermQuery, reader IndexReader) (Query, error) {
	// For now, return the query as-is
	// In a full implementation, this would use DocValues for efficient matching
	return query, nil
}

// GetField returns the field for this rewrite method.
func (m *DocValuesRewriteMethod) GetField() string {
	return m.field
}
