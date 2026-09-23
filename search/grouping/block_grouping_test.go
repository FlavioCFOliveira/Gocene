// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/TestBlockGrouping.java
// (Apache Lucene 10.5.0), which extends AbstractGroupingTestCase.

// storedBook renders searcher.storedFields().document(doc).get("book").
func storedBook(t testing.TB, searcher *search.IndexSearcher, doc int) string {
	t.Helper()
	f := storedDocument(t, mustStoredFields(t, searcher), doc).Get("book")
	if f == nil {
		return ""
	}
	return f.StringValue()
}

// bookFiltered renders new BooleanQuery.Builder().add(topLevel, MUST)
// .add(new TermQuery(new Term("book", bookName)), FILTER).build().
func bookFiltered(topLevel search.Query, bookName string) search.Query {
	return search.NewBooleanQueryBuilder().
		Add(topLevel, search.MUST).
		Add(search.NewTermQuery(index.NewTerm("book", bookName)), search.FILTER).
		Build()
}

func TestBlockGroupingSimple(t *testing.T) {
	shard := newShard(t)
	indexRandomBookDocs(t, shard.writer)
	searcher := shard.getIndexSearcher(t)

	blockEndQuery := search.NewTermQuery(index.NewTerm("blockEnd", "true"))
	grouper := NewGroupingSearchQuery(blockEndQuery)
	grouper.SetGroupDocsLimit(10)

	topLevel := search.NewTermQuery(index.NewTerm("text", "grandmother"))
	tg := mustGroupingSearch[any](t, grouper, searcher, topLevel, 0, 5)

	// We're sorting by score, so the score of the top group should be the same as the
	// score of the top document from the same query with no grouping
	topDoc := mustSearch(t, searcher, topLevel, 1)
	if topDoc.ScoreDocs[0].Score != tg.Groups[0].ScoreDocs[0].Score {
		t.Fatalf("top score: expected %v, got %v", topDoc.ScoreDocs[0].Score, tg.Groups[0].ScoreDocs[0].Score)
	}

	for _, group := range tg.Groups {
		bookName := storedBook(t, searcher, group.ScoreDocs[0].Doc)
		// The contents of each group should be equal to the results of a search for
		// that group alone
		td := mustSearch(t, searcher, bookFiltered(topLevel, bookName), 10)
		assertScoreDocsEquals(t, td.ScoreDocs, group.ScoreDocs)
	}

	shard.close(t)
}

func TestBlockGroupingGroupOffset(t *testing.T) {
	shard := newShard(t)
	indexRandomBookDocs(t, shard.writer)
	searcher := shard.getIndexSearcher(t)

	blockEndQuery := search.NewTermQuery(index.NewTerm("blockEnd", "true"))
	grouper := NewGroupingSearchQuery(blockEndQuery)
	grouper.SetGroupDocsLimit(10)

	topLevel := search.NewTermQuery(index.NewTerm("text", "grandmother"))

	topN := 10
	offset := 1 + random().Intn(topN-1) // random().nextInt(1, topN)
	all := mustGroupingSearch[any](t, grouper, searcher, topLevel, 0, topN)
	offsetGroups := mustGroupingSearch[any](t, grouper, searcher, topLevel, offset, topN-offset)

	if offsetGroups == nil {
		t.Fatal("offsetGroups is null")
	}
	if len(all.Groups)-offset != len(offsetGroups.Groups) {
		t.Fatalf("groups: expected %d, got %d", len(all.Groups)-offset, len(offsetGroups.Groups))
	}

	for i := range offsetGroups.Groups {
		assertScoreDocsEquals(t, all.Groups[i+offset].ScoreDocs, offsetGroups.Groups[i].ScoreDocs)
	}

	shard.close(t)
}

