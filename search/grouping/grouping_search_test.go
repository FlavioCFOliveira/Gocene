// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
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
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestGroupingSearch.java
// (Apache Lucene 10.5.0), which extends AbstractGroupingTestCase.

// Tests some very basic usages...
func TestGroupingSearchBasic(t *testing.T) {
	const groupField = "author"

	customType := document.NewFieldType()
	customType.SetStored(true)

	shard := newShard(t)
	w := shard.writer

	canUseIDV := true
	documents := make([]*document.Document, 0)
	// 0
	doc := document.NewDocument()
	addGroupingSearchGroupField(t, doc, groupField, "author1", canUseIDV)
	doc.Add(newTextField(t, "content", "random text", true))
	doc.Add(newField(t, "id", "1", customType))
	documents = append(documents, doc)

	// 1
	doc = document.NewDocument()
	addGroupingSearchGroupField(t, doc, groupField, "author1", canUseIDV)
	doc.Add(newTextField(t, "content", "some more random text", true))
	doc.Add(newField(t, "id", "2", customType))
	documents = append(documents, doc)

	// 2
	doc = document.NewDocument()
	addGroupingSearchGroupField(t, doc, groupField, "author1", canUseIDV)
	doc.Add(newTextField(t, "content", "some more random textual data", true))
	doc.Add(newField(t, "id", "3", customType))
	doc.Add(newStringField(t, "groupend", "x", false))
	documents = append(documents, doc)
	mustAddDocuments(t, w, documents)
	documents = documents[:0]

	// 3
	doc = document.NewDocument()
	addGroupingSearchGroupField(t, doc, groupField, "author2", canUseIDV)
	doc.Add(newTextField(t, "content", "some random text", true))
	doc.Add(newField(t, "id", "4", customType))
	doc.Add(newStringField(t, "groupend", "x", false))
	mustAddDocument(t, w, doc)

	// 4
	doc = document.NewDocument()
	addGroupingSearchGroupField(t, doc, groupField, "author3", canUseIDV)
	doc.Add(newTextField(t, "content", "some more random text", true))
	doc.Add(newField(t, "id", "5", customType))
	documents = append(documents, doc)

	// 5
	doc = document.NewDocument()
	addGroupingSearchGroupField(t, doc, groupField, "author3", canUseIDV)
	doc.Add(newTextField(t, "content", "random", true))
	doc.Add(newField(t, "id", "6", customType))
	doc.Add(newStringField(t, "groupend", "x", false))
	documents = append(documents, doc)
	mustAddDocuments(t, w, documents)
	documents = documents[:0]

	// 6 -- no author field
	doc = document.NewDocument()
	doc.Add(newTextField(t, "content", "random word stuck in alot of other text", true))
	doc.Add(newField(t, "id", "6", customType))
	doc.Add(newStringField(t, "groupend", "x", false))

	mustAddDocument(t, w, doc)

	indexSearcher := shard.getIndexSearcher(t)
	indexSearcher.SetSimilarity(search.NewLuceneBM25Similarity())
	mustClose(t, w)

	groupSort := search.RELEVANCE
	groupingSearch, byFunction := createRandomGroupingSearch(groupField, groupSort, 5, canUseIDV)

	query := search.NewTermQuery(index.NewTerm("content", "random"))
	if byFunction {
		checkGroupingSearchBasicGroups(t, mustGroupingSearch[mutable.MutableValue](t, groupingSearch, indexSearcher, query, 0, 10))
	} else {
		checkGroupingSearchBasicGroups(t, mustGroupingSearch[*util.BytesRef](t, groupingSearch, indexSearcher, query, 0, 10))
	}

	lastDocInBlock := search.NewTermQuery(index.NewTerm("groupend", "x"))
	groupingSearch = NewGroupingSearchQuery(lastDocInBlock)
	groups := mustGroupingSearch[any](t, groupingSearch, indexSearcher, search.NewTermQuery(index.NewTerm("content", "random")), 0, 10)

	if groups.TotalHitCount != 7 {
		t.Fatalf("totalHitCount: expected 7, got %d", groups.TotalHitCount)
	}
	if groups.TotalGroupedHitCount != 7 {
		t.Fatalf("totalGroupedHitCount: expected 7, got %d", groups.TotalGroupedHitCount)
	}
	if groups.TotalGroupCount == nil || *groups.TotalGroupCount != 4 {
		t.Fatalf("totalGroupCount: expected 4, got %v", groups.TotalGroupCount)
	}
	if len(groups.Groups) != 4 {
		t.Fatalf("groups: expected 4, got %d", len(groups.Groups))
	}

	shard.close(t)
}

// checkGroupingSearchBasicGroups renders the assertions testBasic makes on
// the TopGroups<?> of the field or function grouping search.
func checkGroupingSearchBasicGroups[T any](t *testing.T, groups *TopGroups[T]) {
	t.Helper()
	if groups.TotalHitCount != 7 {
		t.Fatalf("totalHitCount: expected 7, got %d", groups.TotalHitCount)
	}
	if groups.TotalGroupedHitCount != 7 {
		t.Fatalf("totalGroupedHitCount: expected 7, got %d", groups.TotalGroupedHitCount)
	}
	if len(groups.Groups) != 4 {
		t.Fatalf("groups: expected 4, got %d", len(groups.Groups))
	}

	// relevance order: 5, 0, 3, 4, 1, 2, 6

	// the later a document is added the higher this docId
	// value
	group := groups.Groups[0]
	compareGroupValue(t, ptr("author3"), group.GroupValue)
	assertGroupDocs(t, group, 5, 4)
	if !(group.ScoreDocs[0].Score >= group.ScoreDocs[1].Score) {
		t.Fatal("scoreDocs[0].score < scoreDocs[1].score")
	}

	group = groups.Groups[1]
	compareGroupValue(t, ptr("author1"), group.GroupValue)
	assertGroupDocs(t, group, 0, 1, 2)
	if !(group.ScoreDocs[0].Score >= group.ScoreDocs[1].Score) {
		t.Fatal("scoreDocs[0].score < scoreDocs[1].score")
	}
	if !(group.ScoreDocs[1].Score >= group.ScoreDocs[2].Score) {
		t.Fatal("scoreDocs[1].score < scoreDocs[2].score")
	}

	group = groups.Groups[2]
	compareGroupValue(t, ptr("author2"), group.GroupValue)
	assertGroupDocs(t, group, 3)

	group = groups.Groups[3]
	compareGroupValue(t, nil, group.GroupValue)
	assertGroupDocs(t, group, 6)
}

