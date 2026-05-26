// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// unified_highlighter_compat_test.go addresses the highlight audit row
// (verbatim from docs/compat-coverage.tsv, row 63):
//
//	highlight\tUnifiedHighlighter offset retrieval\t
//	  org.apache.lucene.search.uhighlight.UnifiedHighlighter\t
//	  highlight/uhighlight/\t
//	  partial:highlight/uhighlight/unified_highlighter_test_base_test.go\t
//	  no\tno\t
//	  No Lucene-side parity test for offset retrieval.
//
// Driven by the "highlight-offset-corpus" scenario: 16 docs, 4 queries
// (3 term queries + 1 boolean disjunction), body indexed with
// DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS. UH renders snippets straight
// from the postings offset store; verify-highlight-offsets re-asserts.
package highlight

import (
	"bytes"
	"strings"
	"testing"
)

// expectedUHQueryIDs MUST match HighlightOffsetCorpusScenario.QUERY_IDS.
var expectedUHQueryIDs = []string{
	"term-alpha", "term-gamma", "term-epsilon", "bool-alpha-or-zeta",
}

// TestUnifiedHighlighter_ReadFixture (class a): every query_id appears,
// every doc_id matches doc-<i>, every snippet carries <b>..</b> markup,
// rows are sorted by (query_id, doc_id, snippet_index).
func TestUnifiedHighlighter_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioHighlightOffsetCorpus, seed)
			rows := readHighlightsTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("highlights.tsv empty (no snippets returned by UH?)")
			}
			seen := make(map[string]int, len(expectedUHQueryIDs))
			for _, r := range rows {
				if !strings.HasPrefix(r.docID, "doc-") {
					t.Errorf("doc_id %q does not match doc-<i> pattern", r.docID)
				}
				if !strings.Contains(r.snippetText, "<b>") || !strings.Contains(r.snippetText, "</b>") {
					t.Errorf("query=%s doc=%s: snippet missing <b>..</b>: %q",
						r.queryID, r.docID, r.snippetText)
				}
				seen[r.queryID]++
			}
			for _, want := range expectedUHQueryIDs {
				if seen[want] == 0 {
					t.Errorf("query_id %q absent (catalogue drift?)", want)
				}
			}
			for i := 1; i < len(rows); i++ {
				a, b := rows[i-1], rows[i]
				if a.queryID > b.queryID {
					t.Errorf("row %d: query_id not sorted: %q after %q", i, b.queryID, a.queryID)
				} else if a.queryID == b.queryID && a.docID > b.docID {
					t.Errorf("row %d (q=%s): doc_id not sorted: %q after %q",
						i, a.queryID, b.docID, a.docID)
				}
			}
		})
	}
}

// TestUnifiedHighlighter_ByteDeterminism (class b, part 1): two runs at
// the same seed produce byte-identical highlights.tsv.
func TestUnifiedHighlighter_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioHighlightOffsetCorpus, seed)
			b := generate(t, ScenarioHighlightOffsetCorpus, seed)
			ab := readFileBytes(t, a, tsvHighlights)
			bb := readFileBytes(t, b, tsvHighlights)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("highlights.tsv drift at seed=%d:\n A=%q\n B=%q", seed, ab, bb)
			}
		})
	}
}

// TestUnifiedHighlighter_VerifySubcommand (class b, part 2): the new
// `verify-highlight-offsets <dir>` re-runs UH and re-asserts every row.
// Symmetric hook a future Gocene UH port will re-use for parity.
func TestUnifiedHighlighter_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioHighlightOffsetCorpus, seed)
			out, err := runHarness(t, "verify-highlight-offsets", dir)
			if err != nil {
				t.Fatalf("verify-highlight-offsets failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-highlight-offsets") {
				t.Errorf("expected 'ok verify-highlight-offsets' in stdout, got: %s", out)
			}
		})
	}
}
