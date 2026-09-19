// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/compressing/CompressionMode.java

package compressing

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

// CompressionMode is a compression mode. It tells how much effort should be
// spent on compression and decompression of stored fields.
//
// Mirrors the abstract class
// org.apache.lucene.backward_codecs.compressing.CompressionMode (Lucene
// 10.5.0). Go has no abstract classes, so the type is an interface; the three
// Java singletons are the package variables [FAST], [HIGH_COMPRESSION] and
// [FAST_DECOMPRESSION].
//
// String() renders the toString() each anonymous subclass overrides; Java
// inherits Object.toString on the type itself, so the member is declared here
// because every instance that exists supplies it.
type CompressionMode interface {
	// NewCompressor creates a new Compressor instance.
	//
	// Mirrors {@code public abstract Compressor newCompressor()}.
	NewCompressor() Compressor

	// NewDecompressor creates a new Decompressor instance.
	//
	// Mirrors {@code public abstract Decompressor newDecompressor()}.
	NewDecompressor() Decompressor

	// String renders toString().
	String() string
}

// FAST is a compression mode that trades compression ratio for speed.
// Although the compression ratio might remain high, compression and
// decompression are very fast. Use this mode with indices that have a high
// update rate but should be able to load documents from disk quickly.
//
// Mirrors {@code public static final CompressionMode FAST}.
var FAST CompressionMode = fastMode{}

// HIGH_COMPRESSION is a compression mode that trades speed for compression
// ratio. Although compression and decompression might be slow, this
// compression mode should provide a good compression ratio. This mode might
// be interesting if/when your index size is much bigger than your OS cache.
//
// Mirrors {@code public static final CompressionMode HIGH_COMPRESSION}.
var HIGH_COMPRESSION CompressionMode = highCompressionMode{}

// FAST_DECOMPRESSION is similar to [FAST] but it spends more time compressing
// in order to improve the compression ratio. This compression mode is best
// used with indices that have a low update rate but should be able to load
// documents from disk quickly.
//
// Mirrors {@code public static final CompressionMode FAST_DECOMPRESSION}.
var FAST_DECOMPRESSION CompressionMode = fastDecompressionMode{}

// fastMode is the anonymous CompressionMode assigned to FAST.
type fastMode struct{}

// NewCompressor returns {@code new LZ4FastCompressor()}.
func (fastMode) NewCompressor() Compressor { return newLZ4FastCompressor() }

// NewDecompressor returns {@code LZ4_DECOMPRESSOR}.
func (fastMode) NewDecompressor() Decompressor { return lz4Decompressor{} }

// String returns "FAST".
func (fastMode) String() string { return "FAST" }

// highCompressionMode is the anonymous CompressionMode assigned to
// HIGH_COMPRESSION.
type highCompressionMode struct{}

// NewCompressor returns {@code new DeflateCompressor(6)}.
//
// Java's comment on the level: "3 is the highest level that doesn't have lazy
// match evaluation; 6 is the default, higher than that is just a waste of
// cpu".
func (highCompressionMode) NewCompressor() Compressor { return newDeflateCompressor(6) }

// NewDecompressor returns {@code new DeflateDecompressor()}.
func (highCompressionMode) NewDecompressor() Decompressor { return newDeflateDecompressor() }

// String returns "HIGH_COMPRESSION".
func (highCompressionMode) String() string { return "HIGH_COMPRESSION" }

// fastDecompressionMode is the anonymous CompressionMode assigned to
// FAST_DECOMPRESSION.
type fastDecompressionMode struct{}

// NewCompressor returns {@code new LZ4HighCompressor()}.
func (fastDecompressionMode) NewCompressor() Compressor { return newLZ4HighCompressor() }

// NewDecompressor returns {@code LZ4_DECOMPRESSOR}.
func (fastDecompressionMode) NewDecompressor() Decompressor { return lz4Decompressor{} }

// String returns "FAST_DECOMPRESSION".
func (fastDecompressionMode) String() string { return "FAST_DECOMPRESSION" }

// ─── LZ4_DECOMPRESSOR ───────────────────────────────────────────────────────

// lz4Decompressor is the anonymous Decompressor assigned to the private
// static field LZ4_DECOMPRESSOR. It carries no state, which is why Java's
// clone() returns {@code this}.
type lz4Decompressor struct{}

