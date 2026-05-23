// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation_test

// TestWithCJKBigramFilter documents the Java test
// org.apache.lucene.analysis.icu.segmentation.TestWithCJKBigramFilter.
//
// This test class exercises ICUTokenizer + CJKBigramFilter together. The
// Java tests depend on ICU4J's dictionary-based CJK segmentation to produce
// individual ideograph tokens that the CJKBigramFilter then joins into bigrams.
//
// In this Go port, each Han character is already emitted as a separate token
// (IDEOGRAPHIC), so the CJKBigramFilter can still function. However, the
// specific bigram boundaries expected by the Java tests match ICU4J's output
// and would require end-to-end validation with a real CJKBigramFilter.
//
// The CJKBigramFilter is available in Gocene under analysis/cjk. A full
// integration test exercising the pipeline is deferred until a separate
// sprint that covers the CJK analysis integration.
//
// Java @Test methods not ported at this stage:
//   - testJa1, testJa2, testJa3
//   - testKorean1, testKorean2
//   - testJapaneseNumerics1 through testJapaneseNumerics3
//   - testJapaneseAlphanumerics1
//   - testJapanesePhrases1 through testJapanesePhrases3
//   - testMassiveAnalyzing
