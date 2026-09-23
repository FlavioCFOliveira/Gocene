// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

// Port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/compressing/TestHighCompressionMode.java
// (Apache Lucene 10.5.0): setUp() sets mode = CompressionMode.HIGH_COMPRESSION and the
// test methods are inherited from AbstractTestCompressionMode.

import "testing"

func TestHighCompressionMode_testDecompress(t *testing.T) {
	newAbstractTestCompressionMode(t, HIGH_COMPRESSION).testDecompress()
}

func TestHighCompressionMode_testPartialDecompress(t *testing.T) {
	newAbstractTestCompressionMode(t, HIGH_COMPRESSION).testPartialDecompress()
}

func TestHighCompressionMode_testEmptySequence(t *testing.T) {
	newAbstractTestCompressionMode(t, HIGH_COMPRESSION).testEmptySequence()
}

func TestHighCompressionMode_testShortSequence(t *testing.T) {
	newAbstractTestCompressionMode(t, HIGH_COMPRESSION).testShortSequence()
}

func TestHighCompressionMode_testIncompressible(t *testing.T) {
	newAbstractTestCompressionMode(t, HIGH_COMPRESSION).testIncompressible()
}

func TestHighCompressionMode_testConstant(t *testing.T) {
	newAbstractTestCompressionMode(t, HIGH_COMPRESSION).testConstant()
}
