// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_search_compat_test.go is the explicit landing pad for the
// "search" audit rows that Sprint 114 T9 (rmp 4617) acknowledged but did
// NOT fully cover. Each entry below cites its audit row verbatim from
// docs/compat-coverage.tsv with the reason it remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without failing
// the build.
package search

import "testing"

// TestSearchAudit_DeferredRows iterates every search-side leg that T9
// recognised but could not complete with the current state of the
// Gocene IndexSearcher / KNN-search port. The body of each subtest is a
// t.Skip with the row's audit citation.
func TestSearchAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		artefact  string // audit row "artefact" column
		luceneCls string // audit row "lucene_class" column
		gapNotes  string // audit row "gap_notes" column
		reason    string // why this is deferred from Sprint 114 T9
	}{
		{
			artefact:  "Gocene IndexSearcher BM25 score parity vs Lucene",
			luceneCls: "org.apache.lucene.search.IndexSearcher",
			gapNotes:  "No persisted search artefact; gap is the absence of a numerical-parity corpus vs Lucene scores",
			reason: "rmp 4617 ships the Lucene-side scoring corpus and its " +
				"verifier (verify-scoring); the Gocene-side leg (open the " +
				"Lucene-emitted segment with Gocene's IndexSearcher and " +
				"re-evaluate the BM25 query set) is blocked on the " +
				"SegmentReader core-readers gap recorded under " +
				"memory-index reference 'gocene-segmentreader-corereaders-" +
				"gap': OpenDirectoryReader -> NewSegmentReader does not " +
				"wire postings/Terms for a Lucene-emitted segment, so " +
				"IndexSearcher.search(TermQuery) trips on a nil core " +
				"reader. Tracked alongside the SegmentReader wiring " +
				"backlog task. The harness-side verifier IS exercised " +
				"by scoring_parity_compat_test.go::" +
				"TestScoringCorpus_VerifySubcommand, which proves the " +
				"contract is honoured cross-run.",
		},
		{
			artefact:  "Gocene-write -> Lucene-verify scoring round-trip",
			luceneCls: "org.apache.lucene.index.IndexWriter (Gocene port)",
			gapNotes:  "No persisted search artefact; gap is the absence of a numerical-parity corpus vs Lucene scores",
			reason: "The full Lucene -> Gocene -> Lucene -> Gocene loop " +
				"requires Gocene's IndexWriter to produce a " +
				"Lucene-readable image of the 12-doc BM25 corpus and " +
				"emit a Gocene-written scoring.tsv that Lucene's " +
				"verify-scoring can re-evaluate. The IndexWriter port " +
				"is incomplete for end-to-end commit (see memory-index " +
				"reference 'gocene-segment-merger-baseline' / backlog " +
				"#2707). Deferred until that task lands. The Lucene " +
				"forward direction is fully exercised by " +
				"TestScoringCorpus_ReadFixture and the harness's own " +
				"determinism JUnit covers the byte-stability axis.",
		},
		{
			artefact:  "Gocene KnnFloatVectorQuery hit-ordering parity vs Lucene",
			luceneCls: "org.apache.lucene.search.KnnFloatVectorQuery",
			gapNotes:  "HNSW bytes in fixture exist but no end-to-end search verifies identical hit ordering vs Lucene",
			reason: "rmp 4617 ships the Lucene-side KNN hit-ordering corpus " +
				"and its verifier (verify-knn-hits); the Gocene-side leg " +
				"(open the Lucene-emitted .vex/.vec with Gocene and " +
				"re-evaluate KnnFloatVectorQuery against the fixed " +
				"query catalogue) is blocked by two compounding gaps: " +
				"(1) the SegmentReader core-readers wiring gap noted " +
				"above, and (2) Gocene's Lucene99HnswVectorsReader is " +
				"not yet wired into IndexSearcher dispatch (see " +
				"memory-index reference 'gocene-sprint-55-lucene99-" +
				"hnsw-writer' for the writer-side state; the reader-" +
				"side parity is the missing piece). The harness " +
				"verifier IS exercised by " +
				"knn_hit_ordering_compat_test.go::" +
				"TestKnnHitOrdering_VerifySubcommand.",
		},
		{
			artefact:  "Gocene-write -> Lucene-verify KNN round-trip",
			luceneCls: "org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsFormat (Gocene port)",
			gapNotes:  "HNSW bytes in fixture exist but no end-to-end search verifies identical hit ordering vs Lucene",
			reason: "The full Lucene -> Gocene -> Lucene -> Gocene loop " +
				"requires Gocene's Lucene99HnswVectorsWriter to " +
				"produce a Lucene-readable HNSW graph for the 30-doc / " +
				"dim=4 corpus AND to emit a Gocene-written knn-hits.tsv " +
				"that Lucene's verify-knn-hits can re-evaluate. The " +
				"writer is implemented in stub form (.vex/.vem byte-" +
				"parity, see memory-index reference 'gocene-sprint-55-" +
				"lucene99-hnsw-writer') but .vec emission and merge " +
				"are deferred until the FlatVectorsWriter port lands. " +
				"Deferred until those tasks ship.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Fatalf("deferred: %s (lucene_class=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.gapNotes, row.reason)
		})
	}
}
