// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package surround

import "github.com/FlavioCFOliveira/Gocene/search"

// SrndBooleanQuery provides static helper methods for assembling BooleanQuery
// instances from slices of Lucene queries. It mirrors the static-utility class
// org.apache.lucene.queryparser.surround.query.SrndBooleanQuery.
type SrndBooleanQuery struct{}

// AddQueriesToBoolean appends each query in queries to the BooleanQuery bq
// using the supplied occurrence constraint.
func AddQueriesToBoolean(bq *search.BooleanQuery, queries []search.Query, occur search.Occur) {
	for _, q := range queries {
		bq.Add(q, occur)
	}
}

// MakeBooleanQuery assembles a BooleanQuery from the supplied queries and
// occurrence. The slice must contain at least two queries; passing fewer
// panics following the Java contract ("Too few subqueries").
func MakeBooleanQuery(queries []search.Query, occur search.Occur) search.Query {
	if len(queries) <= 1 {
		panic("surround: too few subqueries for MakeBooleanQuery: " + itoa(len(queries)))
	}
	bq := search.NewBooleanQuery()
	AddQueriesToBoolean(bq, queries, occur)
	return bq
}

// itoa converts a non-negative integer to its decimal string representation
// without importing strconv to keep the dependency footprint minimal.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
