// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestLatLonDocValuesQueries.java
// (Apache Lucene 10.5.0).
//
// The class declares no test method of its own: it overrides the factory
// methods of org.apache.lucene.tests.geo.BaseGeoPointTestCase and inherits all
// of that base class's test methods. The base class is not ported, so the
// inherited suite fails naming it; the overrides are ported together with it.

package search_test

import "testing"

// baseGeoPointTestCaseBlocker names org.apache.lucene.tests.geo.BaseGeoPointTestCase.
const baseGeoPointTestCaseBlocker = "requires org.apache.lucene.tests.geo.BaseGeoPointTestCase (not ported)"

// TestLatLonDocValuesQueries renders the test methods TestLatLonDocValuesQueries
// inherits from BaseGeoPointTestCase.
func TestLatLonDocValuesQueries(t *testing.T) {
	t.Fatal(baseGeoPointTestCaseBlocker)
}
