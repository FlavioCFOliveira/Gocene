// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestDoubleRangeGroupSelector.java
// (Apache Lucene 10.5.0), which extends BaseGroupSelectorTestCase<DoubleRange>.

func newDoubleRangeGroupSelectorTestCase() *baseGroupSelectorTestCase[*DoubleRange] {
	return &baseGroupSelectorTestCase[*DoubleRange]{
		addGroupField: func(t testing.TB, doc *document.Document, id int) {
			if rarely() {
				return // missing value
			}
			// numbers between 0 and 1000, groups are 100 wide from 100 to 900
			value := random().Float64() * 1000
			doc.Add(document.NewDoublePoint("double", value))
			doc.Add(mustNumericDVField(t, "double", int64(javaDoubleToLongBits(value))))
		},
		getGroupSelector: func() GroupSelector[*DoubleRange] {
			// search.NewDoubleValuesSource renders DoubleValuesSource.fromDoubleField:
			// the NUMERIC doc value read through Double.longBitsToDouble.
			return NewDoubleRangeGroupSelector(search.NewDoubleValuesSource("double"), NewDoubleRangeFactory(100, 100, 900))
		},
		filterQuery: func(t testing.TB, groupValue *DoubleRange) search.Query {
			if groupValue == nil {
				return search.NewBooleanQueryBuilder().
					Add(search.Instance, search.FILTER).
					Add(search.NewFieldExistsQuery("double"), search.MUST_NOT).
					Build()
			}
			// DoublePoint.newRangeQuery("double", groupValue.min, Math.nextDown(groupValue.max))
			t.Fatal(pointNewRangeQueryBlocker("DoublePoint"))
			return nil
		},
	}
}

// javaDoubleToLongBits renders Double.doubleToLongBits: every NaN collapses
// onto the canonical NaN.
func javaDoubleToLongBits(v float64) uint64 {
	return canonicalFloat64Bits(v)
}

func TestDoubleRangeGroupSelectorSortByRelevance(t *testing.T) {
	newDoubleRangeGroupSelectorTestCase().testSortByRelevance(t)
}

func TestDoubleRangeGroupSelectorSortGroups(t *testing.T) {
	newDoubleRangeGroupSelectorTestCase().testSortGroups(t)
}

func TestDoubleRangeGroupSelectorSortWithinGroups(t *testing.T) {
	newDoubleRangeGroupSelectorTestCase().testSortWithinGroups(t)
}

func TestDoubleRangeGroupSelectorGroupHeads(t *testing.T) {
	newDoubleRangeGroupSelectorTestCase().testGroupHeads(t)
}

func TestDoubleRangeGroupSelectorGroupHeadsWithSort(t *testing.T) {
	newDoubleRangeGroupSelectorTestCase().testGroupHeadsWithSort(t)
}

func TestDoubleRangeGroupSelectorShardedGrouping(t *testing.T) {
	newDoubleRangeGroupSelectorTestCase().testShardedGrouping(t)
}

func TestDoubleRangeGroupSelectorIgnoreDocsWithoutGroupField(t *testing.T) {
	newDoubleRangeGroupSelectorTestCase().testIgnoreDocsWithoutGroupField(t)
}
