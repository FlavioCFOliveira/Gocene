// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// MultiFieldQueryParser is a query parser that searches across multiple fields.
// This is the Go port of Lucene's org.apache.lucene.queryparser.MultiFieldQueryParser.
type MultiFieldQueryParser struct {
	*QueryParserBase
	fields        []string
	boosts        map[string]float32
	defaultOperator BooleanClauseOccur
}

// BooleanClauseOccur represents the occurrence type for boolean clauses.
type BooleanClauseOccur int

const (
	// BooleanClauseOccurShould means the clause should occur.
	BooleanClauseOccurShould BooleanClauseOccur = iota
	// BooleanClauseOccurMust means the clause must occur.
	BooleanClauseOccurMust
	// BooleanClauseOccurMustNot means the clause must not occur.
	BooleanClauseOccurMustNot
)

// NewMultiFieldQueryParser creates a new MultiFieldQueryParser.
func NewMultiFieldQueryParser(fields []string, analyzer analysis.Analyzer) *MultiFieldQueryParser {
	if len(fields) == 0 {
		fields = []string{"default"}
	}

	return &MultiFieldQueryParser{
		QueryParserBase: NewQueryParserBase(fields[0], analyzer),
		fields:          fields,
		boosts:          make(map[string]float32),
		defaultOperator: BooleanClauseOccurShould,
	}
}

// NewMultiFieldQueryParserWithDefaultField creates a new MultiFieldQueryParser with a default field.
func NewMultiFieldQueryParserWithDefaultField(defaultField string, fields []string, analyzer analysis.Analyzer) *MultiFieldQueryParser {
	mfp := NewMultiFieldQueryParser(fields, analyzer)
	mfp.SetDefaultField(defaultField)
	return mfp
}

// Parse parses a query string and returns a Query that searches across all configured fields.
func (mfp *MultiFieldQueryParser) Parse(queryText string) (search.Query, error) {
	if queryText == "" {
		return mfp.GetMatchAllDocsQuery(), nil
	}

	// Parse the query using the base parser
	// For now, we'll create a simple boolean query across all fields
	bq := search.NewBooleanQuery()

	for _, field := range mfp.fields {
		// Create a query for this field
		query, err := mfp.parseFieldQuery(field, queryText)
		if err != nil {
			return nil, fmt.Errorf("parsing query for field %s: %w", field, err)
		}

		// Apply boost if configured
		if boost, ok := mfp.boosts[field]; ok {
			query = search.NewBoostQuery(query, boost)
		}

		// Add to boolean query
		bq.Add(query, search.SHOULD)
	}

	return bq, nil
}

// ParseWithField parses a query string for a specific field.
func (mfp *MultiFieldQueryParser) ParseWithField(field, queryText string) (search.Query, error) {
	return mfp.parseFieldQuery(field, queryText)
}

// parseFieldQuery parses a query for a specific field.
func (mfp *MultiFieldQueryParser) parseFieldQuery(field, queryText string) (search.Query, error) {
	// Use the base parser to create the query
	// This is a simplified implementation
	if queryText == "" {
		return mfp.GetMatchAllDocsQuery(), nil
	}

	// For simple term queries, create a term query
	// For more complex queries, we'd need a full parser
	return mfp.GetFieldQuery(field, queryText)
}

// GetFields returns the fields being searched.
func (mfp *MultiFieldQueryParser) GetFields() []string {
	return mfp.fields
}

// SetFields sets the fields to search.
func (mfp *MultiFieldQueryParser) SetFields(fields []string) {
	mfp.fields = fields
	if len(fields) > 0 {
		mfp.SetDefaultField(fields[0])
	}
}

// GetBoost returns the boost for a field.
func (mfp *MultiFieldQueryParser) GetBoost(field string) float32 {
	if boost, ok := mfp.boosts[field]; ok {
		return boost
	}
	return 1.0
}

// SetBoost sets the boost for a field.
func (mfp *MultiFieldQueryParser) SetBoost(field string, boost float32) {
	mfp.boosts[field] = boost
}

// GetBoosts returns all field boosts.
func (mfp *MultiFieldQueryParser) GetBoosts() map[string]float32 {
	// Return a copy to prevent external modification
	boosts := make(map[string]float32, len(mfp.boosts))
	for k, v := range mfp.boosts {
		boosts[k] = v
	}
	return boosts
}

// GetDefaultOperator returns the default boolean operator.
func (mfp *MultiFieldQueryParser) GetDefaultOperator() BooleanClauseOccur {
	return mfp.defaultOperator
}

// SetDefaultOperator sets the default boolean operator.
func (mfp *MultiFieldQueryParser) SetDefaultOperator(op BooleanClauseOccur) {
	mfp.defaultOperator = op
}

// AddField adds a field to the search.
func (mfp *MultiFieldQueryParser) AddField(field string) {
	for _, f := range mfp.fields {
		if f == field {
			return // Field already exists
		}
	}
	mfp.fields = append(mfp.fields, field)
}

// RemoveField removes a field from the search.
func (mfp *MultiFieldQueryParser) RemoveField(field string) {
	for i, f := range mfp.fields {
		if f == field {
			mfp.fields = append(mfp.fields[:i], mfp.fields[i+1:]...)
			delete(mfp.boosts, field)
			return
		}
	}
}

// HasField returns whether a field is configured.
func (mfp *MultiFieldQueryParser) HasField(field string) bool {
	for _, f := range mfp.fields {
		if f == field {
			return true
		}
	}
	return false
}

// ClearFields removes all fields.
func (mfp *MultiFieldQueryParser) ClearFields() {
	mfp.fields = nil
	mfp.boosts = make(map[string]float32)
}

// GetFieldCount returns the number of fields.
func (mfp *MultiFieldQueryParser) GetFieldCount() int {
	return len(mfp.fields)
}
