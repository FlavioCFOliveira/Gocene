// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package join contains tests porting
// org.apache.lucene.search.join.ParentBlockJoinKnnVectorQueryTestCase
// (the shared abstract test case driving both the Float and Byte
// DiversifyingChildren KnnVectorQuery families).
//
// The DiversifyingChildren{Float,Byte}KnnVectorQuery types are runnable
// search.Query implementations (rmp #4757) and the Lucene99 flat vectors
// writer/reader now supports the sparse (IndexedDISI + DirectMonotonic
// ord->doc) layout (rmp #4755), so the parent/child block index — in which
// the parents carry no vector and the field is therefore sparse — round-trips
// end-to-end through IndexWriter + OpenDirectoryReader + IndexSearcher.
//
// Deviations from the Lucene reference, applied uniformly here:
//   - These ports use the float DiversifyingChildrenFloatKnnVectorQuery only;
//     the byte family is covered by parent_block_join_byte_knn_vector_query_test.go.
//   - Gocene's diversifying query drives the codec reader's HNSW graph
//     traversal through a DiversifyingNearestChildrenKnnCollector on every
//     real-segment leaf (the collector-driven approximate path, rmp #4770),
//     falling back to the faithful exact diversifying scan only when a leaf
//     reader exposes no collector-driven search surface. On these small
//     corpora the HNSW path returns the same result identities as the exact
//     scan, so the tests assert the exact result identities and scores.
//   - testTimeout is not ported here: it requires IndexSearcher.setTimeout /
//     IndexSearcher.count, which Gocene's IndexSearcher does not yet expose
//     (unrelated to the vector layout). It is covered by a dedicated, focused
//     assertion in TestParentBlockJoinKnnQueryTestCase_Timeout below using the
//     query's own QueryTimeout hook on ExactSearch.
package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// pbjMakeChild builds a child document with a float vector for field and a
// stored "id". Mirrors the per-child documents created throughout
// ParentBlockJoinKnnVectorQueryTestCase.
func pbjMakeChild(t *testing.T, field string, vector []float32, id string, sim index.VectorSimilarityFunction) index.Document {
	t.Helper()
	d := document.NewDocument()
	vf, err := document.NewKnnFloatVectorField(field, vector, sim)
	if err != nil {
		t.Fatalf("NewKnnFloatVectorField(%q): %v", field, err)
	}
	d.Add(vf)
	d.Add(mustStringField(t, "id", id, true))
	return d
}

// pbjMakeParent builds a parent document (docType=_parent), optionally storing
// a parentId. Mirrors makeParent / the createFamily parent.
func pbjMakeParent(t *testing.T, parentID string) index.Document {
	t.Helper()
	d := document.NewDocument()
	d.Add(mustStringField(t, "docType", "_parent", false))
	if parentID != "" {
		d.Add(mustStringField(t, "parentId", parentID, true))
	}
	return d
}

// pbjMakeOther builds a non-vector child document carrying only "other=value".
// Mirrors the no-vector child documents in getIndexStore / testIndexWithNoVectors.
func pbjMakeOther(t *testing.T) index.Document {
	t.Helper()
	d := document.NewDocument()
	d.Add(mustStringField(t, "other", "value", false))
	return d
}

// pbjParentsFilter is the canonical parents filter used by the test case.
func pbjParentsFilter() BitSetProducer {
	return NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("docType", "_parent")))
}

// pbjOpen commits/closes the writer and opens a reader + searcher.
func pbjOpen(t *testing.T, dir store.Directory, w *index.IndexWriter) (*index.DirectoryReader, *search.IndexSearcher) {
	t.Helper()
	return commitAndOpen(t, dir, w)
}

