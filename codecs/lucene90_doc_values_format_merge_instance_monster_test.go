// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly methods that
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90DocValuesFormatMergeInstance.java
// (Apache Lucene 10.5.0) inherits from TestLucene90DocValuesFormat, built only
// with the gocene_monsters tag.

package codecs_test

import "testing"

func TestLucene90DocValuesFormatMergeInstance_testSortedSetVariableLengthManyVsStoredFields(t *testing.T) {
	lucene90DVTestSortedSetVariableLengthManyVsStoredFields(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSortedVariableLengthManyVsStoredFields(t *testing.T) {
	lucene90DVTestSortedVariableLengthManyVsStoredFields(t)
}

func TestLucene90DocValuesFormatMergeInstance_testTermsEnumFixedWidth(t *testing.T) {
	lucene90DVTestTermsEnumFixedWidth(t)
}

func TestLucene90DocValuesFormatMergeInstance_testTermsEnumVariableWidth(t *testing.T) {
	lucene90DVTestTermsEnumVariableWidth(t)
}

func TestLucene90DocValuesFormatMergeInstance_testTermsEnumRandomMany(t *testing.T) {
	lucene90DVTestTermsEnumRandomMany(t)
}

func TestLucene90DocValuesFormatMergeInstance_testTermsEnumLongSharedPrefixes(t *testing.T) {
	lucene90DVTestTermsEnumLongSharedPrefixes(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSortedSetAroundBlockSize(t *testing.T) {
	lucene90DVTestSortedSetAroundBlockSize(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSortedNumericAroundBlockSize(t *testing.T) {
	lucene90DVTestSortedNumericAroundBlockSize(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSortedNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	lucene90DVTestSortedNumericBlocksOfVariousBitsPerValue(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSparseSortedNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	lucene90DVTestSparseSortedNumericBlocksOfVariousBitsPerValue(t)
}

func TestLucene90DocValuesFormatMergeInstance_testNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	lucene90DVTestNumericBlocksOfVariousBitsPerValue(t)
}

func TestLucene90DocValuesFormatMergeInstance_testSparseNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	lucene90DVTestSparseNumericBlocksOfVariousBitsPerValue(t)
}

func TestLucene90DocValuesFormatMergeInstance_testNumericFieldJumpTables(t *testing.T) {
	lucene90DVTestNumericFieldJumpTables(t)
}
