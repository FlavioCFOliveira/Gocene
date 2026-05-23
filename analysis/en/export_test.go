// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package en

// KStemmerForTest is a thin wrapper that exposes the package-private kStemmer
// for black-box testing from the en_test package.
type KStemmerForTest struct{ s *kStemmer }

// NewKStemmerForTest creates a test-accessible KStemmer instance.
func NewKStemmerForTest() *KStemmerForTest { return &KStemmerForTest{s: newKStemmer()} }

// Stem stems the given term and returns the result.
func (w *KStemmerForTest) Stem(term string) string { return w.s.stem(term) }
