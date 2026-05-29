// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// makeLive returns a FixedBitSet of length maxDoc with all bits set (every
// document live), then clears the supplied deleted ordinals.
func makeLive(t *testing.T, maxDoc int, deleted ...int) *util.FixedBitSet {
	t.Helper()
	live, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("NewFixedBitSet(%d): %v", maxDoc, err)
	}
	for i := 0; i < maxDoc; i++ {
		live.Set(i)
	}
	for _, d := range deleted {
		live.Clear(d)
	}
	return live
}

// TestLiveDocs_RoundTrip writes a .liv file via writeLiveDocs and reads it back
// via readLiveDocs, asserting the live bitset and deleted count survive across
// a range of maxDoc values that straddle the 64-bit word and 1024-bit batch
// boundaries used by the Lucene90 wire format.
func TestLiveDocs_RoundTrip(t *testing.T) {
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}

	cases := []struct {
		name    string
		maxDoc  int
		gen     int64
		deleted []int
	}{
		{"single-word-one-del", 5, 1, []int{2}},
		{"single-word-multi-del", 64, 3, []int{0, 7, 63}},
		{"word-boundary", 65, 2, []int{64}},
		{"batch-boundary", 1024, 4, []int{0, 1023}},
		{"cross-batch", 2000, 36, []int{63, 64, 1023, 1024, 1999}},
		{"no-del", 10, 1, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := store.NewByteBuffersDirectory()
			defer dir.Close()

			seg := "_0"
			live := makeLive(t, tc.maxDoc, tc.deleted...)
			wantDel := len(tc.deleted)

			gotDelWrite, err := writeLiveDocs(dir, seg, id, tc.gen, live)
			if err != nil {
				t.Fatalf("writeLiveDocs: %v", err)
			}
			if gotDelWrite != wantDel {
				t.Errorf("write delCount: got %d, want %d", gotDelWrite, wantDel)
			}

			// The file must be named per IndexFileNames.fileNameFromGeneration.
			wantName := liveDocsFileName(seg, tc.gen)
			if !dir.FileExists(wantName) {
				t.Fatalf("expected .liv file %q to exist", wantName)
			}

			back, err := readLiveDocs(dir, seg, id, tc.gen, tc.maxDoc)
			if err != nil {
				t.Fatalf("readLiveDocs: %v", err)
			}
			if back == nil {
				t.Fatalf("readLiveDocs returned nil for an existing file")
			}
			if back.Length() != tc.maxDoc {
				t.Errorf("read length: got %d, want %d", back.Length(), tc.maxDoc)
			}
			gotDelRead := tc.maxDoc - back.Cardinality()
			if gotDelRead != wantDel {
				t.Errorf("read delCount: got %d, want %d", gotDelRead, wantDel)
			}
			for doc := 0; doc < tc.maxDoc; doc++ {
				wantLive := back.Get(doc)
				deleted := false
				for _, d := range tc.deleted {
					if d == doc {
						deleted = true
						break
					}
				}
				if wantLive == deleted {
					t.Errorf("doc %d: read live=%v but deleted=%v", doc, wantLive, deleted)
				}
			}
		})
	}
}

// TestLiveDocs_ReadMissingReturnsNil asserts that reading a .liv file that does
// not exist returns (nil, nil), matching the "segment has no deletions at this
// generation" contract.
func TestLiveDocs_ReadMissingReturnsNil(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	id := make([]byte, 16)

	bits, err := readLiveDocs(dir, "_9", id, 5, 100)
	if err != nil {
		t.Fatalf("readLiveDocs(missing): unexpected error %v", err)
	}
	if bits != nil {
		t.Fatalf("readLiveDocs(missing): expected nil bits, got %v", bits)
	}
}

// TestLiveDocs_GoldenBytes locks the exact on-disk byte layout of a .liv file
// against a golden vector. The vector was captured from BOTH this writer and
// codecs.Lucene90LiveDocsFormat.WriteLiveDocsLucene90 for identical input
// (maxDoc=65, delGen=2, deleted={2,64}, segment ID 1..16); the two produced
// byte-identical files, proving Gocene's two .liv codepaths agree and match the
// Lucene 10.4.0 Lucene90LiveDocsFormat wire format (CodecUtil IndexHeader with
// codec "Lucene90LiveDocs" v0 + suffix "2", two little-endian Int64 live-docs
// words with bits 2 and 64 cleared, CodecUtil footer).
func TestLiveDocs_GoldenBytes(t *testing.T) {
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	maxDoc := 65
	live := makeLive(t, maxDoc, 2, 64)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	if _, err := writeLiveDocs(dir, "_0", id, 2, live); err != nil {
		t.Fatalf("writeLiveDocs: %v", err)
	}
	name := liveDocsFileName("_0", 2)
	n, err := dir.FileLength(name)
	if err != nil {
		t.Fatalf("FileLength: %v", err)
	}
	in, err := dir.OpenInput(name, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()
	got := make([]byte, n)
	if err := in.ReadBytes(got); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}

	// Golden vector: CodecMagic, codec name "Lucene90LiveDocs", version 0,
	// segment ID 1..16, suffix len 1 + "2", word0 with bit 2 cleared (0xFB...),
	// word1 with bit 0 (doc 64) cleared, then CodecUtil footer (magic
	// 0xC02893E8 big-endian = 192 40 147 232, algorithm id 0, 8-byte CRC).
	full := []byte{
		63, 215, 108, 23,
		16, 76, 117, 99, 101, 110, 101, 57, 48, 76, 105, 118, 101, 68, 111, 99, 115,
		0, 0, 0, 0,
		1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		1, 50,
		251, 255, 255, 255, 255, 255, 255, 255, // word0: bit 2 cleared (0xFB)
		0, 0, 0, 0, 0, 0, 0, 0, // word1: doc 64 (bit 0) cleared
		192, 40, 147, 232,
		0, 0, 0, 0, 0, 0, 0, 0,
		54, 64, 230, 209,
	}
	if len(got) != len(full) {
		t.Fatalf("file length: got %d, want %d\n got=%v", len(got), len(full), got)
	}
	for i := range full {
		if got[i] != full[i] {
			t.Fatalf("byte %d: got %d, want %d\n got=%v", i, got[i], full[i], got)
		}
	}
}

// TestLiveDocs_FileName verifies the base-36 generation suffix used by the .liv
// filename, matching IndexFileNames.fileNameFromGeneration with
// Character.MAX_RADIX (36).
func TestLiveDocs_FileName(t *testing.T) {
	cases := []struct {
		gen  int64
		want string
	}{
		{0, "_0.liv"},
		{1, "_0_1.liv"},
		{35, "_0_z.liv"},
		{36, "_0_10.liv"},
	}
	for _, tc := range cases {
		if got := liveDocsFileName("_0", tc.gen); got != tc.want {
			t.Errorf("liveDocsFileName(_0, %d): got %q, want %q", tc.gen, got, tc.want)
		}
	}
}
