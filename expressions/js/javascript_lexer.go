// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.JavascriptLexer.
//
// In Apache Lucene 10.4.0 JavascriptLexer is an ANTLR-4 generated lexer.
// In Gocene the ANTLR runtime is not used; lexing is performed inline by the
// hand-written recursive-descent parser in javascript_compiler.go.
// JavascriptLexer is therefore an empty type that serves as an API marker.
package js

// JavascriptLexer is the Go counterpart of the ANTLR-generated
// org.apache.lucene.expressions.js.JavascriptLexer.
// Tokenisation is performed by the hand-written recursive-descent parser
// (javascript_compiler.go) so this type carries no state.
type JavascriptLexer struct{}
