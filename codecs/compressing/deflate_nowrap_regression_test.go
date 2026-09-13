// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// luceneHighCompressionPayload frames `data` exactly as Java's
// DeflateCompressor.compress does in
// org.apache.lucene.codecs.compressing.CompressionMode (Lucene 10.5.0,
// CompressionMode.java:243-300):
//
//	DeflateCompressor(int level) { compressor = new Deflater(level, true); }
//	...
//	out.writeVInt(totalCount);
//	out.writeBytes(compressed, totalCount);
//
// `new Deflater(level, true)` is java.util.zip's "nowrap" mode: the emitted
// stream is RAW DEFLATE, carrying neither the 2-byte zlib header nor the
// 4-byte Adler-32 trailer. Go's compress/flate is the exact counterpart.
func luceneHighCompressionPayload(t *testing.T, data []byte) []byte {
	t.Helper()
	var raw bytes.Buffer
	// Level 6 is the level Lucene's HIGH_COMPRESSION uses
	// (CompressionMode.java:76 "new DeflateCompressor(6)").
	w, err := flate.NewWriter(&raw, 6)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	out := store.NewByteBuffersDataOutput()
	if err := out.WriteVInt(int32(raw.Len())); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteBytes(raw.Bytes(), 0, raw.Len()); err != nil {
		t.Fatal(err)
	}
	return out.ToArrayCopy()
}

// TestHighCompressionDecodesRawDeflate is the regression guard for the defect
// in which a Deflate decompressor was built on compress/zlib instead of
// compress/flate.
//
// Lucene writes HIGH_COMPRESSION chunks with a nowrap Deflater, so the bytes on
// disk are raw DEFLATE. A zlib reader demands a 2-byte header Lucene never
// emits and therefore fails on every Lucene-written chunk. This test pins both
// halves of that fact:
//
//  1. the ported decompressor recovers the payload byte-for-byte, and
//  2. a zlib reader genuinely cannot read the same bytes — so the assertion in
//     (1) is not vacuous and a regression to zlib would be caught here.
func TestHighCompressionDecodesRawDeflate(t *testing.T) {
	original := []byte(
		"lucene document number 0 with some words for testing postings; " +
			"repeated repeated repeated repeated so DEFLATE actually compresses")

	payload := luceneHighCompressionPayload(t, original)

	// (1) The ported decompressor must recover the payload exactly.
	dec := HIGH_COMPRESSION.NewDecompressor()
	var got util.BytesRef
	if err := dec.Decompress(store.NewByteArrayDataInput(payload), len(original), 0, len(original), &got); err != nil {
		t.Fatalf("HIGH_COMPRESSION.Decompress on a raw-DEFLATE payload: %v", err)
	}
	recovered := got.Bytes[got.Offset : got.Offset+got.Length]
	if !bytes.Equal(recovered, original) {
		t.Fatalf("payload mismatch:\n got %q\nwant %q", recovered, original)
	}

	// (2) Prove the payload really is nowrap DEFLATE: a zlib reader must
	// reject it. Without this, (1) could pass even for a zlib-framed stream
	// and the regression guard would be vacuous.
	//
	// Skip the vInt length prefix to hand zlib exactly the compressed bytes.
	in := store.NewByteArrayDataInput(payload)
	n, err := in.ReadVInt()
	if err != nil {
		t.Fatal(err)
	}
	rawOnly := payload[len(payload)-int(n):]
	if _, err := zlib.NewReader(bytes.NewReader(rawOnly)); err == nil {
		t.Fatal("zlib.NewReader accepted the raw-DEFLATE payload; " +
			"the stream is not nowrap and this regression guard is vacuous")
	}
}

// TestHighCompressionRejectsZlibFraming is the mirror image: bytes carrying a
// zlib wrapper are not what Lucene writes, and must not be decoded as if they
// were. This pins the decompressor to the nowrap contract from the other side.
func TestHighCompressionRejectsZlibFraming(t *testing.T) {
	original := []byte("lucene document number 0 with some words for testing postings")

	var wrapped bytes.Buffer
	zw := zlib.NewWriter(&wrapped)
	if _, err := zw.Write(original); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	out := store.NewByteBuffersDataOutput()
	if err := out.WriteVInt(int32(wrapped.Len())); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteBytes(wrapped.Bytes(), 0, wrapped.Len()); err != nil {
		t.Fatal(err)
	}

	dec := HIGH_COMPRESSION.NewDecompressor()
	var got util.BytesRef
	err := dec.Decompress(store.NewByteArrayDataInput(out.ToArrayCopy()), len(original), 0, len(original), &got)
	if err == nil {
		recovered := got.Bytes[got.Offset : got.Offset+got.Length]
		if bytes.Equal(recovered, original) {
			t.Fatal("zlib-framed bytes decoded successfully; the decompressor " +
				"is not pinned to Lucene's nowrap DEFLATE framing")
		}
	}
}

