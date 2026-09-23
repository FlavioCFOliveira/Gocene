// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90CompoundFormat.java
// (Apache Lucene 10.5.0). The class extends
// org.apache.lucene.tests.index.BaseCompoundFormatTestCase (not ported);
// testFileLengthOrdering builds its segment with the base class helpers
// newSegmentInfo(Directory, String) and createRandomFile(Directory, String,
// int, byte[]), so it fails naming the base class as well.

import "testing"

const lucene90CompoundFormatBlocker = "requires org.apache.lucene.tests.index.BaseCompoundFormatTestCase " +
	"(newSegmentInfo, createRandomFile and the inherited test methods) (not ported)"

func TestLucene90CompoundFormat_BaseCompoundFormatTestCase(t *testing.T) {
	t.Fatal(lucene90CompoundFormatBlocker)
}

func TestLucene90CompoundFormat_testFileLengthOrdering(t *testing.T) {
	t.Fatal(lucene90CompoundFormatBlocker)
}
