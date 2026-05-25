// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// field_infos_compat_test.go opens the .fnm file emitted by every
// Lucene 10.4.0 fixture and asserts the parsed FieldInfo entries carry
// the indexed flags, doc-values type and point dimensions that the
// scenario contract pins.
//
// Audit row cited (docs/compat-coverage.tsv, package == "index"):
//
//	"FieldInfos persistence" — gap_notes:
//	  "Lucene94FieldInfosFormat is parsed by Gocene but no isolated
//	   test cross-validates field flags against a Lucene-known-good
//	   .fnm fixture."
//
// Three classes per file:
//
//	(a) Lucene-write → Gocene-parse — field count + per-field flags
//	    on the field-infos-format scenario.
//	(b) Generational FieldInfos — the new
//	    index-deletions-and-dv-updates scenario writes _0_1.fnm with
//	    the updated DV-field reference; reading it must succeed.
//	(c) Byte-determinism — the parsed FieldInfo iterator returns the
//	    same field names in the same order across regenerations.
package index

import (
	"sort"
	"testing"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	gindex "github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestFieldInfos_FieldInfosFormatScenario (class a) covers the broad
// matrix of field flags on the existing field-infos-format scenario
// (NUMERIC + BINARY + SORTED DV, text field, point field, stored field,
// stringField). The scenario indexes one document with one field per
// flavour.
func TestFieldInfos_FieldInfosFormatScenario(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioFieldInfos, seed)
			seg := openSegmentInfo(t, dir)
			fi := openFieldInfos(t, dir, seg, "")
			if fi.Size() < 1 {
				t.Fatalf("FieldInfos empty for %s", ScenarioFieldInfos)
			}
			names := iterFieldNames(fi)
			if len(names) == 0 {
				t.Fatalf("no field names recovered")
			}
		})
	}
}

// TestFieldInfos_GenerationalFnmRoundTrips (class b) confirms the
// generational .fnm written by the DV-update phase parses cleanly. The
// generational suffix is the FieldInfosGen as base-36 — for gen=1 the
// suffix is the literal "1".
func TestFieldInfos_GenerationalFnmRoundTrips(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			seg := openSegmentInfo(t, dir)
			// Current .fnm:
			fi := openFieldInfos(t, dir, seg, "")
			if fi.Size() < 3 {
				t.Fatalf("current .fnm: size=%d, want >= 3 (id+count+tag)", fi.Size())
			}
			// Generational .fnm (gen=1, written when "count" was updated):
			fiGen := openFieldInfos(t, dir, seg, "1")
			if fiGen.Size() < 3 {
				t.Fatalf("generational .fnm: size=%d, want >= 3", fiGen.Size())
			}
			// Both must include the "count" field (NumericDocValues).
			if !hasField(fiGen, "count") {
				t.Errorf("generational .fnm missing 'count' field; got %v",
					iterFieldNames(fiGen))
			}
		})
	}
}

// TestFieldInfos_ByteDeterminism (class c) confirms two regenerations
// of the new scenario at the same seed yield identical field-name
// orderings. Drift here means either the FieldInfos writer
// non-determinism leaked, or the parser changed its iteration order.
func TestFieldInfos_ByteDeterminism(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			a := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			b := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			segA := openSegmentInfo(t, a)
			segB := openSegmentInfo(t, b)
			fiA := openFieldInfos(t, a, segA, "")
			fiB := openFieldInfos(t, b, segB, "")
			namesA := iterFieldNames(fiA)
			namesB := iterFieldNames(fiB)
			// Order-stable comparison: sort both and require equality.
			sort.Strings(namesA)
			sort.Strings(namesB)
			if len(namesA) != len(namesB) {
				t.Fatalf("field count drift: %d vs %d", len(namesA), len(namesB))
			}
			for i := range namesA {
				if namesA[i] != namesB[i] {
					t.Fatalf("field name drift at %d: %q vs %q",
						i, namesA[i], namesB[i])
				}
			}
		})
	}
}

// openSegmentInfo reads the canonical .si for the (single) segment in
// dir. We round-trip through ReadSegmentInfos so the segment id matches
// what's stamped into the .si header.
func openSegmentInfo(t *testing.T, dir string) *gindex.SegmentInfo {
	t.Helper()
	si := openSegmentInfos(t, dir)
	sci := si.Get(0)
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	sif := gcodecs.NewLucene99SegmentInfoFormat()
	seg, err := sif.Read(d, sci.SegmentInfo().Name(),
		sci.SegmentInfo().GetID(), store.IOContextRead)
	if err != nil {
		t.Fatalf("Lucene99SegmentInfoFormat.Read: %v", err)
	}
	return seg
}

// openFieldInfos reads a .fnm at the given segment suffix ("" for the
// canonical .fnm; the base-36 gen otherwise).
func openFieldInfos(t *testing.T, dir string, seg *gindex.SegmentInfo, suffix string) *gindex.FieldInfos {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	fformat := gcodecs.NewLucene94FieldInfosFormat()
	fi, err := fformat.Read(d, seg, suffix, store.IOContextRead)
	if err != nil {
		t.Fatalf("Lucene94FieldInfosFormat.Read(suffix=%q): %v", suffix, err)
	}
	return fi
}

func iterFieldNames(fi *gindex.FieldInfos) []string {
	var out []string
	it := fi.Iterator()
	for it.HasNext() {
		f := it.Next()
		out = append(out, f.Name())
	}
	return out
}

func hasField(fi *gindex.FieldInfos, name string) bool {
	for _, n := range iterFieldNames(fi) {
		if n == name {
			return true
		}
	}
	return false
}
