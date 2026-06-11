// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestBooleanQueryVisitSubscorers.java
//
// Walks the scorer tree to validate sub-scorer visitation: the disjunction /
// conjunction tests sum per-document term frequencies gathered from the leaf
// TermScorers, and the matches / summary tests assert the shape and class names
// of the scorer tree.
//
// HONEST FEATURE GAP (no t.Skip, faithful assertions retained): two Gocene
// capabilities these tests rely on are not yet implemented, so several
// assertions fail honestly at runtime rather than being silenced:
//   - Scorer tree traversal: Gocene's Scorer and Scorable are deliberately
//     structurally incompatible interfaces (Scorer.Score()->float32 vs
//     Scorable.Score()->(float32,error)), and the composite scorers'
//     getChildren() return nil with the source comment "A future bridging
//     sprint will revisit." (see conjunction_scorer.go / disjunction_scorer.go).
//     So the term-frequency walk collects nothing.
//   - RawTFSimilarity (score == raw freq) only implements the Lucene-faithful
//     LuceneSimilarity surface (Scorer104), not the legacy Similarity interface
//     that TermWeight scores through, so it cannot be wired via SetSimilarity to
//     make a TermScorer's score equal its frequency.
// These tests therefore assert the exact Lucene-expected values and fail until
// the bridging work lands.

package search_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	visitF1 = "title"
	visitF2 = "body"
)

// childrenProvider is the optional interface a Gocene Scorer may implement to
// expose its sub-scorers, mirroring Scorable.getChildren in Lucene.
type childrenProvider interface {
	GetChildren() ([]search.ChildScorable, error)
}

// visitSubscorersIndex builds the three-document corpus used by the suite.
func visitSubscorersIndex(t *testing.T) (*search.IndexSearcher, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	add := func(v1, v2 string) {
		doc := document.NewDocument()
		f1, e1 := document.NewTextField(visitF1, v1, true)
		if e1 != nil {
			t.Fatalf("NewTextField(title): %v", e1)
		}
		f2, e2 := document.NewTextField(visitF2, v2, true)
		if e2 != nil {
			t.Fatalf("NewTextField(body): %v", e2)
		}
		doc.Add(f1)
		doc.Add(f2)
		if addErr := w.AddDocument(doc); addErr != nil {
			t.Fatalf("AddDocument: %v", addErr)
		}
	}
	add("lucene", "lucene is a very popular search engine library")
	add("solr", "solr is a very popular search server and is using lucene")
	add("nutch", "nutch is an internet search engine with web crawler and is using lucene and hadoop")
	if err = w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err = w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return search.NewIndexSearcher(reader), func() {
		_ = reader.Close()
		_ = dir.Close()
	}
}

// visitTerm is a TermQuery on a named field.
func visitTerm(field, text string) *search.TermQuery {
	return search.NewTermQuery(index.NewTerm(field, text))
}

// freqCollector ports MyCollector: on setScorer it walks the scorer tree to
// gather the leaf TermScorers, and on collect it sums their scores (which equal
// the term frequencies under RawTFSimilarity) into a per-doc map.
type freqCollector struct {
	docCounts map[int]int
}

func newFreqCollector() *freqCollector {
	return &freqCollector{docCounts: make(map[int]int)}
}

