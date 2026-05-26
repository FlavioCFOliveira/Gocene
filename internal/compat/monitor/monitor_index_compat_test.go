// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// monitor_index_compat_test.go addresses the monitor audit row
// (verbatim from docs/compat-coverage.tsv): "No fixture from Lucene
// Monitor persistence.". Scenario "monitor-index-segment" boots a
// Monitor backed by an FSDirectory, registers five MonitorQuery objects
// and emits the resulting Lucene 10.4.0 segment files.
//
// Three classes per the rmp 4626 contract:
//
//	(a) read-fixture        — drive the harness, pin the expected set of
//	                          Lucene segment files (segments_1 + a _0.*
//	                          family produced by NoMergePolicy +
//	                          Lucene104Codec).
//	(b) write-and-verify    — byte-determinism (over segments_1) + the
//	                          verify-monitor segment CLI subcommand.
//	(c) full round-trip     — Lucene -> Gocene -> Lucene replay; t.Skip
//	                          with the verbatim audit gap_notes because
//	                          Gocene's QueryIndex port is the placeholder
//	                          described in monitor/query_index.go.
package monitor

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestMonitorIndexSegment_ReadFixture (class a) drives the harness and
// asserts the directory looks like a real Lucene segment: at minimum
// segments_1 must exist and be non-empty, and the directory must contain
// a single segment (_0.si + family). The exact filenames depend on the
// per-field codec dispatch, so we probe by prefix rather than enumerating
// every byte-coded extension.
func TestMonitorIndexSegment_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMonitorIndexSegment, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("%s produced no files under %s",
					ScenarioMonitorIndexSegment, dir)
			}
			sort.Strings(files)
			// 1) segments_1 must exist and be non-empty.
			bytesSeg, nonEmpty := digestStable(t, dir, segmentsGenerationFile)
			if !nonEmpty {
				t.Fatalf("%s exists but is empty (got %d bytes)",
					segmentsGenerationFile, len(bytesSeg))
			}
			// 2) at least one _0.si segment-info marker.
			var sawSegmentInfo bool
			for _, f := range files {
				if strings.HasPrefix(f, "_0.si") {
					sawSegmentInfo = true
					break
				}
			}
			if !sawSegmentInfo {
				t.Errorf("expected at least one _0.si segment info file under %s; got %v",
					dir, files)
			}
		})
	}
}

// TestMonitorIndexSegment_ByteDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms segments_1 — the
// commit-level pointer file that Lucene's CodecUtil-frames and stamps
// with the deterministic segment id — is byte-identical across runs.
// The .si / .fdt / .tim files inherit the same nextId state, so a
// segments_1 match is a strong proxy for full-directory determinism;
// the manifest snapshot test on the Java side asserts the broader
// invariant.
func TestMonitorIndexSegment_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioMonitorIndexSegment, seed)
			b := generate(t, ScenarioMonitorIndexSegment, seed)
			ab := readFileBytes(t, a, segmentsGenerationFile)
			bb := readFileBytes(t, b, segmentsGenerationFile)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
					segmentsGenerationFile, seed, len(ab), len(bb))
			}
		})
	}
}

// TestMonitorIndexSegment_VerifySubcommand (class b, part 2) drives the
// new `verify-monitor segment <dir> <seed>` subcommand. A clean exit
// (code 0) proves the Java verifier reopens the Monitor over the
// fixture and re-resolves every registered MonitorQuery id.
func TestMonitorIndexSegment_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMonitorIndexSegment, seed)
			out, err := runHarness(t, "verify-monitor", "segment", dir, strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-monitor segment failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-monitor variant=segment") {
				t.Errorf("expected 'ok verify-monitor variant=segment' in stdout, got: %s", out)
			}
		})
	}
}

// TestMonitorIndexSegment_RoundTrip (class c) — Gocene-side replay of a
// Lucene-emitted Monitor persistence directory is blocked on the Gocene
// QueryIndex port. The audit row is reproduced verbatim in the Skipf
// message so it surfaces in `go test -v` output as evidence.
func TestMonitorIndexSegment_RoundTrip(t *testing.T) {
	const auditGap = "No fixture from Lucene Monitor persistence."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Skipf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene QueryIndex port (monitor/query_index.go) "+
				"which depends in turn on the SegmentReader core-readers gap "+
				"(memory-index ref 'gocene-segmentreader-corereaders-gap'); "+
				"audit gap_notes (verbatim): %q",
				ScenarioMonitorIndexSegment, seed, auditGap)
		})
	}
}

// _ pins monitorIndexBatchSize as a referenced symbol so the constant
// remains tied to the harness contract even if class (a) is later
// expanded with per-query structural checks.
var _ = monitorIndexBatchSize
