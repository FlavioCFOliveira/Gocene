// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.JavascriptVisitor.
//
// In Apache Lucene 10.4.0 JavascriptVisitor is an ANTLR-4 generated visitor
// interface for the JavaScript expression parse tree. In Gocene the ANTLR
// runtime and parse-tree visitor pattern are not used; expression evaluation
// is performed directly by the hand-written recursive-descent parser in
// javascript_compiler.go. JavascriptVisitor is an empty interface marker.
package js

// JavascriptVisitor is the Go counterpart of the ANTLR-generated
// org.apache.lucene.expressions.js.JavascriptVisitor.
// The ANTLR visitor pattern is not present in Gocene; expression evaluation
// is driven by the hand-written recursive-descent parser.
type JavascriptVisitor interface{}
