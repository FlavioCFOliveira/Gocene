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
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/lucene87/DeflateWithPresetDictCompressionMode.java

package lucene87

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/backward_codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	// deflatePresetDictNumSubBlocks mirrors {@code private static final int
	// NUM_SUB_BLOCKS = 10} — "Shoot for 10 sub blocks".
	deflatePresetDictNumSubBlocks = 10
	// deflatePresetDictSizeFactor mirrors {@code private static final int
	// DICT_SIZE_FACTOR = 6} — "And a dictionary whose size is about 6x
	// smaller than sub blocks".
	deflatePresetDictSizeFactor = 6
	// deflatePresetDictLevel is the level newCompressor() passes: Java's
	// comment reads "3 is the highest level that doesn't have lazy match
	// evaluation; 6 is the default, higher than that is just a waste of cpu".
	deflatePresetDictLevel = 6
)

// DeflateWithPresetDictCompressionMode is a compression mode that trades speed
// for compression ratio. Although compression and decompression might be slow,
// this compression mode should provide a good compression ratio. This mode
// might be interesting if/when your index size is much bigger than your OS
// cache.
//
// Mirrors {@code public final class DeflateWithPresetDictCompressionMode
// extends CompressionMode} (Lucene 10.5.0, @lucene.internal).
type DeflateWithPresetDictCompressionMode struct{}

// NewDeflateWithPresetDictCompressionMode is the sole constructor.
//
// Mirrors {@code public DeflateWithPresetDictCompressionMode() {}}.
func NewDeflateWithPresetDictCompressionMode() DeflateWithPresetDictCompressionMode {
	return DeflateWithPresetDictCompressionMode{}
}

// NewCompressor reproduces
// {@code return new DeflateWithPresetDictCompressor(6);}.
func (DeflateWithPresetDictCompressionMode) NewCompressor() compressing.Compressor {
	return newDeflateWithPresetDictCompressor(deflatePresetDictLevel)
}

// NewDecompressor reproduces
// {@code return new DeflateWithPresetDictDecompressor();}.
func (DeflateWithPresetDictCompressionMode) NewDecompressor() compressing.Decompressor {
	return newDeflateWithPresetDictDecompressor()
}

// String reproduces {@code return "BEST_COMPRESSION";}.
func (DeflateWithPresetDictCompressionMode) String() string { return "BEST_COMPRESSION" }

var _ compressing.CompressionMode = DeflateWithPresetDictCompressionMode{}

// ─── DeflateWithPresetDictDecompressor ──────────────────────────────────────

// deflateWithPresetDictDecompressor mirrors the private static final nested
// class DeflateWithPresetDictCompressionMode.DeflateWithPresetDictDecompressor.
type deflateWithPresetDictDecompressor struct {
	compressed []byte
}

// newDeflateWithPresetDictDecompressor reproduces
// {@code compressed = new byte[0];}.
func newDeflateWithPresetDictDecompressor() *deflateWithPresetDictDecompressor {
	return &deflateWithPresetDictDecompressor{compressed: make([]byte, 0)}
}

// doDecompress reproduces the private helper
// {@code doDecompress(DataInput in, Inflater decompressor, BytesRef bytes)}:
// read the length-prefixed compressed block and append the inflated bytes to
// bytes.
//
// Java pads the compressed run with one trailing zero byte, a historical
// requirement of java.util.zip.Inflater(true); Go's compress/flate reader does
// not need it, so the padding byte is written into the scratch buffer (keeping
// the buffer sizes identical to Java's) but is not handed to the reader. The
// preset dictionary Java installs with Inflater.setDictionary is passed here
// through flate.NewReaderDict, which is the same seeding.
func (d *deflateWithPresetDictDecompressor) doDecompress(
	in store.DataInput, dict []byte, b *util.BytesRef,
) error {
	compressedLength32, err := in.ReadVInt()
	if err != nil {
		return err
	}
	compressedLength := int(compressedLength32)
	if compressedLength == 0 {
		return nil
	}
	if compressedLength < 0 {
		return fmt.Errorf("lucene87: invalid negative compressedLength=%d", compressedLength)
	}
	// pad with extra "dummy byte": see javadocs for using Inflater(true)
	// we do it for compliance, but it's unnecessary for years in zlib.
	paddedLength := compressedLength + 1
	d.compressed = growByte(d.compressed, paddedLength)
	if err := in.ReadBytes(d.compressed, 0, compressedLength); err != nil {
		return err
	}
	d.compressed[compressedLength] = 0 // explicitly set dummy byte to 0

	var r io.ReadCloser
	if len(dict) == 0 {
		r = flate.NewReader(bytes.NewReader(d.compressed[:compressedLength]))
	} else {
		r = flate.NewReaderDict(bytes.NewReader(d.compressed[:compressedLength]), dict)
	}
	available := len(b.Bytes) - b.Length
	if available <= 0 {
		return errors.New("lucene87: BytesRef has no room left for inflate output")
	}
	n, err := io.ReadFull(r, b.Bytes[b.Length:b.Length+available])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		_ = r.Close()
		return err
	}
	if err := r.Close(); err != nil {
		return err
	}
	b.Length += n
	return nil
}

