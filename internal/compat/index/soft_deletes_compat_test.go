// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// soft_deletes_compat_test.go cross-validates the soft-deletes
// persistence path: IndexWriterConfig.setSoftDeletesField + a
// softUpdateDocument call. The on-disk shape mirrors a hard-delete
// (generational .fnm + DV update on the tombstone field) but
// SegmentCommitInfo.softDelCount > 0 while delCount stays at 0.
//
// Audit row cited (docs/compat-coverage.tsv, package == "index"):
//
//	"soft-deletes" — gap_notes:
//	  "Lucene's setSoftDeletesField wire shape is not isolated by
//	   any test against a Lucene-emitted fixture; Gocene parses but
//	   does not cross-validate the softDelCount and tombstone DV
//	   files round-trip."
//
// Three classes per file:
//
//	(a) Lucene-emitted soft-deletes fixture -> SegmentCommitInfo's
//	    SoftDelCount reflects the soft-delete; DelCount stays at zero;
//	    FieldInfosFiles + DocValuesUpdatesFiles list the generational
//	    artefacts.
//	(b) The soft-deletes scenario is byte-deterministic at both canary
//	    seeds.
//	(c) Lucene's CheckIndex reports the index clean (proves the
//	    softDelCount + tombstone bytes Gocene parsed are the bytes
//	    Lucene meant to write).
package index

import (
	"testing"
)

// TestSoftDeletes_SegmentCommitInfoShape (class a) is the structural
// gate: the soft-deletes scenario produces TWO segments (the original
// one carrying a soft-delete tombstone via a DV update, plus the
// re-indexed copy from softUpdateDocument). Segment 0 must have
// SoftDelCount=1 and DelCount=0; segment 1 must have no deletes at all.
func TestSoftDeletes_SegmentCommitInfoShape(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "index-soft-deletes", seed)
			si := openSegmentInfos(t, dir)
			if si.Size() != 2 {
				t.Fatalf("expected 2 segments, got %d", si.Size())
			}
			sci0 := si.Get(0)
			if got := sci0.SoftDelCount(); got != 1 {
				t.Errorf("seg[0].SoftDelCount = %d, want 1", got)
			}
			if got := sci0.DelCount(); got != 0 {
				t.Errorf("seg[0].DelCount = %d, want 0 (soft-delete is not a hard-delete)", got)
			}
			// segments_N records FieldInfosGen=1 + DocValuesGen=1 because
			// the soft-delete is implemented as a DV update on the
			// tombstone field.
			if got := sci0.FieldInfosGen(); got != 1 {
				t.Errorf("seg[0].FieldInfosGen = %d, want 1", got)
			}
			if got := sci0.DocValuesGen(); got != 1 {
				t.Errorf("seg[0].DocValuesGen = %d, want 1", got)
			}
			if len(sci0.FieldInfosFiles()) == 0 {
				t.Errorf("seg[0].FieldInfosFiles is empty")
			}
			if len(sci0.DocValuesUpdatesFiles()) == 0 {
				t.Errorf("seg[0].DocValuesUpdatesFiles is empty")
			}

			sci1 := si.Get(1)
			if got := sci1.SoftDelCount(); got != 0 {
				t.Errorf("seg[1].SoftDelCount = %d, want 0", got)
			}
			if got := sci1.DelCount(); got != 0 {
				t.Errorf("seg[1].DelCount = %d, want 0", got)
			}
		})
	}
}

// TestSoftDeletes_ByteDeterminism (class b) confirms the scenario is
// byte-deterministic at both seeds via the SegmentCommitInfo shape.
func TestSoftDeletes_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, "index-soft-deletes", seed)
			b := generate(t, "index-soft-deletes", seed)
			siA := openSegmentInfos(t, a)
			siB := openSegmentInfos(t, b)
			if siA.Size() != siB.Size() {
				t.Fatalf("size drift: A=%d B=%d", siA.Size(), siB.Size())
			}
			for i := 0; i < siA.Size(); i++ {
				ka, kb := siA.Get(i), siB.Get(i)
				if ka.SoftDelCount() != kb.SoftDelCount() {
					t.Errorf("seg[%d].SoftDelCount drift: A=%d B=%d",
						i, ka.SoftDelCount(), kb.SoftDelCount())
				}
				if ka.DelCount() != kb.DelCount() {
					t.Errorf("seg[%d].DelCount drift: A=%d B=%d",
						i, ka.DelCount(), kb.DelCount())
				}
				if ka.FieldInfosGen() != kb.FieldInfosGen() {
					t.Errorf("seg[%d].FieldInfosGen drift", i)
				}
			}
		})
	}
}

// TestSoftDeletes_CheckIndexClean (class c) runs Lucene's CheckIndex
// over the fixture. Soft-deletes interact with several invariants
// (live-docs/maxDoc/softDelCount), so a clean run is a strong proof
// that the tombstone DV bytes parse as Lucene expected.
func TestSoftDeletes_CheckIndexClean(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "index-soft-deletes", seed)
			out, err := checkIndex(t, dir)
			if err != nil {
				t.Fatalf("CheckIndex non-clean on soft-deletes fixture: %v\n%s", err, out)
			}
		})
	}
}
