// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spans_test

import "testing"

// TestSpanQueryParser is a port of
// org.apache.lucene.queryparser.flexible.spans.TestSpanQueryParser.
//
// The Java test exercises the full spans extension of the flexible query
// parser: parsing, validation, and building SpanOrQuery / SpanTermQuery trees.
//
// Execution is deferred because SpanOrQuery and SpanTermQuery in
// queries/spans are stubs; the full pipeline cannot be validated until they
// have functional implementations.
//
// Port of: queryparser/src/test/.../flexible/spans/TestSpanQueryParser.java
func TestSpanQueryParser(t *testing.T) {
	t.Fatal("deferred: requires functional SpanOrQuery/SpanTermQuery implementations")
}

// TestSpanQueryParserSimpleSample is a port of
// org.apache.lucene.queryparser.flexible.spans.TestSpanQueryParserSimpleSample.
//
// The Java test is a simplified demonstration of building a custom span query
// parser pipeline using the flexible framework.
//
// Execution is deferred for the same reasons as TestSpanQueryParser.
//
// Port of: queryparser/src/test/.../flexible/spans/TestSpanQueryParserSimpleSample.java
func TestSpanQueryParserSimpleSample(t *testing.T) {
	t.Fatal("deferred: requires functional SpanOrQuery/SpanTermQuery implementations")
}
