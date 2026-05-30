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
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
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

// TestBlockJoin_SimpleKnn corresponds to TestBlockJoin.testSimpleKnn: build two
// parent blocks each with two child vectors, run a DiversifyingChildrenFloat
// KnnVectorQuery, and assert one best child per parent plus the exact EUCLIDEAN
// score of the top hit.
//
// The DiversifyingChildrenFloatKnnVectorQuery is runnable (rmp #4757) and the
// Lucene99 flat vectors writer/reader supports the sparse (IndexedDISI +
// DirectMonotonic ord->doc) layout (rmp #4755), so the block index — whose
// parents carry no vector, making the field sparse — round-trips end-to-end.
func TestBlockJoin_SimpleKnn(t *testing.T) {
	dir, w := newBlockWriter(t)
	addBlock(t, w,
		makeVector(t, "vector", "parent1", []float32{1, 2, 3}),
		makeVector(t, "vector", "parent1", []float32{3, 3, 3}),
		makeParent(t, "parent1"),
	)
	addBlock(t, w,
		makeVector(t, "vector", "parent2", []float32{0, 0, 1}),
		makeVector(t, "vector", "parent2", []float32{1, 1, 1}),
		makeParent(t, "parent2"),
	)
	r, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("docType", "_parent")
	if err := Check(r, parentsFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	childKnnJoin := NewDiversifyingChildrenFloatKnnVectorQuery(
		"vector", []float32{4, 4, 4}, 3, nil, parentsFilter)

	topDocs, err := s.Search(childKnnJoin, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if topDocs.TotalHits.Value != 2 {
		t.Fatalf("totalHits = %d, want 2", topDocs.TotalHits.Value)
	}
	if len(topDocs.ScoreDocs) != 2 {
		t.Fatalf("scoreDocs = %d, want 2", len(topDocs.ScoreDocs))
	}

	childDoc, err := s.Doc(topDocs.ScoreDocs[0].Doc)
	if err != nil {
		t.Fatalf("Doc[0]: %v", err)
	}
	if got := storedString(childDoc, "my_parent_id"); got != "parent1" {
		t.Errorf("top child my_parent_id = %q, want parent1", got)
	}
	want := index.VectorSimilarityFunctionEuclidean.Compare(
		[]float32{4, 4, 4}, []float32{3, 3, 3})
	if diff := topDocs.ScoreDocs[0].Score - want; diff > 1e-7 || diff < -1e-7 {
		t.Errorf("top score = %v, want %v", topDocs.ScoreDocs[0].Score, want)
	}

	childDoc, err = s.Doc(topDocs.ScoreDocs[1].Doc)
	if err != nil {
		t.Fatalf("Doc[1]: %v", err)
	}
	if got := storedString(childDoc, "my_parent_id"); got != "parent2" {
		t.Errorf("second child my_parent_id = %q, want parent2", got)
	}
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
	t.Fatal("blocked by PostingsEnum.Advance-after-positioning returning NO_MORE_DOCS, which breaks every block-join MUST + filter conjunction here: rmp #4763")
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

// TestBlockJoin_Random is a randomised differential harness matching the intent
// of TestBlockJoin.testRandom (rmp #4781). It builds two parallel indices:
//
//   - a BLOCK-JOIN index with ToParentBlockJoinQuery
//   - a fully DENORMALISED index with the same logical documents flattened
//
// and verifies that the parent docID sets returned by both approaches are
// identical across ~50 random iterations, with optional block-level deletes.
//
// Deviations from the Lucene reference:
//   - Uses string fields ("blockID" as a StringField) instead of IntPoint for
//     the delete-by-query path; IntPoint end-to-end via point-range queries is
//     tested separately (rmp #4769). The delete effect — some parent blocks and
//     their children are absent from both indices — is equivalent.
//   - RandomApproximationQuery wrapping is omitted (Gocene does not expose that
//     test utility).
//   - Merged parent+child sorts are omitted (separately tested by
//     TestBlockJoinSorting_NestedSorting). Result sets are compared by parentID
//     value only.
//   - ~50 iterations (not 200×RANDOM_MULTIPLIER) for test-suite speed.
func TestBlockJoin_Random(t *testing.T) {
	const (
		numParents         = 20
		maxChildrenPerPar  = 5
		iters              = 50
		numChildFieldVals  = 4
		numParentFieldVals = 3
	)

	rng := newRand(t.Name())

	childValues := [numChildFieldVals]string{"alpha", "beta", "gamma", "delta"}
	parentValues := [numParentFieldVals]string{"x", "y", "z"}

	doDeletes := rng.Intn(2) == 1

	// ---- build both indices ----
	joinDir := newFSDir(t)
	plainDir := newFSDir(t)
	joinW := openBlockWriter(t, joinDir)
	plainW := openBlockWriter(t, plainDir)

	// Track which parents are deleted (by parentID) for result-set accounting.
	deletedParents := make(map[string]bool)
	var deleteIDs []string

	for pid := 0; pid < numParents; pid++ {
		parentID := itoa(pid)
		pVal := parentValues[rng.Intn(numParentFieldVals)]

		// Parent doc for the block-join index.
		joinParent := newDoc(t, map[string]string{
			"parentID":  parentID,
			"parentVal": pVal,
			"isParent":  "x",
			"blockID":   parentID,
		})

		// Parent doc for the denormalised index (same fields minus isParent).
		plainParent := newDoc(t, map[string]string{
			"parentID":  parentID,
			"parentVal": pVal,
			"blockID":   parentID,
		})

		numChildren := 1 + rng.Intn(maxChildrenPerPar)
		joinBlock := make([]index.Document, 0, numChildren+1)

		for cid := 0; cid < numChildren; cid++ {
			cVal := childValues[rng.Intn(numChildFieldVals)]

			// Child doc for block-join: no parentID denorm.
			joinChild := newDoc(t, map[string]string{
				"childVal": cVal,
				"blockID":  parentID,
			})
			joinBlock = append(joinBlock, joinChild)

			// Child doc for denormalised: parent + child fields merged.
			plainChild := newDoc(t, map[string]string{
				"parentID":  parentID,
				"parentVal": pVal,
				"childVal":  cVal,
				"blockID":   parentID,
			})
			if err := plainW.AddDocument(plainChild); err != nil {
				t.Fatalf("plain AddDocument: %v", err)
			}
		}

		joinBlock = append(joinBlock, joinParent)
		if err := joinW.AddDocuments(joinBlock); err != nil {
			t.Fatalf("join AddDocuments: %v", err)
		}
		if err := plainW.AddDocument(plainParent); err != nil {
			t.Fatalf("plain AddDocument: %v", err)
		}

		if doDeletes && rng.Intn(5) == 0 {
			deleteIDs = append(deleteIDs, parentID)
			deletedParents[parentID] = true
		}
	}

	// Apply deletes using a TermQuery on blockID.
	if len(deleteIDs) > 0 {
		for _, bid := range deleteIDs {
			delQ := search.NewTermQuery(index.NewTerm("blockID", bid))
			if err := joinW.DeleteDocumentsQuery(delQ); err != nil {
				t.Fatalf("joinW.DeleteDocumentsQuery: %v", err)
			}
			if err := plainW.DeleteDocumentsQuery(delQ); err != nil {
				t.Fatalf("plainW.DeleteDocumentsQuery: %v", err)
			}
		}
	}

	joinR, joinS := commitAndOpen(t, joinDir, joinW)
	plainR, plainS := commitAndOpen(t, plainDir, plainW)
	defer joinR.Close()
	defer plainR.Close()

	parentsFilter := newQueryBitSetParents("isParent", "x")
	if err := Check(joinR, parentsFilter); err != nil {
		t.Fatalf("CheckJoinIndex: %v", err)
	}

	// ---- random query iterations ----
	for iter := 0; iter < iters; iter++ {
		cVal := childValues[rng.Intn(numChildFieldVals)]
		childQ := search.NewTermQuery(index.NewTerm("childVal", cVal))

		// Block-join result: unique parentIDs of matching parents.
		joinQuery := NewToParentBlockJoinQuery(childQ, parentsFilter, Total)
		joinTop, err := joinS.Search(joinQuery, numParents+1)
		if err != nil {
			t.Fatalf("iter %d: joinS.Search: %v", iter, err)
		}
		joinParents := docIDSet(t, joinS, joinTop, "parentID")

		// Denormalised result: unique parentIDs from matching child docs
		// (child docs in the plain index carry the parentID field).
		plainTop, err := plainS.Search(childQ, numParents*maxChildrenPerPar+1)
		if err != nil {
			t.Fatalf("iter %d: plainS.Search: %v", iter, err)
		}
		plainParents := docIDSet(t, plainS, plainTop, "parentID")

		// Remove deleted parents from both sets.
		for pid := range deletedParents {
			delete(joinParents, pid)
			delete(plainParents, pid)
		}

		if len(joinParents) != len(plainParents) {
			t.Errorf("iter %d childVal=%q: join=%d plain=%d parents\n  join=%v\n  plain=%v",
				iter, cVal, len(joinParents), len(plainParents), joinParents, plainParents)
			continue
		}
		for pid := range joinParents {
			if !plainParents[pid] {
				t.Errorf("iter %d childVal=%q: parentID %q in join but not plain", iter, cVal, pid)
			}
		}
	}
}

// ---- helpers for TestBlockJoin_Random ----

func newFSDir(t *testing.T) store.Directory {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	t.Cleanup(func() { _ = dir.Close() })
	return dir
}

func openBlockWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return w
}

func docIDSet(t *testing.T, s *search.IndexSearcher, top *search.TopDocs, field string) map[string]bool {
	t.Helper()
	out := make(map[string]bool)
	for _, sd := range top.ScoreDocs {
		doc := mustDoc(t, s, sd.Doc)
		if v := storedString(doc, field); v != "" {
			out[v] = true
		}
	}
	return out
}

// newRand creates a new random source seeded deterministically from name.
func newRand(name string) *rand.Rand {
	var h uint64
	for _, c := range name {
		h = h*31 + uint64(c)
	}
	return rand.New(rand.NewSource(int64(h)))
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
// parent block, then runs a ToChild join over a parent query that matches both
// the deleted and the live "parent=2" parents and asserts exactly one hit (the
// child of the live parent block).
//
// liveDocs model (rmp #4762): the QueryBitSetProducer's parent bitset must
// include the deleted parent so the child->parent block boundaries stay stable
// (Lucene's QueryBitSetProducer ignores acceptDocs). Deleted docs are excluded
// from the *results* centrally in IndexSearcher.searchLeaf, not at the scorer.
//
// Deviation from Lucene: where the reference uses RandomIndexWriter with a random
// merge policy and w.getReader(), this uses the project's deterministic
// IndexWriter + commit + OpenDirectoryReader (see block_join_test_helpers_test.go).
func TestBlockJoin_AdvanceSingleDeletedParentNoChild(t *testing.T) {
	dir, w := newBlockWriter(t)

	// First block: one child + parent (parent=1, isparent=yes).
	addBlock(t, w,
		newDoc(t, map[string]string{"child": "1"}),
		newDoc(t, map[string]string{"parent": "1", "isparent": "yes"}),
	)
	// Childless parent block (parent=2, isparent=yes) — this one is deleted.
	addBlock(t, w, newDoc(t, map[string]string{"parent": "2", "isparent": "yes"}))
	if err := w.DeleteDocuments(index.NewTerm("parent", "2")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	// Live block re-adding parent=2 with a child (child=2 + parent=2,isparent=yes).
	addBlock(t, w,
		newDoc(t, map[string]string{"child": "2"}),
		newDoc(t, map[string]string{"parent": "2", "isparent": "yes"}),
	)

	r, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("isparent", "yes")
	// CheckJoinIndex must pass: the parent bitset includes the deleted parent so
	// every child maps to a parent and no liveness mismatch is reported.
	if err := Check(r, parentsFilter); err != nil {
		t.Fatalf("CheckJoinIndex: %v", err)
	}

	parentQuery := search.NewTermQuery(index.NewTerm("parent", "2"))
	parentJoinQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter, None)

	topDocs, err := s.Search(parentJoinQuery, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// parent=2 matches both the deleted (childless) parent and the live parent.
	// The deleted parent contributes no child; the live parent contributes its
	// single child. Exactly one hit.
	if topDocs.TotalHits.Value != 1 {
		t.Fatalf("totalHits = %d, want 1", topDocs.TotalHits.Value)
	}
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

// TestBlockJoin_ParentScoringBug corresponds to TestBlockJoin.testParentScoringBug
// (LUCENE-6588). It deletes the first child of every parent (skill=java) and
// asserts that a ToChild join over PrefixQuery(country, "United") returns exactly
// the two surviving children, each with a non-zero score.
//
// liveDocs model (rmp #4762): the deleted java children are excluded from the
// results centrally in IndexSearcher.searchLeaf (the scorer itself iterates all
// docs). The non-zero score comes from the parent PrefixQuery being propagated to
// the child: doScores follows the SEARCH-level needsScores, not the join's
// child-aggregation ScoreMode (which is None here) — the LUCENE-6588 fix.
//
// Deviations from Lucene: deterministic IndexWriter + commit + OpenDirectoryReader
// instead of RandomIndexWriter; and the "year" StringField substitution noted in
// block_join_test_helpers_test.go (irrelevant here — the parent query is on country).
func TestBlockJoin_ParentScoringBug(t *testing.T) {
	dir, w := newBlockWriter(t)
	addBlock(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))
	addBlock(t, w, makeJob(t, "java", 2006), makeJob(t, "ruby", 2005), makeResume(t, "Frank", "United States"))
	// Delete the first child of every parent.
	if err := w.DeleteDocuments(index.NewTerm("skill", "java")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	_, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("docType", "resume")
	parentQuery := search.NewPrefixQuery(index.NewTerm("country", "United"))
	toChildQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter, None)

	hits, err := s.Search(toChildQuery, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits.ScoreDocs) != 2 {
		t.Fatalf("scoreDocs length = %d, want 2", len(hits.ScoreDocs))
	}
	for i, sd := range hits.ScoreDocs {
		if sd.Score == 0.0 {
			t.Errorf("Failed to calculate score for hit #%d (doc %d)", i, sd.Doc)
		}
	}
}

// TestBlockJoin_ToChildBlockJoinQueryExplain corresponds to
// TestBlockJoin.testToChildBlockJoinQueryExplain. Same corpus and delete as
// TestBlockJoin_ParentScoringBug; it asserts that the per-hit score equals the
// value reported by IndexSearcher.Explain for that doc.
func TestBlockJoin_ToChildBlockJoinQueryExplain(t *testing.T) {
	dir, w := newBlockWriter(t)
	addBlock(t, w, makeJob(t, "java", 2007), makeJob(t, "python", 2010), makeResume(t, "Lisa", "United Kingdom"))
	addBlock(t, w, makeJob(t, "java", 2006), makeJob(t, "ruby", 2005), makeResume(t, "Frank", "United States"))
	if err := w.DeleteDocuments(index.NewTerm("skill", "java")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}
	_, s := commitAndOpen(t, dir, w)

	parentsFilter := newQueryBitSetParents("docType", "resume")
	parentQuery := search.NewPrefixQuery(index.NewTerm("country", "United"))
	toChildQuery := NewToChildBlockJoinQuery(parentQuery, parentsFilter, None)

	hits, err := s.Search(toChildQuery, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits.ScoreDocs) != 2 {
		t.Fatalf("scoreDocs length = %d, want 2", len(hits.ScoreDocs))
	}
	for i, sd := range hits.ScoreDocs {
		exp, err := s.Explain(toChildQuery, sd.Doc)
		if err != nil {
			t.Fatalf("Explain(hit #%d, doc %d): %v", i, sd.Doc, err)
		}
		if !exp.IsMatch() {
			t.Errorf("hit #%d (doc %d): explanation reports no match", i, sd.Doc)
		}
		if exp.GetValue() != sd.Score {
			t.Errorf("hit #%d (doc %d): explain value = %v, want score %v",
				i, sd.Doc, exp.GetValue(), sd.Score)
		}
	}
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
	t.Fatal("requires a runnable PrefixQuery for the job-level parents filter (rmp #4760) and the postings Advance fix (rmp #4763)")
}

// freqSimScorer is a SimScorer that scores every document by its raw term
// frequency, ignoring IDF and length normalisation. It is the Gocene analogue
// of the anonymous SimilarityBase in TestBlockJoin.testScoreMode whose
// score(stats, freq, docLen) returns freq.
type freqSimScorer struct{}

// Score returns the term frequency unchanged.
func (freqSimScorer) Score(doc int, freq float32) float32 { return freq }

// freqSimilarity is a Similarity whose scorer returns score=freq. It mirrors
// the test-only SimilarityBase used by TestBlockJoin.testScoreMode (and is
// injected into the IndexSearcher via SetSimilarity).
type freqSimilarity struct {
	*search.BaseSimilarity
}

func newFreqSimilarity() *freqSimilarity {
	return &freqSimilarity{BaseSimilarity: search.NewBaseSimilarity()}
}

// Scorer returns a SimScorer that scores by raw term frequency.
func (s *freqSimilarity) Scorer(collectionStats *search.CollectionStatistics, termStats *search.TermStatistics) search.SimScorer {
	return freqSimScorer{}
}

// String returns the descriptive name embedded in explanations, matching the
// anonymous SimilarityBase's toString() == "TestSim".
func (s *freqSimilarity) String() string { return "TestSim" }

// TestBlockJoin_ScoreMode corresponds to TestBlockJoin.testScoreMode. It builds
// the parent/child block [child("foo"="bar bar"), child("foo"="bar"), empty,
// parent("type"="parent")] and, with a score=freq Similarity injected via
// IndexSearcher.SetSimilarity, asserts the exact per-ScoreMode aggregated parent
// score: Avg=1.5, Max=2, Min=1, Total=3, None=0.
func TestBlockJoin_ScoreMode(t *testing.T) {
	dir, w := newBlockWriter(t)

	child0 := document.NewDocument()
	tf0, err := document.NewTextField("foo", "bar bar", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	child0.Add(tf0)

	child1 := document.NewDocument()
	tf1, err := document.NewTextField("foo", "bar", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	child1.Add(tf1)

	empty := document.NewDocument()

	parent := document.NewDocument()
	parent.Add(mustStringField(t, "type", "parent", false))

	addBlock(t, w, child0, child1, empty, parent)
	reader, searcher := commitAndOpen(t, dir, w)
	_ = reader

	// Inject the score=freq Similarity, exactly as the Lucene test calls
	// searcher.setSimilarity(sim).
	searcher.SetSimilarity(newFreqSimilarity())

	parents := newQueryBitSetParents("type", "parent")

	cases := []struct {
		mode ScoreMode
		want float32
	}{
		{Avg, 1.5},
		{Max, 2},
		{Min, 1},
		{None, 0},
		{Total, 3},
	}
	for _, tc := range cases {
		query := NewToParentBlockJoinQuery(
			search.NewTermQuery(index.NewTerm("foo", "bar")), parents, tc.mode)
		topDocs, err := searcher.Search(query, 10)
		if err != nil {
			t.Fatalf("Search(%v): %v", tc.mode, err)
		}
		if topDocs.TotalHits.Value != 1 {
			t.Fatalf("%v: TotalHits = %d, want 1", tc.mode, topDocs.TotalHits.Value)
		}
		if topDocs.ScoreDocs[0].Doc != 3 {
			t.Errorf("%v: top doc = %d, want 3", tc.mode, topDocs.ScoreDocs[0].Doc)
		}
		if topDocs.ScoreDocs[0].Score != tc.want {
			t.Errorf("%v: score = %v, want %v", tc.mode, topDocs.ScoreDocs[0].Score, tc.want)
		}
	}
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
