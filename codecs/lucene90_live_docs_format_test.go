// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Mirrors selected cases from
// org.apache.lucene.codecs.lucene90.TestLucene90LiveDocsFormat (Lucene 10.4.0).

package codecs_test

import (
	"crypto/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestLucene90LiveDocsFormat_RoundTrip writes a dense live-docs bitset
// to the directory, reads it back, and asserts every doc's live/deleted
// status matches.
func TestLucene90LiveDocsFormat_RoundTrip(t *testing.T) {
	const maxDoc = 1000
	bits, _ := util.NewFixedBitSet(maxDoc)
	// Mark every doc live initially.
	for i := 0; i < maxDoc; i++ {
		bits.Set(i)
	}
	// Delete a few.
	deleted := []int{0, 1, 17, 256, 999}
	for _, d := range deleted {
		bits.Clear(d)
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	si := newSegmentForTest(t, "_0", maxDoc, dir)

	format := codecs.NewLucene90LiveDocsFormat()
	if err := format.WriteLiveDocsLucene90(bits, dir, si, 0, len(deleted), len(deleted)); err != nil {
		t.Fatalf("WriteLiveDocsLucene90: %v", err)
	}

	got, delCount, err := format.ReadLiveDocsLucene90(dir, si, 0, len(deleted), maxDoc)
	if err != nil {
		t.Fatalf("ReadLiveDocsLucene90: %v", err)
	}
	if delCount != len(deleted) {
		t.Fatalf("delCount = %d, want %d", delCount, len(deleted))
	}
	for i := 0; i < maxDoc; i++ {
		want := bits.Get(i)
		if got.Get(i) != want {
			t.Fatalf("bit %d: got %v want %v", i, got.Get(i), want)
		}
	}
}

// TestLucene90LiveDocsFormat_RoundTripWithGen exercises a non-zero
// del-generation, ensuring the file-name encoding and header suffix
// agree end-to-end.
func TestLucene90LiveDocsFormat_RoundTripWithGen(t *testing.T) {
	const maxDoc = 128
	bits, _ := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		bits.Set(i)
	}
	bits.Clear(42)

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	si := newSegmentForTest(t, "_1", maxDoc, dir)

	format := codecs.NewLucene90LiveDocsFormat()
	const gen = int64(7)
	if err := format.WriteLiveDocsLucene90(bits, dir, si, gen, 1, 1); err != nil {
		t.Fatalf("WriteLiveDocsLucene90: %v", err)
	}
	got, delCount, err := format.ReadLiveDocsLucene90(dir, si, gen, 1, maxDoc)
	if err != nil {
		t.Fatalf("ReadLiveDocsLucene90: %v", err)
	}
	if delCount != 1 {
		t.Fatalf("delCount = %d, want 1", delCount)
	}
	if got.Get(42) {
		t.Fatal("doc 42 should be deleted")
	}
	if !got.Get(41) || !got.Get(43) {
		t.Fatal("doc 41/43 should still be live")
	}
}

// TestLucene90LiveDocsFormat_DelCountMismatch verifies the writer rejects
// a request whose expected del-count disagrees with the bits content.
func TestLucene90LiveDocsFormat_DelCountMismatch(t *testing.T) {
	const maxDoc = 32
	bits, _ := util.NewFixedBitSet(maxDoc)
	for i := 0; i < maxDoc; i++ {
		bits.Set(i)
	}
	bits.Clear(0) // one deletion
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()
	si := newSegmentForTest(t, "_2", maxDoc, dir)
	format := codecs.NewLucene90LiveDocsFormat()
	// Claim 2 deletions but write only 1.
	if err := format.WriteLiveDocsLucene90(bits, dir, si, 0, 2, 0); err == nil {
		t.Fatal("expected del-count mismatch error")
	}
}

func newSegmentForTest(t *testing.T, name string, maxDoc int, dir store.Directory) *index.SegmentInfo {
	t.Helper()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	si := index.NewSegmentInfo(name, maxDoc, dir)
	if err := si.SetID(id); err != nil {
		t.Fatal(err)
	}
	return si
}
