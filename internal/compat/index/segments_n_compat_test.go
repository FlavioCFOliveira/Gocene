// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// segments_n_compat_test.go covers parsing of the segments_N commit
// pointer emitted by Apache Lucene 10.4.0. The same parser path is the
// entry point for every DirectoryReader open: any incompatibility here
// fails every downstream read.
//
// Audit row cited (docs/compat-coverage.tsv, package == "index"):
//
//	"SegmentInfos / segments_N" — gap_notes:
//	  "No isolated test loads a Lucene-emitted segments_N file and
//	   asserts the parsed fields match the Lucene-side contract."
//
// Three classes per file:
//
//	(a) Lucene-write → Gocene-parse — every canary scenario produces a
//	    segments_N that Gocene's ReadSegmentInfos accepts.
//	(b) Byte-determinism — re-running the same scenario at the same
//	    seed produces a segments_N at the same generation number with
//	    the same parsed shape.
//	(c) Generation arithmetic — the new index-deletions-and-dv-updates
//	    scenario commits three times, so the canonical segments_N must
//	    be named "segments_3" (base-36 generation 3) and ReadSegmentInfos
//	    must report Generation() == 3.
package index

import (
	"testing"

	gindex "github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestSegmentsN_LuceneEmittedParses (class a) confirms Gocene's
// ReadSegmentInfos accepts every Lucene-emitted commit pointer in the
// foundational corpus at both canary seeds.
func TestSegmentsN_LuceneEmittedParses(t *testing.T) {
	scenarios := []string{
		ScenarioPostings,
		ScenarioSegmentInfo,
		ScenarioLiveDocs,
		ScenarioDeletionsAndDvUpdates,
	}
	for _, scenario := range scenarios {
		scenario := scenario
		t.Run(scenario, func(t *testing.T) {
			for _, seed := range canarySeeds {
				seed := seed
				t.Run("", func(t *testing.T) {
					dir := generate(t, scenario, seed)
					si := openSegmentInfos(t, dir)
					if si.Size() != 1 {
						t.Fatalf("%s: expected 1 segment, got %d", scenario, si.Size())
					}
					if si.LuceneVersion() != "10.4.0" {
						t.Errorf("%s: lucene version = %q, want 10.4.0",
							scenario, si.LuceneVersion())
					}
					if si.Generation() < 1 {
						t.Errorf("%s: generation = %d, want >= 1",
							scenario, si.Generation())
					}
				})
			}
		})
	}
}

// TestSegmentsN_DeletionsScenarioGeneration (class c) pins the contract
// of the new index-deletions-and-dv-updates scenario: three commits
// produce a segments_3 file whose parsed Generation() is 3, and
// ReadSegmentInfos returns the LuceneVersion stamped by Lucene 10.4.0.
func TestSegmentsN_DeletionsScenarioGeneration(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			name := findSegmentsFile(t, dir)
			if name != "segments_3" {
				t.Fatalf("commit pointer = %q, want segments_3 (3 commits)", name)
			}
			if gen := parseGeneration(t, name); gen != 3 {
				t.Fatalf("parsed generation = %d, want 3", gen)
			}
			si := openSegmentInfos(t, dir)
			if si.Generation() != 3 {
				t.Errorf("ReadSegmentInfos generation = %d, want 3", si.Generation())
			}
			if si.LuceneVersion() != "10.4.0" {
				t.Errorf("lucene version = %q, want 10.4.0", si.LuceneVersion())
			}
			// Counter is per-segment-name allocator; with NoMergePolicy
			// and a single segment created at phase 1, it must be 1.
			if si.Counter() != 1 {
				t.Errorf("counter = %d, want 1", si.Counter())
			}
		})
	}
}

// TestSegmentsN_ByteDeterminism (class b) reproduces the same scenario
// at the same seed twice and asserts the parsed structural shape stays
// stable. The full byte-determinism gate lives in ScenarioDeterminismTest
// on the Java side; this is the symmetric Go-side check that the parser
// observes no drift.
func TestSegmentsN_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			b := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			siA := openSegmentInfos(t, a)
			siB := openSegmentInfos(t, b)
			if siA.Generation() != siB.Generation() {
				t.Fatalf("generation drift: A=%d B=%d",
					siA.Generation(), siB.Generation())
			}
			if siA.Size() != siB.Size() {
				t.Fatalf("size drift: A=%d B=%d", siA.Size(), siB.Size())
			}
			if siA.LuceneVersion() != siB.LuceneVersion() {
				t.Fatalf("luceneVersion drift: A=%q B=%q",
					siA.LuceneVersion(), siB.LuceneVersion())
			}
			if siA.Counter() != siB.Counter() {
				t.Fatalf("counter drift: A=%d B=%d", siA.Counter(), siB.Counter())
			}
		})
	}
}

// openSegmentInfos is the shared SegmentInfos opener used by every
// segments_N-touching test in this package. It returns the parsed
// SegmentInfos and t.Fatalf's on any error.
func openSegmentInfos(t *testing.T, dir string) *gindex.SegmentInfos {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = d.Close() })
	si, err := gindex.ReadSegmentInfos(d)
	if err != nil {
		t.Fatalf("ReadSegmentInfos(%s): %v", dir, err)
	}
	return si
}
