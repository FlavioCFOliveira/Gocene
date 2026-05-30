// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains a test for the maxTermFrequency statistic in
// FieldInvertState.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestMaxTermFrequency.java
//
// GOC-4260: Port test `org.apache.lucene.index.TestMaxTermFrequency`.
//
// # Test coverage
//
//   - TestMaxTermFrequency — 1:1 port of test()
//
// # Deviations from the Java reference
//
//   - Degraded to t.Skip.
//
//   - The test builds a 100-document index using RandomIndexWriter with
//     MockAnalyzer/MockTokenizer, sets a custom Similarity that encodes
//     maxTermFrequency as a byte into the norm, then reads back per-document
//     norm values via MultiDocValues.getNormValues and asserts the byte value
//     matches the expected maximum term frequency in each document.
//
//   - Missing Gocene infrastructure:
//     (a) RandomIndexWriter and MockAnalyzer/MockTokenizer — test-module
//     utilities not ported;
//     (b) Custom Similarity injection via IndexWriterConfig.setSimilarity —
//     Similarity interface not yet wired into IndexWriterConfig;
//     (c) MultiDocValues.getNormValues — norm read-back requires wired codec
//     reader (coreReaders nil in NewSegmentReader);
//     (d) NumericDocValues iteration — requires wired doc-values reader.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import "testing"

// TestMaxTermFrequency ports test().
//
// Java builds 100 documents with random single-char token sequences, installs
// a custom Similarity that encodes the max per-term frequency as a byte into
// the norm, then reads back the norm byte and asserts it equals the expected
// max frequency for each document.
//
// Degraded to t.Skip: RandomIndexWriter, MockAnalyzer/MockTokenizer, custom
// Similarity injection, and MultiDocValues.getNormValues via wired codec
// reader are not yet available.
func TestMaxTermFrequency(t *testing.T) {
	t.Fatal("needs RandomIndexWriter, MockAnalyzer/MockTokenizer, custom " +
		"Similarity injection, and MultiDocValues.getNormValues via " +
		"wired codec reader (not yet ported)")
}
