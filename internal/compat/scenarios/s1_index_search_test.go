// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"strconv"
	"testing"
)

// scenarioS1 is the kebab-case name registered in the Java Scenarios map.
const scenarioS1 = "combined-multi-segment-index-search"

// TestS1_IndexSearch generates the multi-segment Lucene index, asserts
// s1-hits.tsv has the expected (query_id, rank, doc_id, score) shape,
// asserts rank is monotonic per query_id, and re-invokes the harness
// `verify` subcommand to confirm the Lucene-side re-scoring matches.
//
// Class-(c) Gocene-side replay is covered in deferred_combined_compat_test.go.
func TestS1_IndexSearch(t *testing.T) {
	const expectedQueries = 8 // 5 TermQuery + 2 PhraseQuery + 1 BooleanQuery
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := generate(t, scenarioS1, seed)
			tsv := dir + "/s1-hits.tsv"
			mustHaveTSV(t, tsv)
			rows := readTSV(t, tsv)
			if len(rows) == 0 {
				t.Fatalf("s1-hits.tsv has 0 data rows")
			}
			seenQueries := map[string]int{}
			for i, r := range rows {
				if len(r) != 4 {
					t.Fatalf("row %d: expected 4 cols, got %d: %v", i, len(r), r)
				}
				qid, rankStr, docID, scoreStr := r[0], r[1], r[2], r[3]
				rank, err := strconv.Atoi(rankStr)
				if err != nil {
					t.Fatalf("row %d: bad rank %q: %v", i, rankStr, err)
				}
				if prev, ok := seenQueries[qid]; ok && rank != prev+1 {
					t.Fatalf("row %d (%s): rank %d not monotonic (prev=%d)",
						i, qid, rank, prev)
				}
				seenQueries[qid] = rank
				if docID == "" || scoreStr == "" {
					t.Fatalf("row %d: empty doc_id or score: %v", i, r)
				}
				if _, err := strconv.ParseFloat(scoreStr, 64); err != nil {
					t.Fatalf("row %d: bad score %q: %v", i, scoreStr, err)
				}
			}
			if len(seenQueries) != expectedQueries {
				t.Fatalf("expected %d distinct query_ids, saw %d: %v",
					expectedQueries, len(seenQueries), seenQueries)
			}
			// Lucene-side re-verify: confirms BM25 scoring is deterministic
			// and the on-disk index still resolves to the same hits.
			stdout, stderr, err := runHarness(t, "verify", scenarioS1,
				formatSeed(seed), dir)
			assertOK(t, stdout, stderr, "ok scenario="+scenarioS1, err)
		})
	}
}
