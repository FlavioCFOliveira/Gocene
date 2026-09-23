// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90PointsFormatV0.java
// (Apache Lucene 10.5.0). The class only overrides getCodec() (an anonymous
// org.apache.lucene.tests.codecs.asserting.AssertingCodec returning
// new Lucene90PointsFormat(VERSION_START)) and inherits every test method from
// org.apache.lucene.tests.index.BasePointsFormatTestCase; neither
// test-framework class is ported.

import "testing"

func TestLucene90PointsFormatV0_BasePointsFormatTestCase(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.index.BasePointsFormatTestCase and " +
		"org.apache.lucene.tests.codecs.asserting.AssertingCodec (not ported)")
}
