// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/valuesource"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util/mutable"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestValueSourceGroupSelector.java
// (Apache Lucene 10.5.0), which extends BaseGroupSelectorTestCase<MutableValue>.

func newValueSourceGroupSelectorTestCase() *baseGroupSelectorTestCase[mutable.MutableValue] {
	return &baseGroupSelectorTestCase[mutable.MutableValue]{
		addGroupField: func(t testing.TB, doc *document.Document, id int) {
			groupValue := "group" + strconv.Itoa(random().Intn(10))
			doc.Add(mustSortedDVField(t, "groupField", groupValue))
			doc.Add(newTextField(t, "groupField", groupValue, false))
		},
		getGroupSelector: func() GroupSelector[mutable.MutableValue] {
			return NewValueSourceGroupSelector(valuesource.NewSortedSetFieldSource("groupField"), function.Context{})
		},
		filterQuery: func(t testing.TB, groupValue mutable.MutableValue) search.Query {
			return search.NewTermQuery(index.NewTerm("groupField", fmt.Sprint(groupValue.ToObject())))
		},
	}
}

func TestValueSourceGroupSelectorSortByRelevance(t *testing.T) {
	newValueSourceGroupSelectorTestCase().testSortByRelevance(t)
}

func TestValueSourceGroupSelectorSortGroups(t *testing.T) {
	newValueSourceGroupSelectorTestCase().testSortGroups(t)
}

func TestValueSourceGroupSelectorSortWithinGroups(t *testing.T) {
	newValueSourceGroupSelectorTestCase().testSortWithinGroups(t)
}

func TestValueSourceGroupSelectorGroupHeads(t *testing.T) {
	newValueSourceGroupSelectorTestCase().testGroupHeads(t)
}

func TestValueSourceGroupSelectorGroupHeadsWithSort(t *testing.T) {
	newValueSourceGroupSelectorTestCase().testGroupHeadsWithSort(t)
}

func TestValueSourceGroupSelectorShardedGrouping(t *testing.T) {
	newValueSourceGroupSelectorTestCase().testShardedGrouping(t)
}

func TestValueSourceGroupSelectorIgnoreDocsWithoutGroupField(t *testing.T) {
	newValueSourceGroupSelectorTestCase().testIgnoreDocsWithoutGroupField(t)
}