// Decompress reproduces
//
//	assert offset + length <= originalLength;
//	// add 7 padding bytes, this is not necessary but can help decompression run faster
//	if (bytes.bytes.length < originalLength + 7) {
//	  bytes.bytes = new byte[ArrayUtil.oversize(originalLength + 7, 1)];
//	}
//	final int decompressedLength = LZ4.decompress(in, offset + length, bytes.bytes, 0);
//	if (decompressedLength > originalLength) {
//	  throw new CorruptIndexException("Corrupted: lengths mismatch: ...", in);
//	}
//	bytes.offset = offset;
//	bytes.length = length;
func (lz4Decompressor) Decompress(in store.DataInput, originalLength, offset, length int, b *util.BytesRef) error {
	if b == nil {
		return errors.New("backward_codecs/compressing: BytesRef destination is nil")
	}
	if offset+length > originalLength {
		return fmt.Errorf("backward_codecs/compressing: invalid window offset=%d length=%d originalLength=%d",
			offset, length, originalLength)
	}
	// add 7 padding bytes, this is not necessary but can help decompression
	// run faster
	if len(b.Bytes) < originalLength+7 {
		b.Bytes = make([]byte, util.Oversize(originalLength+7, 1))
	}
	decompressedLength, err := compress.LZ4Decompress(in, offset+length, b.Bytes, 0)
	if err != nil {
		return err
	}
	if decompressedLength > originalLength {
		return fmt.Errorf("%w: %d > %d", ErrCorruptLengthsMismatch, decompressedLength, originalLength)
	}
	b.Offset = offset
	b.Length = length
	return nil
}

// Clone reproduces {@code return this;}.
func (d lz4Decompressor) Clone() Decompressor { return d }

// ErrCorruptLengthsMismatch renders the CorruptIndexException raised by the
// LZ4 and DEFLATE decompressors when the decoded byte count contradicts the
// originalLength recorded at compression time.
var ErrCorruptLengthsMismatch = errors.New("backward_codecs/compressing: corrupted: lengths mismatch")

// ─── LZ4FastCompressor ──────────────────────────────────────────────────────

// lz4FastCompressor mirrors the private static final nested class
// CompressionMode.LZ4FastCompressor.
type lz4FastCompressor struct {
	ht *compress.FastCompressionHashTable
}

// newLZ4FastCompressor reproduces {@code ht = new LZ4.FastCompressionHashTable();}.
func newLZ4FastCompressor() *lz4FastCompressor {
	return &lz4FastCompressor{ht: compress.NewFastCompressionHashTable()}
}

// Compress reproduces {@code LZ4.compress(bytes, off, len, out, ht);}.
func (c *lz4FastCompressor) Compress(b []byte, off, length int, out store.DataOutput) error {
	return compress.LZ4Compress(b, off, length, out, c.ht)
}

// Close is the Java no-op.
func (c *lz4FastCompressor) Close() error { return nil }

// ─── LZ4HighCompressor ──────────────────────────────────────────────────────

// lz4HighCompressor mirrors the private static final nested class
// CompressionMode.LZ4HighCompressor.
type lz4HighCompressor struct {
	ht *compress.HighCompressionHashTable
}

// newLZ4HighCompressor reproduces {@code ht = new LZ4.HighCompressionHashTable();}.
func newLZ4HighCompressor() *lz4HighCompressor {
	return &lz4HighCompressor{ht: compress.NewHighCompressionHashTable()}
}

// Compress reproduces {@code LZ4.compress(bytes, off, len, out, ht);}.
func (c *lz4HighCompressor) Compress(b []byte, off, length int, out store.DataOutput) error {
	return compress.LZ4Compress(b, off, length, out, c.ht)
}

// Close is the Java no-op.
func (c *lz4HighCompressor) Close() error { return nil }

// ─── DeflateDecompressor ────────────────────────────────────────────────────

// deflateDecompressor mirrors the private static final nested class
// CompressionMode.DeflateDecompressor.
type deflateDecompressor struct {
	compressed []byte
}

// newDeflateDecompressor reproduces {@code compressed = new byte[0];}.
func newDeflateDecompressor() *deflateDecompressor {
	return &deflateDecompressor{compressed: make([]byte, 0)}
}