func ptr(s string) *string { return &s }

func assertGroupDocs[T any](t *testing.T, group *GroupDocs[T], docs ...int) {
	t.Helper()
	if len(group.ScoreDocs) != len(docs) {
		t.Fatalf("scoreDocs: expected %d, got %d", len(docs), len(group.ScoreDocs))
	}
	for i, d := range docs {
		if group.ScoreDocs[i].Doc != d {
			t.Fatalf("scoreDocs[%d].doc: expected %d, got %d", i, d, group.ScoreDocs[i].Doc)
		}
	}
}

func addGroupingSearchGroupField(t testing.TB, doc *document.Document, groupField, value string, canUseIDV bool) {
	t.Helper()
	doc.Add(newTextField(t, groupField, value, true))
	if canUseIDV {
		doc.Add(mustSortedDVField(t, groupField, value))
	}
}

// compareGroupValue renders the private compareGroupValue(String,
// GroupDocs<?>); a nil expected renders Java's null.
func compareGroupValue(t *testing.T, expected *string, groupValue any) {
	t.Helper()
	if expected == nil {
		switch v := groupValue.(type) {
		case nil:
			return
		case *util.BytesRef:
			if v == nil || v.Length == 0 {
				return
			}
		case *mutable.MutableValueStr:
			return
		}
		t.Fatalf("expected a null group value, got %v", groupValue)
	}

	switch v := groupValue.(type) {
	case *util.BytesRef:
		if !javaEquals(util.NewBytesRef([]byte(*expected)), v) {
			t.Fatalf("group value: expected %q, got %v", *expected, v)
		}
	case *mutable.MutableValueStr:
		mv := mutable.NewMutableValueStr()
		mv.Value = *expected
		if !javaEquals(mv, v) {
			t.Fatalf("group value: expected %q, got %v", *expected, v)
		}
	default:
		t.Fatalf("unexpected group value %T", groupValue)
	}
}

// createRandomGroupingSearch renders the private
// createRandomGroupingSearch(String, Sort, int, boolean); byFunction reports
// whether the ValueSource constructor was chosen, which fixes the group value
// type Java leaves to the TopGroups<?> wildcard.
func createRandomGroupingSearch(groupField string, groupSort *search.Sort, docsInGroup int,
	canUseIDV bool) (groupingSearch *GroupingSearch, byFunction bool) {
	if random().Intn(2) == 0 {
		vs := valuesource.NewBytesRefFieldSource(groupField)
		groupingSearch = NewGroupingSearchValueSource(vs, function.Context{})
		byFunction = true
	} else {
		groupingSearch = NewGroupingSearch(groupField)
	}

	groupingSearch.SetGroupSort(groupSort)
	groupingSearch.SetGroupDocsLimit(docsInGroup)

	if random().Intn(2) == 0 {
		groupingSearch.SetCachingInMB(4.0, true)
	}

	return groupingSearch, byFunction
}

func TestGroupingSearchSetAllGroups(t *testing.T) {
	dir := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	w := newRandomIndexWriterWithConfig(t, dir, iwc)
	doc := document.NewDocument()
	doc.Add(newField(t, "group", "foo", document.StringFieldTypeNotStored))
	doc.Add(mustSortedDVField(t, "group", "foo"))
	mustAddDocument(t, w, doc)

	indexSearcher := newSearcher(t, mustGetReader(t, w))
	mustClose(t, w)

	gs := NewGroupingSearch("group")
	gs.SetAllGroups(true)
	groups := mustGroupingSearch[*util.BytesRef](t, gs, indexSearcher, search.NewTermQuery(index.NewTerm("group", "foo")), 0, 10)
	if groups.TotalHitCount != 1 {
		t.Fatalf("totalHitCount: expected 1, got %d", groups.TotalHitCount)
	}
	// assertEquals(1, groups.totalGroupCount.intValue());
	if groups.TotalGroupedHitCount != 1 {
		t.Fatalf("totalGroupedHitCount: expected 1, got %d", groups.TotalGroupedHitCount)
	}
	if n := len(GroupingSearchGetAllMatchingGroups[*util.BytesRef](gs)); n != 1 {
		t.Fatalf("allMatchingGroups: expected 1, got %d", n)
	}
	mustClose(t, indexSearcher.GetIndexReader(), dir)
}

// mustAddDocuments renders RandomIndexWriter.addDocuments(Iterable).
func mustAddDocuments(t testing.TB, w interface {
	AddDocuments(docs []*document.Document) (int64, error)
}, docs []*document.Document) {
	t.Helper()
	if _, err := w.AddDocuments(docs); err != nil {
		t.Fatalf("addDocuments: %v", err)
	}
}
