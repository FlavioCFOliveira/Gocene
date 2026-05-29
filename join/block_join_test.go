// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.TestBlockJoin.
//
// These exercise the full IndexWriter -> DirectoryReader -> IndexSearcher
// block-join round trip. See block_join_test_helpers_test.go for the document
// builders and the project-wide IntPoint -> StringField "year" substitution.
package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// newDoc builds a document from a field->value map of StringFields (not
// stored). Field iteration order is irrelevant to block joins.
func newDoc(t *testing.T, fields map[string]string) index.Document {
	t.Helper()
	d := document.NewDocument()
	for name, value := range fields {
		d.Add(mustStringField(t, name, value, false))
	}
	return d
}

// skill builds a TermQuery on the skill field, mirroring TestBlockJoin.skill.
func skill(s string) search.Query {
	return search.NewTermQuery(index.NewTerm("skill", s))
}

// yearRange substitutes IntPoint.newRangeQuery("year", lo, hi): it builds a
// SHOULD set of TermQueries over the (zero-padded) year strings in the inclusive
// range [lo, hi]. The substitution is exact for the small block-join corpora
// because their years are integers and the zero-padded encoding sorts and
// compares identically to the integer values. See the deviation note in
// block_join_test_helpers_test.go.
func yearRange(field string, lo, hi int) search.Query {
	bq := search.NewBooleanQuery()
	for y := lo; y <= hi; y++ {
		bq.Add(search.NewTermQuery(index.NewTerm(field, itoa(y))), search.SHOULD)
	}
	return bq
}

// childSkillAndYear builds the recurring child query
// (skill=java MUST) AND (year in [2006,2011] MUST) used by several methods.
func childSkillAndYear(t *testing.T, skillValue string, lo, hi int) search.Query {
	t.Helper()
	bq := search.NewBooleanQuery()
	bq.Add(skill(skillValue), search.MUST)
	bq.Add(yearRange("year", lo, hi), search.MUST)
	return bq
}

// asNameSet returns the set of stored "name" values for the given global docIDs.
func asNameSet(t *testing.T, s *search.IndexSearcher, docIDs ...int) map[string]bool {
	t.Helper()
	out := make(map[string]bool, len(docIDs))
	for _, d := range docIDs {
		doc, err := s.Doc(d)
		if err != nil {
			t.Fatalf("Doc(%d): %v", d, err)
		}
		out[storedString(doc, "name")] = true
	}
	return out
}

// count runs a query and returns its total hit count (Gocene's IndexSearcher
// has no Count method; an unbounded Search yields the same value for these
// small corpora).
func count(t *testing.T, s *search.IndexSearcher, q search.Query) int {
	t.Helper()
	top, err := s.Search(q, 1000)
	if err != nil {
		t.Fatalf("Search(count): %v", err)
	}
	return int(top.TotalHits.Value)
}

