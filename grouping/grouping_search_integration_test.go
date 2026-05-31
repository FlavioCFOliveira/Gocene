// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping_test

// Tests for grouping.GroupingSearch. The pure-logic tests drive the assembly
// pipeline via a synthetic groupValueResolver (exported through
// export_test.go) because IndexSearcher.Doc currently fails on segment
// readers without core readers — see
// project-gocene-segmentreader-corereaders-gap. Once that gap is closed the
// skipped end-to-end test below can be unblocked.

import (
	"context"
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/grouping"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// mapResolver builds a GroupValueResolverForTest from a docID -> value map.
// Docs missing from the map are reported as elided (ok=false).
func mapResolver(values map[int]string) grouping.GroupValueResolverForTest {
	return func(docID int) (string, bool, error) {
		v, ok := values[docID]
		return v, ok, nil
	}
}

// TestGroupingSearch_AssembleTopGroups_GroupByValue exercises the core
// grouping logic: documents share group keys, the assembler buckets them
// and computes TotalHits per group.
func TestGroupingSearch_AssembleTopGroups_GroupByValue(t *testing.T) {
	values := map[int]string{0: "A", 1: "B", 2: "A", 3: "C", 4: "B", 5: "A"}
	hits := []grouping.CapturedHitForTest{
		{DocID: 0, Score: 1.0},
		{DocID: 1, Score: 2.0},
		{DocID: 2, Score: 1.5},
		{DocID: 3, Score: 0.5},
		{DocID: 4, Score: 1.8},
		{DocID: 5, Score: 0.9},
	}

	gs := grouping.NewGroupingSearch("category").SetGroupLimit(10).SetDocLimit(10)
	tg, err := gs.AssembleTopGroupsForTest(mapResolver(values), hits)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if got := tg.GetTotalHitCount(); got != len(hits) {
		t.Errorf("TotalHitCount = %d, want %d", got, len(hits))
	}
	if got := tg.GetTotalGroupCount(); got != 3 {
		t.Errorf("TotalGroupCount = %d, want 3", got)
	}
	if got := tg.GetGroupCount(); got != 3 {
		t.Fatalf("GroupCount = %d, want 3", got)
	}
	want := map[string]int{"A": 3, "B": 2, "C": 1}
	for i := 0; i < tg.GetGroupCount(); i++ {
		g := tg.GetGroup(i)
		v, ok := g.GroupValue.(string)
		if !ok {
			t.Fatalf("group %d value is %T, want string", i, g.GroupValue)
		}
		if g.TotalHits != want[v] {
			t.Errorf("group %q TotalHits = %d, want %d", v, g.TotalHits, want[v])
		}
	}
}

// TestGroupingSearch_AssembleTopGroups_ElideMissingField checks that docs
// whose resolver returns ok=false are dropped from the grouped output but
// still counted in TotalHitCount (the resolver was asked, the doc just
// lacked the field).
func TestGroupingSearch_AssembleTopGroups_ElideMissingField(t *testing.T) {
	values := map[int]string{0: "A", 2: "B"} // docs 1 and 3 elide
	hits := []grouping.CapturedHitForTest{
		{DocID: 0, Score: 1.0},
		{DocID: 1, Score: 1.0},
		{DocID: 2, Score: 1.0},
		{DocID: 3, Score: 1.0},
	}

	gs := grouping.NewGroupingSearch("category").SetGroupLimit(10).SetDocLimit(10)
	tg, err := gs.AssembleTopGroupsForTest(mapResolver(values), hits)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if tg.GetTotalHitCount() != 4 {
		t.Errorf("TotalHitCount = %d, want 4", tg.GetTotalHitCount())
	}
	if tg.GetGroupCount() != 2 {
		t.Errorf("GroupCount = %d, want 2", tg.GetGroupCount())
	}
}

// TestGroupingSearch_AssembleTopGroups_GroupOffsetAndLimit verifies that
// the post-sort slice [groupOffset : groupOffset+groupLimit) is applied.
func TestGroupingSearch_AssembleTopGroups_GroupOffsetAndLimit(t *testing.T) {
	values := map[int]string{0: "alpha", 1: "beta", 2: "gamma", 3: "delta"}
	hits := []grouping.CapturedHitForTest{
		{DocID: 0, Score: 1.0},
		{DocID: 1, Score: 1.0},
		{DocID: 2, Score: 1.0},
		{DocID: 3, Score: 1.0},
	}

	groupSort := search.NewSort(search.NewSortField("category", search.SortFieldTypeString))
	gs := grouping.NewGroupingSearch("category").
		SetGroupSort(*groupSort).
		SetGroupOffset(1).
		SetGroupLimit(2).
		SetDocLimit(1)

	tg, err := gs.AssembleTopGroupsForTest(mapResolver(values), hits)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	// Sorted ascending by string: alpha, beta, delta, gamma. Offset 1 +
	// limit 2 yields [beta, delta].
	if got := tg.GetGroupCount(); got != 2 {
		t.Fatalf("GroupCount = %d, want 2", got)
	}
	wantOrder := []string{"beta", "delta"}
	for i, want := range wantOrder {
		got := tg.GetGroup(i).GroupValue.(string)
		if got != want {
			t.Errorf("group[%d] = %q, want %q", i, got, want)
		}
	}
}

// TestGroupingSearch_AssembleTopGroups_DocOffsetAndLimit verifies that
// docOffset and docLimit slice the per-group document list.
func TestGroupingSearch_AssembleTopGroups_DocOffsetAndLimit(t *testing.T) {
	values := map[int]string{0: "A", 1: "A", 2: "A", 3: "A", 4: "A"}
	hits := []grouping.CapturedHitForTest{
		{DocID: 0, Score: 5.0},
		{DocID: 1, Score: 4.0},
		{DocID: 2, Score: 3.0},
		{DocID: 3, Score: 2.0},
		{DocID: 4, Score: 1.0},
	}

	gs := grouping.NewGroupingSearch("category").
		SetGroupLimit(10).
		SetDocOffset(1).
		SetDocLimit(2)
	tg, err := gs.AssembleTopGroupsForTest(mapResolver(values), hits)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if tg.GetGroupCount() != 1 {
		t.Fatalf("GroupCount = %d, want 1", tg.GetGroupCount())
	}
	g := tg.GetGroup(0)
	if g.TotalHits != 5 {
		t.Errorf("TotalHits = %d, want 5", g.TotalHits)
	}
	if len(g.ScoreDocs) != 2 {
		t.Fatalf("ScoreDocs len = %d, want 2 (docOffset=1, docLimit=2)", len(g.ScoreDocs))
	}
	// Default docSort is by score descending; after offset=1 we drop doc 0
	// (the highest scorer) and keep doc 1 then doc 2.
	if g.ScoreDocs[0].Doc != 1 || g.ScoreDocs[1].Doc != 2 {
		t.Errorf("docs = [%d,%d], want [1,2]", g.ScoreDocs[0].Doc, g.ScoreDocs[1].Doc)
	}
}

// TestGroupingSearch_AssembleTopGroups_DocSortByScoreDescending checks that
// the default docSort (score descending) orders documents within a group
// from highest to lowest score.
func TestGroupingSearch_AssembleTopGroups_DocSortByScoreDescending(t *testing.T) {
	values := map[int]string{0: "A", 1: "A", 2: "A"}
	hits := []grouping.CapturedHitForTest{
		{DocID: 0, Score: 0.1},
		{DocID: 1, Score: 0.9},
		{DocID: 2, Score: 0.5},
	}
	gs := grouping.NewGroupingSearch("category").SetGroupLimit(10).SetDocLimit(10)
	tg, err := gs.AssembleTopGroupsForTest(mapResolver(values), hits)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	g := tg.GetGroup(0)
	if len(g.ScoreDocs) != 3 {
		t.Fatalf("ScoreDocs len = %d, want 3", len(g.ScoreDocs))
	}
	// Expected: highest first.
	wantDocs := []int{1, 2, 0}
	for i, want := range wantDocs {
		if g.ScoreDocs[i].Doc != want {
			t.Errorf("score doc[%d] = %d, want %d", i, g.ScoreDocs[i].Doc, want)
		}
	}
}

// TestGroupingSearch_AssembleTopGroups_IncludeMaxScore confirms that when
// includeMaxScore is enabled the per-group MaxScore reflects the largest
// captured score for that group.
func TestGroupingSearch_AssembleTopGroups_IncludeMaxScore(t *testing.T) {
	values := map[int]string{0: "A", 1: "B", 2: "A"}
	hits := []grouping.CapturedHitForTest{
		{DocID: 0, Score: 0.3},
		{DocID: 1, Score: 5.0},
		{DocID: 2, Score: 7.5},
	}
	gs := grouping.NewGroupingSearch("category").
		SetGroupLimit(10).
		SetDocLimit(10).
		SetIncludeMaxScore(true)
	tg, err := gs.AssembleTopGroupsForTest(mapResolver(values), hits)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	want := map[string]float32{"A": 7.5, "B": 5.0}
	for i := 0; i < tg.GetGroupCount(); i++ {
		g := tg.GetGroup(i)
		v := g.GroupValue.(string)
		if g.MaxScore != want[v] {
			t.Errorf("group %q MaxScore = %f, want %f", v, g.MaxScore, want[v])
		}
	}
}

// TestGroupingSearch_AssembleTopGroups_EmptyHits returns an empty TopGroups
// without invoking the resolver.
func TestGroupingSearch_AssembleTopGroups_EmptyHits(t *testing.T) {
	called := false
	resolver := func(docID int) (string, bool, error) {
		called = true
		return "", false, nil
	}
	gs := grouping.NewGroupingSearch("category")
	tg, err := gs.AssembleTopGroupsForTest(resolver, nil)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if called {
		t.Error("resolver should not be invoked for empty hit set")
	}
	if tg.GetGroupCount() != 0 {
		t.Errorf("GroupCount = %d, want 0", tg.GetGroupCount())
	}
	if tg.GetTotalGroupCount() != 0 {
		t.Errorf("TotalGroupCount = %d, want 0", tg.GetTotalGroupCount())
	}
}

// TestGroupingSearch_AssembleTopGroups_ResolverError surfaces a resolver
// error to the caller.
func TestGroupingSearch_AssembleTopGroups_ResolverError(t *testing.T) {
	sentinel := errors.New("resolver failure")
	resolver := func(docID int) (string, bool, error) {
		return "", false, sentinel
	}
	gs := grouping.NewGroupingSearch("category")
	_, err := gs.AssembleTopGroupsForTest(resolver, []grouping.CapturedHitForTest{{DocID: 0, Score: 1}})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel %v", err, sentinel)
	}
}