// Decompress reproduces DeflateWithPresetDictDecompressor.decompress.
func (d *deflateWithPresetDictDecompressor) Decompress(
	in store.DataInput, originalLength, offset, length int, b *util.BytesRef,
) error {
	if b == nil {
		return errors.New("lucene87: BytesRef destination is nil")
	}
	if offset+length > originalLength {
		return fmt.Errorf("lucene87: invalid window offset=%d length=%d originalLength=%d",
			offset, length, originalLength)
	}
	if length == 0 {
		b.Length = 0
		return nil
	}
	dictLength32, err := in.ReadVInt()
	if err != nil {
		return err
	}
	blockLength32, err := in.ReadVInt()
	if err != nil {
		return err
	}
	dictLength := int(dictLength32)
	blockLength := int(blockLength32)
	b.Bytes = growByte(b.Bytes, dictLength)
	b.Offset, b.Length = 0, 0

	// Read the dictionary
	if err := d.doDecompress(in, nil, b); err != nil {
		return err
	}
	if dictLength != b.Length {
		return fmt.Errorf("lucene87: Unexpected dict length: got %d want %d", b.Length, dictLength)
	}
	// Java keeps the dictionary in bytes.bytes[0:dictLength] and re-seeds the
	// Inflater from it on every block; Go's flate.NewReaderDict copies the
	// window at construction, so the same bytes are snapshotted once.
	dict := make([]byte, dictLength)
	copy(dict, b.Bytes[:dictLength])

	offsetInBlock := dictLength
	offsetInBytesRef := offset

	// Skip unneeded blocks
	for offsetInBlock+blockLength < offset {
		compressedLength, err := in.ReadVInt()
		if err != nil {
			return err
		}
		if err := in.SkipBytes(int64(compressedLength)); err != nil {
			return err
		}
		offsetInBlock += blockLength
		offsetInBytesRef -= blockLength
	}

	// Read blocks that intersect with the interval we need
	for offsetInBlock < offset+length {
		b.Bytes = growByte(b.Bytes, b.Length+blockLength)
		if err := d.doDecompress(in, dict, b); err != nil {
			return err
		}
		offsetInBlock += blockLength
	}

	b.Offset = offsetInBytesRef
	b.Length = length
	return nil
}

// Clone reproduces {@code return new DeflateWithPresetDictDecompressor();}.
func (d *deflateWithPresetDictDecompressor) Clone() compressing.Decompressor {
	return newDeflateWithPresetDictDecompressor()
}

// ─── DeflateWithPresetDictCompressor ────────────────────────────────────────

// deflateWithPresetDictCompressor mirrors the private static nested class
// DeflateWithPresetDictCompressionMode.DeflateWithPresetDictCompressor.
type deflateWithPresetDictCompressor struct {
	level      int
	compressed []byte
	closed     bool
}

// newDeflateWithPresetDictCompressor reproduces
//
//	compressor = new Deflater(level, true);
//	compressed = new byte[64];
func newDeflateWithPresetDictCompressor(level int) *deflateWithPresetDictCompressor {
	return &deflateWithPresetDictCompressor{level: level, compressed: make([]byte, 64)}
}

// doCompress reproduces the private helper
// {@code doCompress(byte[] bytes, int off, int len, DataOutput out)}: deflate
// the window and emit "VInt(compressedLen) || compressedBytes". The dict
// argument carries the window Java installs beforehand with
// Deflater.setDictionary, which Go supplies through flate.NewWriterDict.
func (c *deflateWithPresetDictCompressor) doCompress(
	b []byte, off, length int, dict []byte, out store.DataOutput,
) error {
	if length == 0 {
		return out.WriteVInt(0)
	}
	var buf bytes.Buffer
	var (
		w   *flate.Writer
		err error
	)
	if len(dict) == 0 {
		w, err = flate.NewWriter(&buf, c.level)
	} else {
		w, err = flate.NewWriterDict(&buf, c.level, dict)
	}
	if err != nil {
		return fmt.Errorf("lucene87: flate.NewWriter(level=%d): %w", c.level, err)
	}
	if _, err := w.Write(b[off : off+length]); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	c.compressed = growByte(c.compressed, buf.Len())
	totalCount := copy(c.compressed, buf.Bytes())

	if err := out.WriteVInt(int32(totalCount)); err != nil {
		return err
	}
	return out.WriteBytes(c.compressed, 0, totalCount)
}

// Compress reproduces DeflateWithPresetDictCompressor.compress.
func (c *deflateWithPresetDictCompressor) Compress(b []byte, off, length int, out store.DataOutput) error {
	if c.closed {
		return errors.New("lucene87: deflate compressor closed")
	}
	dictLength := length / (deflatePresetDictNumSubBlocks * deflatePresetDictSizeFactor)
	blockLength := (length - dictLength + deflatePresetDictNumSubBlocks - 1) / deflatePresetDictNumSubBlocks
	if err := out.WriteVInt(int32(dictLength)); err != nil {
		return err
	}
	if err := out.WriteVInt(int32(blockLength)); err != nil {
		return err
	}
	end := off + length

	// Compress the dictionary first
	if err := c.doCompress(b, off, dictLength, nil, out); err != nil {
		return err
	}

	// And then sub blocks
	for start := off + dictLength; start < end; start += blockLength {
		l := blockLength
		if remain := off + length - start; l > remain {
			l = remain
		}
		if err := c.doCompress(b, start, l, b[off:off+dictLength], out); err != nil {
			return err
		}
	}
	return nil
}

// Close reproduces
//
//	if (closed == false) { compressor.end(); closed = true; }
func (c *deflateWithPresetDictCompressor) Close() error {
	c.closed = true
	return nil
}
