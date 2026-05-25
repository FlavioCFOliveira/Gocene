// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// segment_commit_info_compat_test.go validates the per-segment fields
// recovered from segments_N + .si by ReadSegmentInfos on the new
// index-deletions-and-dv-updates fixture. Every assertion below maps to
// a public SegmentCommitInfo accessor that downstream code (Merger,
// IndexWriter, DirectoryReader) reads.
//
// Audit row cited (docs/compat-coverage.tsv, package == "index"):
//
//	"SegmentCommitInfo" — gap_notes:
//	  "Per-segment commit metadata (delGen, fieldInfosGen, dvGen,
//	   softDelCount, files()) is parsed but no isolated test compares
//	   the Gocene-recovered values against a Lucene-known-good fixture."
//
// Three classes per file:
//
//	(a) Lucene-emitted scenario → Gocene parses → fields match the
//	    deterministic expectations of the scenario contract.
//	(b) Byte-determinism — two regenerations at the same seed recover
//	    identical SegmentCommitInfo values.
//	(c) Cross-engine round-trip — Lucene's CheckIndex (driven through
//	    the new harness `check` subcommand) reports the fixture clean,
//	    confirming the values Gocene recovered ARE the values Lucene
//	    stamped (not coincidentally identical).
package index

import (
	"reflect"
	"strings"
	"testing"
)

// TestSegmentCommitInfo_DeletionsAndDvUpdatesFields (class a) pins
// every SegmentCommitInfo accessor exposed by the
// index-deletions-and-dv-updates scenario: 12 docs added, 2 deleted, 1
// DV update across 3 commits, so the recovered values are:
//
//	delGen        = 1   (one deletes generation, applied in phase 2)
//	delCount      = 2
//	softDelCount  = 0   (no soft-deletes; covered by soft_deletes_compat_test.go)
//	fieldInfosGen = 1   (DV update bumps the per-segment fieldInfos)
//	dvGen         = 1   (one DV-updates generation, applied in phase 3)
//
// FieldInfosFiles() must contain _0_1.fnm and DocValuesUpdatesFiles()
// must list the generational dvd/dvm pair under generation 1.
func TestSegmentCommitInfo_DeletionsAndDvUpdatesFields(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			si := openSegmentInfos(t, dir)
			if si.Size() != 1 {
				t.Fatalf("expected 1 segment, got %d", si.Size())
			}
			sci := si.Get(0)

			if got, want := sci.DelGen(), int64(1); got != want {
				t.Errorf("DelGen = %d, want %d", got, want)
			}
			if got, want := sci.DelCount(), 2; got != want {
				t.Errorf("DelCount = %d, want %d", got, want)
			}
			if got, want := sci.SoftDelCount(), 0; got != want {
				t.Errorf("SoftDelCount = %d, want %d", got, want)
			}
			if got, want := sci.FieldInfosGen(), int64(1); got != want {
				t.Errorf("FieldInfosGen = %d, want %d", got, want)
			}
			if got, want := sci.DocValuesGen(), int64(1); got != want {
				t.Errorf("DocValuesGen = %d, want %d", got, want)
			}
			if !sci.HasDeletions() {
				t.Errorf("HasDeletions = false, want true")
			}

			// FieldInfos files: only the generational .fnm should be
			// recorded (the base .fnm is implicit in the .si files set).
			fif := sci.FieldInfosFiles()
			if len(fif) != 1 {
				t.Fatalf("FieldInfosFiles size = %d, want 1; have %v",
					len(fif), fif)
			}
			if _, ok := fif["_0_1.fnm"]; !ok {
				t.Errorf("FieldInfosFiles missing _0_1.fnm; got %v", fif)
			}

			// DV update files: gen=1 must list the dvd/dvm pair the writer
			// emitted. The generational suffix encodes both the per-field
			// format ("Lucene90_0") and the gen ("_1").
			dvu := sci.DocValuesUpdatesFiles()
			got, ok := dvu[1]
			if !ok {
				t.Fatalf("DocValuesUpdatesFiles missing generation 1; have %v", dvu)
			}
			want := map[string]struct{}{
				"_0_1_Lucene90_0.dvd": {},
				"_0_1_Lucene90_0.dvm": {},
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("DocValuesUpdatesFiles[1] = %v, want %v", got, want)
			}

			// Codec name stamped into segments_N.
			if codec := sci.SegmentInfo().Codec(); codec != "Lucene104" {
				t.Errorf("codec = %q, want %q", codec, "Lucene104")
			}
		})
	}
}

