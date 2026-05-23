// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package segmentation_test

// TestICUTokenizerCJK_DictionaryTests documents the Java-originated CJK
// dictionary segmentation tests from
// org.apache.lucene.analysis.icu.segmentation.TestICUTokenizerCJK.
//
// Deviation: ICU4J's dictionary-based CJK segmentation (.brk files loaded
// via getResourceAsStream) has no CGO-free Go equivalent. The
// DefaultICUTokenizerConfig in this port uses goWordBreakIterator, which
// treats each Han character as a separate token. The Java @AwaitsFix
// annotation on TestICUTokenizerCJK (LUCENE-8222) also marks these tests
// as known-failing upstream, indicating they are not stable even in Java.
//
// These tests are therefore intentionally omitted from the Go test suite.
// When a CGO-free Go ICU4J-equivalent is available, they should be ported.
//
// The following Java @Test methods are not ported:
//   - testSimpleChinese
//   - testTraditionalChinese
//   - testChineseNumerics
//   - testSimpleJapanese
//   - testSimpleJapaneseWithEmoji
//   - testJapaneseTypes
//   - testKorean (Korean word-level segmentation)
//   - testKoreanTypes
//   - testRandomStrings
//   - testRandomHugeStrings
