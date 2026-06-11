// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// parser_trees_compat_test.go pins the three rmp 4630 test classes for
// qp-trees.tsv: (a) read-fixture, (b) write-and-verify (byte-determinism
// at both seeds + verify-queryparser subcommand), (c) Lucene -> Gocene
// -> Lucene round-trip deferred per parser_id with the verbatim audit
// gap_notes citation ("No binary artefacts; behavioural parity tested
// only via Gocene-internal cases.").
package queryparser

import (
	"bytes"
	"strings"
	"testing"
)

// TestQueryparserTrees_ReadFixture (class a) drives the harness, parses
// qp-trees.tsv, and pins its structural shape: every parser_id appears
// at least once, every parsed_to_string is non-empty, rows are sorted by
// (parser_id ASC, query_id ASC).
func TestQueryparserTrees_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioQueryparserTreesAndHits, seed)
			rows := readQPTreesTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("qp-trees.tsv empty (no queries parsed?)")
			}
			seen := make(map[string]bool, len(expectedParserIDs))
			for _, r := range rows {
				if r.parsedToString == "" {
					t.Errorf("parser=%s query=%s emitted empty parsed_to_string",
						r.parserID, r.queryID)
				}
				if r.queryText == "" {
					t.Errorf("parser=%s query=%s emitted empty query_text",
						r.parserID, r.queryID)
				}
				seen[r.parserID] = true
			}
			for _, want := range expectedParserIDs {
				if !seen[want] {
					t.Errorf("parser_id %q absent from qp-trees.tsv (catalogue drift?)", want)
				}
			}
			for i := 1; i < len(rows); i++ {
				a, b := rows[i-1], rows[i]
				if a.parserID > b.parserID {
					t.Errorf("row %d: parser_id not sorted ascending: %q after %q",
						i, b.parserID, a.parserID)
				} else if a.parserID == b.parserID && a.queryID > b.queryID {
					t.Errorf("row %d (parser=%s): query_id not sorted ascending: %q after %q",
						i, a.parserID, b.queryID, a.queryID)
				}
			}
		})
	}
}

// TestQueryparserTrees_WriteAndVerify (class b) runs the scenario twice
// at the same seed, asserts qp-trees.tsv is byte-identical across runs,
// and drives the new `verify-queryparser <dir>` subcommand to re-parse
// every entry and assert the recorded toString() matches.
func TestQueryparserTrees_WriteAndVerify(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioQueryparserTreesAndHits, seed)
			b := generate(t, ScenarioQueryparserTreesAndHits, seed)
			ab := readFileBytes(t, a, tsvQPTrees)
			bb := readFileBytes(t, b, tsvQPTrees)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("qp-trees.tsv drift between two runs at seed=%d", seed)
			}
			out, err := runHarness(t, "verify-queryparser", a)
			if err != nil {
				t.Fatalf("verify-queryparser failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-queryparser") {
				t.Errorf("expected 'ok verify-queryparser' in stdout, got: %s", out)
			}
		})
	}
}

// TestQueryparserTrees_RoundTrip (class c) — generate the fixture and verify
// qp-trees.tsv parses correctly. The full Lucene -> Gocene -> Lucene replay
// is blocked on the Gocene queryparser port — audit column 6 records the
// surface as 'partial:queryparser/query_parser_compatibility_test.go' and
// Gocene currently provides no Query.String() emitter that byte-matches
// Lucene's Query.toString() across the catalogue (six parsers / fourteen
// entries).
func TestQueryparserTrees_RoundTrip(t *testing.T) {
	const auditGap = "No binary artefacts; behavioural parity tested only via Gocene-internal cases."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioQueryparserTreesAndHits, seed)
			rows := readQPTreesTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("qp-trees.tsv empty (no queries parsed?)")
			}
			t.Logf("fixture generated in %s (seed=%#x, %d tree rows); "+
				"full Gocene round-trip blocked on queryparser port "+
				"(Query.String() parity; audit gap_notes: %q)",
				dir, seed, len(rows), auditGap)
		})
	}
}
