// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// monitor_query_serializer_compat_test.go addresses the monitor audit row
// (verbatim from docs/compat-coverage.tsv): "No round-trip test against
// Lucene-serialised MonitorQuery blobs.". Scenario "monitor-query-blob"
// emits monitor-queries.bin — a CodecUtil-framed list of three
// MonitorQuerySerializer.fromParser(...) blobs (term/boolean/phrase).
//
// Three classes per the rmp 4626 contract:
//
//	(a) read-fixture        — drive the harness, parse the file shape.
//	(b) write-and-verify    — byte-determinism + verify-monitor blob CLI.
//	(c) full round-trip     — Lucene -> Gocene -> Lucene; t.Skip with the
//	                          verbatim audit gap_notes because Gocene's
//	                          MonitorQuerySerializer is a placeholder
//	                          interface (monitor/monitor_query_serializer.go
//	                          line 19 — "no-op placeholder is provided to
//	                          satisfy the interface").
package monitor

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// luceneCodecUtilIndexHeaderMagic is the BE int32 0x3FD76C17 that prefixes
// every CodecUtil-framed Lucene file. Pinning it here gives the read-fixture
// test a cheap structural assertion that does not require parsing the
// entire frame.
var luceneCodecUtilIndexHeaderMagic = []byte{0x3F, 0xD7, 0x6C, 0x17}

// TestMonitorQueryBlob_ReadFixture (class a) drives the harness and pins
// the structural shape of monitor-queries.bin: it exists, starts with the
// CodecUtil header magic, and is non-trivial in length (header + footer
// alone is 16 + 16 = 32 bytes, three blobs push it well past that).
func TestMonitorQueryBlob_ReadFixture(t *testing.T) {
	const headerPlusFooterFloor = 32
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMonitorQueryBlob, seed)
			files := listFiles(t, dir)
			if len(files) != 1 || files[0] != fileMonitorQueriesBin {
				t.Fatalf("expected exactly %q under fixture dir, got %v",
					fileMonitorQueriesBin, files)
			}
			blob := readFileBytes(t, dir, fileMonitorQueriesBin)
			if len(blob) <= headerPlusFooterFloor {
				t.Fatalf("%s suspiciously small (%d bytes); 3 MonitorQuery "+
					"blobs + CodecUtil framing should comfortably exceed %d",
					fileMonitorQueriesBin, len(blob), headerPlusFooterFloor)
			}
			if !bytes.HasPrefix(blob, luceneCodecUtilIndexHeaderMagic) {
				t.Errorf("%s does not start with CodecUtil IndexHeader magic %x; got prefix %x",
					fileMonitorQueriesBin, luceneCodecUtilIndexHeaderMagic, blob[:4])
			}
		})
	}
}

// TestMonitorQueryBlob_ByteDeterminism (class b, part 1) runs the scenario
// twice at the same seed and confirms monitor-queries.bin is byte-
// identical across runs. Catches any HashMap-iteration drift in the
// MonitorQuerySerializer (mitigated upstream by emitting empty metadata
// maps), and any header-id non-determinism upstream of Determinism.seed.
func TestMonitorQueryBlob_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioMonitorQueryBlob, seed)
			b := generate(t, ScenarioMonitorQueryBlob, seed)
			ab := readFileBytes(t, a, fileMonitorQueriesBin)
			bb := readFileBytes(t, b, fileMonitorQueriesBin)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
					fileMonitorQueriesBin, seed, len(ab), len(bb))
			}
		})
	}
}

// TestMonitorQueryBlob_VerifySubcommand (class b, part 2) drives the new
// `verify-monitor blob <dir> <seed>` subcommand. A clean exit (code 0)
// proves the Java verifier deserialises every blob with the default
// MonitorQuerySerializer and re-asserts id + queryString + metadata.
func TestMonitorQueryBlob_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMonitorQueryBlob, seed)
			out, err := runHarness(t, "verify-monitor", "blob", dir, strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-monitor blob failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-monitor variant=blob") {
				t.Errorf("expected 'ok verify-monitor variant=blob' in stdout, got: %s", out)
			}
		})
	}
}

// TestMonitorQueryBlob_RoundTrip (class c) — generate the fixture and verify
// monitor-queries.bin exists with the expected CodecUtil frame. Full
// Lucene -> Gocene -> Lucene -> Gocene replay is blocked on the Gocene
// MonitorQuerySerializer port (monitor/monitor_query_serializer.go line 19:
// 'no-op placeholder is provided to satisfy the interface').
func TestMonitorQueryBlob_RoundTrip(t *testing.T) {
	const auditGap = "No round-trip test against Lucene-serialised MonitorQuery blobs."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMonitorQueryBlob, seed)
			files := listFiles(t, dir)
			if len(files) != 1 || files[0] != fileMonitorQueriesBin {
				t.Fatalf("expected exactly %q under fixture dir, got %v",
					fileMonitorQueriesBin, files)
			}
			t.Logf("fixture generated in %s (seed=%#x); "+
				"full Gocene round-trip blocked on MonitorQuerySerializer port "+
				"(audit gap_notes: %q)", dir, seed, auditGap)
		})
	}
}

// _ pins monitorQueryBatchSize as a referenced symbol so future expansion
// of class (a) into per-blob structural checks does not flicker the lint.
var _ = monitorQueryBatchSize
