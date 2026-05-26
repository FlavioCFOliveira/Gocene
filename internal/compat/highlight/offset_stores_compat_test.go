// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// offset_stores_compat_test.go addresses the highlight audit row
// (verbatim from docs/compat-coverage.tsv, row 62):
//
//	highlight\tTerm-vector offset stores consumed\t
//	  org.apache.lucene.search.highlight.TermVectorLeafReader\t
//	  highlight/term_vector_leaf_reader.go\t
//	  partial:highlight/highlighter_test.go\t
//	  partial:search/highlighting_compatibility_test.go\tno\t
//	  No fixture proves offsets match Lucene; consumes term vectors
//	  but no end-to-end interop.
//
// Driven by the existing "term-vectors-format" scenario (Sprint 114 T3)
// which ships a Lucene-emitted .tvx/.tvd/.tvm triplet under
// Lucene104Codec — the byte surface TermVectorLeafReader must round-
// trip without loss.
package highlight

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// termVectorFiles is the Lucene90TermVectorsFormat triplet. Offsets
// live inline in .tvd; .tvx anchors per-doc lookup; .tvm carries meta.
var termVectorFiles = []string{".tvx", ".tvd", ".tvm"}

// findSegmentFile returns the first dir entry ending in suffix.
func findSegmentFile(t *testing.T, dir, suffix string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == suffix {
			return filepath.Join(dir, e.Name())
		}
	}
	t.Fatalf("no *%s file in %s", suffix, dir)
	return ""
}

// TestOffsetStores_ReadFixture (class a): the triplet exists with
// non-empty bytes — the minimum precondition for offset-store parity.
func TestOffsetStores_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioTermVectors, seed)
			for _, suf := range termVectorFiles {
				path := findSegmentFile(t, dir, suf)
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("stat %s: %v", path, err)
				}
				if info.Size() == 0 {
					t.Errorf("term-vector file %s at seed=%d is empty", path, seed)
				}
			}
		})
	}
}

// TestOffsetStores_ByteDeterminism (class b): two runs at the same seed
// yield byte-identical .tvx/.tvd/.tvm files. Any drift would imply
// encoder non-determinism and silently break TermVectorLeafReader.
func TestOffsetStores_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioTermVectors, seed)
			b := generate(t, ScenarioTermVectors, seed)
			for _, suf := range termVectorFiles {
				ab, err := os.ReadFile(findSegmentFile(t, a, suf))
				if err != nil {
					t.Fatalf("read a/*%s: %v", suf, err)
				}
				bb, err := os.ReadFile(findSegmentFile(t, b, suf))
				if err != nil {
					t.Fatalf("read b/*%s: %v", suf, err)
				}
				if !bytes.Equal(ab, bb) {
					t.Fatalf("*%s drift between two runs at seed=%d (len A=%d B=%d)",
						suf, seed, len(ab), len(bb))
				}
			}
		})
	}
}

// TestOffsetStores_CheckIndex (class c, partial): `check <dir>` re-
// decodes the triplet under Lucene's eyes. Full Gocene-read round-trip
// is deferred behind the SegmentReader core-readers gap (see
// deferred_highlight_compat_test.go).
func TestOffsetStores_CheckIndex(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioTermVectors, seed)
			out, err := runHarness(t, "check", dir)
			if err != nil {
				t.Fatalf("check failed: %v\nstdout:\n%s", err, out)
			}
		})
	}
}
