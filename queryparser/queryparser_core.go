// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package queryparser

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryParser is the common interface for all query parsers.
//
// This is the Go port of Lucene's org.apache.lucene.queryparser.classic.QueryParser.
type QueryParser interface {
	// Parse parses the given query string.
	Parse(queryString string) (search.Query, error)
}

// SimpleQueryParser is a simple query parser.
//
// This is the Go port of Lucene's org.apache.lucene.queryparser.simple.SimpleQueryParser.
type SimpleQueryParser struct {
	field string
}

func NewSimpleQueryParser(field string) *SimpleQueryParser {
	return &SimpleQueryParser{
		field: field,
	}
}

func (sqp *SimpleQueryParser) Parse(queryString string) (search.Query, error) {
	// Simplified parsing logic.
	return search.NewTermQuery(sqp.field, queryString), nil
}
