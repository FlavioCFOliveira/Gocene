// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package util provides test base infrastructure for the Gocene query parsers.
//
// Port of: queryparser/src/test/.../util/
package util_test

import "testing"

// TestQueryParserTestBase is a marker test acknowledging the port of
// org.apache.lucene.queryparser.util.QueryParserTestBase.
//
// QueryParserTestBase is an abstract JUnit base class (1378 lines) shared by
// both the classic QueryParser and the flexible StandardQueryParser test suites.
// It exercises the full feature set of both parsers: date/numeric range queries,
// fuzzy queries, wildcard, boost, phrase slop, multi-field, analyzer integration,
// and index-round-trip correctness checks.
//
// In Gocene the pattern of abstract base classes does not translate directly;
// shared test logic is instead expressed through helper types and table-driven
// test functions.  Full port is deferred until both the classic QueryParser and
// the flexible StandardQueryParser are feature-complete.
//
// Port of: queryparser/src/test/.../util/QueryParserTestBase.java
func TestQueryParserTestBase(t *testing.T) {
	t.Skip("deferred: abstract base class port requires feature-complete classic QueryParser and StandardQueryParser")
}
