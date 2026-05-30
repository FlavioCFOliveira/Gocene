// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_join_compat_test.go is the landing pad for the join audit
// row that T15 acknowledged but could not fully cover. Each entry cites
// its audit row verbatim from docs/compat-coverage.tsv. Skips evidence
// the row was considered.
package join

import "testing"

// TestJoinAudit_DeferredRows iterates every join-side leg that T15
// recognised but could not complete with the current state of the
// Gocene join port. The body of each subtest is a t.Skip with the
// row's audit citation.
//
// auditGapNotes is reproduced VERBATIM from the only row in
// docs/compat-coverage.tsv that names the join package:
//
//	join\t(none — runtime block-join only)\t(n/a)\tjoin/\t
//	  partial:join/just_compile_search_join_test.go\tno\tno\t
//	  No binary artefacts originate in join; coverage gap is integration
//	  with Lucene-written parent-block segments.
//
// Per-query class entries below pin the runtime-state legs that the
// scenario "parent-block-corpus" emits but Gocene cannot yet replay
// end-to-end. Each lucene_class string is taken verbatim from the
// Lucene 10.4.0 source tree (see /tmp/lucene/lucene/join/src/java).
func TestJoinAudit_DeferredRows(t *testing.T) {
	const auditGap = "No binary artefacts originate in join; coverage gap is integration with Lucene-written parent-block segments"

	deferred := []struct {
		artefact  string // logical leg of the join runtime-state parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T15
	}{
		{
			artefact:  "Gocene ToParentBlockJoinQuery hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.search.join.ToParentBlockJoinQuery",
			gapNotes:  auditGap,
			reason: "rmp 4623 ships the Lucene-side parent-block-corpus and " +
				"its verifier (verify-join-hits). The Gocene-side replay " +
				"(open the Lucene-emitted parent-block segment with " +
				"Gocene's IndexSearcher and re-evaluate " +
				"ToParentBlockJoinQuery against the catalogue) is blocked " +
				"on the SegmentReader core-readers gap recorded under " +
				"memory-index reference 'gocene-segmentreader-corereaders-" +
				"gap'. The harness verifier IS exercised by " +
				"parent_block_join_compat_test.go::" +
				"TestParentBlockJoin_VerifySubcommand.",
		},
		{
			artefact:  "Gocene ToChildBlockJoinQuery hit/score parity vs Lucene",
			luceneCls: "org.apache.lucene.search.join.ToChildBlockJoinQuery",
			gapNotes:  auditGap,
			reason: "rmp 4623 ships the Lucene-side parent-block-corpus and " +
				"its verifier (verify-join-hits). The Gocene-side replay " +
				"(open the Lucene-emitted parent-block segment with " +
				"Gocene's IndexSearcher and re-evaluate " +
				"ToChildBlockJoinQuery for parent_id=p-1) is blocked on " +
				"the SegmentReader core-readers gap ('gocene-segmentreader-" +
				"corereaders-gap'): IndexSearcher.search trips on a nil " +
				"core reader before BitSetProducer materialises the " +
				"parents-filter bitset. The harness verifier IS exercised " +
				"by child_block_join_compat_test.go::" +
				"TestChildBlockJoin_VerifySubcommand.",
		},
		{
			artefact:  "Gocene QueryBitSetProducer parents-filter parity vs Lucene",
			luceneCls: "org.apache.lucene.search.join.QueryBitSetProducer",
			gapNotes:  auditGap,
			reason: "QueryBitSetProducer caches a BitSet per leaf via a " +
				"LeafReader-keyed weak cache. The Gocene-side replay " +
				"requires both (1) the SegmentReader core-readers wiring " +
				"(see 'gocene-segmentreader-corereaders-gap') and " +
				"(2) Gocene's join package to consume a Lucene-emitted " +
				"FixedBitSet through the same caching contract. Both " +
				"legs are pending; deferred until they land.",
		},
		{
			artefact:  "Gocene CheckJoinIndex parent-block invariant parity vs Lucene",
			luceneCls: "org.apache.lucene.search.join.CheckJoinIndex",
			gapNotes:  auditGap,
			reason: "CheckJoinIndex asserts parent-block contiguity " +
				"(every child precedes its parent, no orphan children). " +
				"The Gocene-side replay requires the SegmentReader core-" +
				"readers wiring AND a Gocene CheckJoinIndex port that " +
				"walks segments in the same doc-id order Lucene does. " +
				"Deferred until those legs land.",
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