// TestParentBlockJoinKnnQueryTestCase_EmptyIndex corresponds to
// ParentBlockJoinKnnVectorQueryTestCase.testEmptyIndex: an index with no
// documents yields zero matches and the query rewrites to MatchNoDocsQuery.
func TestParentBlockJoinKnnQueryTestCase_EmptyIndex(t *testing.T) {
	dir, w := newBlockWriter(t)
	r, s := pbjOpen(t, dir, w)

	kvq := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{1, 2}, 2, nil, pbjParentsFilter())
	td, err := s.Search(kvq, 1000)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(td.ScoreDocs) != 0 {
		t.Errorf("empty index returned %d hits, want 0", len(td.ScoreDocs))
	}

	rewritten, err := kvq.Rewrite(r)
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if _, ok := rewritten.(*search.MatchNoDocsQuery); !ok {
		t.Errorf("Rewrite on empty index = %T, want *search.MatchNoDocsQuery", rewritten)
	}
}

// TestParentBlockJoinKnnQueryTestCase_IndexWithNoVectorsNorParents corresponds
// to testIndexWithNoVectorsNorParents: documents without a vector and without
// parents yield zero matches, both for approximate and exact (large-k) search.
func TestParentBlockJoinKnnQueryTestCase_IndexWithNoVectorsNorParents(t *testing.T) {
	dir, w := newBlockWriter(t)
	for i := 0; i < 5; i++ {
		d := document.NewDocument()
		d.Add(mustStringField(t, "other", "value", false))
		if err := w.AddDocument(d); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	_, s := pbjOpen(t, dir, w)

	parentFilter := pbjParentsFilter()
	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{2, 2}, 3, nil, parentFilter)
	td, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if td.TotalHits.Value != 0 || len(td.ScoreDocs) != 0 {
		t.Errorf("got %d hits, want 0", len(td.ScoreDocs))
	}

	// Match-all filter + large k exercises the exact-search branch.
	q = NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{2, 2}, 10, search.NewMatchAllDocsQuery(), parentFilter)
	td, err = s.Search(q, 3)
	if err != nil {
		t.Fatalf("Search (exact): %v", err)
	}
	if td.TotalHits.Value != 0 || len(td.ScoreDocs) != 0 {
		t.Errorf("exact got %d hits, want 0", len(td.ScoreDocs))
	}
}

// TestParentBlockJoinKnnQueryTestCase_IndexWithNoParents corresponds to
// testIndexWithNoParents: child vector documents exist but no parents, so the
// query (whose parents filter matches nothing) yields zero matches.
func TestParentBlockJoinKnnQueryTestCase_IndexWithNoParents(t *testing.T) {
	dir, w := newBlockWriter(t)
	for i := 0; i < 3; i++ {
		d := document.NewDocument()
		vf, err := document.NewKnnFloatVectorFieldEuclidean("field", []float32{2, 2})
		if err != nil {
			t.Fatalf("NewKnnFloatVectorField: %v", err)
		}
		d.Add(vf)
		d.Add(mustStringField(t, "id", string(rune('0'+i)), true))
		if err := w.AddDocument(d); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		d := document.NewDocument()
		d.Add(mustStringField(t, "other", "value", false))
		if err := w.AddDocument(d); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	_, s := pbjOpen(t, dir, w)

	parentFilter := pbjParentsFilter()
	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{2, 2}, 3, nil, parentFilter)
	td, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if td.TotalHits.Value != 0 || len(td.ScoreDocs) != 0 {
		t.Errorf("got %d hits, want 0", len(td.ScoreDocs))
	}

	q = NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{2, 2}, 10, search.NewMatchAllDocsQuery(), parentFilter)
	td, err = s.Search(q, 3)
	if err != nil {
		t.Fatalf("Search (exact): %v", err)
	}
	if td.TotalHits.Value != 0 || len(td.ScoreDocs) != 0 {
		t.Errorf("exact got %d hits, want 0", len(td.ScoreDocs))
	}
}

