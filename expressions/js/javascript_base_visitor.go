// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.JavascriptBaseVisitor.
//
// In Apache Lucene 10.4.0 JavascriptBaseVisitor is an ANTLR-4 generated
// default visitor implementation. In Gocene the ANTLR runtime and parse-tree
// visitor pattern are not used; the hand-written recursive-descent parser in
// javascript_compiler.go replaces the entire ANTLR visitor stack.
// JavascriptBaseVisitor is an empty marker type to satisfy API coverage.
package js

// JavascriptBaseVisitor is the Go counterpart of the ANTLR-generated
// org.apache.lucene.expressions.js.JavascriptBaseVisitor.
// No ANTLR parse-tree visitor infrastructure is present in Gocene.
type JavascriptBaseVisitor struct{}
