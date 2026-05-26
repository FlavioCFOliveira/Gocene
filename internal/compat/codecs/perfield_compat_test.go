// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// perfield_compat_test.go covers the per-field codec dispatch wrappers:
// PerFieldPostingsFormat, PerFieldDocValuesFormat and
// PerFieldKnnVectorsFormat. The new "perfield-postings-doc-values"
// scenario indexes a document whose "title" field routes through
// Lucene104PostingsFormat while "body" routes through FSTPostingsFormat,
// producing per-format suffixed files:
//
//   _0_Lucene104_0.tim / .doc / .pos / .psm / .tip / .tmd  (default)
//   _0_FST50_0.doc / .pos / .psm / .tfp                    (alt)
//   _0_Lucene90_0.dvd / .dvm                                (default DV)
//
// Audit rows cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"PerFieldPostingsFormat dispatch" — gap_notes:
//	  "No fixture proves the suffix encoding matches Lucene exactly
//	   when multiple PFs coexist."
//	"PerFieldDocValuesFormat dispatch" — gap_notes:
//	  "No cross-engine fixture for per-field DV."
//	"PerFieldKnnVectorsFormat dispatch" — gap_notes:
//	  "No Lucene-emitted multi-format vector fixture." (NOTE: deferred —
//	  see deferred_codecs_compat_test.go for the rationale; the
//	  scalar-quantized-knn scenario uses a single KNN format).
package codecs

import (
	"strings"
	"testing"
)

// TestPerField_TwoPostingsFormatsCoexist confirms the per-field
// dispatch wrote two materially different posting-format suffixes into
// the segment. The mere presence of "_FST50_0" and "_Lucene104_0"
// filename infixes proves the dispatch metadata in .fnm was honoured.
func TestPerField_TwoPostingsFormatsCoexist(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "perfield-postings-doc-values", seed)
			files := listSegmentFiles(t, dir, true)
			var sawDefault, sawAlt bool
			for _, n := range files {
				if strings.Contains(n, "_Lucene104_0.") {
					sawDefault = true
				}
				if strings.Contains(n, "_FST50_0.") {
					sawAlt = true
				}
			}
			if !sawDefault {
				t.Fatalf("perfield: no _Lucene104_0.* file found; have: %v", files)
			}
			if !sawAlt {
				t.Fatalf("perfield: no _FST50_0.* file found (FSTPostingsFormat dispatch is broken); have: %v", files)
			}
		})
	}
}

// TestPerField_DocValuesDispatch confirms PerFieldDocValuesFormat
// stamped the .dvd/.dvm filenames with the Lucene90 suffix. Both DV
// fields in the scenario route through the same format, so only one
// suffix should appear.
func TestPerField_DocValuesDispatch(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "perfield-postings-doc-values", seed)
			dvd := findUniqueByExt(t, dir, ".dvd")
			if !strings.Contains(dvd, "_Lucene90_0.") {
				t.Fatalf("perfield-dv: .dvd lacks _Lucene90_0 suffix: %s", dvd)
			}
			if err := validateOneEnvelope(t, dir, dvd); err != nil {
				t.Fatalf("%s: CRC validation failed: %v", dvd, err)
			}
		})
	}
}

// TestPerField_AltPostingsEnvelopes opens the FST50 alternate postings
// files and validates their codec envelopes. FST50 emits its own
// .doc/.pos/.psm/.tfp set; we validate the CRC on every one.
func TestPerField_AltPostingsEnvelopes(t *testing.T) {
	requireHarness(t)
	dir := generate(t, "perfield-postings-doc-values", 0xC0FFEE)
	files := listSegmentFiles(t, dir, true)
	checked := 0
	for _, n := range files {
		if !strings.Contains(n, "_FST50_0.") {
			continue
		}
		if err := validateOneEnvelope(t, dir, n); err != nil {
			t.Fatalf("%s: CRC validation failed: %v", n, err)
		}
		checked++
	}
	if checked < 3 {
		t.Fatalf("expected >=3 FST50 sidecar files (doc/pos/psm/tfp); got %d", checked)
	}
}
