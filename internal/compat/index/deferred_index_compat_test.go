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
			artefact:  "Gocene SegmentReader generational DV update value visibility",
			luceneCls: "org.apache.lucene.index.SegmentReader",
			gapNotes:  "DV update generational files are parsed and the file pair is registered; the actual updated value is not surfaced through a Gocene SegmentReader API in tests.",
			reason: "Gocene's OpenDirectoryReader path uses NewSegmentReader, " +
				"which does not yet wire core readers for the Lucene90 DV " +
				"format on a Lucene-emitted segment (see memory-index " +
				"reference 'gocene-segmentreader-corereaders-gap'). Reading " +
				"the updated long value via reader.getNumericDocValues " +
				"is therefore not exercised here. The Java-side scenario " +
				"verify() DOES assert count(doc-5) == 999 via " +
				"DirectoryReader.open, so the byte contract IS validated " +
				"cross-engine; only the Gocene-side reader leg is " +
				"deferred. Tracked separately for the SegmentReader " +
				"core-readers wiring task.",
		},
		{
			artefact:  "CheckIndex on a Gocene-round-tripped index",
			luceneCls: "org.apache.lucene.index.CheckIndex",
			gapNotes:  "Lucene's CheckIndex run over a Gocene-written index would confirm the writer side of the binary contract end-to-end.",
			reason: "Gocene's IndexWriter is not yet produces a " +
				"Lucene-readable on-disk image for the multi-commit / " +
				"DV-update / soft-delete scenarios covered here (no " +
				"compatibility test in the codebase asserts the inverse " +
				"direction at this depth). A Gocene-write -> Lucene-check " +
				"leg requires the SegmentMerger and IndexWriter porting " +
				"backlog tasks; see memory-index reference " +
				"'gocene-segment-merger-baseline' (#2707). The Lucene-side " +
				"forward direction (Lucene-write -> Lucene-CheckIndex) is " +
				"covered by check_index_compat_test.go.",
		},
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
			t.Skipf("deferred: %s (lucene_class=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.gapNotes, row.reason)
		})
	}
}
