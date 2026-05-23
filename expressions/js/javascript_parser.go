// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.JavascriptParser.
//
// In Apache Lucene 10.4.0 JavascriptParser is an ANTLR-4 generated parser
// class. In Gocene the ANTLR runtime is not used; instead the grammar is
// implemented as a hand-written recursive-descent parser embedded in
// JavascriptCompiler (javascript_compiler.go). JavascriptParser exposes the
// parse entry point as a named exported type so that callers in the js package
// can reference it explicitly, matching the Java package-level API surface.
package js

import (
	"github.com/FlavioCFOliveira/Gocene/expressions"
)

// JavascriptParser parses the Lucene JavaScript expression grammar and
// produces an expressions.Expression.
//
// Mirrors the top-level API surface of
// org.apache.lucene.expressions.js.JavascriptParser. The ANTLR-generated
// parse-tree types (CompileContext, ExpressionContext, …) are not exposed
// because Gocene uses a hand-written recursive-descent parser instead of the
// ANTLR runtime.
type JavascriptParser struct{}

// Parse parses source and returns an Expression backed by the hand-written
// recursive-descent parser. It is the Go equivalent of calling
// JavascriptParser.compile() in the ANTLR-generated Java class.
func (JavascriptParser) Parse(source string) (*expressions.Expression, error) {
	return JavascriptCompiler{}.Compile(source)
}