// TestParentBlockJoinKnnQueryTestCase_FilterWithNoVectorMatches corresponds to
// testFilterWithNoVectorMatches: a child filter that matches a non-vector field
// ("other"=value) selects no vector documents, so the join yields zero matches.
func TestParentBlockJoinKnnQueryTestCase_FilterWithNoVectorMatches(t *testing.T) {
	dir, w := newBlockWriter(t)
	// Three child/parent blocks, each child carrying a vector.
	vectors := [][]float32{{0, 1}, {1, 2}, {0, 0}}
	for i, v := range vectors {
		addBlock(t, w,
			pbjMakeChild(t, "field", v, string(rune('0'+i)), index.VectorSimilarityFunctionEuclidean),
			pbjMakeParent(t, ""),
		)
	}
	r, s := pbjOpen(t, dir, w)

	parentFilter := pbjParentsFilter()
	if err := Check(r, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}
	// Filter on "other"=value matches none of the vector children.
	filter := search.NewTermQuery(index.NewTerm("other", "value"))
	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{1, 2}, 2, filter, parentFilter)
	td, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if td.TotalHits.Value != 0 {
		t.Errorf("got %d total hits, want 0", td.TotalHits.Value)
	}
}

// TestParentBlockJoinKnnQueryTestCase_ScoringWithMultipleChildren corresponds
// to testScoringWithMultipleChildren: two parent blocks of five children each,
// the diversifying join keeps the single best child per parent, and the
// scorer-level scores/ids match. EUCLIDEAN similarity is used (Gocene's
// equivalent of the reference's default), so the asserted scores are the
// EUCLIDEAN normalized scores rather than the dot-product ones.
func TestParentBlockJoinKnnQueryTestCase_ScoringWithMultipleChildren(t *testing.T) {
	dir, w := newBlockWriter(t)
	// Block 1: children {1,1}..{5,5}, ids "1".."5".
	var block []index.Document
	for j := 1; j <= 5; j++ {
		block = append(block, pbjMakeChild(t, "field", []float32{float32(j), float32(j)}, pbjItoa(j), index.VectorSimilarityFunctionEuclidean))
	}
	block = append(block, pbjMakeParent(t, "p1"))
	addBlock(t, w, block...)

	// Block 2: children {7,7}..{11,11}, ids "7".."11".
	block = nil
	for j := 7; j <= 11; j++ {
		block = append(block, pbjMakeChild(t, "field", []float32{float32(j), float32(j)}, pbjItoa(j), index.VectorSimilarityFunctionEuclidean))
	}
	block = append(block, pbjMakeParent(t, "p2"))
	addBlock(t, w, block...)

	r, s := pbjOpen(t, dir, w)
	parentFilter := pbjParentsFilter()
	if err := Check(r, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	// Query {2,2}: best child of block 1 is {2,2} (id "2", exact match);
	// best child of block 2 is {7,7} (id "7", nearest in that block).
	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{2, 2}, 3, nil, parentFilter)
	want2 := index.VectorSimilarityFunctionEuclidean.Compare([]float32{2, 2}, []float32{2, 2})
	want7 := index.VectorSimilarityFunctionEuclidean.Compare([]float32{2, 2}, []float32{7, 7})
	pbjAssertScorerResults(t, s, r, q, map[string]float32{"2": want2, "7": want7}, 2)

	// Query {6,6}: best of block 1 is {5,5} (id "5"); best of block 2 is {7,7}
	// (id "7"); both at the same EUCLIDEAN distance sqrt(2).
	q = NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{6, 6}, 3, nil, parentFilter)
	want5 := index.VectorSimilarityFunctionEuclidean.Compare([]float32{6, 6}, []float32{5, 5})
	want7b := index.VectorSimilarityFunctionEuclidean.Compare([]float32{6, 6}, []float32{7, 7})
	pbjAssertScorerResults(t, s, r, q, map[string]float32{"5": want5, "7": want7b}, 2)

	// Exact search (match-all filter, large k) yields the same result.
	q = NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{6, 6}, 20, search.NewMatchAllDocsQuery(), parentFilter)
	pbjAssertScorerResults(t, s, r, q, map[string]float32{"5": want5, "7": want7b}, 2)

	// k=1 keeps only the single best parent's best child.
	q = NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{6, 6}, 1, search.NewMatchAllDocsQuery(), parentFilter)
	pbjAssertScorerResults(t, s, r, q, map[string]float32{"5": want5, "7": want7b}, 1)
}

