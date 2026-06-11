// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package facets is the Sprint 114 T12 (rmp 4620) binary-compatibility
// harness for Gocene's facets/ package against artefacts produced by
// Apache Lucene 10.4.0.
//
// Audit rows addressed (cited verbatim from docs/compat-coverage.tsv,
// column 1 == "facets"):
//
//	"Taxonomy directory index files"
//	    lucene_class:
//	        org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyWriter
//	    gap_notes: "No fixture from Lucene-emitted taxonomy directory."
//	    -> taxonomy_directory_compat_test.go (scenario "taxonomy-directory")
//
//	"FacetField association payload encoding"
//	    lucene_class:
//	        org.apache.lucene.facet.taxonomy.AssociationFacetField
//	    gap_notes: "No byte-level fixture for association payloads."
//	    -> association_payload_compat_test.go (scenario "facet-association-payload")
//
//	"SortedSetDocValues facet ord encoding"
//	    lucene_class:
//	        org.apache.lucene.facet.sortedset.DefaultSortedSetDocValuesReaderState
//	    gap_notes: "No Lucene-emitted sorted-set ord file consumed by tests."
//	    -> sortedset_ords_compat_test.go (scenario "facet-sortedset-ords")
//
//	"FacetSet packed-bytes encoding"
//	    lucene_class: org.apache.lucene.facet.facetset.FacetSet
//	    gap_notes: "No Lucene-produced FacetSet bytes used in tests."
//	    -> facet_set_compat_test.go (scenario "facet-set-packed-bytes")
//
// The package itself carries no build tag; the per-file tests are gated
// by //go:build compat so the production module never picks up a runtime
// dependency on the Java harness jar.
package facets

import (
	"bytes"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// canarySeeds is the two-seed sweep enforced by rmp 4620 acceptance
// criterion #2: every new scenario MUST be byte-deterministic at both
// seeds. Tests iterate it in t.Run subtests.
var canarySeeds = [...]int64{
	0xC0FFEE, // Sprint 114 baseline canary (decimal 12648430).
	0xDECAF,  // Sprint 114 T7+T8+T9+T10+T11+T12 second canary (decimal 912559).
}

// ---- Seed-derived helper functions (mirror Java scenario logic) ----

// seededMultiplier1 is the signed int64 equivalent of Java's
// 0x9E3779B97F4A7C15L (Golden Ratio hash constant).
const seededMultiplier1 int64 = -7046029258636353131

// seededMultiplier2 is the signed int64 equivalent of Java's
// 0xBF58476D1CE4E5B9L (SplitMix64 mixing constant).
const seededMultiplier2 int64 = -4658895280553007687

// seededInt returns the deterministic int value for (seed, doc) used by
// FacetAssociationPayloadScenario.seededInt.
func seededInt(seed int64, doc int) int {
	return int((seed * seededMultiplier1) ^ (int64(doc)*31 + 17))
}

// seededFloat returns the deterministic float in [1.0, 2.0) for (seed, doc)
// used by FacetAssociationPayloadScenario.seededFloat.
func seededFloat(seed int64, doc int) float32 {
	bits := int((seed * seededMultiplier2) ^ (int64(doc)*41 + 23))
	return math.Float32frombits(uint32((127 << 23) | (bits & 0x7FFFFF)))
}

// facetFieldDefaultName is the default SortedSetDocValues field name used by
// FacetsConfig for SortedSetDocValuesFacetField (matches Java
// FacetsConfig.DEFAULT_INDEX_FIELD_NAME = "$facets").
const facetFieldDefaultName = "$facets"

// facetSetFieldName is the field name for FacetSets, matching Java
// FacetSetPackedBytesScenario.FIELD = "fset".
const facetSetFieldName = "fset"

// Scenario names registered by the Java harness for Sprint 114 T12. Kept
// as constants so the audit-row -> scenario mapping is explicit and the
// kebab-case string is spelled exactly once.
const (
	ScenarioTaxonomyDirectory       = "taxonomy-directory"
	ScenarioFacetAssociationPayload = "facet-association-payload"
	ScenarioFacetSortedsetOrds      = "facet-sortedset-ords"
	ScenarioFacetSetPackedBytes     = "facet-set-packed-bytes"

	// Subdirectory holding the taxonomy sidecar (matches Java-side constants).
	taxoSubdir = "taxo"
)

// requireHarness skips the test when the Java fixture harness jar is not
// reachable. Mirrors internal/compat/{codecs,index,search,analysis,queries}.requireHarness.
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

// runHarness invokes the harness with the supplied args and returns
// stdout. Non-zero exit codes surface as Go errors with stderr attached.
func runHarness(t *testing.T, args ...string) (string, error) {
	t.Helper()
	jar, err := gcompat.Locate()
	if err != nil {
		return "", err
	}
	cmd := exec.Command("java", append([]string{"-jar", jar}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if runErr := cmd.Run(); runErr != nil {
		return stdout.String(), &harnessError{args: args, err: runErr, stderr: stderr.String()}
	}
	return stdout.String(), nil
}

// harnessError carries the failed CLI invocation context.
type harnessError struct {
	args   []string
	err    error
	stderr string
}

func (e *harnessError) Error() string {
	return "java -jar lucene-fixtures.jar " + strings.Join(e.args, " ") +
		": " + e.err.Error() + " (stderr: " + strings.TrimSpace(e.stderr) + ")"
}

func (e *harnessError) Unwrap() error { return e.err }

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

// hasFileWithSuffix returns true if dir (recursive) contains at least one
// regular file whose name ends with suffix. Used by the read-fixture
// classes to assert the on-disk format files the scenario emits.
func hasFileWithSuffix(t *testing.T, dir, suffix string) bool {
	t.Helper()
	found := false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), suffix) {
			found = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return found
}
