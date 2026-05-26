// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

const scenarioS5 = "combined-suggester-fst"

// TestS5_SuggesterFst generates the AnalyzingSuggester FST + the
// per-prefix TSV, asserts the FST blob is non-empty, asserts the TSV has
// the expected 5 prefixes each with at least one suggestion that begins
// with the prefix, and re-runs `verify` to confirm load+lookup round-trip.
//
// Class-(c) Gocene replay deferred — see deferred_combined_compat_test.go.
func TestS5_SuggesterFst(t *testing.T) {
	const expectedPrefixCount = 5
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := generate(t, scenarioS5, seed)
			fst, err := os.ReadFile(dir + "/s5-completion.fst")
			if err != nil {
				t.Fatalf("read s5-completion.fst: %v", err)
			}
			if len(fst) < 32 {
				t.Fatalf("s5-completion.fst suspiciously small (%d bytes)", len(fst))
			}
			tsv := dir + "/s5-suggestions.tsv"
			mustHaveTSV(t, tsv)
			rows := readTSV(t, tsv)
			if len(rows) == 0 {
				t.Fatalf("s5-suggestions.tsv has 0 rows")
			}
			prefixes := map[string]int{}
			for i, r := range rows {
				if len(r) != 3 {
					t.Fatalf("row %d: expected 3 cols, got %d: %v", i, len(r), r)
				}
				prefix, rankStr, sug := r[0], r[1], r[2]
				rank, err := strconv.Atoi(rankStr)
				if err != nil {
					t.Fatalf("row %d: bad rank %q: %v", i, rankStr, err)
				}
				if !strings.HasPrefix(sug, prefix) {
					t.Errorf("row %d: suggestion %q does not start with prefix %q",
						i, sug, prefix)
				}
				if prev, ok := prefixes[prefix]; ok && rank != prev+1 {
					t.Fatalf("row %d (%s): rank %d not monotonic (prev=%d)",
						i, prefix, rank, prev)
				}
				prefixes[prefix] = rank
			}
			if len(prefixes) != expectedPrefixCount {
				t.Fatalf("expected %d distinct prefixes, got %d: %v",
					expectedPrefixCount, len(prefixes), prefixes)
			}
			stdout, stderr, err := runHarness(t, "verify", scenarioS5,
				formatSeed(seed), dir)
			assertOK(t, stdout, stderr, "ok scenario="+scenarioS5, err)
		})
	}
}
