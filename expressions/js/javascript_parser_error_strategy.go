// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.JavascriptParserErrorStrategy.
//
// In Apache Lucene 10.4.0 JavascriptParserErrorStrategy overrides the ANTLR 4
// DefaultErrorStrategy to propagate parse errors as RuntimeExceptions
// wrapping ParseExceptions. In Gocene the ANTLR runtime is not used; parse
// errors are propagated as Go error values returned by the hand-written
// recursive-descent parser (javascript_compiler.go). This type is an empty
// marker to preserve API coverage.
package js

// JavascriptParserErrorStrategy is the Go counterpart of
// org.apache.lucene.expressions.js.JavascriptParserErrorStrategy.
// Parse errors are returned as Go errors by the hand-written parser;
// no ANTLR DefaultErrorStrategy machinery is present in Gocene.
type JavascriptParserErrorStrategy struct{}