func (c *freqCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

func (c *freqCollector) GetLeafCollector(ctx *index.LeafReaderContext) (search.LeafCollector, error) {
	return &freqLeafCollector{parent: c, docBase: ctx.DocBase()}, nil
}

type freqLeafCollector struct {
	parent  *freqCollector
	docBase int
	leaves  []search.Scorer
	root    search.Scorer
}

func (lc *freqLeafCollector) SetScorer(scorer search.Scorer) error {
	lc.root = scorer
	lc.leaves = lc.leaves[:0]
	lc.fillLeaves(scorer)
	return nil
}

// fillLeaves walks the scorer tree adding TermScorers to the leaf set,
// mirroring MyCollector.fillLeaves.
//
// In Gocene the root scorer is a Scorer but its children are exposed as
// Scorable (the two interfaces are structurally incompatible). The leaf
// TermScorers needed to read scores are therefore unreachable through the
// children of a composite scorer until the Scorer/Scorable bridge lands; this
// walk records what it can (a root TermScorer) and otherwise collects nothing,
// which is the honest gap these tests surface.
func (lc *freqLeafCollector) fillLeaves(scorer search.Scorer) {
	if _, ok := scorer.(*search.TermScorer); ok {
		lc.leaves = append(lc.leaves, scorer)
		return
	}
	provider, ok := scorer.(childrenProvider)
	if !ok {
		return
	}
	children, err := provider.GetChildren()
	if err != nil {
		return
	}
	for _, child := range children {
		lc.fillLeavesScorable(child.Child)
	}
}

// fillLeavesScorable continues the walk over Scorable children. A Scorable is
// never a Scorer in Gocene (the interfaces are structurally incompatible), so a
// leaf Scorable's score cannot be read here; the walk recurses to deeper
// children but cannot record leaves until the Scorer/Scorable bridge lands.
func (lc *freqLeafCollector) fillLeavesScorable(scorable search.Scorable) {
	children, err := scorable.GetChildren()
	if err != nil {
		return
	}
	for _, child := range children {
		lc.fillLeavesScorable(child.Child)
	}
}

func (lc *freqLeafCollector) Collect(doc int) error {
	freq := 0
	for _, scorer := range lc.leaves {
		if doc == scorer.DocID() {
			freq += int(scorer.Score())
		}
	}
	lc.parent.docCounts[doc+lc.docBase] = freq
	return nil
}

// getDocCounts runs query with the freq-summing collector and returns the
// per-doc frequency map, mirroring TestBooleanQueryVisitSubscorers.getDocCounts.
func getDocCounts(t *testing.T, searcher *search.IndexSearcher, query search.Query) map[int]int {
	t.Helper()
	collector := newFreqCollector()
	if err := searcher.SearchWithCollector(query, collector); err != nil {
		t.Fatalf("SearchWithCollector: %v", err)
	}
	return collector.docCounts
}

func assertFreq(t *testing.T, tfs map[int]int, doc, want int) {
	t.Helper()
	if got := tfs[doc]; got != want {
		t.Errorf("doc %d: term frequency = %d, want %d", doc, got, want)
	}
}

func TestBooleanQueryVisitSubscorers_Disjunctions(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()
	searcher.SetSimilarity(search.NewRawTFSimilarity())

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "lucene"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "search"), search.SHOULD)

	tfs := getDocCounts(t, searcher, bq)
	if len(tfs) != 3 {
		t.Errorf("doc count = %d, want 3", len(tfs))
	}
	assertFreq(t, tfs, 0, 3) // f1:lucene + f2:lucene + f2:search
	assertFreq(t, tfs, 1, 2) // f2:search + f2:lucene
	assertFreq(t, tfs, 2, 2) // f2:search + f2:lucene
}

func TestBooleanQueryVisitSubscorers_NestedDisjunctions(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()
	searcher.SetSimilarity(search.NewRawTFSimilarity())

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq2 := search.NewBooleanQuery()
	bq2.Add(visitTerm(visitF2, "lucene"), search.SHOULD)
	bq2.Add(visitTerm(visitF2, "search"), search.SHOULD)
	bq.Add(bq2, search.SHOULD)

	tfs := getDocCounts(t, searcher, bq)
	if len(tfs) != 3 {
		t.Errorf("doc count = %d, want 3", len(tfs))
	}
	assertFreq(t, tfs, 0, 3)
	assertFreq(t, tfs, 1, 2)
	assertFreq(t, tfs, 2, 2)
}

func TestBooleanQueryVisitSubscorers_Conjunctions(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()
	searcher.SetSimilarity(search.NewRawTFSimilarity())

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF2, "lucene"), search.MUST)
	bq.Add(visitTerm(visitF2, "is"), search.MUST)

	tfs := getDocCounts(t, searcher, bq)
	if len(tfs) != 3 {
		t.Errorf("doc count = %d, want 3", len(tfs))
	}
	assertFreq(t, tfs, 0, 2) // f2:lucene + f2:is
	assertFreq(t, tfs, 1, 3) // f2:is + f2:is + f2:lucene
	assertFreq(t, tfs, 2, 3) // f2:is + f2:is + f2:lucene
}

