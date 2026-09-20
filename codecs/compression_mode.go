// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

// This file carries what package codecs still needs from
// org.apache.lucene.codecs.compressing.CompressionMode, whose faithful port is
// codecs/compressing/compression_mode.go.
//
// It is the residue of codecs/compressing_stored_fields_format.go, which held a
// 402-line CompressingStoredFieldsFormat/Reader pair that Apache Lucene 10.5.0
// does not have at any address: `find /tmp/lucene -name
// "CompressingStoredFields*.java"` matches nothing, and
// org.apache.lucene.codecs.compressing contains only CompressionMode,
// Compressor, Decompressor and MatchingReaders. The real classes are
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFields{Format,Reader,Writer},
// now ported to codecs/lucene90/compressing. The fabricated pair has been
// removed; the declarations below are the ones that do answer to a Lucene
// artefact and that other files in this package depend on.

// CompressionMode is org.apache.lucene.codecs.compressing.CompressionMode.
//
// Lucene declares exactly one CompressionMode, an abstract class with
// newCompressor()/newDecompressor() and three static instances
// (CompressionMode.java:44 FAST, :68 HIGH_COMPRESSION, :94 FAST_DECOMPRESSION).
// Gocene's faithful port of that class lives in codecs/compressing; this alias
// keeps that single definition authoritative. The int enum that used to stand
// here was a second, divergent CompressionMode with a []byte-shaped
// decompressor() and no compressor() at all — a shape Lucene does not have.
type CompressionMode = compressing.CompressionMode

// The three CompressionMode singletons declared by the Java class.
var (
	// CompressionModeLZ4Fast is CompressionMode.FAST: an LZ4 fast compressor
	// paired with the shared LZ4_DECOMPRESSOR (CompressionMode.java:44-59).
	CompressionModeLZ4Fast = compressing.FAST

	// CompressionModeLZ4High is CompressionMode.FAST_DECOMPRESSION: an LZ4
	// high compressor paired with the same shared LZ4_DECOMPRESSOR
	// (CompressionMode.java:94-108).
	CompressionModeLZ4High = compressing.FAST_DECOMPRESSION

	// CompressionModeDeflate is CompressionMode.HIGH_COMPRESSION:
	// DeflateCompressor(6) with DeflateDecompressor
	// (CompressionMode.java:68-85).
	CompressionModeDeflate = compressing.HIGH_COMPRESSION
)

// lz4Decompress decompresses an LZ4 block produced by Apache Lucene.
//
// It is the adapter that gives this package's []byte-shaped decompressor
// signature access to the faithful port of org.apache.lucene.util.compress.LZ4
// in util/compress. The reference is the anonymous LZ4_DECOMPRESSOR held by
// org.apache.lucene.codecs.compressing.CompressionMode (Lucene 10.5.0,
// CompressionMode.java:118-141), which is the Decompressor returned by both
// CompressionMode.FAST and CompressionMode.FAST_DECOMPRESSION:
//
//	public void decompress(DataInput in, int originalLength, int offset,
//	                       int length, BytesRef bytes) throws IOException {
//	  assert offset + length <= originalLength;
//	  // add 7 padding bytes, this is not necessary but can help decompression run faster
//	  if (bytes.bytes.length < originalLength + 7) {
//	    bytes.bytes = new byte[ArrayUtil.oversize(originalLength + 7, 1)];
//	  }
//	  final int decompressedLength = LZ4.decompress(in, offset + length, bytes.bytes, 0);
//	  if (decompressedLength > originalLength) {
//	    throw new CorruptIndexException(
//	        "Corrupted: lengths mismatch: " + decompressedLength + " > " + originalLength, in);
//	  }
//	  bytes.offset = offset;
//	  bytes.length = length;
//	}
//
// This entry point carries the whole-block case (Java offset == 0 and
// length == originalLength), so Java's LZ4.decompress(in, offset + length, ...)
// becomes LZ4Decompress(in, uncompressedLen, ...). The 7 padding bytes and the
// "lengths mismatch" guard are reproduced exactly.
//
// LZ4.decompress is DataInput-based in Java and so is its Go port, so the
// []byte block is wrapped in a store.ByteArrayDataInput rather than the LZ4
// decoder being written a second time against a slice.
func lz4Decompress(data []byte, uncompressedLen int) ([]byte, error) {
	dest := make([]byte, uncompressedLen+lz4DecompressorPadding)
	in := store.NewByteArrayDataInput(data)
	decompressedLength, err := compress.LZ4Decompress(in, uncompressedLen, dest, 0)
	if err != nil {
		return nil, err
	}
	if decompressedLength > uncompressedLen {
		return nil, fmt.Errorf("Corrupted: lengths mismatch: %d > %d", decompressedLength, uncompressedLen)
	}
	return dest[:decompressedLength], nil
}

// deflateDecompress inflates a raw DEFLATE payload produced by Apache Lucene.
//
// The reference is DeflateDecompressor.decompress in
// org.apache.lucene.codecs.compressing.CompressionMode (Lucene 10.5.0,
// CompressionMode.java:186-241), whose decoder is:
//
//	final Inflater decompressor = new Inflater(true);
//
// The `true` argument is java.util.zip's "nowrap" mode: RAW DEFLATE, with no
// zlib wrapper. Lucene writes the payload with `new Deflater(level, true)`, so
// the stream carries neither the 2-byte zlib header nor the 4-byte Adler-32
// trailer. Go's counterpart of a nowrap Inflater is compress/flate; using
// compress/zlib here — as this function previously did — makes every
// Lucene-written HIGH_COMPRESSION chunk fail to parse, because zlib.NewReader
// demands a header Lucene never emits.
//
// Java also appends one dummy padding byte before inflating ("we do it for
// compliance, but it's unnecessary for years in zlib"). compress/flate does not
// require it and its presence or absence does not change the decoded output, so
// no padding byte is added here.
func deflateDecompress(data []byte, uncompressedLen int) ([]byte, error) {
	result := make([]byte, uncompressedLen)
	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()
	n, err := io.ReadFull(r, result)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if n != uncompressedLen {
		return nil, fmt.Errorf("Lengths mismatch: %d != %d", n, uncompressedLen)
	}
	return result, nil
}