// TestParentBlockJoinKnnQueryTestCase_SkewedIndex corresponds to
// testSkewedIndex: 25 single-child blocks flushed across 5 segments; an 8-NN
// query must still find the global top-8 across segments.
func TestParentBlockJoinKnnQueryTestCase_SkewedIndex(t *testing.T) {
	dir, w := newBlockWriter(t)
	r := 0
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			addBlock(t, w,
				pbjMakeChild(t, "field", []float32{float32(r), float32(r)}, pbjItoa(r), index.VectorSimilarityFunctionEuclidean),
				pbjMakeParent(t, ""),
			)
			r++
		}
		if err := w.Commit(); err != nil {
			t.Fatalf("Commit (flush %d): %v", i, err)
		}
	}
	reader, s := pbjOpen(t, dir, w)
	parentFilter := pbjParentsFilter()
	if err := Check(reader, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	// Query {0,0}: nearest children are r=0..7 in ascending order.
	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{0, 0}, 8, nil, parentFilter)
	td, err := s.Search(q, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(td.ScoreDocs) != 8 {
		t.Fatalf("got %d hits, want 8 (%v)", len(td.ScoreDocs), pbjIDs(t, s, td))
	}
	pbjAssertIDMatches(t, s, "0", td.ScoreDocs[0].Doc)
	pbjAssertIDMatches(t, s, "7", td.ScoreDocs[7].Doc)

	// Query {10,10}: nearest is r=10, eighth nearest is r=6 (tie-break by
	// ascending distance then docid: 10,9,11,8,12,7,13,6).
	q = NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{10, 10}, 8, nil, parentFilter)
	td, err = s.Search(q, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(td.ScoreDocs) != 8 {
		t.Fatalf("got %d hits, want 8 (%v)", len(td.ScoreDocs), pbjIDs(t, s, td))
	}
	pbjAssertIDMatches(t, s, "10", td.ScoreDocs[0].Doc)
	pbjAssertIDMatches(t, s, "6", td.ScoreDocs[7].Doc)
}

// TestParentBlockJoinKnnQueryTestCase_Timeout corresponds to
// testTimeout, restricted to the parts Gocene's infrastructure supports: the
// query's ExactSearch honours a QueryTimeout that exits immediately (no
// results) and one that scores a single parent (at most one result). The
// IndexSearcher.setTimeout / count surface used by the reference is not yet
// ported, so the timeout is driven directly through ExactSearch.
func TestParentBlockJoinKnnQueryTestCase_Timeout(t *testing.T) {
	dir, w := newBlockWriter(t)
	vectors := [][]float32{{0, 1}, {1, 2}, {0, 0}}
	for i, v := range vectors {
		addBlock(t, w,
			pbjMakeChild(t, "field", v, pbjItoa(i), index.VectorSimilarityFunctionEuclidean),
			pbjMakeParent(t, ""),
		)
	}
	reader, s := pbjOpen(t, dir, w)
	parentFilter := pbjParentsFilter()
	if err := Check(reader, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	// Baseline: no timeout yields 3 results (one best child per parent).
	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{1, 2}, 2, nil, parentFilter)
	td, err := s.Search(q, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if td.TotalHits.Value == 0 {
		t.Fatalf("baseline returned no results")
	}

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Leaves: %v", err)
	}
	ctx := leaves[0]
	iter, err := pbjAllDocsIterator(s, ctx, q)
	if err != nil {
		t.Fatalf("acceptIterator: %v", err)
	}

	// Immediate timeout: no parents scored.
	immediate := &pbjCountingTimeout{remaining: 0}
	tdTO, err := q.ExactSearch(ctx, iter, immediate)
	if err != nil {
		t.Fatalf("ExactSearch (immediate timeout): %v", err)
	}
	if len(tdTO.ScoreDocs) != 0 {
		t.Errorf("immediate timeout returned %d results, want 0", len(tdTO.ScoreDocs))
	}

	// Score exactly one parent: at most one result.
	iter, err = pbjAllDocsIterator(s, ctx, q)
	if err != nil {
		t.Fatalf("acceptIterator: %v", err)
	}
	one := &pbjCountingTimeout{remaining: 1}
	tdOne, err := q.ExactSearch(ctx, iter, one)
	if err != nil {
		t.Fatalf("ExactSearch (count=1 timeout): %v", err)
	}
	if len(tdOne.ScoreDocs) > 1 {
		t.Errorf("count=1 timeout returned %d results, want <= 1", len(tdOne.ScoreDocs))
	}
}