// scorerChildrenCount advances the leaf-0 scorer for query to its first doc and
// returns the number of immediate children of the scorer tree root, mirroring
// the s.getChildren().size() assertions.
func scorerChildrenCount(t *testing.T, searcher *search.IndexSearcher, query search.Query) int {
	t.Helper()
	rewritten, err := query.Rewrite(searcher.GetIndexReader())
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	w, err := searcher.CreateWeight(rewritten, search.COMPLETE, 1)
	if err != nil {
		t.Fatalf("CreateWeight: %v", err)
	}
	leaves, err := searcher.GetIndexReader().Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	s, err := w.Scorer(leaves[0])
	if err != nil {
		t.Fatalf("Scorer: %v", err)
	}
	if s == nil {
		t.Fatalf("scorer is nil")
	}
	nd, err := s.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if nd != 0 {
		t.Errorf("nextDoc = %d, want 0", nd)
	}
	provider, ok := s.(childrenProvider)
	if !ok {
		t.Errorf("scorer %T does not expose children (Scorer/Scorable bridge not implemented)", s)
		return 0
	}
	children, err := provider.GetChildren()
	if err != nil {
		t.Fatalf("GetChildren: %v", err)
	}
	return len(children)
}

func TestBooleanQueryVisitSubscorers_DisjunctionMatches(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()
	searcher.SetSimilarity(search.NewRawTFSimilarity())

	bq1 := search.NewBooleanQuery()
	bq1.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq1.Add(search.NewPhraseQueryWithStrings(visitF2, "search", "engine"), search.SHOULD)
	if n := scorerChildrenCount(t, searcher, bq1); n != 2 {
		t.Errorf("bq1 children = %d, want 2", n)
	}

	bq2 := search.NewBooleanQuery()
	bq2.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq2.Add(search.NewPhraseQueryWithStrings(visitF2, "search", "library"), search.SHOULD)
	if n := scorerChildrenCount(t, searcher, bq2); n != 1 {
		t.Errorf("bq2 children = %d, want 1", n)
	}
}

func TestBooleanQueryVisitSubscorers_MinShouldMatchMatches(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()
	searcher.SetSimilarity(search.NewRawTFSimilarity())

	bq := search.NewBooleanQuery()
	bq.Add(visitTerm(visitF1, "lucene"), search.SHOULD)
	bq.Add(visitTerm(visitF2, "lucene"), search.SHOULD)
	bq.Add(search.NewPhraseQueryWithStrings(visitF2, "search", "library"), search.SHOULD)
	bq.SetMinimumNumberShouldMatch(2)

	if n := scorerChildrenCount(t, searcher, bq); n != 2 {
		t.Errorf("children = %d, want 2", n)
	}
}

// summarizeScorer renders the scorer tree as the indented class-name summary
// the Java ScorerSummarizingCollector produces.
func summarizeScorer(builder *strings.Builder, scorer search.Scorer, indent int) {
	builder.WriteString(scorerSimpleName(scorer))
	provider, ok := scorer.(childrenProvider)
	if !ok {
		return
	}
	children, err := provider.GetChildren()
	if err != nil {
		return
	}
	for _, child := range children {
		indentSummary(builder, indent+1)
		builder.WriteString(child.Relationship)
		builder.WriteString(" ")
		// Children are Scorable, not Scorer, in Gocene; recurse via the
		// Scorable summary which renders its class name and walks deeper.
		summarizeScorable(builder, child.Child, indent+2)
	}
}

// summarizeScorable renders a Scorable subtree as the indented class-name
// summary, mirroring the Scorer side of summarizeScorer.
func summarizeScorable(builder *strings.Builder, scorable search.Scorable, indent int) {
	builder.WriteString(scorableSimpleName(scorable))
	children, err := scorable.GetChildren()
	if err != nil {
		return
	}
	for _, child := range children {
		indentSummary(builder, indent+1)
		builder.WriteString(child.Relationship)
		builder.WriteString(" ")
		summarizeScorable(builder, child.Child, indent+2)
	}
}

