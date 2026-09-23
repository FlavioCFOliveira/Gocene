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
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestLongRangeGroupSelector.java
// (Apache Lucene 10.5.0), which extends BaseGroupSelectorTestCase<LongRange>.

func newLongRangeGroupSelectorTestCase() *baseGroupSelectorTestCase[*LongRange] {
	return &baseGroupSelectorTestCase[*LongRange]{
		addGroupField: func(t testing.TB, doc *document.Document, id int) {
			if rarely() {
				return // missing value
			}
			// numbers between 0 and 1000, groups are 100 wide from 100 to 900
			value := int64(random().Intn(1000))
			doc.Add(document.NewLongPoint("long", value))
			doc.Add(mustNumericDVField(t, "long", value))
		},
		getGroupSelector: func() GroupSelector[*LongRange] {
			return NewLongRangeGroupSelector(search.LongValuesSourceFromLongField("long"), NewLongRangeFactory(100, 100, 900))
		},
		filterQuery: func(t testing.TB, groupValue *LongRange) search.Query {
			if groupValue == nil {
				return search.NewBooleanQueryBuilder().
					Add(search.Instance, search.FILTER).
					Add(search.NewFieldExistsQuery("long"), search.MUST_NOT).
					Build()
			}
			// LongPoint.newRangeQuery("long", groupValue.min, groupValue.max - 1)
			t.Fatal(pointNewRangeQueryBlocker("LongPoint"))
			return nil
		},
	}
}

func TestLongRangeGroupSelectorSortByRelevance(t *testing.T) {
	newLongRangeGroupSelectorTestCase().testSortByRelevance(t)
}

func TestLongRangeGroupSelectorSortGroups(t *testing.T) {
	newLongRangeGroupSelectorTestCase().testSortGroups(t)
}

func TestLongRangeGroupSelectorSortWithinGroups(t *testing.T) {
	newLongRangeGroupSelectorTestCase().testSortWithinGroups(t)
}

func TestLongRangeGroupSelectorGroupHeads(t *testing.T) {
	newLongRangeGroupSelectorTestCase().testGroupHeads(t)
}

func TestLongRangeGroupSelectorGroupHeadsWithSort(t *testing.T) {
	newLongRangeGroupSelectorTestCase().testGroupHeadsWithSort(t)
}

func TestLongRangeGroupSelectorShardedGrouping(t *testing.T) {
	newLongRangeGroupSelectorTestCase().testShardedGrouping(t)
}

func TestLongRangeGroupSelectorIgnoreDocsWithoutGroupField(t *testing.T) {
	newLongRangeGroupSelectorTestCase().testIgnoreDocsWithoutGroupField(t)
}
