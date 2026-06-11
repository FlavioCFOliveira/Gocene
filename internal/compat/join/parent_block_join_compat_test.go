// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// parent_block_join_compat_test.go addresses the join audit row
// (verbatim from docs/compat-coverage.tsv): "No binary artefacts
// originate in join; coverage gap is integration with Lucene-written
// parent-block segments". Focus: ToParentBlockJoinQuery TSV.
//
// Three test classes per the rmp 4623 contract: (a) read-fixture, (b)
// write-and-verify (byte-determinism + verify-join-hits subcommand),
// (c) full round-trip — deferred behind the SegmentReader core-readers
// gap.
package join

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// expectedParentQueryID MUST match ParentBlockCorpusScenario.QUERY_IDS[0].
const expectedParentQueryID = "to-parent-color0"

// TestParentBlockJoin_ReadFixture (class a) — structural shape.
func TestParentBlockJoin_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioParentBlockCorpus, seed)
			rows := readParentTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("%s empty (no hits from ToParentBlockJoinQuery?)", tsvToParent)
			}
			for i, r := range rows {
				if r.queryID != expectedParentQueryID {
					t.Errorf("row %d: query_id %q, want %q", i, r.queryID, expectedParentQueryID)
				}
				if !strings.HasPrefix(r.parentID, "p-") {
					t.Errorf("row %d: parent_id %q does not match p-<i>", i, r.parentID)
				}
				if math.IsNaN(r.score) || math.IsInf(r.score, 0) {
					t.Errorf("row %d: non-finite score %g", i, r.score)
				}
				if r.rank != i {
					t.Errorf("row %d: rank=%d, want %d (contiguous)", i, r.rank, i)
				}
			}
		})
	}
}

// TestParentBlockJoin_ByteDeterminism (class b, part 1).
func TestParentBlockJoin_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioParentBlockCorpus, seed)
			b := generate(t, ScenarioParentBlockCorpus, seed)
			ab := readFileBytes(t, a, tsvToParent)
			bb := readFileBytes(t, b, tsvToParent)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d:\n A=%q\n B=%q",
					tsvToParent, seed, ab, bb)
			}
		})
	}
}

// TestParentBlockJoin_VerifySubcommand (class b, part 2) — drives the
// new `verify-join-hits <dir>` subcommand against both TSVs.
func TestParentBlockJoin_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioParentBlockCorpus, seed)
			out, err := runHarness(t, "verify-join-hits", dir)
			if err != nil {
				t.Fatalf("verify-join-hits failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-join-hits") {
				t.Errorf("expected 'ok verify-join-hits' in stdout, got: %s", out)
			}
		})
	}
}

// TestParentBlockJoin_RoundTrip (class c) — full L -> G -> L replay is
// blocked on the SegmentReader core-readers gap. Generate the fixture and
// verify the expected parent-hits TSV exists as a minimum viability check.
func TestParentBlockJoin_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioParentBlockCorpus, seed)
			rows := readParentTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("%s empty (no hits from ToParentBlockJoinQuery?) at seed=%d",
					tsvToParent, seed)
			}
		})
	}
}
