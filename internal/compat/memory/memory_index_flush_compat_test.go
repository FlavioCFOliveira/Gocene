// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// memory_index_flush_compat_test.go addresses the memory audit row
// (verbatim from docs/compat-coverage.tsv): "No persisted binary
// artefact; gap is the absence of byte-for-byte parity tests vs Lucene
// MemoryIndex internal layout (where applicable to merges).". Scenario
// "memory-index-flush" boots an in-memory MemoryIndex, wraps its leaf
// via SlowCodecReaderWrapper, and flushes it into a Directory-backed
// IndexWriter (addIndexes + forceMerge(1)).
//
// Three classes per the rmp 4633 contract:
//
//	(a) read-fixture     — drive the harness, pin the expected shape of
//	                       the flushed directory (segments_1 + a single
//	                       _0.* segment family).
//	(b) write-and-verify — byte-determinism over segments_1 + the new
//	                       verify-memory-flush CLI subcommand.
//	(c) full round-trip  — Lucene -> Gocene -> Lucene replay; t.Skip with
//	                       the verbatim audit gap_notes because the
//	                       Gocene MemoryIndex equivalent may not support
//	                       the addIndexes flush path required to assert
//	                       byte parity on merges.
package memory

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// TestMemoryIndexFlush_ReadFixture (class a) drives the harness and
// asserts the directory looks like a real Lucene segment: at minimum
// segments_1 must exist and be non-empty, and at least one _0.si
// segment-info file must be present (single-segment forceMerge(1)).
func TestMemoryIndexFlush_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMemoryIndexFlush, seed)
			files := listFiles(t, dir)
			if len(files) == 0 {
				t.Fatalf("%s produced no files under %s",
					ScenarioMemoryIndexFlush, dir)
			}
			segBytes := readFileBytes(t, dir, segmentsGenerationFile)
			if len(segBytes) == 0 {
				t.Fatalf("%s exists but is empty", segmentsGenerationFile)
			}
			// Single-segment expectation: at least one _0.si must exist
			// (forceMerge(1) under NoMergePolicy collapses everything
			// to a single segment generation).
			var sawSegmentInfo bool
			for _, f := range files {
				if strings.HasPrefix(f, "_0.si") {
					sawSegmentInfo = true
					break
				}
			}
			if !sawSegmentInfo {
				t.Errorf("expected at least one _0.si segment-info file under %s; got %v",
					dir, files)
			}
		})
	}
}

// TestMemoryIndexFlush_ByteDeterminism (class b, part 1) runs the
// scenario twice at the same seed and confirms segments_1 — the
// commit-level pointer file Lucene's CodecUtil-frames and stamps with
// the deterministic segment id — is byte-identical across runs. The
// .si / .fdt / .tim files inherit the same nextId state, so a
// segments_1 match is a strong proxy for full-directory determinism;
// the Java-side ScenarioDeterminismTest asserts the broader invariant.
func TestMemoryIndexFlush_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioMemoryIndexFlush, seed)
			b := generate(t, ScenarioMemoryIndexFlush, seed)
			ab := readFileBytes(t, a, segmentsGenerationFile)
			bb := readFileBytes(t, b, segmentsGenerationFile)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d (lenA=%d lenB=%d)",
					segmentsGenerationFile, seed, len(ab), len(bb))
			}
		})
	}
}

// TestMemoryIndexFlush_VerifySubcommand (class b, part 2) drives the
// new `verify-memory-flush <dir> <seed>` subcommand. A clean exit
// (code 0) proves the Java verifier reopens the flushed directory,
// re-resolves every indexed token term and confirms every payload byte.
func TestMemoryIndexFlush_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioMemoryIndexFlush, seed)
			out, err := runHarness(t, "verify-memory-flush", dir, strconv.FormatInt(seed, 10))
			if err != nil {
				t.Fatalf("verify-memory-flush failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-memory-flush") {
				t.Errorf("expected 'ok verify-memory-flush' in stdout, got: %s", out)
			}
		})
	}
}

// TestMemoryIndexFlush_RoundTrip (class c) — Gocene-side replay of the
// Lucene-emitted flushed segment is blocked on the Gocene MemoryIndex
// surface. The audit row is reproduced verbatim in the Skipf message so
// it surfaces in `go test -v` output as evidence.
func TestMemoryIndexFlush_RoundTrip(t *testing.T) {
	const auditGap = "No persisted binary artefact; gap is the absence of byte-for-byte parity tests vs Lucene MemoryIndex internal layout (where applicable to merges)."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Fatalf("deferred: Gocene round-trip for scenario %q at seed=%d is "+
				"blocked because the Gocene MemoryIndex equivalent "+
				"(memory/memory_index.go) may not support the "+
				"addIndexes(CodecReader...) flush path required to "+
				"assert byte parity on merges; the harness verifier IS "+
				"exercised by TestMemoryIndexFlush_VerifySubcommand; "+
				"audit gap_notes (verbatim): %q",
				ScenarioMemoryIndexFlush, seed, auditGap)
		})
	}
}
