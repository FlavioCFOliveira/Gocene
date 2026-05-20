// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for TokenStream reuse by DefaultIndexingChain.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestFieldReuse.java
//
// GOC-4257: Port test `org.apache.lucene.index.TestFieldReuse`.
//
// # Test coverage
//
//   - TestFieldReuse_StringField           — 1:1 port of testStringField()
//   - TestFieldReuse_IndexWriterActuallyReuses — 1:1 port of testIndexWriterActuallyReuses()
//
// # Deviations from the Java reference
//
//   - All tests are degraded to t.Skip.
//
//   - testStringField creates a StringField, calls tokenStream(null, nil),
//     verifies the resulting TokenStream contents using assertTokenStreamContents
//     (a BaseTokenStreamTestCase utility), then verifies stream reuse on the
//     second call.  Blockers:
//     (a) BaseTokenStreamTestCase.assertTokenStreamContents — test-module
//     utility not ported;
//     (b) CannedTokenStream — test-module utility not ported;
//     (c) Field.tokenStream(Analyzer, TokenStream) — Gocene's Field type does
//     not currently expose a tokenStream method with reuse semantics.
//
//   - testIndexWriterActuallyReuses defines an inline MyField implementing
//     IndexableField whose tokenStream method records the previous reuse
//     argument, then asserts that the second addDocument call passes the
//     TokenStream returned by the first.  Blockers:
//     (a) CannedTokenStream not ported;
//     (b) IndexWriter.addDocument(Collection<IndexableField>) — Gocene's
//     AddDocument takes a Document value type, not an arbitrary collection
//     of IndexableField;
//     (c) IndexableField.tokenStream(Analyzer, TokenStream) reuse contract —
//     DefaultIndexingChain reuse path not observable from tests.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import "testing"

// TestFieldReuse_StringField ports testStringField().
//
// Java creates a StringField, calls tokenStream(null, null), asserts the
// output is ["bar"] at [0,3), then reuses the stream with a new value and
// asserts the same TokenStream object is returned and contains ["baz"].
//
// Degraded to t.Skip: assertTokenStreamContents (BaseTokenStreamTestCase),
// CannedTokenStream, and Field.tokenStream reuse semantics are not yet ported.
func TestFieldReuse_StringField(t *testing.T) {
	t.Skip("needs assertTokenStreamContents (BaseTokenStreamTestCase), " +
		"CannedTokenStream, and Field.tokenStream(Analyzer,TokenStream) " +
		"reuse contract (not yet ported)")
}

// TestFieldReuse_IndexWriterActuallyReuses ports testIndexWriterActuallyReuses().
//
// Java defines a custom IndexableField (MyField) whose tokenStream method
// records the reuse argument, adds the field twice, and asserts that the
// second add receives the TokenStream produced by the first.
//
// Degraded to t.Skip: CannedTokenStream not ported; Gocene's AddDocument
// takes a Document, not a Collection<IndexableField>; DefaultIndexingChain
// reuse path is not observable from tests without IndexableField.tokenStream.
func TestFieldReuse_IndexWriterActuallyReuses(t *testing.T) {
	t.Skip("needs CannedTokenStream, IndexWriter.addDocument(Collection" +
		"<IndexableField>), and DefaultIndexingChain TokenStream reuse " +
		"contract (not yet ported)")
}
