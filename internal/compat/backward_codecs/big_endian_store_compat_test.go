// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// big_endian_store_compat_test.go addresses the backward_codecs audit row
// (verbatim from docs/compat-coverage.tsv):
//
//	backward_codecs	Legacy big-endian store wrappers
//	    lucene_class:  org.apache.lucene.backward_codecs.store.EndiannessReverserUtil
//	    gocene_class:  backward_codecs/store/store.go
//	    isolated:      yes:backward_codecs/store/store_test.go
//	    integration:   yes:backward_codecs/store/test_endianness_reverser_checksum_index_input_test.go
//	    binary_compat: no
//	    gap_notes:     "No fixture from an old big-endian Lucene index."
//
// Scenario "bwc-big-endian-store" writes a tiny BE-framed payload (magic
// + version int + count + 16 records of short/int/long/string) via
// EndiannessReverserUtil.createOutput. The Java verifier re-opens through
// the reverser, reads every record, AND opens a SECOND time WITHOUT the
// reverser to prove the version int's raw LE-interpretation equals
// Integer.reverseBytes(VERSION) — i.e. that the wrapper actually emitted
// big-endian bytes rather than being a no-op.
//
// Three test classes per the rmp 4634 contract:
//
//	(a) read-fixture     — Lucene-generated payload exists and the byte
//	                        layout is stable across two runs at the same seed.
//	(b) write-and-verify — Implemented: Gocene writes its own
//	                        bwc-big-endian-store.dat via
//	                        backward_codecs/store.EndiannessReverserUtil.CreateOutput
//	                        against a SimpleFSDirectory and re-verifies with the
//	                        Java harness (end-to-end write-path compat).
//	(c) round-trip       — Implemented: same as (b) plus Gocene re-reads every
//	                        record through EndiannessReverserUtil.OpenInput and
//	                        asserts correctness.
package backward_codecs

import (
	"fmt"
	"path/filepath"
	"testing"

	breverse "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

// TestBigEndianStore_ReadFixture (class a) drives the harness and asserts
// the resulting fixture carries the expected single-file shape.
func TestBigEndianStore_ReadFixture(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioBwcBigEndianStore, seed)
			path := filepath.Join(dir, fileBwcBigEndianStore)
			if !hasFile(t, dir, fileBwcBigEndianStore) {
				t.Fatalf("expected %s in %s (BE store payload missing)", path, dir)
			}
			assertDigestStable(t, ScenarioBwcBigEndianStore, seed)
		})
	}
}

// TestBigEndianStore_VerifySubcommand (class b, harness leg) drives the
// harness `verify` subcommand against a fresh fixture. A clean exit
// proves the Java verifier read the records back through the reverser
// AND confirmed the on-disk bytes are big-endian.
func TestBigEndianStore_VerifySubcommand(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, ScenarioBwcBigEndianStore, seed)
			verifyHarness(t, ScenarioBwcBigEndianStore, seed, dir)
		})
	}
}

// TestBigEndianStore_WriteAndVerify (class b, Gocene-side leg) has Gocene
// write its own bwc-big-endian-store.dat via
// backward_codecs/store.EndiannessReverserUtil.CreateOutput against a
// SimpleFSDirectory and re-verifies with the Java harness.
func TestBigEndianStore_WriteAndVerify(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := t.TempDir()
			fsDir, err := gstore.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("create FSDirectory: %v", err)
			}
			defer fsDir.Close()

			out, err := breverse.CreateOutput(fsDir, fileBwcBigEndianStore, gstore.IOContextDefault)
			if err != nil {
				t.Fatalf("CreateOutput: %v", err)
			}

			// Magic: "BE\x00" (passes through, single bytes).
			magic := []byte{0x42, 0x45, 0x00}
			if err := out.WriteBytes(magic); err != nil {
				t.Fatalf("WriteBytes(magic): %v", err)
			}
			// Version = 1 (int32, emitted big-endian by the reverser wrapper).
			if err := out.WriteInt(1); err != nil {
				t.Fatalf("WriteInt(version): %v", err)
			}
			// Count = 16 (vInt, passes through unchanged).
			if err := gstore.WriteVInt(out, 16); err != nil {
				t.Fatalf("WriteVInt(count): %v", err)
			}
			// 16 records: short, int, long, string.
			for i := 0; i < 16; i++ {
				if err := out.WriteShort(int16(seed + int64(i))); err != nil {
					t.Fatalf("WriteShort(%d): %v", i, err)
				}
				if err := out.WriteInt(int32(seed * int64(i+5))); err != nil {
					t.Fatalf("WriteInt(%d): %v", i, err)
				}
				if err := out.WriteLong(int64(seed^int64(i)) << (i & 0x3F)); err != nil {
					t.Fatalf("WriteLong(%d): %v", i, err)
				}
				s := fmt.Sprintf("be-frame-%d-seed-%d", i, seed)
				if err := out.WriteString(s); err != nil {
					t.Fatalf("WriteString(%d): %v", i, err)
				}
			}
			if err := out.Close(); err != nil {
				t.Fatalf("Close output: %v", err)
			}
			if err := fsDir.Close(); err != nil {
				t.Fatalf("Close directory: %v", err)
			}
			verifyHarness(t, ScenarioBwcBigEndianStore, seed, dir)
		})
	}
}

