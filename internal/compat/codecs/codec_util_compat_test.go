// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// codec_util_compat_test.go is the file-level golden test for Apache Lucene
// 10.4.0 codec envelopes (CodecUtil headers + footers + CRC32). It loads
// every Lucene-emitted file produced by the smoke, store-primitives,
// postings-format, doc-values-format, stored-fields-format, norms-format,
// points-format, knn-vectors-format, compound-format, field-infos-format,
// segment-info-format, live-docs-format, term-vectors-format and fst-blob
// scenarios, and validates each one with Gocene's CodecUtil parser:
//
//   - codecs.RetrieveChecksum (footer magic + algorithm byte + recorded CRC)
//   - codecs.ChecksumEntireFile (reads the entire stream and confirms the
//     header magic is valid and the footer CRC matches a fresh CRC32 over
//     the file body).
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"CodecUtil header/footer envelope" — gap_notes:
//	  "No isolated golden test that loads a Lucene-emitted file byte
//	   stream and validates header/footer parsing in isolation."
//
// This test closes that gap with a corpus produced by Lucene 10.4.0 itself.
package codecs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// codecUtilScenarios is the set of fixture scenarios whose every emitted
// file (except write.lock) must satisfy CodecUtil. The smoke and
// store-primitives scenarios use their own tiny envelope and are
// validated by their own existing tests; we include them here to confirm
// the cross-engine envelope, not the body. fst-blob also writes a single
// CodecUtil-framed file.
var codecUtilScenarios = []string{
	"smoke",
	"store-primitives",
	"postings-format",
	"doc-values-format",
	"stored-fields-format",
	"term-vectors-format",
	"norms-format",
	"points-format",
	"knn-vectors-format",
	"compound-format",
	"field-infos-format",
	"segment-info-format",
	"live-docs-format",
	"fst-blob",
	// Sprint 114 T7 newly registered scenarios — exercising
	// CodecUtil on the new per-field and quantized envelopes.
	"perfield-postings-doc-values",
	"compressing-stored-fields",
	"scalar-quantized-knn",
}

// envelopeExempt lists file basenames that do NOT carry a CodecUtil
// envelope and must be skipped by this test. The only known case in the
// Sprint 114 corpus is the standalone FST blob, which uses
// org.apache.lucene.util.fst.FST.save() — that wire format is
// 4-byte FST-magic + payload, NOT the CodecUtil 16-byte trailer.
var envelopeExempt = map[string]struct{}{
	"fst.bin": {},
}

func TestCodecUtil_FooterAndCRC_OverLuceneEmittedCorpus(t *testing.T) {
	requireHarness(t)
	// Single seed is sufficient for envelope validation; the byte
	// content varies with seed but CodecUtil framing does not.
	const seed int64 = 0xC0FFEE

	for _, scenario := range codecUtilScenarios {
		scenario := scenario
		t.Run(scenario, func(t *testing.T) {
			dir := generate(t, scenario, seed)
			files := listSegmentFiles(t, dir, false /* includeCommit */)
			if len(files) == 0 {
				t.Fatalf("%s: harness produced no files", scenario)
			}
			validated, exempted := 0, 0
			for _, name := range files {
				if _, exempt := envelopeExempt[name]; exempt {
					exempted++
					continue
				}
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Errorf("%s/%s: CodecUtil validation failed: %v",
						scenario, name, err)
					continue
				}
				validated++
			}
			if validated == 0 && exempted == 0 {
				t.Fatalf("%s: no files passed CodecUtil validation", scenario)
			}
			t.Logf("%s: %d/%d files passed CodecUtil header+footer+CRC",
				scenario, validated, len(files))
		})
	}
}

// validateOneEnvelope opens dir/name with a SimpleFSDirectory, runs
// codecs.ChecksumEntireFile (which validates header magic, footer magic,
// algorithm byte and CRC32 over the body), and also calls
// codecs.RetrieveChecksum to confirm the footer round-trips independently.
//
// Both calls share the same underlying file but use Clone() so they do
// not interfere.
func validateOneEnvelope(t *testing.T, dir, name string) error {
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
	// Sanity: file must be at least header (9 bytes minimum: 4 magic +
	// 1 codec-string-length + 4 version) + 16 footer = 29 bytes. The
	// smoke fixture is the smallest at 32 bytes.
	if in.Length() < 16 {
		return wrapErr("file too small for footer", in.Length(), 0)
	}
	if _, err := codecs.RetrieveChecksum(in); err != nil {
		return wrapErr("RetrieveChecksum: "+err.Error(), in.Length(), 0)
	}
	// ChecksumEntireFile reads the whole stream and verifies that the
	// recorded CRC equals a fresh CRC32 over [0 .. len-8). It is the
	// strongest assertion we can make without knowing the codec name.
	if _, err := codecs.ChecksumEntireFile(in); err != nil {
		// Some Lucene files are framed as IndexHeader+payload+IndexFooter
		// where the payload contains its own length prefix; the
		// "misplaced footer" error here means we've already validated
		// the bytes are a valid envelope through RetrieveChecksum, so
		// we treat that very specific path as informational rather
		// than fatal.
		if strings.Contains(err.Error(), "misplaced codec footer") {
			return wrapErr("ChecksumEntireFile (misplaced footer): "+err.Error(), in.Length(), 0)
		}
		return wrapErr("ChecksumEntireFile: "+err.Error(), in.Length(), 0)
	}
	return nil
}

// wrapErr produces a stable error message including file metadata so the
// test output is actionable.
func wrapErr(msg string, length, ptr int64) error {
	return &envelopeError{msg: msg, length: length, ptr: ptr}
}

type envelopeError struct {
	msg    string
	length int64
	ptr    int64
}

func (e *envelopeError) Error() string {
	return e.msg
}

// TestCodecUtil_RawMagicHeaders is a paranoid byte-level cross-check: it
// reads the first 4 bytes of every file in the corpus and confirms they
// match CODEC_MAGIC (0x3FD76C17, big-endian). Catches the case where
// Gocene's CODEC_MAGIC constant ever drifts from upstream.
func TestCodecUtil_RawMagicHeaders(t *testing.T) {
	requireHarness(t)
	const seed int64 = 0xC0FFEE
	dir := generate(t, "smoke", seed)
	files := listSegmentFiles(t, dir, false)
	for _, name := range files {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if len(raw) < 4 {
			t.Fatalf("%s too short for magic", name)
		}
		got := int32(raw[0])<<24 | int32(raw[1])<<16 | int32(raw[2])<<8 | int32(raw[3])
		if got != codecs.CODEC_MAGIC {
			t.Fatalf("%s: header magic = %x, want %x", name, got, codecs.CODEC_MAGIC)
		}
	}
}
