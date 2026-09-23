// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestSegmentReader.java
// (Apache Lucene 10.5.0).
//
// Its setUp() builds the test document with DocHelper.setupDoc and writes it with DocHelper.writeDoc;
// org.apache.lucene.tests.index.DocHelper is not ported to Gocene, so each
// port fails naming the missing test-framework class.

package index_test

import "testing"

const testSegmentReaderMissing = "org.apache.lucene.tests.index.DocHelper is not ported: TestSegmentReader sets up every test with DocHelper.setupDoc/writeDoc"

// TestSegmentReader ports test().
func TestSegmentReader(t *testing.T) {
	t.Fatal(testSegmentReaderMissing)
}

// TestSegmentReaderDocument ports testDocument().
func TestSegmentReaderDocument(t *testing.T) {
	t.Fatal(testSegmentReaderMissing)
}

// TestSegmentReaderGetFieldNameVariations ports testGetFieldNameVariations().
func TestSegmentReaderGetFieldNameVariations(t *testing.T) {
	t.Fatal(testSegmentReaderMissing)
}

// TestSegmentReaderTerms ports testTerms().
func TestSegmentReaderTerms(t *testing.T) {
	t.Fatal(testSegmentReaderMissing)
}

// TestSegmentReaderNorms ports testNorms().
func TestSegmentReaderNorms(t *testing.T) {
	t.Fatal(testSegmentReaderMissing)
}

// TestSegmentReaderTermVectors ports testTermVectors().
func TestSegmentReaderTermVectors(t *testing.T) {
	t.Fatal(testSegmentReaderMissing)
}

// TestSegmentReaderOutOfBoundsAccess ports testOutOfBoundsAccess().
func TestSegmentReaderOutOfBoundsAccess(t *testing.T) {
	t.Fatal(testSegmentReaderMissing)
}
