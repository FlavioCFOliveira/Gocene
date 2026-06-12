// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package backward_codecs is the Sprint 114 T26 (rmp 4634) binary-
// compatibility harness for Gocene's backward_codecs/ surface against
// artefacts produced by Apache Lucene 10.4.0.
//
// Audit rows addressed (verbatim from docs/compat-coverage.tsv, column 1
// == "backward_codecs"):
//
//	"Lucene70 SegmentInfoFormat (.si v7)"
//	    lucene_class: org.apache.lucene.backward_codecs.lucene70.Lucene70SegmentInfoFormat
//	    gap_notes:    "No real Lucene 7 fixture committed; rw tests are self-emitted."
//	    -> lucene70_si_compat_test.go                  (REAL — Gocene write, Java read via CheckIndex)
//
//	"Lucene90 HNSW vectors (v0)"
//	    lucene_class: org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsFormat
//	    gap_notes:    "No Lucene-9 fixture committed."
//	    -> lucene90_hnsw_v0_compat_test.go             (REAL — Gocene write, Java read via CheckIndex)
//
//	"Lucene99 PostingsFormat (older skip variant)"
//	    lucene_class: org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat
//	    gap_notes:    "No Lucene-emitted .doc/.pos fixture for v99."
//	    -> lucene99_postings_compat_test.go            (REAL — Gocene write, Java read via CheckIndex)
//
//	"Lucene99 ScalarQuantized vectors"
//	    lucene_class: org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsFormat
//	    gap_notes:    "No Lucene fixture."
//	    -> lucene99_scalar_quantized_compat_test.go    (REAL — Gocene write, Java read via CheckIndex)
//
//	"Lucene103 PostingsFormat (older variant)"
//	    lucene_class: org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsFormat
//	    gap_notes:    "No Lucene-emitted v103 corpus."
//	    -> lucene103_postings_compat_test.go           (REAL — Gocene write, Java read via CheckIndex)
//
//	"Lucene40 BlockTree"
//	    lucene_class: org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsReader
//	    gap_notes:    "Only reader port; no rw or fixture test."
//	    -> lucene40_blocktree_compat_test.go           (BLOCKER — writer ported but cross-engine blocked on missing Lucene84/Lucene50 postings writer)
//
//	"Legacy Packed64 / Packed64SingleBlock"
//	    lucene_class: org.apache.lucene.backward_codecs.packed.LegacyPacked64
//	    gap_notes:    "No Lucene fixture; covered by self-roundtrip only."
//	    -> packed64_legacy_compat_test.go              (REAL scenario "bwc-packed64-legacy")
//
//	"Legacy big-endian store wrappers"
//	    lucene_class: org.apache.lucene.backward_codecs.store.EndiannessReverserUtil
//	    gap_notes:    "No fixture from an old big-endian Lucene index."
//	    -> big_endian_store_compat_test.go             (REAL scenario "bwc-big-endian-store")
//
//	"Backwards-compat full index corpora (multi-version)"
//	    lucene_class: org.apache.lucene.backward_index.TestBasicBackwardsCompatibility
//	    gap_notes:    "Tests are skeletons; no actual multi-version Lucene index ZIPs committed."
//	    -> multi_version_corpora_compat_test.go        (DEFERRED — multi-version jars not in scope)
//
// The package itself carries no build tag; the per-file tests are gated
// by //go:build compat so the production module never picks up a runtime
// dependency on the Java harness jar.
//
// Compatibility-mandate carve-out (read-only formats): the seven deferred
// rows above name Lucene 10.4.0 classes whose write paths throw
// {@code UnsupportedOperationException} (e.g. {@code
// org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat
// #fieldsConsumer}) or that ship without any writer class at all (e.g.
// {@code lucene40/blocktree} which contains only readers). Producing
// fixtures for those formats requires building older Apache Lucene major
// branches (7.x / 9.x / 10.3.x), which is outside the binary-compatibility
// mandate's 10.4.0 reference pin (CLAUDE.md §"Binary Compatibility
// Mandate"). Each deferred scenario lives in tools/lucene-fixtures/
// Manifest.DEFERRED_ROWS and carries the verbatim audit gap_notes plus
// the read-only justification.
package backward_codecs

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by rmp 4634 acceptance
// criterion #2: every NEW scenario MUST be byte-deterministic at both
// seeds. Tests iterate it in t.Run subtests.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 T7+ second canary (decimal 912559).
}

