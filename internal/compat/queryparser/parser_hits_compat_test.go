// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// parser_hits_compat_test.go pins the three rmp 4630 test classes for
// qp-hits.tsv: (a) read-fixture (rank contiguity, finite scores), (b)
// write-and-verify (byte-determinism + verify-queryparser subcommand),
// (c) Lucene -> Gocene -> Lucene round-trip per (parser_id, query_id),
// deferred with the verbatim audit gap_notes citation.
package queryparser

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// TestQueryparserHits_ReadFixture (class a) parses qp-hits.tsv and pins
// its structural shape.
func TestQueryparserHits_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioQueryparserTreesAndHits, seed)
			rows := readQPHitsTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("qp-hits.tsv empty (no hits returned by Lucene?)")
			}
			// Per-(parser, query) rank contiguity check.
			type key struct{ parser, query string }
			ranks := make(map[key][]int, len(expectedParserIDs))
			seenParsers := make(map[string]bool, len(expectedParserIDs))
			for _, r := range rows {
				if !strings.HasPrefix(r.docID, "doc-") {
					t.Errorf("doc_id %q does not match doc-<i> pattern", r.docID)
				}
				if math.IsNaN(r.score) || math.IsInf(r.score, 0) {
					t.Errorf("parser=%s query=%s rank=%d non-finite score: %v",
						r.parserID, r.queryID, r.rank, r.score)
				}
				k := key{r.parserID, r.queryID}
				ranks[k] = append(ranks[k], r.rank)
				seenParsers[r.parserID] = true
			}
			for k, list := range ranks {
				for i, r := range list {
					if r != i {
						t.Errorf("parser=%s query=%s: rank[%d]=%d, want %d",
							k.parser, k.query, i, r, i)
					}
				}
			}
			for _, want := range expectedParserIDs {
				if !seenParsers[want] {
					t.Errorf("parser_id %q absent from qp-hits.tsv", want)
				}
			}
			// Rows must be sorted by (parser_id, query_id, rank) ASC.
			for i := 1; i < len(rows); i++ {
				a, b := rows[i-1], rows[i]
				if a.parserID > b.parserID {
					t.Errorf("row %d: parser_id not sorted ascending: %q after %q",
						i, b.parserID, a.parserID)
				} else if a.parserID == b.parserID {
					if a.queryID > b.queryID {
						t.Errorf("row %d: query_id not sorted ascending: %q after %q",
							i, b.queryID, a.queryID)
					} else if a.queryID == b.queryID && a.rank > b.rank {
						t.Errorf("row %d: rank not sorted ascending: %d after %d",
							i, b.rank, a.rank)
					}
				}
			}
		})
	}
}

// TestQueryparserHits_WriteAndVerify (class b) is the byte-determinism +
// `verify-queryparser` round-trip gate for the hits TSV. The single
// verify subcommand also re-asserts qp-trees.tsv, but the explicit hit
// check here keeps the failure surface narrow if a future change drifts
// only one of the two TSVs.
func TestQueryparserHits_WriteAndVerify(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioQueryparserTreesAndHits, seed)
			b := generate(t, ScenarioQueryparserTreesAndHits, seed)
			ab := readFileBytes(t, a, tsvQPHits)
			bb := readFileBytes(t, b, tsvQPHits)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("qp-hits.tsv drift between two runs at seed=%d", seed)
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

// TestQueryparserHits_RoundTrip (class c) iterates the catalogue rows
// from a Lucene-emitted qp-hits.tsv and emits one t.Skip per unique
// (parser_id, query_id) pair, citing the verbatim audit gap_notes. The
// Gocene side cannot yet execute the parsed Query trees against a
// Lucene-emitted segment (SegmentReader core-readers gap + the partial
// queryparser surface), so the L->G->L leg is structurally blocked.
func TestQueryparserHits_RoundTrip(t *testing.T) {
	const auditGap = "No binary artefacts; behavioural parity tested only via Gocene-internal cases."
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioQueryparserTreesAndHits, seed)
			rows := readQPHitsTSV(t, dir)
			seenPair := make(map[string]bool)
			for _, r := range rows {
				pairKey := r.parserID + "/" + r.queryID
				if seenPair[pairKey] {
					continue
				}
				seenPair[pairKey] = true
				pid := r.parserID
				qid := r.queryID
				t.Run(pairKey, func(t *testing.T) {
					t.Fatalf("deferred: Gocene round-trip for parser=%q query=%q at "+
						"seed=%d is blocked on (1) the partial Gocene queryparser "+
						"port (audit column 6: "+
						"'partial:queryparser/query_parser_compatibility_test.go') "+
						"and (2) the SegmentReader core-readers gap that prevents "+
						"Gocene from executing Lucene-emitted segments. Audit "+
						"gap_notes (verbatim): %q",
						pid, qid, seed, auditGap)
				})
			}
		})
	}
}
