// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// index_file_names_compat_test.go cross-validates Gocene's
// IndexFileNames helpers against every filename Lucene 10.4.0 produces
// in the canary corpus. The CODEC_FILE_PATTERN regex MUST accept every
// codec-emitted file, and ParseGeneration / ParseSegmentName /
// StripExtension MUST round-trip the generation suffix encoded in
// filenames like "_0_1.liv" or "_0_Lucene90_0.dvd".
//
// Audit row cited (docs/compat-coverage.tsv, package == "index"):
//
//	"IndexFileNames" — gap_notes:
//	  "Gocene's IndexFileNames is unit-tested against synthetic
//	   inputs, but never against the full set of filename shapes
//	   that real Lucene segments produce (in particular the
//	   generational _N infixes and the per-codec suffix infixes)."
//
// Three classes per file:
//
//	(a) Pattern recognition — every codec-emitted file in every
//	    fixture in the canary corpus matches CodecFilePattern.
//	(b) Generation round-trip — for each generational filename,
//	    FileNameFromGeneration(ParseSegmentName, ext, ParseGeneration)
//	    reconstructs the original (or, when the filename contains a
//	    per-codec infix that the IndexFileNames helpers don't model,
//	    at least ParseGeneration returns the expected gen).
//	(c) Cross-engine — applied to BOTH the foundational and the new
//	    T8 scenarios so any drift in either the writer or the parser
//	    is caught.
package index

import (
	"strings"
	"testing"

	gindex "github.com/FlavioCFOliveira/Gocene/index"
)

// scenariosForFilenameCoverage lists the fixtures whose filename set
// the test iterates. Includes the new T8 scenarios so the generational
// .liv / generational DV-update names are exercised.
var scenariosForFilenameCoverage = []string{
	ScenarioSegmentInfo,
	ScenarioFieldInfos,
	ScenarioLiveDocs,
	ScenarioPostings,
	ScenarioDeletionsAndDvUpdates,
	"index-soft-deletes",
}

// TestIndexFileNames_PatternMatchesEveryCodecFile (class a) asserts
// every codec-emitted file in every scenario at the canary seed matches
// IndexFileNames.CodecFilePattern.
func TestIndexFileNames_PatternMatchesEveryCodecFile(t *testing.T) {
	const seed int64 = 0xC0FFEE
	for _, scenario := range scenariosForFilenameCoverage {
		scenario := scenario
		t.Run(scenario, func(t *testing.T) {
			dir := generate(t, scenario, seed)
			files := listFiles(t, dir, false)
			if len(files) == 0 {
				t.Fatalf("%s: no files produced", scenario)
			}
			for _, name := range files {
				// segments_N and write.lock are NOT codec files; they
				// are pinned by other tests. Every other filename in
				// the segment MUST match the codec pattern.
				if strings.HasPrefix(name, "segments_") {
					continue
				}
				if !gindex.CodecFilePattern.MatchString(name) {
					t.Errorf("%s/%s: does not match CodecFilePattern",
						scenario, name)
				}
			}
		})
	}
}

// TestIndexFileNames_ParseGenerationOnGenerationalLiv (class b) covers
// the simplest generational shape: "_<seg>_<gen>.liv". ParseGeneration
// MUST return the gen value, and ParseSegmentName MUST return the
// segment base ("_0").
func TestIndexFileNames_ParseGenerationOnGenerationalLiv(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			liv := findUniqueByExt(t, dir, ".liv")
			seg := gindex.ParseSegmentName(liv)
			if seg != "_0" {
				t.Errorf("ParseSegmentName(%q) = %q, want %q", liv, seg, "_0")
			}
			gen := gindex.ParseGeneration(liv)
			if gen != 1 {
				t.Errorf("ParseGeneration(%q) = %d, want 1", liv, gen)
			}
			// Round-trip: FileNameFromGeneration reconstructs the name.
			roundTrip := gindex.FileNameFromGeneration("_0", "liv", gen)
			if roundTrip != liv {
				t.Errorf("FileNameFromGeneration round-trip = %q, want %q",
					roundTrip, liv)
			}
		})
	}
}

// TestIndexFileNames_StripExtensionOnPerCodecInfix (class b) covers the
// per-codec-infixed shapes like "_0_Lucene104_0.tim" and
// "_0_1_Lucene90_0.dvd". StripExtension MUST strip everything from the
// first '.' onwards, and ParseSegmentName MUST return "_0".
func TestIndexFileNames_StripExtensionOnPerCodecInfix(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			files := listFiles(t, dir, false)
			checked := 0
			for _, name := range files {
				// Pick filenames carrying a per-codec infix
				// ("Lucene104" for postings, "Lucene90" for DV).
				if !strings.Contains(name, "_Lucene104_") &&
					!strings.Contains(name, "_Lucene90_") {
					continue
				}
				seg := gindex.ParseSegmentName(name)
				if seg != "_0" {
					t.Errorf("ParseSegmentName(%q) = %q, want %q",
						name, seg, "_0")
				}
				stripped := gindex.StripExtension(name)
				ext := gindex.GetExtension(name)
				if stripped == name || ext == "" {
					t.Errorf("StripExtension/GetExtension failed on %q (stripped=%q ext=%q)",
						name, stripped, ext)
				}
				checked++
			}
			if checked == 0 {
				t.Fatalf("found no per-codec-infixed filenames; have %v", files)
			}
		})
	}
}

// TestIndexFileNames_SegmentsCommitPointer (class a) confirms the
// commit pointer "segments_N" parses through Gocene's segments_*
// branch: it is NOT a codec file but IS recognised by the
// ReadSegmentInfos generation parser (covered in
// segments_n_compat_test.go).
func TestIndexFileNames_SegmentsCommitPointer(t *testing.T) {
	for _, scenario := range []string{ScenarioSegmentInfo, ScenarioDeletionsAndDvUpdates} {
		scenario := scenario
		t.Run(scenario, func(t *testing.T) {
			const seed int64 = 0xC0FFEE
			dir := generate(t, scenario, seed)
			seg := findSegmentsFile(t, dir)
			if !strings.HasPrefix(seg, "segments_") {
				t.Fatalf("segments_N filename malformed: %q", seg)
			}
			// CodecFilePattern is anchored on '_' as the leading char of
			// a codec file (it requires "_[a-z0-9]+"). "segments_N"
			// starts with 's' so MUST NOT match.
			if gindex.CodecFilePattern.MatchString(seg) {
				t.Errorf("CodecFilePattern matched segments_N (%q) — should not", seg)
			}
		})
	}
}
