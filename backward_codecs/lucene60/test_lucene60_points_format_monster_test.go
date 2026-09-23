// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/lucene60/TestLucene60PointsFormat.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled ("N-2 formats are only tested on
// nightly runs").
//
// Blocker: the class extends BasePointsFormatTestCase and its constructor builds a Lucene84RWCodec, whose points format is the test-side Lucene60RWPointsFormat writing through Lucene60PointsWriter; none is ported.

package lucene60

import "testing"

const TestLucene60PointsFormatBlocker = "requires org.apache.lucene.tests.index.BasePointsFormatTestCase and the test-side org.apache.lucene.backward_codecs.lucene84.Lucene84RWCodec with Lucene60RWPointsFormat/Lucene60PointsWriter (not ported)"

func TestLucene60PointsFormat_BasePointsFormatTestCase(t *testing.T) {
	t.Fatal(TestLucene60PointsFormatBlocker)
}

func TestLucene60PointsFormat_testEstimatePointCount(t *testing.T) {
	t.Fatal(TestLucene60PointsFormatBlocker)
}

func TestLucene60PointsFormat_testEstimatePointCount2Dims(t *testing.T) {
	t.Fatal(TestLucene60PointsFormatBlocker)
}
