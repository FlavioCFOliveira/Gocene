// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// buffered_checksum_compat_test.go addresses the audit row
// "BufferedChecksum framing" from docs/compat-coverage.tsv. The test
// generates Lucene fixtures across several scenarios, then reads every byte
// through Gocene's BufferedChecksumIndexInput and asserts the final CRC32
// matches the checksum embedded in the file's CodecUtil footer. A successful
// match proves Gocene's CRC32 implementation is byte-for-byte identical to
// Lucene's over real, framed fixtures (header + payload + footer).
package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/internal/compat"
	gostore "github.com/FlavioCFOliveira/Gocene/store"
)

// TestBufferedChecksum_AgainstLuceneFixtures runs the BufferedChecksum CRC32
// gate against several real scenarios. Each scenario emits one or more files
// framed by CodecUtil; for each emitted file we open it twice (once for the
// streaming checksum, once via RetrieveChecksum from the footer) and require
// the two values to agree.
func TestBufferedChecksum_AgainstLuceneFixtures(t *testing.T) {
	requireHarness(t)

	const seed int64 = 12648430 // 0xC0FFEE

	cases := []string{
		"smoke",
		"store-primitives",
		"postings-format",
		"segment-info-format",
	}

	for _, scenario := range cases {
		scenario := scenario
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			if err := compat.GenerateInto(scenario, seed, dir); err != nil {
				t.Fatalf("harness gen %s: %v", scenario, err)
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read dir: %v", err)
			}
			if len(entries) == 0 {
				t.Fatalf("scenario %s produced no files", scenario)
			}

			checked := 0
			for _, e := range entries {
				if e.IsDir() || !hasCodecFooter(filepath.Join(dir, e.Name())) {
					continue
				}
				name := e.Name()
				t.Run(name, func(t *testing.T) {
					verifyFileChecksum(t, dir, name)
				})
				checked++
			}
			if checked == 0 {
				t.Fatalf("scenario %s: no CodecUtil-framed files found to checksum", scenario)
			}
		})
	}
}

// verifyFileChecksum opens dir/name twice: (1) it streams the entire file
// through a BufferedChecksumIndexInput and computes its CRC32; (2) it
// retrieves the CRC32 stored in the footer via codecs.RetrieveChecksum.
// Both values MUST be equal. The streaming CRC covers every byte EXCEPT the
// last 8 (the stored CRC itself), matching Lucene's CodecUtil footer
// contract.
func verifyFileChecksum(t *testing.T, dir, name string) {
	t.Helper()
	gd, err := gostore.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer gd.Close()

	// 1. Read the footer's stored CRC32 via RetrieveChecksum.
	in0, err := gd.OpenInput(name, gostore.IOContextDefault)
	if err != nil {
		t.Fatalf("open input #1: %v", err)
	}
	storedCRC, err := codecs.RetrieveChecksum(in0)
	in0.Close()
	if err != nil {
		t.Fatalf("RetrieveChecksum: %v", err)
	}

	// 2. Stream every byte except the trailing 8 (the stored CRC) through
	//    a BufferedChecksumIndexInput and compare its running CRC32.
	in1, err := gd.OpenInput(name, gostore.IOContextDefault)
	if err != nil {
		t.Fatalf("open input #2: %v", err)
	}
	defer in1.Close()

	bc := gostore.NewBufferedChecksumIndexInput(in1)
	total := in1.Length()
	if total < 8 {
		t.Fatalf("file too small: %d bytes", total)
	}
	const chunk = 4096
	remaining := total - 8
	buf := make([]byte, chunk)
	for remaining > 0 {
		step := int64(chunk)
		if remaining < step {
			step = remaining
		}
		if err := bc.ReadBytes(buf[:step]); err != nil {
			t.Fatalf("ReadBytes: %v", err)
		}
		remaining -= step
	}
	computed := int64(bc.GetChecksum())
	if computed != storedCRC {
		t.Fatalf("CRC mismatch for %s/%s:\n  BufferedChecksumIndexInput = 0x%08x\n  RetrieveChecksum            = 0x%08x",
			dir, name, uint32(computed), uint32(storedCRC))
	}
}

// hasCodecFooter does a cheap heuristic check by file size only: a CodecUtil
// footer is 16 bytes, so any file shorter than 16 bytes cannot be framed.
// This excludes empty/sentinel files that scenarios may emit alongside the
// real fixtures (e.g. write.lock).
func hasCodecFooter(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Size() >= 16
}
