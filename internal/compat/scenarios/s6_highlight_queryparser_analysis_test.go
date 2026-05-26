// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"strconv"
	"strings"
	"testing"
)

const scenarioS6 = "combined-highlight-queryparser-analysis"

// expectedS6Queries mirrors the QUERY_TEXTS list defined Java-side in
// CombinedHighlightQueryparserAnalysisScenario.
var expectedS6Queries = []string{
	"alpha AND gamma",
	"\"alpha beta\"",
	"epsilon OR zeta",
}

// TestS6_HighlightQueryparserAnalysis generates the end-to-end
// QueryParser → StandardAnalyzer → UnifiedHighlighter chain output,
// asserts s6-highlights.tsv has the expected (query_text, doc_id,
// snippet_index, snippet) shape, every query produced at least one
// snippet, and re-runs `verify`.
//
// Class-(c) Gocene replay deferred — see deferred_combined_compat_test.go.
func TestS6_HighlightQueryparserAnalysis(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := generate(t, scenarioS6, seed)
			tsv := dir + "/s6-highlights.tsv"
			mustHaveTSV(t, tsv)
			rows := readTSV(t, tsv)
			if len(rows) == 0 {
				t.Fatalf("s6-highlights.tsv has 0 rows")
			}
			perQuery := map[string]int{}
			for i, r := range rows {
				if len(r) != 4 {
					t.Fatalf("row %d: expected 4 cols, got %d: %v", i, len(r), r)
				}
				qtext, docID, snipIdxStr, snippet := r[0], r[1], r[2], r[3]
				// Reverse the TSV escape so we can match the expected
				// raw query strings.
				qtext = tsvUnescape(qtext)
				if _, err := strconv.Atoi(snipIdxStr); err != nil {
					t.Fatalf("row %d: bad snippet_index %q: %v", i, snipIdxStr, err)
				}
				if docID == "" || snippet == "" {
					t.Fatalf("row %d: empty doc_id or snippet: %v", i, r)
				}
				perQuery[qtext]++
			}
			for _, q := range expectedS6Queries {
				if perQuery[q] == 0 {
					t.Errorf("query %q produced 0 snippets", q)
				}
			}
			stdout, stderr, err := runHarness(t, "verify", scenarioS6,
				formatSeed(seed), dir)
			assertOK(t, stdout, stderr, "ok scenario="+scenarioS6, err)
		})
	}
}

// tsvUnescape mirrors the Java TsvEscape.unescape helper (\\\\, \\t, \\n, \\r).
// Used only to compare the recorded query_text column against the raw
// query strings.
func tsvUnescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case '\\':
				b.WriteByte('\\')
			case 't':
				b.WriteByte('\t')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}
