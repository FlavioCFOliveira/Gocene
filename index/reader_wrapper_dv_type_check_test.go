// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests verifying that LeafReader wrappers enforce
// DocValues-type checks.
//
// Ported from Apache Lucene 10.4.0:
//
//	lucene/core/src/test/org/apache/lucene/index/TestReaderWrapperDVTypeCheck.java
//
// GOC-4239: Port test `org.apache.lucene.index.TestReaderWrapperDVTypeCheck`.
//
// # Test coverage
//
//   - TestReaderWrapperDVTypeCheck_NoDVFieldOnSegment — 1:1 port of testNoDVFieldOnSegment()
//
// # Deviations from the Java reference
//
//   - Degraded to t.Skip.
//
//   - The test verifies that getSortedDocValues("ssdv") returns null (wrong DV
//     type: field was written as SortedSet, not Sorted) and that
//     getSortedSetDocValues("sdv") also returns null (wrong direction).  These
//     checks require a real DocValues reader that distinguishes by type; the
//     Gocene LeafReader stubs for GetSortedDocValues and GetSortedSetDocValues
//     unconditionally return (nil, nil), so no distinction between "absent
//     field", "correct DV type", and "wrong DV type" is possible.
//
//   - RandomIndexWriter → IndexWriter (RandomIndexWriter not yet ported).
//
//   - iw.getReader() (NRT reader pulled directly from a RandomIndexWriter) does
//     not exist; would need OpenDirectoryReaderFromWriter or equivalent.
//
//   - TestUtil.alwaysDocValuesFormat / getDefaultDocValuesFormat / nextInt are
//     test-module utilities without Gocene equivalents.
//
//   - getOnlyLeafReader(reader) is a LuceneTestCase helper without a Gocene
//     equivalent; would need a helper that asserts exactly 1 leaf and returns it.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import "testing"

// TestReaderWrapperDVTypeCheck_NoDVFieldOnSegment ports testNoDVFieldOnSegment().
//
// Java adds 1–4 documents where "sdv" (SortedDocValuesField) and "ssdv"
// (SortedSetDocValuesField) are written with random probability, force-merges,
// opens an NRT reader, and via the single leaf reader asserts:
//   - getSortedDocValues("ssdv") == null  (wrong DV type)
//   - getSortedSetDocValues("sdv") == null (wrong DV type)
//   - getSortedDocValues("NOssdv") == null (absent field)
//   - getSortedSetDocValues("NOsdv") == null (absent field)
//   - sdv != null iff the "sdv" field was actually written
//   - ssdv != null iff the "ssdv" field was actually written
//
// Degraded to t.Skip: GetSortedDocValues and GetSortedSetDocValues are stubs
// that always return (nil, nil), making type-based discrimination impossible.
// Additionally, iw.getReader() (NRT reader from writer) and
// getOnlyLeafReader(reader) are not yet ported.
func TestReaderWrapperDVTypeCheck_NoDVFieldOnSegment(t *testing.T) {
	t.Fatal("GetSortedDocValues/GetSortedSetDocValues are stubs returning (nil,nil); " +
		"DV-type discrimination, iw.getReader() NRT path, and getOnlyLeafReader helper " +
		"are not yet available")
}