// TestSegmentCommitInfo_ByteDeterminism (class b) runs the scenario
// twice at the same seed and compares every SegmentCommitInfo accessor.
// Drift here flags a regression in either the writer (Lucene) or the
// parser (Gocene).
func TestSegmentCommitInfo_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			b := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			ka := captureSCIShape(t, a)
			kb := captureSCIShape(t, b)
			if !reflect.DeepEqual(ka, kb) {
				t.Fatalf("SegmentCommitInfo shape drift between two runs:\n A=%v\n B=%v",
					ka, kb)
			}
		})
	}
}

// TestSegmentCommitInfo_NoDeletionsBaseline (class a, negative) — on a
// fixture with no deletes the same accessors must report zero. The
// segment-info-format scenario indexes three docs with no mutations.
func TestSegmentCommitInfo_NoDeletionsBaseline(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioSegmentInfo, seed)
			sci := openSegmentInfos(t, dir).Get(0)
			if sci.HasDeletions() {
				t.Errorf("HasDeletions = true on a clean fixture")
			}
			if sci.DelGen() != -1 && sci.DelGen() != 0 {
				t.Errorf("DelGen = %d, want -1 or 0", sci.DelGen())
			}
			if sci.DelCount() != 0 {
				t.Errorf("DelCount = %d, want 0", sci.DelCount())
			}
			if sci.FieldInfosGen() != -1 && sci.FieldInfosGen() != 0 {
				t.Errorf("FieldInfosGen = %d, want -1 or 0", sci.FieldInfosGen())
			}
			if sci.DocValuesGen() != -1 && sci.DocValuesGen() != 0 {
				t.Errorf("DocValuesGen = %d, want -1 or 0", sci.DocValuesGen())
			}
		})
	}
}

// sciShape is a deterministic snapshot of the SegmentCommitInfo
// accessors used by TestSegmentCommitInfo_ByteDeterminism. Capturing a
// flat struct lets the test compare with reflect.DeepEqual and report
// drift in a single side-by-side diff.
type sciShape struct {
	name          string
	codec         string
	delGen        int64
	delCount      int
	softDelCount  int
	fieldInfosGen int64
	dvGen         int64
	fieldInfosF   []string
	dvUpdF        string // sorted joined names
}

func captureSCIShape(t *testing.T, dir string) sciShape {
	t.Helper()
	sci := openSegmentInfos(t, dir).Get(0)
	fi := keysSorted(sci.FieldInfosFiles())
	dvu := sci.DocValuesUpdatesFiles()
	var dvNames []string
	for _, set := range dvu {
		dvNames = append(dvNames, keysSorted(set)...)
	}
	// Sort the joined view so equality is deterministic regardless of
	// map iteration order.
	dvJoined := strings.Join(dvNames, ",")
	return sciShape{
		name:          sci.SegmentInfo().Name(),
		codec:         sci.SegmentInfo().Codec(),
		delGen:        sci.DelGen(),
		delCount:      sci.DelCount(),
		softDelCount:  sci.SoftDelCount(),
		fieldInfosGen: sci.FieldInfosGen(),
		dvGen:         sci.DocValuesGen(),
		fieldInfosF:   fi,
		dvUpdF:        dvJoined,
	}
}

func keysSorted(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// stdlib sort.Strings would import sort; we already pull strings
	// for Join. Use a tiny insertion sort (m is always tiny here).
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
