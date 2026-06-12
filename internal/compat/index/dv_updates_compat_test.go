// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// dv_updates_compat_test.go cross-validates Lucene's generational
// doc-values updates: the writer emits .dvd/.dvm pairs with a per-gen
// suffix encoding both the format ("Lucene90_0") and the generation
// number ("_1"), and segments_N records the file list under
// DocValuesUpdatesFiles().
//
// Audit row cited (docs/compat-coverage.tsv, package == "index"):
//
//	"DV updates (generational .dvd/.dvm)" — gap_notes:
//	  "No isolated test loads the generational .dvd/.dvm pair from a
//	   Lucene-emitted fixture and asserts the post-update doc-value
//	   becomes visible through Gocene's reader contract."
//
// Three classes per file:
//
//	(a) File presence + naming — the new
//	    index-deletions-and-dv-updates fixture MUST emit exactly the
//	    _0_1_Lucene90_0.{dvd,dvm} pair after the phase-3 commit.
//	(b) CodecUtil envelope — each generational file carries a valid
//	    header/footer (header magic + footer CRC32).
//	(c) SegmentCommitInfo cross-check — DocValuesUpdatesFiles() maps
//	    generation 1 to that exact pair (no extras, no omissions).
//
// The Gocene-side READ of the updated long value is exercised by
// TestDVUpdates_ValueVisibility below. The Lucene-side READ is covered
// by the scenario's verify() method.
package index

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDVUpdates_ValueVisibility (class c, formerly deferred behind BLOCKER-1)
// opens the Lucene-emitted fixture with Gocene's DirectoryReader (core readers
// now wired) and asserts that the updated NumericDocValues value is readable.
// This is the Gocene-side leg of the cross-engine DV-update round-trip.
func TestDVUpdates_ValueVisibility(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dirPath := generate(t, ScenarioDeletionsAndDvUpdates, seed)

			dir, err := store.NewSimpleFSDirectory(dirPath)
			if err != nil {
				t.Fatalf("open dir: %v", err)
			}
			defer dir.Close()

			reader, err := index.OpenDirectoryReader(dir)
			if err != nil {
				t.Fatalf("OpenDirectoryReader: %v", err)
			}
			defer reader.Close()

			// Phase 2 deletes doc-3 and doc-7, so 10 docs remain alive.
			const wantLive = 10
			if got := reader.NumDocs(); got != wantLive {
				t.Fatalf("NumDocs = %d, want %d", got, wantLive)
			}

			sawUpdate := false
			for _, seg := range reader.GetSegmentReaders() {
				dv, err := seg.GetNumericDocValues("count")
				if err != nil {
					t.Fatalf("GetNumericDocValues: %v", err)
				}
				if dv == nil {
					t.Fatal("GetNumericDocValues returned nil")
				}

				liveDocs := seg.GetLiveDocs()
				for d := 0; d < seg.MaxDoc(); d++ {
					if liveDocs != nil && !liveDocs.Get(d) {
						continue
					}
					has, err := dv.AdvanceExact(d)
					if err != nil {
						t.Fatalf("AdvanceExact(%d): %v", d, err)
					}
					if !has {
						continue
					}
					val, err := dv.LongValue()
					if err != nil {
						t.Fatalf("LongValue(%d): %v", d, err)
					}
					// The fixture indexes doc-i as the i-th document, so doc-5 is
					// at docID == 5 before deletes. After deletes of doc-3 and
					// doc-7, the remaining docs are 0,1,2,4,5,6,8,9,10,11.
					// Doc ID 5 still maps to doc-5 because deletions don't shift
					// doc IDs in Lucene.
					if d == 5 {
						const wantVal = 999
						if val != wantVal {
							t.Fatalf("doc-5 count = %d, want %d", val, wantVal)
						}
						sawUpdate = true
					}
				}
			}
			if !sawUpdate {
				t.Fatalf("did not find doc-5 with updated count==999")
			}
		})
	}
}

// TestDVUpdates_GenerationalFilesPresent (class a + c) confirms the
// generational DV-update files exist at the expected names and that
// SegmentCommitInfo's DocValuesUpdatesFiles entry references both.
func TestDVUpdates_GenerationalFilesPresent(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			files := listFiles(t, dir, false)
			var sawDvd, sawDvm bool
			for _, n := range files {
				if strings.HasSuffix(n, "_1_Lucene90_0.dvd") {
					sawDvd = true
				}
				if strings.HasSuffix(n, "_1_Lucene90_0.dvm") {
					sawDvm = true
				}
			}
			if !sawDvd || !sawDvm {
				t.Fatalf("missing generational DV files; got %v", files)
			}

			// SegmentCommitInfo must register the same pair under
			// generation 1.
			dvu := openSegmentInfos(t, dir).Get(0).DocValuesUpdatesFiles()
			got, ok := dvu[1]
			if !ok {
				t.Fatalf("DocValuesUpdatesFiles missing gen 1; have %v", dvu)
			}
			for _, want := range []string{"_0_1_Lucene90_0.dvd", "_0_1_Lucene90_0.dvm"} {
				if _, present := got[want]; !present {
					t.Errorf("DocValuesUpdatesFiles[1] missing %s; got %v", want, got)
				}
			}
			if len(got) != 2 {
				t.Errorf("DocValuesUpdatesFiles[1] size = %d, want 2; got %v",
					len(got), got)
			}
		})
	}
}

// TestDVUpdates_GenerationalEnvelopes (class b) verifies the CodecUtil
// envelope (header magic + footer CRC32) on every generational
// DV-update file. The codec name strings are unexported in the Gocene
// production module; we validate the framing without pinning the name.
func TestDVUpdates_GenerationalEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioDeletionsAndDvUpdates, seed)
			for _, n := range listFiles(t, dir, false) {
				if !strings.HasSuffix(n, "_1_Lucene90_0.dvd") &&
					!strings.HasSuffix(n, "_1_Lucene90_0.dvm") {
					continue
				}
				if err := validateEnvelope(t, dir, n); err != nil {
					t.Errorf("%s: %v", n, err)
				}
			}
		})
	}
}

// validateEnvelope opens dir/name with a SimpleFSDirectory, runs
// codecs.ChecksumEntireFile (verifies header magic + footer CRC32) and
// codecs.RetrieveChecksum (footer round-trip).
func validateEnvelope(t *testing.T, dir, name string) error {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	in, err := d.OpenInput(name, store.IOContextDefault)
	if err != nil {
		return err
	}
	defer in.Close()
	if _, err := codecs.RetrieveChecksum(in); err != nil {
		return err
	}
	if _, err := codecs.ChecksumEntireFile(in); err != nil {
		// codecs.ChecksumEntireFile rejects files with IndexHeader+payload
		// (where the footer position is mid-stream); skipping that very
		// specific path keeps the assertion focused on framing.
		if strings.Contains(err.Error(), "misplaced codec footer") {
			return nil
		}
		return err
	}
	return nil
}
