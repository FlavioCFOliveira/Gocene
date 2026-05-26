// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// deferred_monitor_compat_test.go is the explicit landing pad for the
// monitor audit rows that Sprint 114 T18 (rmp 4626) acknowledged but
// did NOT fully cover. Each entry below cites its audit row verbatim
// from docs/compat-coverage.tsv with the reason it remains deferred.
//
// Every deferral runs as a t.Skip subtest so the row appears in the
// `go test -v` output (evidence the row was considered) without failing
// the build.
package monitor

import "testing"

// TestMonitorAudit_DeferredRows iterates every monitor-side leg that T18
// recognised but could not complete with the current state of the
// Gocene monitor port. The body of each subtest is a t.Skip with the
// row's audit citation.
//
// auditGapNotes for the two rows are reproduced VERBATIM from
// docs/compat-coverage.tsv:
//
//	monitor MonitorQuerySerializer wire format
//	  org.apache.lucene.monitor.MonitorQuerySerializer
//	  monitor/monitor_query_serializer.go
//	  partial:monitor/monitor_test.go no no
//	  No round-trip test against Lucene-serialised MonitorQuery blobs.
//
//	monitor Monitor query index segment files
//	  org.apache.lucene.monitor.QueryIndex
//	  monitor/query_index.go
//	  partial:monitor/monitor_test.go no no
//	  No fixture from Lucene Monitor persistence.
func TestMonitorAudit_DeferredRows(t *testing.T) {
	const auditGapBlob = "No round-trip test against Lucene-serialised MonitorQuery blobs."
	const auditGapSegment = "No fixture from Lucene Monitor persistence."

	deferred := []struct {
		artefact  string // logical leg of the monitor binary-parity gap
		luceneCls string // canonical Lucene 10.4.0 class name
		goceneRef string // Gocene source-file reference (relative)
		gapNotes  string // audit row gap_notes column (verbatim)
		reason    string // why this is deferred from Sprint 114 T18
	}{
		{
			artefact:  "Gocene MonitorQuerySerializer wire-format parity vs Lucene",
			luceneCls: "org.apache.lucene.monitor.MonitorQuerySerializer",
			goceneRef: "monitor/monitor_query_serializer.go",
			gapNotes:  auditGapBlob,
			reason: "rmp 4626 ships the Lucene-side monitor-query-blob scenario " +
				"and its verifier (verify-monitor blob). The Gocene-side replay " +
				"(deserialise the Lucene-emitted blob with Gocene's " +
				"MonitorQuerySerializer.Deserialize and re-Serialize byte-for-byte) " +
				"is blocked on the placeholder implementation documented in " +
				"monitor/monitor_query_serializer.go line 19: 'A no-op " +
				"placeholder is provided to satisfy the interface'. " +
				"The harness verifier IS exercised by " +
				"monitor_query_serializer_compat_test.go::" +
				"TestMonitorQueryBlob_VerifySubcommand.",
		},
		{
			artefact:  "Gocene Monitor query index segment-file parity vs Lucene",
			luceneCls: "org.apache.lucene.monitor.QueryIndex",
			goceneRef: "monitor/query_index.go",
			gapNotes:  auditGapSegment,
			reason: "rmp 4626 ships the Lucene-side monitor-index-segment scenario " +
				"and its verifier (verify-monitor segment). The Gocene-side " +
				"replay requires (1) a working Gocene QueryIndex port that " +
				"can register MonitorQuery objects against an FSDirectory and " +
				"reopen the resulting commit, AND (2) the SegmentReader " +
				"core-readers wiring (memory-index reference " +
				"'gocene-segmentreader-corereaders-gap') so a Lucene-emitted " +
				"segment can be read through Gocene's IndexSearcher. Both " +
				"legs are pending; deferred until they land. The harness " +
				"verifier IS exercised by monitor_index_compat_test.go::" +
				"TestMonitorIndexSegment_VerifySubcommand.",
		},
	}

	for _, row := range deferred {
		row := row
		t.Run(row.artefact, func(t *testing.T) {
			t.Skipf("deferred: %s (lucene_class=%q gocene_ref=%q gap_notes=%q): %s",
				row.artefact, row.luceneCls, row.goceneRef, row.gapNotes, row.reason)
		})
	}
}