// Decompress reproduces DeflateDecompressor.decompress.
//
// Java pads the compressed run with one trailing zero byte because
// java.util.zip.Inflater(true) demands it; Go's compress/flate reader does
// not, so the padding byte is written into the scratch buffer (keeping the
// buffer sizes identical to Java's) but is not handed to the reader. The
// bytes consumed from in, and the bytes produced, are unchanged.
func (d *deflateDecompressor) Decompress(in store.DataInput, originalLength, offset, length int, b *util.BytesRef) error {
	if b == nil {
		return errors.New("backward_codecs/compressing: BytesRef destination is nil")
	}
	if offset+length > originalLength {
		return fmt.Errorf("backward_codecs/compressing: invalid window offset=%d length=%d originalLength=%d",
			offset, length, originalLength)
	}
	if length == 0 {
		b.Length = 0
		return nil
	}
	compressedLength32, err := in.ReadVInt()
	if err != nil {
		return err
	}
	compressedLength := int(compressedLength32)
	if compressedLength < 0 {
		return fmt.Errorf("backward_codecs/compressing: invalid negative compressedLength=%d", compressedLength)
	}
	// pad with extra "dummy byte": see javadocs for using Inflater(true)
	// we do it for compliance, but it's unnecessary for years in zlib.
	paddedLength := compressedLength + 1
	d.compressed = growNoCopyByte(d.compressed, paddedLength)
	if compressedLength > 0 {
		if err := in.ReadBytes(d.compressed, 0, compressedLength); err != nil {
			return err
		}
	}
	d.compressed[compressedLength] = 0 // explicitly set dummy byte to 0

	b.Offset, b.Length = 0, 0
	b.Bytes = growNoCopyByte(b.Bytes, originalLength)

	r := flate.NewReader(bytes.NewReader(d.compressed[:compressedLength]))
	n, err := io.ReadFull(r, b.Bytes[:originalLength])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		_ = r.Close()
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	b.Length = n
	if b.Length != originalLength {
		return fmt.Errorf("%w: %d != %d", ErrCorruptLengthsMismatch, b.Length, originalLength)
	}
	b.Offset = offset
	b.Length = length
	return nil
}

// Clone reproduces {@code return new DeflateDecompressor();}.
func (d *deflateDecompressor) Clone() Decompressor { return newDeflateDecompressor() }

// ─── DeflateCompressor ──────────────────────────────────────────────────────

// deflateCompressor mirrors the private static nested class
// CompressionMode.DeflateCompressor.
type deflateCompressor struct {
	level      int
	compressor *flate.Writer
	compressed []byte
	closed     bool
}

// newDeflateCompressor reproduces
//
//	compressor = new Deflater(level, true);
//	compressed = new byte[64];
func newDeflateCompressor(level int) *deflateCompressor {
	w, err := flate.NewWriter(io.Discard, level)
	if err != nil {
		panic(fmt.Sprintf("backward_codecs/compressing: flate.NewWriter(level=%d): %v", level, err))
	}
	return &deflateCompressor{level: level, compressor: w, compressed: make([]byte, 64)}
}

// Compress reproduces DeflateCompressor.compress: the deflater is reset, fed
// the window, finished, and the resulting raw DEFLATE run is written as a
// VInt length followed by the bytes. The Java loop that grows `compressed`
// until the deflater reports finished() is expressed here by letting the
// bytes.Buffer grow, which produces the identical byte run.
func (c *deflateCompressor) Compress(b []byte, off, length int, out store.DataOutput) error {
	if c.closed {
		return errors.New("backward_codecs/compressing: deflate compressor closed")
	}
	if length == 0 {
		// Java: compressor.needsInput() is true only for an empty window, and
		// the body then asserts len == 0 and writes a zero-length payload.
		return out.WriteVInt(0)
	}
	var buf bytes.Buffer
	c.compressor.Reset(&buf)
	if _, err := c.compressor.Write(b[off : off+length]); err != nil {
		return err
	}
	if err := c.compressor.Close(); err != nil {
		return err
	}
	c.compressed = growNoCopyByte(c.compressed, buf.Len())
	totalCount := copy(c.compressed, buf.Bytes())
	if err := out.WriteVInt(int32(totalCount)); err != nil {
		return err
	}
	return out.WriteBytes(c.compressed, 0, totalCount)
}

// Close reproduces
//
//	if (closed == false) { compressor.end(); closed = true; }
func (c *deflateCompressor) Close() error {
	if !c.closed {
		c.compressor = nil
		c.closed = true
	}
	return nil
}

// ─── helpers ────────────────────────────────────────────────────────────────

// growNoCopyByte renders org.apache.lucene.util.ArrayUtil#growNoCopy(byte[],
// int): a new array of ArrayUtil.oversize(minSize, 1) elements when the
// current one is too small, and the current one otherwise. The contents are
// not preserved, exactly as the Java name states.
func growNoCopyByte(array []byte, minSize int) []byte {
	if len(array) < minSize {
		return make([]byte, util.Oversize(minSize, 1))
	}
	return array
}