func indentSummary(builder *strings.Builder, indent int) {
	if builder.Len() != 0 {
		builder.WriteString("\n")
	}
	for i := 0; i < indent; i++ {
		builder.WriteString("    ")
	}
}

// scorerSimpleName returns the Lucene-style simple class name for a scorer.
func scorerSimpleName(scorer search.Scorer) string {
	name := fmt.Sprintf("%T", scorer)
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimPrefix(name, "*")
}

func scorableSimpleName(scorable search.Scorable) string {
	name := fmt.Sprintf("%T", scorable)
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	return strings.TrimPrefix(name, "*")
}

// scorerSummaries runs query and returns one indented scorer-tree summary per
// matched leaf, plus the number of hits.
func scorerSummaries(t *testing.T, searcher *search.IndexSearcher, query search.Query) ([]string, int) {
	t.Helper()
	collector := &summaryCollector{}
	if err := searcher.SearchWithCollector(query, collector); err != nil {
		t.Fatalf("SearchWithCollector: %v", err)
	}
	sort.Strings(collector.summaries)
	return collector.summaries, collector.numHits
}

type summaryCollector struct {
	summaries []string
	numHits   int
}

func (c *summaryCollector) ScoreMode() search.ScoreMode { return search.COMPLETE }

func (c *summaryCollector) GetLeafCollector(_ *index.LeafReaderContext) (search.LeafCollector, error) {
	return &summaryLeafCollector{parent: c}, nil
}

type summaryLeafCollector struct {
	parent *summaryCollector
}

func (lc *summaryLeafCollector) SetScorer(scorer search.Scorer) error {
	var b strings.Builder
	summarizeScorer(&b, scorer, 0)
	lc.parent.summaries = append(lc.parent.summaries, b.String())
	return nil
}

func (lc *summaryLeafCollector) Collect(_ int) error {
	lc.parent.numHits++
	return nil
}

func TestBooleanQueryVisitSubscorers_GetChildrenMinShouldMatchSumScorer(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()
	searcher.SetSimilarity(search.NewClassicSimilarity())

	query := search.NewBooleanQuery()
	query.Add(visitTerm(visitF2, "nutch"), search.SHOULD)
	query.Add(visitTerm(visitF2, "web"), search.SHOULD)
	query.Add(visitTerm(visitF2, "crawler"), search.SHOULD)
	query.SetMinimumNumberShouldMatch(2)
	query.Add(search.NewMatchAllDocsQuery(), search.MUST)

	summaries, numHits := scorerSummaries(t, searcher, query)
	if numHits != 1 {
		t.Errorf("numHits = %d, want 1", numHits)
	}
	if len(summaries) == 0 {
		t.Errorf("expected at least one scorer summary")
	}
	want := "ConjunctionScorer\n" +
		"    MUST ConstantScoreScorer\n" +
		"    MUST WANDScorer\n" +
		"            SHOULD TermScorer\n" +
		"            SHOULD TermScorer\n" +
		"            SHOULD TermScorer"
	for _, summary := range summaries {
		if summary != want {
			t.Errorf("scorer summary mismatch:\n got:\n%s\nwant:\n%s", summary, want)
		}
	}
}

func TestBooleanQueryVisitSubscorers_GetChildrenBoosterScorer(t *testing.T) {
	searcher, cleanup := visitSubscorersIndex(t)
	defer cleanup()
	searcher.SetSimilarity(search.NewRawTFSimilarity())

	query := search.NewBooleanQuery()
	query.Add(visitTerm(visitF2, "nutch"), search.SHOULD)
	query.Add(visitTerm(visitF2, "miss"), search.SHOULD)

	summaries, numHits := scorerSummaries(t, searcher, query)
	if numHits != 1 {
		t.Errorf("numHits = %d, want 1", numHits)
	}
	if len(summaries) == 0 {
		t.Errorf("expected at least one scorer summary")
	}
	for _, summary := range summaries {
		if summary != "TermScorer" {
			t.Errorf("scorer summary = %q, want %q", summary, "TermScorer")
		}
}