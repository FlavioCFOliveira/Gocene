// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"strconv"
	"testing"
)

const scenarioS3 = "combined-facets-search"

// TestS3_FacetsSearch generates the faceted Lucene index + taxonomy
// sidecar, asserts s3-facet-counts.tsv (dim, label, count) is well-formed,
// counts sum to a positive number per dim, and re-runs the verifier.
//
// Class-(c) Gocene replay deferred — see deferred_combined_compat_test.go.
func TestS3_FacetsSearch(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := generate(t, scenarioS3, seed)
			tsv := dir + "/s3-facet-counts.tsv"
			mustHaveTSV(t, tsv)
			rows := readTSV(t, tsv)
			if len(rows) == 0 {
				t.Fatalf("s3-facet-counts.tsv has 0 rows")
			}
			perDim := map[string]int{}
			for i, r := range rows {
				if len(r) != 3 {
					t.Fatalf("row %d: expected 3 cols, got %d: %v", i, len(r), r)
				}
				dim, label, countStr := r[0], r[1], r[2]
				if dim == "" || label == "" {
					t.Fatalf("row %d: empty dim/label: %v", i, r)
				}
				c, err := strconv.Atoi(countStr)
				if err != nil || c <= 0 {
					t.Fatalf("row %d: bad count %q: %v", i, countStr, err)
				}
				perDim[dim] += c
			}
			// Per the Java scenario each of the 16 docs contributes
			// to BOTH dims; the per-dim total over a MatchAll-equivalent
			// drill-down query must therefore be 16.
			for dim, total := range perDim {
				if total != 16 {
					t.Errorf("dim %q: expected total count 16, got %d", dim, total)
				}
			}
			stdout, stderr, err := runHarness(t, "verify", scenarioS3,
				formatSeed(seed), dir)
			assertOK(t, stdout, stderr, "ok scenario="+scenarioS3, err)
		})
	}
}
