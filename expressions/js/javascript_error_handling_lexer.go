// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.JavascriptErrorHandlingLexer.
//
// In Apache Lucene 10.4.0 JavascriptErrorHandlingLexer extends the
// ANTLR-generated JavascriptLexer to propagate lexer errors as
// RuntimeExceptions wrapping ParseExceptions. In Gocene the ANTLR runtime
// is not used; lexer errors are handled inline by the hand-written
// recursive-descent parser (javascript_compiler.go). This type is an empty
// marker to preserve API coverage.
package js

// JavascriptErrorHandlingLexer is the Go counterpart of
// org.apache.lucene.expressions.js.JavascriptErrorHandlingLexer.
// Lexer errors are returned as Go errors by the hand-written parser;
// no ANTLR lexer error handling is present in Gocene.
type JavascriptErrorHandlingLexer struct {
	JavascriptLexer
}
