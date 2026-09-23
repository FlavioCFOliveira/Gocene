// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

// Port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/compressing/TestFastDecompressionMode.java
// (Apache Lucene 10.5.0): setUp() sets mode = CompressionMode.FAST_DECOMPRESSION and the
// test methods are inherited from AbstractTestCompressionMode.

import "testing"

func TestFastDecompressionMode_testDecompress(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST_DECOMPRESSION).testDecompress()
}

func TestFastDecompressionMode_testPartialDecompress(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST_DECOMPRESSION).testPartialDecompress()
}

func TestFastDecompressionMode_testEmptySequence(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST_DECOMPRESSION).testEmptySequence()
}

func TestFastDecompressionMode_testShortSequence(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST_DECOMPRESSION).testShortSequence()
}

func TestFastDecompressionMode_testIncompressible(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST_DECOMPRESSION).testIncompressible()
}

func TestFastDecompressionMode_testConstant(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST_DECOMPRESSION).testConstant()
}
