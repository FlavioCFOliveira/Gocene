// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

// Port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/compressing/TestFastCompressionMode.java
// (Apache Lucene 10.5.0): setUp() sets mode = CompressionMode.FAST and the
// test methods are inherited from AbstractTestCompressionMode.

import "testing"

func TestFastCompressionMode_testDecompress(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST).testDecompress()
}

func TestFastCompressionMode_testPartialDecompress(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST).testPartialDecompress()
}

func TestFastCompressionMode_testEmptySequence(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST).testEmptySequence()
}

func TestFastCompressionMode_testShortSequence(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST).testShortSequence()
}

func TestFastCompressionMode_testIncompressible(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST).testIncompressible()
}

func TestFastCompressionMode_testConstant(t *testing.T) {
	newAbstractTestCompressionMode(t, FAST).testConstant()
}
