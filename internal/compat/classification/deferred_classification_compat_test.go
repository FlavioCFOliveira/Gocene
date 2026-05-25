// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_classification_compat_test.go is the explicit landing pad for
// the classification audit row that Sprint 114 T17 (rmp 4625)
// acknowledged but did NOT fully cover. Each entry below cites its audit
// row verbatim from docs/compat-coverage.tsv with the reason it remains
// deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without failing
// the build.
package classification

import "testing"

// TestClassificationAudit_DeferredRows iterates every classification-side
// leg that T17 recognised but could not complete with the current state
// of the Gocene classification port. The body of each subtest is a
// t.Skip with the row's audit citation.
//
// auditGapNotes is reproduced VERBATIM from the only row in
// docs/compat-coverage.tsv that names the classification package:
//
//	classification\t(none — runtime only, consumes index)\t(n/a)\t
//	  classification/\tpartial:classification/classification_test.go\t
//	  no\tno\tNo binary artefacts identified; classification reads
//	  existing indices.
//
// Per-classifier entries below pin the runtime-state legs that the
// scenario "classifier-label-corpus" emits but Gocene cannot yet replay
// end-to-end. Each lucene_class string is taken verbatim from the
// Lucene 10.4.0 source tree (see /tmp/lucene/lucene/classification/src/java).
func TestClassificationAudit_DeferredRows(t *testing.T) {
	const auditGap = "No binary artefacts identified; classification reads existing indices."

	deferred := []struct {
		artefact  string // logical leg of the classification runtime-state parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T17
	}{
		{
			artefact:  "Gocene SimpleNaiveBayesClassifier label/confidence parity vs Lucene",
			luceneCls: "org.apache.lucene.classification.SimpleNaiveBayesClassifier",
			gapNotes:  auditGap,
			reason: "rmp 4625 ships the Lucene-side classifier-label-corpus and " +
				"its verifier (verify-classifier-labels). The Gocene-side replay " +
				"(open the Lucene-emitted segment with Gocene's IndexSearcher " +
				"and re-run SimpleNaiveBayesClassifier against the catalogue) " +
				"is blocked on the SegmentReader core-readers gap recorded under " +
				"memory-index reference 'gocene-segmentreader-corereaders-gap'. " +
				"The harness verifier IS exercised by " +
				"classifier_label_compat_test.go::TestClassifierLabels_VerifySubcommand.",
		},
		{
			artefact:  "Gocene BM25NBClassifier label/confidence parity vs Lucene",
			luceneCls: "org.apache.lucene.classification.BM25NBClassifier",
			gapNotes:  auditGap,
			reason: "BM25NBClassifier extends the simple NB model by replacing the " +
				"term-frequency factor with BM25 score. The Gocene-side replay " +
				"requires (1) the SegmentReader core-readers wiring AND (2) " +
				"Gocene's BM25Similarity to agree byte-for-byte with Lucene's " +
				"on a Lucene-emitted segment. Both legs are pending; deferred " +
				"until they land.",
		},
		{
			artefact:  "Gocene KNearestNeighborClassifier label/confidence parity vs Lucene",
			luceneCls: "org.apache.lucene.classification.KNearestNeighborClassifier",
			gapNotes:  auditGap,
			reason: "KNearestNeighborClassifier drives MoreLikeThis to retrieve k " +
				"neighbours and votes by class. The Gocene-side replay requires " +
				"(1) the SegmentReader core-readers wiring, (2) a Gocene " +
				"MoreLikeThis port that mirrors Lucene's term-vector / TF-IDF " +
				"selection exactly, and (3) Gocene's IndexSearcher to score the " +
				"resulting BooleanQuery byte-for-byte. All three legs are " +
				"pending; deferred until they land.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Skipf("deferred: %s (lucene_class=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.gapNotes, row.reason)
		})
	}
}
