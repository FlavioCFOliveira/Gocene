// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// fast_vector_highlight_compat_test.go addresses the highlight audit
// row (verbatim from docs/compat-coverage.tsv, row 64):
//
//	highlight\tFastVectorHighlighter phrase list\t
//	  org.apache.lucene.search.vectorhighlight.FastVectorHighlighter\t
//	  highlight/vectorhighlight/\t
//	  partial:highlight/vectorhighlight/index_time_synonym_test.go\t
//	  no\tno\t
//	  No Lucene fixture for vector-highlight inputs.
//
// Driven by "fast-vector-highlight-phrases": 14 docs indexed with
// positions+offsets+term-vectors, 3 phrase queries
// ("alpha beta", "gamma delta", "epsilon zeta"). The Java side reaches
// into FieldTermStack+FieldPhraseList so per-phrase start/end offsets
// land directly in fvh-phrases.tsv.
package highlight

import (
	"bytes"
	"strings"
	"testing"
)

// expectedFvhQueryIDs MUST match FastVectorHighlightPhrasesScenario.QUERY_IDS.
var expectedFvhQueryIDs = []string{
	"ph-alpha-beta", "ph-gamma-delta", "ph-epsilon-zeta",
}

// TestFastVectorHighlight_ReadFixture (class a): every query_id
// appears, every doc_id matches doc-<i>, offsets are sane, and per-
// (query, doc) phrase indices form a contiguous [0, n) sequence.
func TestFastVectorHighlight_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFvhPhrases, seed)
			rows := readFvhPhrasesTSV(t, dir)
			if len(rows) == 0 {
				t.Fatalf("fvh-phrases.tsv empty (no phrases returned by FVH?)")
			}
			seen := make(map[string]int, len(expectedFvhQueryIDs))
			type key struct{ q, d string }
			perGroup := make(map[key][]int)
			for _, r := range rows {
				if !strings.HasPrefix(r.docID, "doc-") {
					t.Errorf("doc_id %q does not match doc-<i>", r.docID)
				}
				if r.startOffset < 0 || r.endOffset <= r.startOffset {
					t.Errorf("q=%s doc=%s idx=%d: invalid offsets [%d,%d)",
						r.queryID, r.docID, r.phraseIndex, r.startOffset, r.endOffset)
				}
				if r.phraseText == "" {
					t.Errorf("q=%s doc=%s idx=%d: empty phrase_text",
						r.queryID, r.docID, r.phraseIndex)
				}
				seen[r.queryID]++
				k := key{r.queryID, r.docID}
				perGroup[k] = append(perGroup[k], r.phraseIndex)
			}
			for _, want := range expectedFvhQueryIDs {
				if seen[want] == 0 {
					t.Errorf("query_id %q absent (catalogue drift?)", want)
				}
			}
			for k, idxs := range perGroup {
				for i, got := range idxs {
					if got != i {
						t.Errorf("q=%s doc=%s: phrase_index[%d] = %d, want %d",
							k.q, k.d, i, got, i)
					}
				}
			}
			for i := 1; i < len(rows); i++ {
				a, b := rows[i-1], rows[i]
				if a.queryID > b.queryID {
					t.Errorf("row %d: query_id not sorted: %q after %q", i, b.queryID, a.queryID)
				} else if a.queryID == b.queryID && a.docID > b.docID {
					t.Errorf("row %d (q=%s): doc_id not sorted: %q after %q",
						i, a.queryID, b.docID, a.docID)
				} else if a.queryID == b.queryID && a.docID == b.docID && a.phraseIndex >= b.phraseIndex {
					t.Errorf("row %d (q=%s doc=%s): phrase_index not ascending: %d -> %d",
						i, a.queryID, a.docID, a.phraseIndex, b.phraseIndex)
				}
			}
		})
	}
}

// TestFastVectorHighlight_ByteDeterminism (class b, part 1): two runs
// at the same seed produce byte-identical fvh-phrases.tsv.
func TestFastVectorHighlight_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioFvhPhrases, seed)
			b := generate(t, ScenarioFvhPhrases, seed)
			ab := readFileBytes(t, a, tsvFvhPhrases)
			bb := readFileBytes(t, b, tsvFvhPhrases)
			if !bytes.Equal(ab, bb) {
				t.Fatalf("fvh-phrases.tsv drift at seed=%d:\n A=%q\n B=%q", seed, ab, bb)
			}
		})
	}
}

// TestFastVectorHighlight_VerifySubcommand (class b, part 2): the new
// `verify-fvh-phrases <dir>` re-runs FVH and re-asserts every row.
func TestFastVectorHighlight_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFvhPhrases, seed)
			out, err := runHarness(t, "verify-fvh-phrases", dir)
			if err != nil {
				t.Fatalf("verify-fvh-phrases failed: %v\nstdout:\n%s", err, out)
			}
			if !strings.Contains(out, "ok verify-fvh-phrases") {
				t.Errorf("expected 'ok verify-fvh-phrases' in stdout, got: %s", out)
			}
		})
	}
}
