// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestTermGroupSelector.java
// (Apache Lucene 10.5.0), which extends BaseGroupSelectorTestCase<BytesRef>.

func newTermGroupSelectorTestCase() *baseGroupSelectorTestCase[*util.BytesRef] {
	return &baseGroupSelectorTestCase[*util.BytesRef]{
		addGroupField: func(t testing.TB, doc *document.Document, id int) {
			if rarely() {
				return // missing value
			}
			groupValue := "group" + strconv.Itoa(random().Intn(10))
			doc.Add(mustSortedDVField(t, "groupField", groupValue))
			doc.Add(newTextField(t, "groupField", groupValue, false))
		},
		getGroupSelector: func() GroupSelector[*util.BytesRef] {
			return NewTermGroupSelector("groupField")
		},
		filterQuery: func(t testing.TB, groupValue *util.BytesRef) search.Query {
			if groupValue == nil {
				return search.NewBooleanQueryBuilder().
					Add(search.Instance, search.FILTER).
					Add(search.NewFieldExistsQuery("groupField"), search.MUST_NOT).
					Build()
			}
			return search.NewTermQuery(index.NewTermFromBytesRef("groupField", groupValue))
		},
	}
}

func TestTermGroupSelectorSortByRelevance(t *testing.T) {
	newTermGroupSelectorTestCase().testSortByRelevance(t)
}

func TestTermGroupSelectorSortGroups(t *testing.T) { newTermGroupSelectorTestCase().testSortGroups(t) }

func TestTermGroupSelectorSortWithinGroups(t *testing.T) {
	newTermGroupSelectorTestCase().testSortWithinGroups(t)
}

func TestTermGroupSelectorGroupHeads(t *testing.T) { newTermGroupSelectorTestCase().testGroupHeads(t) }

func TestTermGroupSelectorGroupHeadsWithSort(t *testing.T) {
	newTermGroupSelectorTestCase().testGroupHeadsWithSort(t)
}

func TestTermGroupSelectorShardedGrouping(t *testing.T) {
	newTermGroupSelectorTestCase().testShardedGrouping(t)
}

func TestTermGroupSelectorIgnoreDocsWithoutGroupField(t *testing.T) {
	newTermGroupSelectorTestCase().testIgnoreDocsWithoutGroupField(t)
}
