// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90DocValuesFormatMergeInstance.java
// (Apache Lucene 10.5.0): tests the merge instance of the Lucene90 DV format.
// The class extends TestLucene90DocValuesFormat and only overrides
// shouldTestMergeInstance() to return true; that flag is read solely by the
// BaseDocValuesFormatTestCase helpers, so the inherited own test methods run
// the same bodies. The @Nightly ones are in
// lucene90_doc_values_format_merge_instance_monster_test.go.

import "testing"

func TestLucene90DocValuesFormatMergeInstance_BaseCompressingDocValuesFormatTestCase(t *testing.T) {
	t.Fatal(lucene90DVBaseBlocker + " (shouldTestMergeInstance)")
}

func TestLucene90DocValuesFormatMergeInstance_testSortedSetVariableLengthBigVsStoredFields(t *testing.T) {
	lucene90DVTestSortedSetVariableLengthBigVsStoredFields(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSortedVariableLengthBigVsStoredFields(t *testing.T) {
	lucene90DVTestSortedVariableLengthBigVsStoredFields(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSparseDocValuesVsStoredFields(t *testing.T) {
	lucene90DVTestSparseDocValuesVsStoredFields(t)
}

func TestLucene90DocValuesFormatMergeInstance_testDenseNumericLongValuesBulkFetch(t *testing.T) {
	lucene90DVTestDenseNumericLongValuesBulkFetch(t)
}

func TestLucene90DocValuesFormatMergeInstance_testReseekAfterSkipDecompression(t *testing.T) {
	lucene90DVTestReseekAfterSkipDecompression(t)
}

func TestLucene90DocValuesFormatMergeInstance_testLargeTermsCompression(t *testing.T) {
	lucene90DVTestLargeTermsCompression(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSortedTermsDictLookupOrd(t *testing.T) {
	lucene90DVTestSortedTermsDictLookupOrd(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSortedSetTermsDictLookupOrd(t *testing.T) {
	lucene90DVTestSortedSetTermsDictLookupOrd(t)
}

func TestLucene90DocValuesFormatMergeInstance_testTermsEnumDictionary(t *testing.T) {
	lucene90DVTestTermsEnumDictionary(t)
}

func TestLucene90DocValuesFormatMergeInstance_testTermsEnumConsistency(t *testing.T) {
	lucene90DVTestTermsEnumConsistency(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSkipIndexStoredSeparately(t *testing.T) {
	lucene90DVTestSkipIndexStoredSeparately(t)
}
