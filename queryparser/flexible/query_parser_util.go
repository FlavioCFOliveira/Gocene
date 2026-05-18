// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// EscapeQueryParserUtil escapes the characters reserved by Lucene's classic
// query syntax (`+ - && || ! ( ) { } [ ] ^ " ~ * ? : \ /` and the keywords
// "AND", "OR", "NOT"). Mirrors org.apache.lucene.queryparser.flexible.standard.QueryParserUtil.escape.
func EscapeQueryParserUtil(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, r := range s {
		switch r {
		case '\\', '+', '-', '!', '(', ')', ':',
			'^', '[', ']', '"', '{', '}', '~',
			'*', '?', '|', '&', '/':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ParseWithFields is the convenience helper mirroring
// QueryParserUtil.parse(String[] queries, String[] fields, BooleanClause.Occur[] flags, Analyzer analyzer):
// each query string is parsed against its corresponding field and the
// resulting Lucene Query is added to a BooleanQuery with the matching Occur.
// Slices must have the same length.
func ParseWithFields(queries, fields []string, occurs []search.Occur, analyzer analysis.Analyzer) (search.Query, error) {
	if len(queries) != len(fields) || len(queries) != len(occurs) {
		return nil, NewQueryNodeError(NewMessageImpl(MsgInvalidSyntax, "queries, fields and occurs must have identical lengths"))
	}
	bq := search.NewBooleanQuery()
	for i, q := range queries {
		parser := NewStandardQueryParser()
		parser.SetDefaultField(fields[i])
		if analyzer != nil {
			parser.SetAnalyzer(analyzer)
		}
		sub, err := parser.Parse(q)
		if err != nil {
			return nil, err
		}
		if sub != nil {
			bq.Add(sub, occurs[i])
		}
	}
	return bq, nil
}

// ParseAcrossFields mirrors QueryParserUtil.parse(String query, String[] fields, BooleanClause.Occur[] flags, Analyzer analyzer):
// the same query string is applied across every field with the matching Occur,
// and the results are OR/AND-combined inside one BooleanQuery.
func ParseAcrossFields(query string, fields []string, occurs []search.Occur, analyzer analysis.Analyzer) (search.Query, error) {
	if len(fields) != len(occurs) {
		return nil, NewQueryNodeError(NewMessageImpl(MsgInvalidSyntax, "fields and occurs must have identical lengths"))
	}
	bq := search.NewBooleanQuery()
	for i, f := range fields {
		parser := NewStandardQueryParser()
		parser.SetDefaultField(f)
		if analyzer != nil {
			parser.SetAnalyzer(analyzer)
		}
		sub, err := parser.Parse(query)
		if err != nil {
			return nil, err
		}
		if sub != nil {
			bq.Add(sub, occurs[i])
		}
	}
	return bq, nil
}