// Scenario names registered by the Java harness for Sprint 114 T26. Kept
// as constants so the audit-row -> scenario mapping is explicit and the
// kebab-case string is spelled exactly once.
const (
	// Real scenarios (Lucene 10.4.0 lucene-backward-codecs ships writers).
	ScenarioBwcPacked64Legacy = "bwc-packed64-legacy"
	ScenarioBwcBigEndianStore = "bwc-big-endian-store"

	// Deferred scenario names — registered in Manifest.DEFERRED_ROWS, NOT
	// in Scenarios.REGISTRY. Recorded here so the audit-row -> name
	// mapping stays explicit for downstream tooling.
	ScenarioBwcLucene70Si              = "bwc-lucene70-si"
	ScenarioBwcLucene90HnswV0          = "bwc-lucene90-hnsw-v0"
	ScenarioBwcLucene99Postings        = "bwc-lucene99-postings"
	ScenarioBwcLucene99ScalarQuantized = "bwc-lucene99-scalar-quantized"
	ScenarioBwcLucene103Postings       = "bwc-lucene103-postings"
	ScenarioBwcLucene40Blocktree       = "bwc-lucene40-blocktree"
	ScenarioBwcMultiVersionCorpora     = "bwc-multi-version-corpora"

	// File names emitted by the Java scenarios.
	fileBwcPacked64Legacy = "bwc-packed64-legacy.dat"
	fileBwcBigEndianStore = "bwc-big-endian-store.dat"
)

// requireHarness skips the test when the Java fixture harness jar is not
// reachable. Mirrors internal/compat/{codecs,facets,suggest,...}.requireHarness.
func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := gcompat.Locate(); err != nil {
		if errors.Is(err, gcompat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

// generate runs the harness `gen` subcommand into a fresh t.TempDir() and
// returns the resulting directory path.
func generate(t *testing.T, scenario string, seed int64) string {
	t.Helper()
	requireHarness(t)
	dir := t.TempDir()
	if err := gcompat.GenerateInto(scenario, seed, dir); err != nil {
		t.Fatalf("harness gen %s seed=%d: %v", scenario, seed, err)
	}
	return dir
}

// verifyHarness invokes the Java verifier against an existing fixture
// directory. A clean exit (code 0) proves the scenario contract holds.
func verifyHarness(t *testing.T, scenario string, seed int64, dir string) {
	t.Helper()
	if err := gcompat.Verify(scenario, seed, dir); err != nil {
		t.Fatalf("harness verify %s seed=%d dir=%s: %v", scenario, seed, dir, err)
	}
}

// fileMapRecursive reads every regular file under dir (recursive) into a
// map keyed by the slash-separated relative path. The .si exclusion
// mirrors Manifest.includeForHash on the Java side: Lucene stamps a
// wall-clock value into the .si diagnostics map and must not contaminate
// determinism checks. The write.lock file is empty and unrelated to
// format compatibility.
func fileMapRecursive(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte, 16)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".si") || name == "write.lock" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = b
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return out
}

// assertDigestStable runs the scenario twice at the same seed into two
// fresh tempdirs and asserts every non-.si file is byte-identical. Go
// mirror of ScenarioDeterminismTest.
func assertDigestStable(t *testing.T, scenario string, seed int64) {
	t.Helper()
	a := generate(t, scenario, seed)
	b := generate(t, scenario, seed)
	ma := fileMapRecursive(t, a)
	mb := fileMapRecursive(t, b)
	if len(ma) != len(mb) {
		t.Fatalf("file count mismatch for %s seed=%d: A=%d B=%d",
			scenario, seed, len(ma), len(mb))
	}
	for name, ba := range ma {
		bb, ok := mb[name]
		if !ok {
			t.Errorf("%s seed=%d: file %q present in A but missing from B",
				scenario, seed, name)
			continue
		}
		if !bytes.Equal(ba, bb) {
			t.Errorf("%s seed=%d: file %q content drift between two runs",
				scenario, seed, name)
		}
	}
}

// hasFile returns true if dir (recursive) contains a regular file with
// the given exact name (recursive over subdirectories).
func hasFile(t *testing.T, dir, name string) bool {
	t.Helper()
	found := false
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if info.Name() == name {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return found
}