// TestCompressionModeSingletonsMatchJava pins the three CompressionMode
// instances Lucene declares, and the compressor/decompressor each one hands
// out. The Java reference is CompressionMode.java (Lucene 10.5.0):
//
//	FAST               -> new LZ4FastCompressor()  + LZ4_DECOMPRESSOR   (:44)
//	HIGH_COMPRESSION   -> new DeflateCompressor(6) + DeflateDecompressor (:68)
//	FAST_DECOMPRESSION -> new LZ4HighCompressor()  + LZ4_DECOMPRESSOR   (:94)
//
// The shape matters as much as the names: Java's CompressionMode is an abstract
// class with newCompressor() AND newDecompressor(). A CompressionMode modelled
// as an int enum exposing only a decompressor cannot express FAST versus
// FAST_DECOMPRESSION at all, because those two differ only in their compressor.
func TestCompressionModeSingletonsMatchJava(t *testing.T) {
	for _, tc := range []struct {
		mode CompressionMode
		name string
	}{
		{FAST, "FAST"},
		{HIGH_COMPRESSION, "HIGH_COMPRESSION"},
		{FAST_DECOMPRESSION, "FAST_DECOMPRESSION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.mode.String(); got != tc.name {
				t.Fatalf("String() = %q, want %q", got, tc.name)
			}
			c := tc.mode.NewCompressor()
			if c == nil {
				t.Fatal("NewCompressor() returned nil")
			}
			defer c.Close()
			if d := tc.mode.NewDecompressor(); d == nil {
				t.Fatal("NewDecompressor() returned nil")
			}
		})
	}

	// FAST and FAST_DECOMPRESSION share one decompressor implementation but
	// must NOT share a compressor: that distinction is the whole point of the
	// two singletons (CompressionMode.java:52-54 vs :101-103).
	fastC := FAST.NewCompressor()
	defer fastC.Close()
	fastDecC := FAST_DECOMPRESSION.NewCompressor()
	defer fastDecC.Close()
	if _, ok := fastC.(*lz4FastCompressor); !ok {
		t.Fatalf("FAST.NewCompressor() = %T, want *lz4FastCompressor", fastC)
	}
	if _, ok := fastDecC.(*lz4HighCompressor); !ok {
		t.Fatalf("FAST_DECOMPRESSION.NewCompressor() = %T, want *lz4HighCompressor", fastDecC)
	}
	if _, ok := HIGH_COMPRESSION.NewCompressor().(*deflateCompressor); !ok {
		t.Fatal("HIGH_COMPRESSION.NewCompressor() is not the deflate compressor")
	}
}

// TestCompressionModeRoundTrips exercises every mode end to end through the
// Compressor/Decompressor contract Java declares, so that a change to either
// half of a pair is caught.
func TestCompressionModeRoundTrips(t *testing.T) {
	original := []byte(
		"lucene document number 0 with some words for testing postings " +
			"lucene document number 1 with some words for testing postings " +
			"lucene document number 2 with some words for testing postings")

	for _, tc := range []struct {
		mode CompressionMode
		name string
	}{
		{FAST, "FAST"},
		{HIGH_COMPRESSION, "HIGH_COMPRESSION"},
		{FAST_DECOMPRESSION, "FAST_DECOMPRESSION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.mode.NewCompressor()
			defer c.Close()
			out := store.NewByteBuffersDataOutput()
			if err := c.Compress(store.NewByteBuffersDataInput(original), out); err != nil {
				t.Fatalf("Compress: %v", err)
			}

			var got util.BytesRef
			err := tc.mode.NewDecompressor().Decompress(
				store.NewByteArrayDataInput(out.ToArrayCopy()),
				len(original), 0, len(original), &got)
			if err != nil {
				t.Fatalf("Decompress: %v", err)
			}
			recovered := got.Bytes[got.Offset : got.Offset+got.Length]
			if !bytes.Equal(recovered, original) {
				t.Fatalf("round trip mismatch:\n got %q\nwant %q", recovered, original)
			}
		})
	}
}
