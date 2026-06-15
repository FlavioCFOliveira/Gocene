// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_index_compat_test.go is the explicit landing pad for index/
// audit rows that Sprint 114 T8 (rmp 4616) acknowledged but did NOT
// fully cover. Each entry below is cited verbatim from
// docs/compat-coverage.tsv with the reason it remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without
// failing the build.
package index

import "testing"

// TestIndexAudit_DeferredRows iterates every audit row not covered by
// a dedicated compat test in this directory. The body of each subtest
// is a t.Skip with the row's audit citation.
func TestIndexAudit_DeferredRows(t *testing.T) {
	deferred := []struct {
		artefact  string // audit row "artefact" column
		luceneCls string // audit row "lucene_class" column
		gapNotes  string // audit row "gap_notes" column
		reason    string // why this is deferred from Sprint 114 T8
	}{
		{
			artefact:  "SegmentCommitInfo.diagnostics map round-trip",
			luceneCls: "org.apache.lucene.index.SegmentCommitInfo",
			gapNotes:  "Diagnostics carries a wall-clock timestamp Lucene stamps on every commit, so byte-equal comparison is not meaningful; logical round-trip is not pinned.",
			reason: "Diagnostics are a wall-clock-stamped map that varies " +
				"every run by design (see the existing .si byte-equal " +
				"carve-out in internal/compat/codecs/" +
				"lucene99_segment_info_compat_test.go). Round-tripping the " +
				"map values requires the SegmentInfo .si reader/writer to " +
				"round-trip through Gocene, which IS exercised by " +
				"codecs/segment_info_format_test.go on Gocene-written .si " +
				"files. The cross-engine logical comparison would require " +
				"a per-key allow-list (timestamp excluded) and is outside " +
				"the T8 scope.",
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