func TestBlockGroupingTopLevelSort(t *testing.T) {
	shard := newShard(t)
	indexRandomBookDocs(t, shard.writer)
	searcher := shard.getIndexSearcher(t)

	sort := search.NewSort(search.NewSortField("length", spi.SortFieldTypeLong))

	blockEndQuery := search.NewTermQuery(index.NewTerm("blockEnd", "true"))
	grouper := NewGroupingSearchQuery(blockEndQuery)
	grouper.SetGroupDocsLimit(10)
	// groups returned sorted by length, chapters within group sorted by relevancy
	grouper.SetGroupSort(sort)

	topLevel := search.NewTermQuery(index.NewTerm("text", "grandmother"))
	tg := mustGroupingSearch[any](t, grouper, searcher, topLevel, 0, 5)

	// The sort value of the top doc in the top group should be the same as the sort value
	// of the top result from the same search done with no grouping
	topDoc := mustSearchSort(t, searcher, topLevel, 1, sort)
	if !javaEquals(topDoc.FieldDocs[0].Fields[0], tg.Groups[0].GroupSortValues[0]) {
		t.Fatalf("top sort value: expected %v, got %v", topDoc.FieldDocs[0].Fields[0], tg.Groups[0].GroupSortValues[0])
	}

	for i, group := range tg.Groups {
		bookName := storedBook(t, searcher, group.ScoreDocs[0].Doc)
		// The contents of each group should be equal to the results of a search for
		// that group alone, sorted by score
		td := mustSearch(t, searcher, bookFiltered(topLevel, bookName), 10)
		assertScoreDocsEquals(t, td.ScoreDocs, group.ScoreDocs)
		if i > 1 {
			assertLengthSortsBefore(t, tg.Groups[i-1], group)
		}
	}

	shard.close(t)
}

func TestBlockGroupingWithinGroupSort(t *testing.T) {
	shard := newShard(t)
	indexRandomBookDocs(t, shard.writer)
	searcher := shard.getIndexSearcher(t)

	sort := search.NewSort(search.NewSortField("length", spi.SortFieldTypeLong))

	blockEndQuery := search.NewTermQuery(index.NewTerm("blockEnd", "true"))
	grouper := NewGroupingSearchQuery(blockEndQuery)
	grouper.SetGroupDocsLimit(10)
	// groups returned sorted by relevancy, chapters within group sorted by length
	grouper.SetSortWithinGroup(sort)

	topLevel := search.NewTermQuery(index.NewTerm("text", "grandmother"))
	tg := mustGroupingSearch[any](t, grouper, searcher, topLevel, 0, 5)

	// We're sorting by score, so the score of the top group should be the same as the
	// score of the top document from the same query with no grouping
	topDoc := mustSearch(t, searcher, topLevel, 1)
	if topDoc.ScoreDocs[0].Score != tg.Groups[0].GroupSortValues[0].(float32) {
		t.Fatalf("top group sort value: expected %v, got %v", topDoc.ScoreDocs[0].Score, tg.Groups[0].GroupSortValues[0])
	}

	for i, group := range tg.Groups {
		bookName := storedBook(t, searcher, group.ScoreDocs[0].Doc)
		// The contents of each group should be equal to the results of a search for
		// that group alone, sorted by length
		td := mustSearchSort(t, searcher, bookFiltered(topLevel, bookName), 10, sort)
		assertFieldDocsEquals(t, td, group.ScoreDocs)
		// We're sorting by score, so the group sort value for each group should be a float,
		// and the value for the previous group should be higher or equal to the value for this one
		if i > 0 {
			prevScore := tg.Groups[i-1].GroupSortValues[0].(float32)
			thisScore := group.GroupSortValues[0].(float32)
			if !(prevScore >= thisScore) {
				t.Fatalf("group %d sort value %v > previous %v", i, thisScore, prevScore)
			}
		}
	}

	shard.close(t)
}

func TestBlockGroupingMergeBlockGroupsWithEmptyGroups(t *testing.T) {
	groupSort := search.RELEVANCE.GetSort()
	withinGroupSort := search.RELEVANCE.GetSort()

	group := NewGroupDocs(
		1.0,
		1.0,
		search.NewTotalHits(1, search.EQUAL_TO),
		[]*search.ScoreDoc{search.NewScoreDoc(0, 1.0, -1)},
		util.NewBytesRef([]byte("group1")),
		[]any{float32(1.0)})
	one := 1
	shardWithGroups := NewTopGroupsWithTotalGroupCount(
		NewTopGroups(groupSort, withinGroupSort, 1, 1, []*GroupDocs[*util.BytesRef]{group}, 1.0), &one)

	zero := 0
	shardWithNoGroups := NewTopGroupsWithTotalGroupCount(
		NewTopGroups(groupSort, withinGroupSort, 0, 0, []*GroupDocs[*util.BytesRef]{}, float32(math.NaN())), &zero)

	merged := MergeBlockGroups(
		[]*TopGroups[*util.BytesRef]{shardWithGroups, shardWithNoGroups}, search.RELEVANCE, 0, 5, search.RELEVANCE)

	if merged == nil {
		t.Fatal("merged is null")
	}
	if merged.TotalHitCount != 1 {
		t.Fatalf("totalHitCount: expected 1, got %d", merged.TotalHitCount)
	}
	if len(merged.Groups) != 1 {
		t.Fatalf("groups: expected 1, got %d", len(merged.Groups))
	}
	if !javaEquals(util.NewBytesRef([]byte("group1")), merged.Groups[0].GroupValue) {
		t.Fatalf("groupValue: expected group1, got %v", merged.Groups[0].GroupValue)
	}
}

