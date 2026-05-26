// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package scenarios

import (
	"bytes"
	"os"
	"strconv"
	"testing"
)

const scenarioS4 = "combined-replicator-roundtrip"

// codecUtilIndexHeaderMagic is the BE int32 0x3FD76C17 prefixed to every
// CodecUtil-framed Lucene file (see Lucene 10.4.0
// org.apache.lucene.codecs.CodecUtil.CODEC_MAGIC = 0x3FD76C17).
var codecUtilIndexHeaderMagic = []byte{0x3F, 0xD7, 0x6C, 0x17}

// TestS4_ReplicatorRoundtrip generates the primary→replica wire transcript
// (s4-frames.bin + s4-files.tsv), asserts s4-frames.bin starts with the
// CodecUtil magic + has a non-trivial length, asserts the TSV lists the
// canonical 3 files, and re-runs `verify` to confirm the wire frame
// round-trips through TestSimpleServer.readCopyState.
//
// Class-(c) Gocene-write leg deferred — Gocene's replicator/nrt port
// ships CopyState/FileMetaData types but no wire encoder. See
// deferred_combined_compat_test.go.
func TestS4_ReplicatorRoundtrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(strconv.FormatInt(seed, 10), func(t *testing.T) {
			dir := generate(t, scenarioS4, seed)
			framesPath := dir + "/s4-frames.bin"
			filesPath := dir + "/s4-files.tsv"
			blob, err := os.ReadFile(framesPath)
			if err != nil {
				t.Fatalf("read s4-frames.bin: %v", err)
			}
			if len(blob) < 64 {
				t.Fatalf("s4-frames.bin suspiciously small (%d bytes)", len(blob))
			}
			if !bytes.HasPrefix(blob, codecUtilIndexHeaderMagic) {
				t.Fatalf("s4-frames.bin missing CodecUtil magic; got prefix %x",
					blob[:4])
			}
			mustHaveTSV(t, filesPath)
			rows := readTSV(t, filesPath)
			if len(rows) != 3 {
				t.Fatalf("s4-files.tsv: expected 3 rows (segments_1, _0.cfe, _0.cfs); got %d",
					len(rows))
			}
			for i, r := range rows {
				if len(r) != 3 {
					t.Fatalf("row %d: expected 3 cols, got %d: %v", i, len(r), r)
				}
				if r[0] == "" || len(r[2]) != 16 { // 16-hex-char checksum
					t.Fatalf("row %d: bad path/checksum: %v", i, r)
				}
				if _, err := strconv.ParseInt(r[1], 10, 64); err != nil {
					t.Fatalf("row %d: bad length %q: %v", i, r[1], err)
				}
			}
			stdout, stderr, err := runHarness(t, "verify", scenarioS4,
				formatSeed(seed), dir)
			assertOK(t, stdout, stderr, "ok scenario="+scenarioS4, err)
		})
	}
}
