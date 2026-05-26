// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"bytes"
	"os"
	"strconv"
	"testing"
)

const scenarioS2 = "combined-reverse-index-search"

// TestS2_ReverseIndexSearch generates the single-segment reference index
// over the same deterministic doc set as S1, asserts s2-hits.tsv exists,
// re-runs `verify`, and asserts the recorded hit-rows are byte-identical
// to s1-hits.tsv at the same seed (per rmp 4611 acceptance criterion #3:
// "both produced Lucene-side so they should match").
//
// The Gocene-write leg (Gocene produces the index from the same doc set,
// Lucene reads + verifies) is the deferred audit gap captured in
// deferred_combined_compat_test.go.
func TestS2_ReverseIndexSearch(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			// Generate BOTH scenarios at the same seed; compare the TSVs.
			dirS1 := generate(t, scenarioS1, seed)
			dirS2 := generate(t, scenarioS2, seed)
			tsvS1, err := os.ReadFile(dirS1 + "/s1-hits.tsv")
			if err != nil {
				t.Fatalf("read s1-hits.tsv: %v", err)
			}
			tsvS2, err := os.ReadFile(dirS2 + "/s2-hits.tsv")
			if err != nil {
				t.Fatalf("read s2-hits.tsv: %v", err)
			}
			// The first comment line carries the file name; strip it
			// (header line begins with '#') for the comparison, then
			// compare the data rows verbatim.
			s1Body := stripFirstHeader(tsvS1)
			s2Body := stripFirstHeader(tsvS2)
			if !bytes.Equal(s1Body, s2Body) {
				// Find offset of first divergence for diagnostic output.
				min := len(s1Body)
				if len(s2Body) < min {
					min = len(s2Body)
				}
				var off int
				for off = 0; off < min && s1Body[off] == s2Body[off]; off++ {
				}
				t.Fatalf("s1-hits.tsv and s2-hits.tsv diverge at byte offset %d "+
					"(s1 len=%d s2 len=%d). S1[0..200]=%q S2[0..200]=%q",
					off, len(s1Body), len(s2Body),
					previewBytes(s1Body, 200), previewBytes(s2Body, 200))
			}
			// Re-run the Lucene verifier on S2 to confirm round-trip.
			stdout, stderr, err := runHarness(t, "verify", scenarioS2,
				formatSeed(seed), dirS2)
			assertOK(t, stdout, stderr, "ok scenario="+scenarioS2, err)
		})
	}
}

// stripFirstHeader removes the leading "# ..." comment line if present.
// Both S1 and S2 emit a single header that differs only in TSV name; the
// data rows are what AC #3 pins as byte-identical.
func stripFirstHeader(b []byte) []byte {
	if len(b) == 0 || b[0] != '#' {
		return b
	}
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' {
			return b[i+1:]
		}
	}
	return nil
}

// previewBytes returns up to n bytes of b. Used by error messages only.
func previewBytes(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
