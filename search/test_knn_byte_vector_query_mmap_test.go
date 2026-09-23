// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search_test

// Ported from Apache Lucene 10.5.0:
//   lucene/core/src/test/org/apache/lucene/search/TestKnnByteVectorQueryMMap.java

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// newTestKnnByteVectorQueryMMap renders TestKnnByteVectorQueryMMap extends
// TestKnnByteVectorQuery: it overrides newDirectoryForTest() with a
// MockDirectoryWrapper over an MMapDirectory in a new temporary directory.
func newTestKnnByteVectorQueryMMap(t *testing.T) *testKnnByteVectorQuery {
	c := newTestKnnByteVectorQuery(t)
	c.newDirectoryForTestOverride = func() store.Directory {
		mmap, err := store.NewMMapDirectory(t.TempDir())
		if err != nil {
			t.Fatalf("MMapDirectory: %v", err)
		}
		return store.NewMockDirectoryWrapper(mmap)
	}
	return c
}

// Inherited from BaseKnnVectorQueryTestCase.
func TestKnnByteVectorQueryMMapEquals(t *testing.T) { newTestKnnByteVectorQueryMMap(t).testEquals() }
func TestKnnByteVectorQueryMMapGetField(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testGetField()
}
func TestKnnByteVectorQueryMMapGetK(t *testing.T) { newTestKnnByteVectorQueryMMap(t).testGetK() }
func TestKnnByteVectorQueryMMapGetFilter(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testGetFilter()
}
func TestKnnByteVectorQueryMMapEmptyIndex(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testEmptyIndex()
}
func TestKnnByteVectorQueryMMapFindAll(t *testing.T) { newTestKnnByteVectorQueryMMap(t).testFindAll() }
func TestKnnByteVectorQueryMMapFindFewer(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testFindFewer()
}
func TestKnnByteVectorQueryMMapSearchBoost(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testSearchBoost()
}
func TestKnnByteVectorQueryMMapSimpleFilter(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testSimpleFilter()
}
func TestKnnByteVectorQueryMMapFilterWithNoVectorMatches(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testFilterWithNoVectorMatches()
}
func TestKnnByteVectorQueryMMapMatchAllFilter(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testMatchAllFilter()
}
func TestKnnByteVectorQueryMMapDimensionMismatch(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testDimensionMismatch()
}
func TestKnnByteVectorQueryMMapNonVectorField(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testNonVectorField()
}
func TestKnnByteVectorQueryMMapIllegalArguments(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testIllegalArguments()
}
func TestKnnByteVectorQueryMMapDifferentReader(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testDifferentReader()
}
func TestKnnByteVectorQueryMMapScoreEuclidean(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testScoreEuclidean()
}
func TestKnnByteVectorQueryMMapScoreCosine(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testScoreCosine()
}
func TestKnnByteVectorQueryMMapScoreMIP(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testScoreMIP()
}
func TestKnnByteVectorQueryMMapExplain(t *testing.T) { newTestKnnByteVectorQueryMMap(t).testExplain() }
func TestKnnByteVectorQueryMMapExplainMultipleSegments(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testExplainMultipleSegments()
}
func TestKnnByteVectorQueryMMapSkewedIndex(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testSkewedIndex()
}
func TestKnnByteVectorQueryMMapRandomConsistencySingleThreaded(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testRandomConsistencySingleThreaded()
}
func TestKnnByteVectorQueryMMapRandomConsistencyMultiThreaded(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testRandomConsistencyMultiThreaded()
}
func TestKnnByteVectorQueryMMapRandom(t *testing.T) { newTestKnnByteVectorQueryMMap(t).testRandom() }
func TestKnnByteVectorQueryMMapRandomWithFilter(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testRandomWithFilter()
}
func TestKnnByteVectorQueryMMapFilterWithSameScore(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testFilterWithSameScore()
}
func TestKnnByteVectorQueryMMapDeletes(t *testing.T) { newTestKnnByteVectorQueryMMap(t).testDeletes() }
func TestKnnByteVectorQueryMMapAllDeletes(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testAllDeletes()
}
func TestKnnByteVectorQueryMMapMergeAwayAllValues(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testMergeAwayAllValues()
}
func TestKnnByteVectorQueryMMapNoLiveDocsReader(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testNoLiveDocsReader()
}
func TestKnnByteVectorQueryMMapBitSetQuery(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testBitSetQuery()
}
func TestKnnByteVectorQueryMMapTimeLimitingKnnCollectorManager(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testTimeLimitingKnnCollectorManager()
}
func TestKnnByteVectorQueryMMapTimeout(t *testing.T) { newTestKnnByteVectorQueryMMap(t).testTimeout() }
func TestKnnByteVectorQueryMMapSameFieldDifferentFormats(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testSameFieldDifferentFormats()
}
func TestKnnByteVectorQueryMMapStrategy(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testStrategy()
}

// Inherited from TestKnnByteVectorQuery.
func TestKnnByteVectorQueryMMapToString(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testToString()
}
func TestKnnByteVectorQueryMMapGetTarget(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testGetTarget()
}
func TestKnnByteVectorQueryMMapVectorEncodingMismatch(t *testing.T) {
	newTestKnnByteVectorQueryMMap(t).testVectorEncodingMismatch()
}
