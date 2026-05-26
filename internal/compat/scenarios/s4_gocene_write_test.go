// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

// This file implements the Gocene-write leg for S4
// (combined-replicator-roundtrip).
//
// The Java-side leg (TestS4_ReplicatorRoundtrip) verifies that the Java
// harness can produce a CopyState wire frame that Gocene can round-trip.
// This test proves the inverse: Gocene produces the s4-frames.bin + s4-
// files.tsv that the Java verifier accepts via
// CombinedReplicatorRoundtripScenario.verify().
//
// The CodecUtil envelope uses:
//   - codec:   "GoceneCombinedReplicatorRoundtrip"
//   - version: 0
//   - id:      IDFromSeed(seed)
//   - suffix:  ""
//
// The CopyState payload is bit-for-bit identical to the Java reference when
// orderedFiles / orderedSet preserve LinkedHashMap / LinkedHashSet insertion
// order.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	gcompat "github.com/FlavioCFOliveira/Gocene/internal/compat"
	"github.com/FlavioCFOliveira/Gocene/replicator/nrt"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Codec constants mirroring CombinedReplicatorRoundtripScenario.
const (
	s4FramesName = "s4-frames.bin"
	s4FilesName  = "s4-files.tsv"
	s4Codec      = "GoceneCombinedReplicatorRoundtrip"
	s4Version    = int32(0)
)

// TestS4_GoceneWriteLeg produces a CopyState wire frame from Gocene and
// verifies it with the Java harness.
//
// The CopyState is built by BuildCopyStateOrdered which replicates
// ReplicatorNrtCopyStateScenario.buildCopyState byte-for-byte.
func TestS4_GoceneWriteLeg(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			requireHarness(t)

			dir := t.TempDir()
			state := nrt.BuildCopyStateOrdered(seed)

			// --- Write s4-frames.bin ---
			fsDir, err := store.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("open dir: %v", err)
			}
			defer fsDir.Close()

			raw, err := fsDir.CreateOutput(s4FramesName, store.IOContextDefault)
			if err != nil {
				t.Fatalf("create %s: %v", s4FramesName, err)
			}
			out := store.NewChecksumIndexOutput(raw)

			id := nrt.IDFromSeed(seed)
			if err := codecs.WriteIndexHeader(out, s4Codec, s4Version, id, ""); err != nil {
				out.Close()
				t.Fatalf("write index header: %v", err)
			}
			if err := nrt.WriteCopyStateOrdered(state, out); err != nil {
				out.Close()
				t.Fatalf("write copy state: %v", err)
			}
			if err := codecs.WriteFooter(out); err != nil {
				out.Close()
				t.Fatalf("write footer: %v", err)
			}
			if err := out.Close(); err != nil {
				t.Fatalf("close output: %v", err)
			}

			// --- Write s4-files.tsv ---
			// Sort rows by path for stability (mirrors CombinedReplicatorRoundtripScenario).
			type fileRow struct {
				path        string
				length      int64
				checksumHex string
			}
			var rows []fileRow
			for _, name := range state.Files.Names() {
				fmd, _ := state.Files.Get(name)
				rows = append(rows, fileRow{
					path:        name,
					length:      fmd.Length,
					checksumHex: fmt.Sprintf("%016x", uint64(fmd.Checksum)),
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].path < rows[j].path
			})

			tsvPath := filepath.Join(dir, s4FilesName)
			tsvFile, err := os.Create(tsvPath)
			if err != nil {
				t.Fatalf("create %s: %v", s4FilesName, err)
			}
			fmt.Fprintf(tsvFile, "# path\tlength\tchecksum_hex16\n")
			for _, r := range rows {
				fmt.Fprintf(tsvFile, "%s\t%d\t%s\n", r.path, r.length, r.checksumHex)
			}
			if err := tsvFile.Close(); err != nil {
				t.Fatalf("close TSV: %v", err)
			}

			// --- Verify with Java harness ---
			if err := gcompat.Verify(scenarioS4, seed, dir); err != nil {
				t.Fatalf("harness verify: %v", err)
			}
		})
	}
}