// TestBigEndianStore_RoundTrip (class c) is the full Gocene-write →
// Java-verify → Gocene-read-back loop.  The Java verify step proves the
// Gocene-produced byte stream is identical to what Lucene's own
// EndiannessReverserUtil.createOutput emits for the same seed.  The
// Gocene read-back step proves every record round-trips losslessly.
func TestBigEndianStore_RoundTrip(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			// Step 1: Gocene writes a BE-framed file (same sequence as
			// WriteAndVerify).
			dir := t.TempDir()
			fsDir, err := gstore.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("create FSDirectory: %v", err)
			}

			out, err := breverse.CreateOutput(fsDir, fileBwcBigEndianStore, gstore.IOContextDefault)
			if err != nil {
				t.Fatalf("CreateOutput: %v", err)
			}

			magic := []byte{0x42, 0x45, 0x00}
			if err := out.WriteBytes(magic); err != nil {
				t.Fatalf("WriteBytes(magic): %v", err)
			}
			if err := out.WriteInt(1); err != nil {
				t.Fatalf("WriteInt(version): %v", err)
			}
			if err := gstore.WriteVInt(out, 16); err != nil {
				t.Fatalf("WriteVInt(count): %v", err)
			}
			for i := 0; i < 16; i++ {
				if err := out.WriteShort(int16(seed + int64(i))); err != nil {
					t.Fatalf("WriteShort(%d): %v", i, err)
				}
				if err := out.WriteInt(int32(seed * int64(i+5))); err != nil {
					t.Fatalf("WriteInt(%d): %v", i, err)
				}
				if err := out.WriteLong(int64(seed^int64(i)) << (i & 0x3F)); err != nil {
					t.Fatalf("WriteLong(%d): %v", i, err)
				}
				s := fmt.Sprintf("be-frame-%d-seed-%d", i, seed)
				if err := out.WriteString(s); err != nil {
					t.Fatalf("WriteString(%d): %v", i, err)
				}
			}
			if err := out.Close(); err != nil {
				t.Fatalf("Close output: %v", err)
			}
			if err := fsDir.Close(); err != nil {
				t.Fatalf("Close directory: %v", err)
			}

			// Step 2: Java verifier confirms the Gocene-written file.
			verifyHarness(t, ScenarioBwcBigEndianStore, seed, dir)

			// Step 3: Gocene re-opens through the reverser and asserts
			// every record matches.
			fsDir2, err := gstore.NewSimpleFSDirectory(dir)
			if err != nil {
				t.Fatalf("re-open FSDirectory: %v", err)
			}
			defer fsDir2.Close()

			in, err := breverse.OpenInput(fsDir2, fileBwcBigEndianStore, gstore.IOContextRead)
			if err != nil {
				t.Fatalf("OpenInput: %v", err)
			}
			defer in.Close()

			gotMagic := make([]byte, 3)
			if err := in.ReadBytes(gotMagic); err != nil {
				t.Fatalf("ReadBytes(magic): %v", err)
			}
			if gotMagic[0] != 0x42 || gotMagic[1] != 0x45 || gotMagic[2] != 0x00 {
				t.Fatalf("bad magic: got %x, want [42 45 00]", gotMagic)
			}

			gotVersion, err := in.ReadInt()
			if err != nil {
				t.Fatalf("ReadInt(version): %v", err)
			}
			if gotVersion != 1 {
				t.Fatalf("bad version: got %d, want 1", gotVersion)
			}

			gotCount, err := gstore.ReadVInt(in)
			if err != nil {
				t.Fatalf("ReadVInt(count): %v", err)
			}
			if gotCount != 16 {
				t.Fatalf("bad count: got %d, want 16", gotCount)
			}

			for i := 0; i < 16; i++ {
				gotShort, err := in.ReadShort()
				if err != nil {
					t.Fatalf("ReadShort(%d): %v", i, err)
				}
				if wantShort := int16(seed + int64(i)); gotShort != wantShort {
					t.Fatalf("record %d short: got %d, want %d", i, gotShort, wantShort)
				}

				gotInt, err := in.ReadInt()
				if err != nil {
					t.Fatalf("ReadInt(%d): %v", i, err)
				}
				if wantInt := int32(seed * int64(i+5)); gotInt != wantInt {
					t.Fatalf("record %d int: got %d, want %d", i, gotInt, wantInt)
				}

				gotLong, err := in.ReadLong()
				if err != nil {
					t.Fatalf("ReadLong(%d): %v", i, err)
				}
				if wantLong := int64(seed^int64(i)) << (i & 0x3F); gotLong != wantLong {
					t.Fatalf("record %d long: got %d, want %d", i, gotLong, wantLong)
				}

				gotStr, err := in.ReadString()
				if err != nil {
					t.Fatalf("ReadString(%d): %v", i, err)
				}
				if wantStr := fmt.Sprintf("be-frame-%d-seed-%d", i, seed); gotStr != wantStr {
					t.Fatalf("record %d string: got %q, want %q", i, gotStr, wantStr)
				}
			}
		})
	}
}