func indexRandomBookDocs(t testing.TB, writer *testindex.RandomIndexWriter) {
	t.Helper()
	bookCount := atLeast(20)
	for i := 0; i < bookCount; i++ {
		mustAddDocuments(t, writer, createRandomBlock(t, i))
	}
}

func createRandomBlock(t testing.TB, book int) []*document.Document {
	t.Helper()
	block := make([]*document.Document, 0)
	bookName := "book" + strconv.Itoa(book)
	chapterCount := atLeast(10)
	for j := 0; j < chapterCount; j++ {
		doc := document.NewDocument()
		chapterName := "chapter" + strconv.Itoa(j)
		chapterText := randomText()
		doc.Add(newTextField(t, "book", bookName, true))
		doc.Add(newTextField(t, "chapter", chapterName, true))
		doc.Add(newTextField(t, "text", chapterText, false))
		// String.length() counts UTF-16 units; the texts are ASCII.
		doc.Add(mustNumericDVField(t, "length", int64(len(chapterText))))
		doc.Add(mustSortedDVField(t, "book", bookName))
		if j == chapterCount-1 {
			doc.Add(newTextField(t, "blockEnd", "true", false))
		}
		block = append(block, doc)
	}
	return block
}

var blockGroupingText = []string{
	"It was the day my grandmother exploded",
	"It was the best of times, it was the worst of times",
	"It was a bright cold morning in April",
	"It is a truth universally acknowledged",
	"I have just returned from a visit to my landlord",
	"I've been here and I've been there",
}

func randomText() string {
	var sb strings.Builder
	sb.WriteString(blockGroupingText[random().Intn(len(blockGroupingText))])
	sentences := random().Intn(20)
	for i := 0; i < sentences; i++ {
		sb.WriteString(" ")
		sb.WriteString(blockGroupingText[random().Intn(len(blockGroupingText))])
	}
	return sb.String()
}

// assertLengthSortsBefore renders TestBlockGrouping's private
// assertSortsBefore(GroupDocs<?>, GroupDocs<?>).
func assertLengthSortsBefore[T any](t testing.TB, first, second *GroupDocs[T]) {
	t.Helper()
	groupSortValues := second.GroupSortValues
	prevSortValues := first.GroupSortValues
	if !(prevSortValues[0].(int64) <= groupSortValues[0].(int64)) {
		t.Fatalf("sort values out of order: %v then %v", prevSortValues, groupSortValues)
	}
}

// fieldDocOfScoreDocBlocker names the production defect that keeps
// assertFieldDocsEquals from reading the sort values of a group's hits: Java's
// ScoreDoc[] holds the FieldDoc instances themselves, while Gocene's
// TopDocs.ScoreDocs holds *ScoreDoc values from which the FieldDoc (and its
// fields) cannot be recovered.
const fieldDocOfScoreDocBlocker = "requires (FieldDoc) scoreDoc: Gocene's TopDocs.ScoreDocs holds *ScoreDoc, " +
	"so the FieldDoc.fields of GroupDocs.scoreDocs() are unreachable (not ported)"

// assertFieldDocsEquals renders the static assertFieldDocsEquals(ScoreDoc[],
// ScoreDoc[]); expected is the TopFieldDocs whose ScoreDocs Java casts to
// FieldDoc.
func assertFieldDocsEquals(t testing.TB, expected *search.TopFieldDocs, actual []*search.ScoreDoc) {
	t.Helper()
	if len(expected.ScoreDocs) != len(actual) {
		t.Fatalf("scoreDocs length: expected %d, got %d", len(expected.ScoreDocs), len(actual))
	}
	for i := range expected.ScoreDocs {
		if expected.ScoreDocs[i].Doc != actual[i].Doc {
			t.Fatalf("scoreDocs[%d].doc: expected %d, got %d", i, expected.ScoreDocs[i].Doc, actual[i].Doc)
		}
		t.Fatal(fieldDocOfScoreDocBlocker)
	}
}
