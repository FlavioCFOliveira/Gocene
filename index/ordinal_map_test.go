// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestOrdinalMap.java
// (Apache Lucene 10.5.0).
//
// Both test methods configure the writer with
// TestUtil.alwaysDocValuesFormat(TestUtil.getDefaultDocValuesFormat());
// testRamBytesUsed also measures the OrdinalMap with RamUsageTester. Neither
// test-framework member is ported to Gocene, so each port fails naming them.

package index

import "testing"

func TestOrdinalMapRamBytesUsed(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.util.TestUtil#alwaysDocValuesFormat(DocValuesFormat), " +
		"TestUtil#getDefaultDocValuesFormat() and org.apache.lucene.tests.util.RamUsageTester " +
		"are not ported")
}

func TestOrdinalMapOneSegmentWithAllValues(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.util.TestUtil#alwaysDocValuesFormat(DocValuesFormat) and " +
		"TestUtil#getDefaultDocValuesFormat() are not ported")
}