// TestBlockJoin_EmptyChildFilter corresponds to TestBlockJoin.testEmptyChildFilter.
func TestBlockJoin_EmptyChildFilter(t *testing.T) {
	dir, w := newBlockWriter(t)
	addBlock(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))
	addBlock(t, w, makeJob(t, "ruby", 2005), makeJob(t, "java", 2006), makeResume(t, "Frank", "United States"))
	r, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("docType", "resume")
	if err := Check(r, parentsFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	childQuery := childSkillAndYear(t, "java", 2006, 2011)
	childJoinQuery := NewToParentBlockJoinQuery(childQuery, parentsFilter, Avg)

	fullQuery := search.NewBooleanQuery()
	fullQuery.Add(childJoinQuery, search.MUST)
	fullQuery.Add(search.NewMatchAllDocsQuery(), search.MUST)

	topDocs, err := s.Search(fullQuery, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 2 {
		t.Fatalf("totalHits = %d, want 2", topDocs.TotalHits.Value)
	}
	got := asNameSet(t, s, topDocs.ScoreDocs[0].Doc, topDocs.ScoreDocs[1].Doc)
	if !got["Lisa"] || !got["Frank"] {
		t.Errorf("names = %v, want {Lisa, Frank}", got)
	}
}

// TestBlockJoin_BQShouldJoinedChild corresponds to TestBlockJoin.testBQShouldJoinedChild.
func TestBlockJoin_BQShouldJoinedChild(t *testing.T) {
	dir, w := newBlockWriter(t)
	addBlock(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))
	addBlock(t, w, makeJob(t, "ruby", 2005), makeJob(t, "java", 2006), makeResume(t, "Frank", "United States"))
	r, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("docType", "resume")
	if err := Check(r, parentsFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	childQuery := childSkillAndYear(t, "java", 2006, 2011)
	parentQuery := search.NewTermQuery(index.NewTerm("country", "United Kingdom"))
	childJoinQuery := NewToParentBlockJoinQuery(childQuery, parentsFilter, Avg)

	fullQuery := search.NewBooleanQuery()
	fullQuery.Add(parentQuery, search.SHOULD)
	fullQuery.Add(childJoinQuery, search.SHOULD)

	topDocs, err := s.Search(fullQuery, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 2 {
		t.Fatalf("totalHits = %d, want 2", topDocs.TotalHits.Value)
	}
	got := asNameSet(t, s, topDocs.ScoreDocs[0].Doc, topDocs.ScoreDocs[1].Doc)
	if !got["Lisa"] || !got["Frank"] {
		t.Errorf("names = %v, want {Lisa, Frank}", got)
	}
}

// TestBlockJoin_SimpleKnn corresponds to TestBlockJoin.testSimpleKnn. It needs a
// runnable DiversifyingChildrenFloatKnnVectorQuery, which is still a descriptor
// stub (does not implement search.Query).
func TestBlockJoin_SimpleKnn(t *testing.T) {
	t.Skip("requires a runnable DiversifyingChildrenFloatKnnVectorQuery (currently a descriptor stub, not a search.Query): rmp #4757")
}

// TestBlockJoin_Simple corresponds to TestBlockJoin.testSimple.
func TestBlockJoin_Simple(t *testing.T) {
	dir, w := newBlockWriter(t)
	addBlock(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))
	addBlock(t, w, makeJob(t, "ruby", 2005), makeJob(t, "java", 2006), makeResume(t, "Frank", "United States"))
	r, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("docType", "resume")
	if err := Check(r, parentsFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	childQuery := childSkillAndYear(t, "java", 2006, 2011)
	parentQuery := search.NewTermQuery(index.NewTerm("country", "United Kingdom"))
	childJoinQuery := NewToParentBlockJoinQuery(childQuery, parentsFilter, Avg)

	fullQuery := search.NewBooleanQuery()
	fullQuery.Add(parentQuery, search.MUST)
	fullQuery.Add(childJoinQuery, search.MUST)

	topDocs, err := s.Search(fullQuery, 1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Fatalf("totalHits = %d, want 1", topDocs.TotalHits.Value)
	}
	parentDoc, err := s.Doc(topDocs.ScoreDocs[0].Doc)
	if err != nil {
		t.Fatalf("Doc: %v", err)
	}
	if got := storedString(parentDoc, "name"); got != "Lisa" {
		t.Errorf("parent name = %q, want Lisa", got)
	}

	// Now join "up" (map parent hits to child docs).
	parentJoinQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter, None)
	fullChildQuery := search.NewBooleanQuery()
	fullChildQuery.Add(parentJoinQuery, search.MUST)
	fullChildQuery.Add(childQuery, search.MUST)

	hits, err := s.Search(fullChildQuery, 10)
	if err != nil {
		t.Fatalf("Search up: %v", err)
	}
	if hits.TotalHits.Value != 1 {
		t.Fatalf("up totalHits = %d, want 1", hits.TotalHits.Value)
	}
	childDoc, err := s.Doc(hits.ScoreDocs[0].Doc)
	if err != nil {
		t.Fatalf("Doc child: %v", err)
	}
	if got := storedString(childDoc, "skill"); got != "java" {
		t.Errorf("child skill = %q, want java", got)
	}
	if got := storedString(childDoc, "year"); got != itoa(2007) {
		t.Errorf("child year = %q, want %q", got, itoa(2007))
	}
	if got := storedString(getParentDoc(t, s, r, parentsFilter, hits.ScoreDocs[0].Doc), "name"); got != "Lisa" {
		t.Errorf("getParentDoc name = %q, want Lisa", got)
	}

	// Filter on child docs: skill=foosball matches nothing.
	fullChildQuery.Add(skill("foosball"), search.FILTER)
	if c := count(t, s, fullChildQuery); c != 0 {
		t.Errorf("count with foosball filter = %d, want 0", c)
	}
}

// TestBlockJoin_SimpleFilter corresponds to TestBlockJoin.testSimpleFilter.
// Every assertion composes a block-join query as a MUST clause alongside a
// TermQuery FILTER/MUST clause, so the BooleanQuery conjunction drives the
// block-join scorer with Advance after the lead clause's NextDoc. That hits the
// postings-reader Advance-after-positioning bug (a second Advance on an
// already-positioned PostingsEnum returns NO_MORE_DOCS), so the conjunctions
// drop valid parents. This is a codec/search-core defect, not a block-join one.
func TestBlockJoin_SimpleFilter(t *testing.T) {
	t.Skip("blocked by PostingsEnum.Advance-after-positioning returning NO_MORE_DOCS, which breaks every block-join MUST + filter conjunction here: rmp #4763")
}

// mustDoc fetches a stored document, failing the test on error.
func mustDoc(t *testing.T, s *search.IndexSearcher, docID int) *document.Document {
	t.Helper()
	d, err := s.Doc(docID)
	if err != nil {
		t.Fatalf("Doc(%d): %v", docID, err)
	}
	return d
}

// TestBlockJoin_BoostBug corresponds to TestBlockJoin.testBoostBug. It is a
// regression test that block-join scoring does not blow up over an empty index
// with MatchNoDocs children and a boost wrapper.
func TestBlockJoin_BoostBug(t *testing.T) {
	dir, w := newBlockWriter(t)
	r, s := commitAndOpen(t, dir, w)
	_ = r

	q := NewToParentBlockJoinQuery(
		search.NewMatchNoDocsQuery(),
		NewQueryBitSetProducer(search.NewMatchAllDocsQuery()),
		Avg)
	if _, err := s.Search(q, 10); err != nil {
		t.Fatalf("Search join: %v", err)
	}
	bq := search.NewBooleanQuery()
	bq.Add(q, search.MUST)
	if _, err := s.Search(search.NewBoostQuery(bq, 2), 10); err != nil {
		t.Fatalf("Search boosted: %v", err)
	}
}

// TestBlockJoin_Random corresponds to TestBlockJoin.testRandom.
func TestBlockJoin_Random(t *testing.T) {
	t.Skip("requires block-join sort (ToParentBlockJoinSortField) + NumericDocValues/SortedDocValues sorting end-to-end: rmp #4758")
}

// TestBlockJoin_MultiChildTypes corresponds to TestBlockJoin.testMultiChildTypes.
func TestBlockJoin_MultiChildTypes(t *testing.T) {
	dir, w := newBlockWriter(t)
	addBlock(t, w,
		makeJob(t, "java", 2007),
		makeJob(t, "python", 2010),
		makeQualification(t, "maths", 1999),
		makeResume(t, "Lisa", "United Kingdom"))
	r, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("docType", "resume")
	if err := Check(r, parentsFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	childJobQuery := childSkillAndYear(t, "java", 2006, 2011)

	childQualificationQuery := search.NewBooleanQuery()
	childQualificationQuery.Add(search.NewTermQuery(index.NewTerm("qualification", "maths")), search.MUST)
	childQualificationQuery.Add(yearRange("year", 1980, 2000), search.MUST)

	parentQuery := search.NewTermQuery(index.NewTerm("country", "United Kingdom"))
	childJobJoinQuery := NewToParentBlockJoinQuery(childJobQuery, parentsFilter, Avg)
	childQualificationJoinQuery := NewToParentBlockJoinQuery(childQualificationQuery, parentsFilter, Avg)

	fullQuery := search.NewBooleanQuery()
	fullQuery.Add(parentQuery, search.MUST)
	fullQuery.Add(childJobJoinQuery, search.MUST)
	fullQuery.Add(childQualificationJoinQuery, search.MUST)

	topDocs, err := s.Search(fullQuery, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 1 {
		t.Fatalf("totalHits = %d, want 1", topDocs.TotalHits.Value)
	}
	if got := storedString(mustDoc(t, s, topDocs.ScoreDocs[0].Doc), "name"); got != "Lisa" {
		t.Errorf("parent name = %q, want Lisa", got)
	}
}

// TestBlockJoin_AdvanceSingleParentSingleChild corresponds to
// TestBlockJoin.testAdvanceSingleParentSingleChild.
func TestBlockJoin_AdvanceSingleParentSingleChild(t *testing.T) {
	dir, w := newBlockWriter(t)
	childDoc := newDoc(t, map[string]string{"child": "1"})
	parentDoc := newDoc(t, map[string]string{"parent": "1"})
	addBlock(t, w, childDoc, parentDoc)
	r, s := commitAndOpen(t, dir, w)

	tq := search.NewTermQuery(index.NewTerm("child", "1"))
	parentFilter := newQueryBitSetParents("parent", "1")
	if err := Check(r, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	q := NewToParentBlockJoinQuery(tq, parentFilter, Avg)
	sc := firstLeafScorer(t, s, r, q)
	if sc == nil {
		t.Fatal("expected non-nil scorer")
	}
	doc, err := sc.Advance(1)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc != 1 {
		t.Errorf("Advance(1) = %d, want 1", doc)
	}
}

// TestBlockJoin_AdvanceSingleParentNoChild corresponds to
// TestBlockJoin.testAdvanceSingleParentNoChild.
func TestBlockJoin_AdvanceSingleParentNoChild(t *testing.T) {
	dir, w := newBlockWriter(t)
	// First block: a single childless parent (parent=1).
	addBlock(t, w, newDoc(t, map[string]string{"parent": "1", "isparent": "yes"}))
	// Second block: child=2 then parent=2.
	addBlock(t, w,
		newDoc(t, map[string]string{"child": "2"}),
		newDoc(t, map[string]string{"parent": "2", "isparent": "yes"}))
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	r, s := commitAndOpen(t, dir, w)

	tq := search.NewTermQuery(index.NewTerm("child", "2"))
	parentFilter := newQueryBitSetParents("isparent", "yes")
	if err := Check(r, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	q := NewToParentBlockJoinQuery(tq, parentFilter, Avg)
	sc := firstLeafScorer(t, s, r, q)
	if sc == nil {
		t.Fatal("expected non-nil scorer")
	}
	// The only matching child is doc 2 (child=2), whose parent is doc 2.
	doc, err := sc.Advance(0)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc != 2 {
		t.Errorf("Advance(0) = %d, want 2", doc)
	}
}

// TestBlockJoin_ChildQueryNeverMatches corresponds to
// TestBlockJoin.testChildQueryNeverMatches.
func TestBlockJoin_ChildQueryNeverMatches(t *testing.T) {
	dir, w := newBlockWriter(t)
	addBlock(t, w,
		newDoc(t, map[string]string{"childText": "text"}),
		newDoc(t, map[string]string{"parentText": "text", "isParent": "yes"}))
	addBlock(t, w, newDoc(t, map[string]string{"parentText": "text", "isParent": "yes"}))
	r, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("isParent", "yes")
	if err := Check(r, parentsFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	// childBogusField never matches -> the join produces a nil scorer.
	childQuery := search.NewTermQuery(index.NewTerm("childBogusField", "bogus"))
	childJoinQuery := NewToParentBlockJoinQuery(childQuery, parentsFilter, Avg)
	if sc := firstLeafScorer(t, s, r, childJoinQuery); sc != nil {
		t.Errorf("expected nil scorer for never-matching child query, got %T", sc)
	}

	// A different never-matching field likewise yields a nil scorer.
	childQuery2 := search.NewTermQuery(index.NewTerm("bogus", "bogus"))
	childJoinQuery2 := NewToParentBlockJoinQuery(childQuery2, parentsFilter, Avg)
	if sc := firstLeafScorer(t, s, r, childJoinQuery2); sc != nil {
		t.Errorf("expected nil scorer for second never-matching child query, got %T", sc)
	}
}

// TestBlockJoin_AdvanceSingleDeletedParentNoChild corresponds to
// TestBlockJoin.testAdvanceSingleDeletedParentNoChild. It deletes a childless
// parent block; under Lucene the parent bitset still includes deleted parents
// (QueryBitSetProducer ignores acceptDocs), but Gocene's QueryBitSetProducer
// builds the bitset from a TermScorer that already filters deleted docs, so the
// deleted parent drops out and the block boundary is mis-attributed.
func TestBlockJoin_AdvanceSingleDeletedParentNoChild(t *testing.T) {
	t.Skip("requires QueryBitSetProducer to include deleted parents (Lucene ignores acceptDocs in the parent bitset); Gocene filters liveDocs at the scorer level: rmp #4762")
}

// TestBlockJoin_IntersectionWithRandomApproximation corresponds to
// TestBlockJoin.testIntersectionWithRandomApproximation.
func TestBlockJoin_IntersectionWithRandomApproximation(t *testing.T) {
	dir, w := newBlockWriter(t)
	// Deterministic stand-in for the random corpus: a spread of blocks with 0-2
	// children, varied foo_child / foo_parent values in {bar, baz}.
	type blockSpec struct {
		children  []string // foo_child values
		fooParent string
	}
	blocks := []blockSpec{
		{children: nil, fooParent: "bar"},
		{children: []string{"bar"}, fooParent: "bar"},
		{children: []string{"baz"}, fooParent: "bar"},
		{children: []string{"bar", "baz"}, fooParent: "baz"},
		{children: []string{"baz", "baz"}, fooParent: "bar"},
		{children: []string{"bar"}, fooParent: "baz"},
		{children: []string{"baz"}, fooParent: "baz"},
		{children: []string{"bar", "bar"}, fooParent: "bar"},
	}
	for _, b := range blocks {
		docs := make([]index.Document, 0, len(b.children)+1)
		for _, fc := range b.children {
			docs = append(docs, newDoc(t, map[string]string{"foo_child": fc}))
		}
		docs = append(docs, newDoc(t, map[string]string{"parent": "true", "foo_parent": b.fooParent}))
		addBlock(t, w, docs...)
	}
	r, s := commitAndOpen(t, dir, w)
	_ = r

	parentsFilter := newQueryBitSetParents("parent", "true")
	toChild := NewToChildBlockJoinQuery(search.NewTermQuery(index.NewTerm("foo_parent", "bar")), parentsFilter, None)
	childQuery := search.NewTermQuery(index.NewTerm("foo_child", "baz"))

	bq1 := search.NewBooleanQuery()
	bq1.Add(toChild, search.MUST)
	bq1.Add(childQuery, search.MUST)

	// The Lucene test wraps childQuery in a RandomApproximationQuery to force
	// real advance() calls through a two-phase approximation. Gocene's Scorer
	// interface exposes two-phase only via the optional HasTwoPhaseIterator
	// helper, and the BooleanQuery conjunction consumes it; randomApproxQuery
	// wraps the child with such a two-phase view so this exercises the same
	// approximation/advance path.
	bq2 := search.NewBooleanQuery()
	bq2.Add(toChild, search.MUST)
	bq2.Add(newRandomApproximationQuery(childQuery, 12345), search.MUST)

	if count(t, s, bq1) != count(t, s, bq2) {
		t.Errorf("count(bq1)=%d != count(bq2)=%d", count(t, s, bq1), count(t, s, bq2))
	}
}

// TestBlockJoin_ParentScoringBug corresponds to TestBlockJoin.testParentScoringBug.
// It deletes the first child of every parent and asserts that a ToChild join
// returns only the surviving children with non-zero scores; this depends on
// deleted docs being excluded from composite-scorer results.
func TestBlockJoin_ParentScoringBug(t *testing.T) {
	t.Skip("requires deleted docs to be excluded from block-join (ToChild) results; Gocene does not apply liveDocs to composite scorers: rmp #4762")
}

// TestBlockJoin_ToChildBlockJoinQueryExplain corresponds to
// TestBlockJoin.testToChildBlockJoinQueryExplain. Same delete dependency as
// TestBlockJoin_ParentScoringBug.
func TestBlockJoin_ToChildBlockJoinQueryExplain(t *testing.T) {
	t.Skip("requires deleted docs to be excluded from block-join (ToChild) results; Gocene does not apply liveDocs to composite scorers: rmp #4762")
}

// TestBlockJoin_ToChildInitialAdvanceParentButNoKids corresponds to
// TestBlockJoin.testToChildInitialAdvanceParentButNoKids.
func TestBlockJoin_ToChildInitialAdvanceParentButNoKids(t *testing.T) {
	dir, w := newBlockWriter(t)
	// Degenerate case: first doc is a childless parent.
	addBlock(t, w, makeResume(t, "first", "nokids"))
	addBlock(t, w, makeJob(t, "job", 42), makeResume(t, "second", "haskid"))
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	r, s := commitAndOpen(t, dir, w)

	parentFilter := newQueryBitSetParents("docType", "resume")
	parentQuery := search.NewTermQuery(index.NewTerm("docType", "resume"))
	parentJoinQuery := NewToChildBlockJoinQuery(parentQuery, parentFilter, None)

	sc := firstLeafScorer(t, s, r, parentJoinQuery)
	if sc == nil {
		t.Fatal("expected non-nil scorer")
	}
	// The first parent (doc 0) has no children, so the first child returned must
	// be doc 1 (the only child, belonging to the second parent at doc 2).
	doc, err := sc.Advance(0)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if doc != 1 {
		t.Errorf("Advance(0) = %d, want 1", doc)
	}
}

// TestBlockJoin_MultiChildQueriesOfDiffParentLevels corresponds to
// TestBlockJoin.testMultiChildQueriesOfDiffParentLevels.
func TestBlockJoin_MultiChildQueriesOfDiffParentLevels(t *testing.T) {
	// The job-level parents filter is "anything with a skill", expressed in
	// Lucene as PrefixQuery(skill, "") wrapped in a QueryBitSetProducer. Gocene's
	// PrefixQuery yields a nil weight (its ConstantScoreQuery weight is a stub),
	// so QueryBitSetProducer cannot build the job parent bitset; and the two
	// stacked ToChild joins are composed in a conjunction that also hits the
	// postings Advance-after-positioning bug. Both are out-of-scope core gaps.
	t.Skip("requires a runnable PrefixQuery for the job-level parents filter (rmp #4760) and the postings Advance fix (rmp #4763)")
}

// TestBlockJoin_ScoreMode corresponds to TestBlockJoin.testScoreMode.
func TestBlockJoin_ScoreMode(t *testing.T) {
	t.Skip("requires a custom Similarity (score=freq) injected via IndexSearcher.setSimilarity to assert exact aggregated scores 1.5/2/1/3; Gocene IndexSearcher has no similarity hook: rmp #4759")
}

// TestBlockJoin_ToParentQueryConstruction verifies that
// ToParentBlockJoinQuery and ToChildBlockJoinQuery can be composed,
// mirroring the query-construction pattern shared by all TestBlockJoin
// test methods.
func TestBlockJoin_ToParentQueryConstruction(t *testing.T) {
	tpq := NewToParentBlockJoinQuery(nil, nil, Max)
	if tpq == nil {
		t.Fatal("expected non-nil ToParentBlockJoinQuery")
	}
	if tpq.GetScoreMode() != Max {
		t.Errorf("GetScoreMode() = %v, want Max", tpq.GetScoreMode())
	}

	tcq := NewToChildBlockJoinQuery(nil, nil, None)
	if tcq == nil {
		t.Fatal("expected non-nil ToChildBlockJoinQuery")
	}
	if tcq.GetScoreMode() != None {
		t.Errorf("GetScoreMode() = %v, want None", tcq.GetScoreMode())
	}
}