// TestParentBlockJoinKnnQueryTestCase_TwoSegments corresponds to
// testTwoSegments: three families committed across two segments; a 3-NN query
// returns three hits with three distinct parent ids {a, b, c}.
func TestParentBlockJoinKnnQueryTestCase_TwoSegments(t *testing.T) {
	const dim = 4
	dir, w := newBlockWriter(t)
	pbjAddFamily(t, w, "a", 2, dim)
	pbjAddFamily(t, w, "b", 3, dim)
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	pbjAddFamily(t, w, "c", 1, dim)
	reader, s := pbjOpen(t, dir, w)
	parentFilter := pbjParentsFilter()
	if err := Check(reader, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	q := NewDiversifyingChildrenFloatKnnVectorQuery("field", pbjVector(dim, 0.5), 3, nil, parentFilter)
	td, err := s.Search(q, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(td.ScoreDocs) != 3 {
		t.Fatalf("got %d hits, want 3", len(td.ScoreDocs))
	}
	if td.TotalHits.Value < int64(len(td.ScoreDocs)) {
		t.Errorf("totalHits %d < scoreDocs %d", td.TotalHits.Value, len(td.ScoreDocs))
	}
	seen := map[string]bool{}
	for _, sd := range td.ScoreDocs {
		doc, err := s.Doc(sd.Doc)
		if err != nil {
			t.Fatalf("Doc(%d): %v", sd.Doc, err)
		}
		pid := storedString(doc, "parentId")
		if seen[pid] {
			t.Errorf("duplicate parentId %q in results", pid)
		}
		seen[pid] = true
	}
	for _, want := range []string{"a", "b", "c"} {
		if !seen[want] {
			t.Errorf("missing parentId %q in results (got %v)", want, seen)
		}
	}
}

// TestParentBlockJoinKnnQueryTestCase_Random corresponds to testRandom: build a
// block index of many families with random vectors and verify the join returns
// exactly min(n, k, numParentsWithChildren) hits, each a distinct parent, in
// descending score order. A fixed seed keeps the test deterministic (Gocene has
// no RandomIndexWriter scaffolding); this is a behavioural, not byte, match.
func TestParentBlockJoinKnnQueryTestCase_Random(t *testing.T) {
	const (
		dim       = 5
		numFamily = 40
	)
	rng := newPBJRand(0x5DEECE66D)
	dir, w := newBlockWriter(t)
	numParentsWithChildren := 0
	for i := 0; i < numFamily; i++ {
		size := 1 + rng.intn(3) // 1..3 children
		var block []index.Document
		for c := 0; c < size; c++ {
			block = append(block, pbjMakeChild(t, "field", pbjRandomVector(rng, dim), pbjItoa(i*10+c), index.VectorSimilarityFunctionEuclidean))
		}
		block = append(block, pbjMakeParent(t, pbjItoa(i)))
		addBlock(t, w, block...)
		numParentsWithChildren++
	}
	reader, s := pbjOpen(t, dir, w)
	parentFilter := pbjParentsFilter()
	if err := Check(reader, parentFilter); err != nil {
		t.Fatalf("Check: %v", err)
	}

	for iter := 0; iter < 10; iter++ {
		k := rng.intn(20) + 1
		n := rng.intn(30) + 1
		q := NewDiversifyingChildrenFloatKnnVectorQuery("field", pbjRandomVector(rng, dim), k, nil, parentFilter)
		td, err := s.Search(q, n)
		if err != nil {
			t.Fatalf("Search (iter %d): %v", iter, err)
		}
		expected := min3(n, k, numParentsWithChildren)
		if len(td.ScoreDocs) != expected {
			t.Fatalf("iter %d: got %d hits, want %d (k=%d n=%d parents=%d)",
				iter, len(td.ScoreDocs), expected, k, n, numParentsWithChildren)
		}
		if td.TotalHits.Value < int64(len(td.ScoreDocs)) {
			t.Errorf("iter %d: totalHits %d < scoreDocs %d", iter, td.TotalHits.Value, len(td.ScoreDocs))
		}
		last := float32(3.4e38)
		for _, sd := range td.ScoreDocs {
			if sd.Score > last {
				t.Errorf("iter %d: scores not descending: %v", iter, pbjScores(td))
				break
			}
			last = sd.Score
		}
	}
}

// TestParentBlockJoinKnnQueryTestCase_DescriptorConstruction verifies that
// both Float and Byte query descriptors can be constructed without error,
// mirroring the structural intent of the test case setup methods.
func TestParentBlockJoinKnnQueryTestCase_DescriptorConstruction(t *testing.T) {
	floatQ := NewDiversifyingChildrenFloatKnnVectorQuery("field", []float32{1, 2}, 10, nil, nil)
	if floatQ == nil {
		t.Fatal("expected non-nil DiversifyingChildrenFloatKnnVectorQuery")
	}
	if floatQ.Field != "field" {
		t.Errorf("Field = %q, want %q", floatQ.Field, "field")
	}
	if floatQ.K != 10 {
		t.Errorf("K = %d, want 10", floatQ.K)
	}
	if len(floatQ.Target) != 2 || floatQ.Target[0] != 1 || floatQ.Target[1] != 2 {
		t.Errorf("Target = %v, want [1 2]", floatQ.Target)
	}

	byteQ := NewDiversifyingChildrenByteKnnVectorQuery("vec", []byte{3, 4}, 5, nil, nil)
	if byteQ == nil {
		t.Fatal("expected non-nil DiversifyingChildrenByteKnnVectorQuery")
	}
	if byteQ.Field != "vec" {
		t.Errorf("Field = %q, want %q", byteQ.Field, "vec")
	}
	if byteQ.K != 5 {
		t.Errorf("K = %d, want 5", byteQ.K)
	}
	if len(byteQ.Target) != 2 || byteQ.Target[0] != 3 || byteQ.Target[1] != 4 {
		t.Errorf("Target = %v, want [3 4]", byteQ.Target)
	}
}

// TestParentBlockJoinKnnQueryTestCase_TargetImmutability verifies that the
// query clones its target vector so external mutations do not affect the query.
func TestParentBlockJoinKnnQueryTestCase_TargetImmutability(t *testing.T) {
	orig := []float32{1, 2, 3}
	q := NewDiversifyingChildrenFloatKnnVectorQuery("f", orig, 5, nil, nil)
	orig[0] = 99
	if q.Target[0] == 99 {
		t.Error("query target was mutated by modifying original slice — clone is missing")
	}

	origB := []byte{10, 20}
	bq := NewDiversifyingChildrenByteKnnVectorQuery("f", origB, 3, nil, nil)
	origB[0] = 0
	if bq.Target[0] == 0 {
		t.Error("byte query target was mutated — clone is missing")
	}
}
