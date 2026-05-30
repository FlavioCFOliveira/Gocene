// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// index_merge_tool_compat_test.go addresses the misc audit row for
// IndexMergeTool (verbatim from docs/compat-coverage.tsv row 85, column
// 8): "No interop test merging a Lucene-written input." Reuses the
// misc-index-splitter-input fixture (3 Lucene-written segments) which is
// the canonical input shape IndexMergeTool exists to consume.
//
// Three classes: (a) read-fixture (asserts the 3-segment input is
// present), (b) byte-determinism + verify-misc splitter CLI (the same
// CLI gates the 3-segment shape both tools consume), (c) round-trip
// Skip (the Gocene IndexMergeTool port at misc/index_merge_tool.go has
// no end-to-end binary-parity gate against Lucene-written input).
package misc

import (
	"strconv"
	"strings"
	"testing"
)

// expectedMergeInputSegments mirrors expectedSplitterSegments — kept as a
// separate constant so the assertion intent at the call site is explicit.
const expectedMergeInputSegments = 3

// TestMiscIndexMergeTool_ReadFixture (class a) probes the input shape
// IndexMergeTool consumes: 3 Lucene-written segments with at least one
// non-empty .doc file per segment (a merge with no postings is a no-op).
func TestMiscIndexMergeTool_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMiscIndexSplitterInput, seed)
			files := listFiles(t, dir)
			siCount := countMatching(files, "_", ".si")
			if siCount != expectedMergeInputSegments {
				t.Fatalf("expected %d .si files (one per input segment for merge), "+
					"got %d; files=%v",
					expectedMergeInputSegments, siCount, files)
			}
			// Per-segment .doc files (PerField suffixed). Expect at least one
			// for each of segments _0, _1, _2.
			for s := 0; s < expectedMergeInputSegments; s++ {
				prefix := "_" + strconv.Itoa(s) + "_"
				if countMatching(files, prefix, ".doc") < 1 {
					t.Fatalf("expected at least one %s*.doc under fixture dir, "+
						"got files=%v", prefix, files)
				}
			}
		})
	}
}

// TestMiscIndexMergeTool_VerifySubcommand (class b) gates the input
// preconditions IndexMergeTool relies on via the same verify-misc
// splitter CLI: leaf-count==3 + totalDocs==18. A passing verify proves
// IndexMergeTool would have multiple non-empty segments to merge.
func TestMiscIndexMergeTool_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMiscIndexSplitterInput, seed)
			out, err := runHarness(t, "verify-misc", "splitter", dir,
				strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-misc splitter (merge-input gate) failed: %v\nstdout:\n%s",
					err, out)
			}
			if !strings.Contains(out, "ok verify-misc variant=splitter") {
				t.Errorf("expected 'ok verify-misc variant=splitter' in stdout, got: %s",
					out)
			}
		})
	}
}

// TestMiscIndexMergeTool_RoundTrip (class c) — full L -> G -> L replay
// is blocked on the Gocene misc/index_merge_tool.go port: there is no
// end-to-end gate that runs Gocene's merge tool over a Lucene-written
// directory and asserts byte-identity of the merged output against a
// Lucene-produced reference. The Lucene-side input gate IS exercised by
// TestMiscIndexMergeTool_VerifySubcommand.
func TestMiscIndexMergeTool_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for IndexMergeTool over scenario "+
				"%q at seed=%d is blocked on the Gocene misc/index_merge_tool.go "+
				"port — the package ships the tool implementation but has no "+
				"end-to-end gate that merges a Lucene-written multi-segment "+
				"directory and asserts byte-identity of the merged output. "+
				"The Lucene-side input gate IS exercised by "+
				"TestMiscIndexMergeTool_VerifySubcommand. "+
				"Audit gap_notes (verbatim): %q",
				ScenarioMiscIndexSplitterInput, seed, auditGapIndexMergeTool)
		})
	}
}
