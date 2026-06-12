// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_backward_codecs_compat_test.go is the explicit landing pad
// for the backward_codecs audit rows whose Lucene-side write surface is
// READ-ONLY in Apache Lucene 10.4.0 and whose Gocene-side replay
// therefore cannot be exercised against a Lucene-emitted fixture. Each
// entry below cites its audit row VERBATIM from
// docs/compat-coverage.tsv with the read-only deferral reason.
//
// Why this file exists alongside the per-scenario *_compat_test.go
// landing pads: the per-scenario files cover the (lucene-side, gocene-
// side) test-class triple for that single row. THIS file aggregates ALL
// remaining deferred rows in one place so a single `go test -v -run
// Audit_DeferredRows` invocation surfaces the complete deferral
// footprint with citations — handy for sprint-close reports and for
// future backward-compat sprints that revisit the corpus.
//
// Rows REMOVED from this file in Sprint 14 (T81/T82):
//   - Lucene70 SegmentInfoFormat    → REAL (T82a, Gocene-write → Java-read)
//   - Lucene90 HNSW v0              → REAL (T82b, Gocene-write → Java-read)
//   - Lucene99 PostingsFormat       → REAL (T81a, Gocene-write → Java-read)
//   - Lucene99 ScalarQuantized      → REAL (T81c, Gocene-write → Java-read)
//   - Lucene103 PostingsFormat      → REAL (T81b, Gocene-write → Java-read)
//   - Lucene40 BlockTree            → BLOCKER documented (T82c)
package backward_codecs

import "testing"

// TestBackwardCodecsAudit_DeferredRows iterates every backward_codecs
// audit row whose Lucene 10.4.0 surface cannot produce a fixture AND that
// does not yet have a Gocene-side writer or blocker test. The body of
// each subtest is a t.Fatal with the row's audit citation, matching the
// no-skip policy (gap must fail, not be silently deferred).
func TestBackwardCodecsAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		scenario  string // kebab-case scenario name (also in Manifest.DEFERRED_ROWS)
		artefact  string // logical leg of the backward_codecs parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		gocenePkg string // canonical Gocene gocene_class column
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T26
	}{
		{
			scenario:  ScenarioBwcMultiVersionCorpora,
			artefact:  "Backwards-compat full index corpora (multi-version) cross-engine fixture",
			luceneCls: "org.apache.lucene.backward_index.TestBasicBackwardsCompatibility",
			gocenePkg: "backward_codecs/backward_index/",
			gapNotes:  "Tests are skeletons; no actual multi-version Lucene index ZIPs committed.",
			reason: "Producing the per-major-version index ZIPs that " +
				"TestBasicBackwardsCompatibility consumes requires building EACH " +
				"old Lucene major (7/8/9/10) and emitting an index per branch; " +
				"out of binary-compat-mandate scope (10.4.0 reference pin).",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Fatalf("deferred: %s (scenario=%q lucene_class=%q gocene_class=%q "+
				"gap_notes=%q): %s",
				row.artefact, row.scenario, row.luceneCls, row.gocenePkg,
				row.gapNotes, row.reason)
		})
	}
}
