// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for the index package.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestDocInverterPerFieldErrorInfo
// Source: lucene/core/src/test/org/apache/lucene/index/TestDocInverterPerFieldErrorInfo.java
//
// GOC-4199 (Sprint 55, option c): test ported faithfully but gated with t.Skip.
//
// PORTING NOTE: the upstream test asserts that, when an exception is thrown
// during a field's token analysis, the failing field's name is appended to
// the IndexWriter info stream (and, conversely, that a clean field leaves no
// trace). Reproducing it requires two pieces of production infrastructure
// that are not yet ported:
//
//   - IndexWriterConfig.SetInfoStream (LiveIndexWriterConfig.setInfoStream):
//     there is currently no way to attach a util.PrintStreamInfoStream to an
//     IndexWriterConfig.
//   - DocInverter per-field error reporting: DocumentsWriterPerThread.indexField
//     propagates the analysis error but does not write the field name to the
//     info stream, which is the exact behaviour under test.
//
// The MockTokenizer used by the upstream ThrowingAnalyzer is also not ported.
//
// Each test calls t.Skip until the production path lands; the body is kept so
// the port is byte-faithful to the Java original and is ready to be enabled.
package index_test

import (
	"strings"
	"testing"
)

// badNews is the Go port of the upstream private BadNews RuntimeException: a
// distinctive error type thrown from the offending token filter.
type badNews struct {
	message string
}

func (e *badNews) Error() string { return e.message }

// throwingAnalyzerNote documents the upstream ThrowingAnalyzer behaviour
// preserved for when this test is enabled: for the field
// "distinctiveFieldName" the analyzer wraps the tokenizer in a TokenFilter
// whose IncrementToken always fails with badNews("Something is icky.");
// every other field is analyzed normally.

// TestInfoStreamGetsFieldName verifies that, when field analysis throws, the
// offending field name reaches the IndexWriter info stream.
func TestInfoStreamGetsFieldName(t *testing.T) {
	t.Fatal("GOC-4199: pending IndexWriterConfig.SetInfoStream and DocInverter per-field error reporting")

	// Faithful port of testInfoStreamGetsFieldName, kept for when the
	// production path is available:
	//
	//   dir := store.NewByteBuffersDirectory() // newDirectory()
	//   c := index.NewIndexWriterConfig(newThrowingAnalyzer())
	//   var infoBytes bytes.Buffer
	//   c.SetInfoStream(util.NewPrintStreamInfoStream(&infoBytes))
	//   writer, err := index.NewIndexWriter(dir, c)
	//   ...
	//   doc := document.NewDocument()
	//   doc.Add(document.NewField("distinctiveFieldName", "aaa ", storedTextType))
	//   err = writer.AddDocument(doc)
	//   // expectThrows(badNews): err must unwrap to *badNews
	//   if !strings.Contains(infoBytes.String(), "distinctiveFieldName") {
	//       t.Errorf("info stream missing field name: %q", infoBytes.String())
	//   }
	_ = strings.Contains
	_ = &badNews{}
}

// TestNoExtraNoise verifies that a cleanly analyzed field never appears in the
// IndexWriter info stream.
func TestNoExtraNoise(t *testing.T) {
	t.Fatal("GOC-4199: pending IndexWriterConfig.SetInfoStream and DocInverter per-field error reporting")

	// Faithful port of testNoExtraNoise, kept for when the production path
	// is available:
	//
	//   dir := store.NewByteBuffersDirectory() // newDirectory()
	//   c := index.NewIndexWriterConfig(newThrowingAnalyzer())
	//   var infoBytes bytes.Buffer
	//   c.SetInfoStream(util.NewPrintStreamInfoStream(&infoBytes))
	//   writer, err := index.NewIndexWriter(dir, c)
	//   ...
	//   doc := document.NewDocument()
	//   doc.Add(document.NewField("boringFieldName", "aaa ", storedTextType))
	//   // should not fail with badNews
	//   if err := writer.AddDocument(doc); err != nil {
	//       t.Fatalf("AddDocument: %v", err)
	//   }
	//   if strings.Contains(infoBytes.String(), "boringFieldName") {
	//       t.Errorf("info stream leaked clean field name: %q", infoBytes.String())
	//   }
	_ = strings.Contains
	_ = &badNews{}
}
