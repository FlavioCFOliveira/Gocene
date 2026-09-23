// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/valuesource"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/mutable"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestAllGroupsCollector.java
// (Apache Lucene 10.5.0).

func TestAllGroupsCollectorTotalGroupCount(t *testing.T) {
	const groupField = "author"
	customType := document.NewFieldType()
	customType.SetStored(true)

	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)

	// 0
	doc := document.NewDocument()
	addAllGroupsGroupField(t, doc, groupField, "author1")
	doc.Add(newTextField(t, "content", "random text", true))
	doc.Add(newField(t, "id", "1", customType))
	mustAddDocument(t, w, doc)

	// 1
	doc = document.NewDocument()
	addAllGroupsGroupField(t, doc, groupField, "author1")
	doc.Add(newTextField(t, "content", "some more random text blob", true))
	doc.Add(newField(t, "id", "2", customType))
	mustAddDocument(t, w, doc)

	// 2
	doc = document.NewDocument()
	addAllGroupsGroupField(t, doc, groupField, "author1")
	doc.Add(newTextField(t, "content", "some more random textual data", true))
	doc.Add(newField(t, "id", "3", customType))
	mustAddDocument(t, w, doc)
	mustCommit(t, w) // To ensure a second segment

	// 3
	doc = document.NewDocument()
	addAllGroupsGroupField(t, doc, groupField, "author2")
	doc.Add(newTextField(t, "content", "some random text", true))
	doc.Add(newField(t, "id", "4", customType))
	mustAddDocument(t, w, doc)

	// 4
	doc = document.NewDocument()
	addAllGroupsGroupField(t, doc, groupField, "author3")
	doc.Add(newTextField(t, "content", "some more random text", true))
	doc.Add(newField(t, "id", "5", customType))
	mustAddDocument(t, w, doc)

	// 5
	doc = document.NewDocument()
	addAllGroupsGroupField(t, doc, groupField, "author3")
	doc.Add(newTextField(t, "content", "random blob", true))
	doc.Add(newField(t, "id", "6", customType))
	mustAddDocument(t, w, doc)

	// 6 -- no author field
	doc = document.NewDocument()
	doc.Add(newTextField(t, "content", "random word stuck in alot of other text", true))
	doc.Add(newField(t, "id", "6", customType))
	mustAddDocument(t, w, doc)

	indexSearcher := newSearcher(t, mustGetReader(t, w))
	mustClose(t, w)

	allGroupsCollectorManager := createRandomAllGroupsCollectorManager(groupField)
	groups := searchAllGroupsSize(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "random")), allGroupsCollectorManager)
	if groups != 4 {
		t.Fatalf("groups: expected 4, got %d", groups)
	}

	allGroupsCollectorManager = createRandomAllGroupsCollectorManager(groupField)
	groups = searchAllGroupsSize(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "some")), allGroupsCollectorManager)
	if groups != 3 {
		t.Fatalf("groups: expected 3, got %d", groups)
	}

	allGroupsCollectorManager = createRandomAllGroupsCollectorManager(groupField)
	groups = searchAllGroupsSize(t, indexSearcher, search.NewTermQuery(index.NewTerm("content", "blob")), allGroupsCollectorManager)
	if groups != 2 {
		t.Fatalf("groups: expected 2, got %d", groups)
	}

	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

func addAllGroupsGroupField(t testing.TB, doc *document.Document, groupField, value string) {
	t.Helper()
	doc.Add(newTextField(t, groupField, value, true))
	doc.Add(mustSortedDVField(t, groupField, value))
}

// createRandomAllGroupsCollectorManager renders the private
// createRandomCollectorManager(String), whose AllGroupsCollectorManager<?>
// holds either BytesRef or MutableValue groups.
func createRandomAllGroupsCollectorManager(groupField string) any {
	if random().Intn(2) == 0 {
		return NewAllGroupsCollectorManager(func() GroupSelector[*util.BytesRef] { return NewTermGroupSelector(groupField) })
	}
	vs := valuesource.NewBytesRefFieldSource(groupField)
	return NewAllGroupsCollectorManager(func() GroupSelector[mutable.MutableValue] {
		return NewValueSourceGroupSelector(vs, function.Context{})
	})
}

// searchAllGroupsSize renders indexSearcher.search(query, manager).size() for
// the AllGroupsCollectorManager<?> wildcard.
func searchAllGroupsSize(t testing.TB, s *search.IndexSearcher, q search.Query, manager any) int {
	t.Helper()
	switch m := manager.(type) {
	case *AllGroupsCollectorManager[*util.BytesRef]:
		groups, err := search.SearchWithCollectorManager[*AllGroupsCollector[*util.BytesRef], []*util.BytesRef](s, q, m)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		return len(groups)
	case *AllGroupsCollectorManager[mutable.MutableValue]:
		groups, err := search.SearchWithCollectorManager[*AllGroupsCollector[mutable.MutableValue], []mutable.MutableValue](s, q, m)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		return len(groups)
	default:
		panic(fmt.Sprintf("unexpected manager %T", manager))
	}
}
