// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// child_block_join_compat_test.go — same audit row as parent_block_join_
// compat_test.go, focused on the to-child TSV emitted by
// ToChildBlockJoinQuery (parent parent_id=p-1 -> its children).
package join

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// expectedChildQueryID MUST match ParentBlockCorpusScenario.QUERY_IDS[1].
const expectedChildQueryID = "to-child-p1"

// TestChildBlockJoin_ReadFixture (class a) drives the harness, parses
// join-to-child-hits.tsv, and pins its structural shape: every row
// belongs to the expected query, child_id matches p-1-c-<j> (since
// the to-child query filters parent_id=p-1), ranks are contiguous, and
// scores are finite.
func TestChildBlockJoin_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioParentBlockCorpus, seed)
			rows := readChildTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("%s empty (no hits from ToChildBlockJoinQuery?)", tsvToChild)
			}
			for i, r := range rows {
				if r.queryID != expectedChildQueryID {
					t.Errorf("row %d: query_id %q, want %q", i, r.queryID, expectedChildQueryID)
				}
				if !strings.HasPrefix(r.childID, "p-1-c-") {
					t.Errorf("row %d: child_id %q does not match p-1-c-<j>", i, r.childID)
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

// TestChildBlockJoin_ByteDeterminism (class b, part 1).
func TestChildBlockJoin_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioParentBlockCorpus, seed)
			b := generate(t, ScenarioParentBlockCorpus, seed)
			ab := readFileBytes(t, a, tsvToChild)
			bb := readFileBytes(t, b, tsvToChild)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("%s drift between two runs at seed=%d:\n A=%q\n B=%q",
					tsvToChild, seed, ab, bb)
			}
		})
	}
}

// TestChildBlockJoin_VerifySubcommand (class b, part 2).
func TestChildBlockJoin_VerifySubcommand(t *testing.T) {
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

// TestChildBlockJoin_RoundTrip (class c) — full L -> G -> L replay is
// blocked on the SegmentReader core-readers gap. Generate the fixture and
// verify the expected child-hits TSV exists as a minimum viability check.
func TestChildBlockJoin_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioParentBlockCorpus, seed)
			rows := readChildTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("%s empty (no hits from ToChildBlockJoinQuery?) at seed=%d",
					tsvToChild, seed)
			}
		})
	}
}

