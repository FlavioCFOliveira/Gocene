// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// index_splitter_compat_test.go addresses the misc audit row for
// IndexSplitter (verbatim from docs/compat-coverage.tsv row 85, column
// 8): "No interop test merging a Lucene-written input." Three classes:
// (a) read-fixture, (b) byte-determinism + verify-misc splitter CLI,
// (c) round-trip Skip (Gocene port of misc/index has no end-to-end
// binary-parity gate against Lucene-written multi-segment input).
package misc

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// expectedSplitterSegments is the segment count produced by the
// MiscIndexSplitterInputScenario (3 batches of 6 docs, commit per batch,
// NoMergePolicy). Used by the read-fixture test to assert by file shape.
const expectedSplitterSegments = 3

// TestMiscIndexSplitter_ReadFixture (class a) pins the structural shape:
// the splitter input fixture must surface exactly three .si files
// (one per segment, _0.si .. _2.si) — the input shape IndexSplitter
// operates on per-segment.
func TestMiscIndexSplitter_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMiscIndexSplitterInput, seed)
			files := listFiles(t, dir)
			siCount := countMatching(files, "_", ".si")
			if siCount != expectedSplitterSegments {
				t.Fatalf("expected %d .si files (one per segment), got %d; files=%v",
					expectedSplitterSegments, siCount, files)
			}
			// Sanity: segments_<N> commit marker must exist (committed three
			// times, so the latest generation is segments_3).
			haveCommit := false
			for _, f := range files {
				if strings.HasPrefix(f, "segments_") {
					haveCommit = true
					break
				}
			}
			if !haveCommit {
				t.Fatalf("expected a segments_<gen> commit marker, got files=%v", files)
			}
		})
	}
}

// TestMiscIndexSplitter_ByteDeterminism (class b, part 1) re-runs the
// scenario at the same seed and asserts the per-segment .doc / .pos /
// .fdt files are byte-identical — proves IndexWriter ordering is stable
// across runs, which is the prerequisite for any IndexSplitter parity
// claim.
func TestMiscIndexSplitter_ByteDeterminism(t *testing.T) {
	// Probe a representative set of per-segment artefacts. PerField
	// suffixes (e.g. _0_Lucene104_0.doc) are stable but seed-independent.
	probeSuffixes := []string{".doc", ".pos", ".fdt", ".fdx", ".fdm"}
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioMiscIndexSplitterInput, seed)
			b := generate(t, ScenarioMiscIndexSplitterInput, seed)
			filesA := listFiles(t, a)
			compared := 0
			for _, name := range filesA {
				match := false
				for _, sx := range probeSuffixes {
					if strings.HasSuffix(name, sx) {
						match = true
						break
					}
				}
				if !match {
					continue
				}
				ab := readFileBytes(t, a, name)
				bb := readFileBytes(t, b, name)
				if !bytes.Equal(ab, bb) {
					t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
						name, seed, len(ab), len(bb))
				}
				compared++
			}
			if compared == 0 {
				t.Fatalf("expected at least one probe file under %s, got files=%v",
					a, filesA)
			}
		})
	}
}

// TestMiscIndexSplitter_VerifySubcommand (class b, part 2) drives
// `verify-misc splitter`. Clean exit proves the Java verifier reopens
// the directory and asserts (1) leaf-count==3, (2) totalDocs==18 — the
// exact preconditions IndexSplitter expects on its input.
func TestMiscIndexSplitter_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMiscIndexSplitterInput, seed)
			out, err := runHarness(t, "verify-misc", "splitter", dir,
				strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-misc splitter failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-misc variant=splitter") {
				t.Errorf("expected 'ok verify-misc variant=splitter' in stdout, got: %s", out)
			}
		})
	}
}

// TestMiscIndexSplitter_RoundTrip (class c) — full L -> G -> L replay is
// blocked on the Gocene misc/index port: the Java-side
// org.apache.lucene.misc.index.IndexSplitter has no Gocene counterpart
// that has been validated against a Lucene-produced multi-segment
// directory. The Lucene-side verifier IS exercised by class b
// (TestMiscIndexSplitter_VerifySubcommand) which proves the input shape
// is correct.
func TestMiscIndexSplitter_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked on the Gocene misc/index port — the package "+
				"(misc/index/) ships no IndexSplitter equivalent that has "+
				"been validated against a Lucene-produced multi-segment "+
				"directory. The Lucene-side verifier IS exercised by "+
				"TestMiscIndexSplitter_VerifySubcommand. "+
				"Audit gap_notes (verbatim): %q",
				ScenarioMiscIndexSplitterInput, seed, auditGapIndexSplitter)
		})
	}
}