// TestGroupingSearch_NilSearcher exercises the input-validation guard.
func TestGroupingSearch_NilSearcher(t *testing.T) {
	gs := grouping.NewGroupingSearch("category")
	if _, err := gs.Search(context.Background(), nil, search.NewMatchAllDocsQuery()); err == nil {
		t.Error("expected error for nil searcher")
	}
	if _, err := gs.SearchWithCollector(context.Background(), nil, search.NewMatchAllDocsQuery(), &countingCollector{}); err == nil {
		t.Error("expected error for nil searcher in SearchWithCollector")
	}
}

// TestGroupingSearch_EndToEnd_StoredFields is the round-trip test against a
// real IndexWriter + DirectoryReader + IndexSearcher. It is currently
// skipped because IndexSearcher.Doc fails on segment readers that were not
// opened with core readers — see
// project-gocene-segmentreader-corereaders-gap. The test body is preserved
// verbatim so the gap closure can re-enable it by removing the t.Skip.
func TestGroupingSearch_EndToEnd_StoredFields(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i, cat := range []string{"A", "B", "A", "C", "B", "A"} {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('a'+i)), true)
		doc.Add(idField)
		catField, _ := document.NewStringField("category", cat, true)
		doc.Add(catField)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()
	searcher := search.NewIndexSearcher(reader)
	gs := grouping.NewGroupingSearch("category").SetGroupLimit(10).SetDocLimit(10)
	tg, err := gs.Search(context.Background(), searcher, search.NewMatchAllDocsQuery())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if tg.GetTotalGroupCount() != 3 {
		t.Errorf("TotalGroupCount = %d, want 3", tg.GetTotalGroupCount())
	}
}

// countingCollector is a minimal Collector that counts every Collect() call.
type countingCollector struct {
	collected int
}

func (c *countingCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

func (c *countingCollector) GetLeafCollector(_ *index.LeafReaderContext) (search.LeafCollector, error) {
	return &countingLeafCollector{parent: c}, nil
}

type countingLeafCollector struct {
	parent *countingCollector
	scorer search.Scorer
}

func (c *countingLeafCollector) SetScorer(s search.Scorer) error { c.scorer = s; return nil }
func (c *countingLeafCollector) Collect(_ int) error             { c.parent.collected++; return nil }
