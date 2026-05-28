// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// fuzzCFESegmentID is the fixed 16-byte segment identifier used both to
// stamp the well-formed seed and as the expected ID passed to the reader.
// readCompoundEntries validates the on-disk ID against this value via
// CheckIndexHeader, so seed and reader must agree.
var fuzzCFESegmentID = []byte{
	0, 1, 2, 3, 4, 5, 6, 7,
	8, 9, 10, 11, 12, 13, 14, 15,
}

// buildValidCFE produces a well-formed .cfe byte stream (index header +
// entry count + one entry + codec footer) using the production writers.
// It gives the fuzzer a realistic seed that reaches the entry-decoding loop
// and the footer/checksum validation rather than failing at the magic check.
func buildValidCFE(tb testing.TB) []byte {
	tb.Helper()
	dir := store.NewByteBuffersDirectory()
	raw, err := dir.CreateOutput("seed.cfe", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		tb.Fatalf("create seed output: %v", err)
	}
	// Gocene store outputs do not track a running CRC, so wrap to satisfy
	// the GetChecksum contract WriteFooter requires (mirrors the production
	// .cfe writer in compound_format.go).
	out := store.NewChecksumIndexOutput(raw)
	if err := WriteIndexHeader(out, Lucene90CompoundEntriesCodec, Lucene90CompoundVersionCurrent, fuzzCFESegmentID, ""); err != nil {
		tb.Fatalf("write header: %v", err)
	}
	if err := store.WriteVInt(out, 1); err != nil { // one entry
		tb.Fatalf("write count: %v", err)
	}
	if err := store.WriteString(out, "_0.fdt"); err != nil {
		tb.Fatalf("write entry name: %v", err)
	}
	if err := store.WriteInt64(out, 0); err != nil { // offset
		tb.Fatalf("write offset: %v", err)
	}
	if err := store.WriteInt64(out, 42); err != nil { // length
		tb.Fatalf("write length: %v", err)
	}
	if err := WriteFooter(out); err != nil {
		tb.Fatalf("write footer: %v", err)
	}
	if err := out.Close(); err != nil {
		tb.Fatalf("close output: %v", err)
	}

	in, err := dir.OpenInput("seed.cfe", store.IOContext{Context: store.ContextRead})
	if err != nil {
		tb.Fatalf("open seed input: %v", err)
	}
	defer in.Close()
	n := in.Length()
	buf := make([]byte, n)
	if err := in.ReadBytes(buf); err != nil {
		tb.Fatalf("read seed bytes: %v", err)
	}
	return buf
}

// FuzzLucene90CompoundEntriesRead fuzzes the Lucene 9.0 compound-file entries
// (.cfe) reader over arbitrary byte streams.
//
// The .cfe sidecar is an on-disk artefact: when Gocene opens a Lucene-written
// index it parses bytes it did not author, so a corrupt or truncated (or
// adversarial) file must be rejected with an error, never a panic. This
// fuzzes that boundary directly.
//
// Property: malformed bytes never panic; the reader returns an error.
// A successful parse (when the fuzzer happens to reconstruct a valid stream
// from the seed) is also fine — both outcomes are non-panicking, which is the
// invariant under test.
//
// The seed corpus pairs degenerate inputs (empty, pure garbage, a truncated
// header) with one fully valid .cfe stream so the mutation engine can explore
// both the early header-rejection paths and the deeper entry/footer-decoding
// loop, where an attacker-controlled VInt entry count or string length could
// otherwise drive an out-of-bounds slice or unbounded allocation.
func FuzzLucene90CompoundEntriesRead(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add(bytes.Repeat([]byte{0xff}, 32))
	// Correct magic number (CODEC_MAGIC, big-endian) followed by garbage,
	// to push the parser past the first 4 bytes into header-name validation.
	f.Add([]byte{0x3F, 0xD7, 0x6C, 0x17, 'x', 'y', 'z'})
	f.Add(buildValidCFE(f))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := store.NewByteBuffersDirectory()
		out, err := dir.CreateOutput("fuzz.cfe", store.IOContext{Context: store.ContextWrite})
		if err != nil {
			t.Fatalf("create fuzz output: %v", err)
		}
		if err := out.WriteBytes(data); err != nil {
			t.Fatalf("write fuzz bytes: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("close fuzz output: %v", err)
		}

		// The property under test is the absence of a panic. Both an error
		// (the expected outcome for malformed input) and a successful parse
		// are acceptable; only a panic is a bug.
		_, _, _ = readCompoundEntries(dir, "fuzz.cfe", fuzzCFESegmentID)
	})
}
