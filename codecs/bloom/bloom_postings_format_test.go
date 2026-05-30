// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bloom_test

import "testing"

// TestBloomPostingsFormat mirrors the Java class
// org.apache.lucene.codecs.bloom.TestBloomPostingsFormat (Lucene 10.4.0).
//
// The Java class extends BasePostingsFormatTestCase and registers
// TestBloomFilteredLucenePostings as the codec under test. No @Test methods
// are declared; the entire test suite comes from the superclass framework.
//
// The test is skipped because TestBloomFilteredLucenePostings (the test codec
// that wraps BloomPostingsFormat) is not yet ported to the Gocene bloom
// package.
func TestBloomPostingsFormat(t *testing.T) {
	t.Fatal(
		"TestBloomFilteredLucenePostings (test codec wrapper for BloomPostingsFormat) " +
			"is not yet ported to Gocene; test deferred until that component lands",
	)
}
