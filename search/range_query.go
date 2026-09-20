// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// This file had no counterpart in the Apache Lucene 10.5.0 source tree. It
// re-declared TermRangeQuery, NewTermRangeQuery and their accessors, plus a
// termRangeWeight that Lucene's TermRangeQuery does not have at all (it extends
// AutomatonQuery and inherits its weight). The port of
// lucene/core/src/java/org/apache/lucene/search/TermRangeQuery.java lives in
// term_range_query.go, matching that class member for member.

// NewTermRangeQueryWithStrings creates a new TermRangeQuery using strings.
//
// It forwards to NewStringRange, the port of TermRangeQuery.newStringRange.
func NewTermRangeQueryWithStrings(field string, lowerTerm, upperTerm string, includeLower, includeUpper bool) *TermRangeQuery {
	return NewStringRange(field, lowerTerm, upperTerm, includeLower, includeUpper)
}
