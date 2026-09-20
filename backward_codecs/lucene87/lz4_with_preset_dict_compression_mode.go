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
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/lucene87/LZ4WithPresetDictCompressionMode.java

package lucene87

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/backward_codecs/compressing"
	bstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

const (
	// lz4PresetDictNumSubBlocks mirrors {@code private static final int
	// NUM_SUB_BLOCKS = 10} — "Shoot for 10 sub blocks".
	lz4PresetDictNumSubBlocks = 10
	// lz4PresetDictSizeFactor mirrors {@code private static final int
	// DICT_SIZE_FACTOR = 2} — "And a dictionary whose size is about 2x
	// smaller than sub blocks".
	lz4PresetDictSizeFactor = 2
	// lz4DecodePadBytes is the trailing padding LZ4Decompress needs so its
	// 8-bytes-at-a-time fast-copy path does not read past the destination.
	// Java's decoder has the same requirement and the callers there over-size
	// their arrays through ArrayUtil.grow; Gocene sizes it explicitly.
	lz4DecodePadBytes = 7
)

// LZ4WithPresetDictCompressionMode is a compression mode that compromises on
// the compression ratio to provide fast compression and decompression.
//
// Mirrors {@code public final class LZ4WithPresetDictCompressionMode extends
// CompressionMode} (Lucene 10.5.0, @lucene.internal).
type LZ4WithPresetDictCompressionMode struct{}

// NewLZ4WithPresetDictCompressionMode is the sole constructor.
//
// Mirrors {@code public LZ4WithPresetDictCompressionMode() {}}.
func NewLZ4WithPresetDictCompressionMode() LZ4WithPresetDictCompressionMode {
	return LZ4WithPresetDictCompressionMode{}
}

// NewCompressor reproduces {@code return new LZ4WithPresetDictCompressor();}.
func (LZ4WithPresetDictCompressionMode) NewCompressor() compressing.Compressor {
	return newLZ4WithPresetDictCompressor()
}

// NewDecompressor reproduces {@code return new LZ4WithPresetDictDecompressor();}.
func (LZ4WithPresetDictCompressionMode) NewDecompressor() compressing.Decompressor {
	return newLZ4WithPresetDictDecompressor()
}

// String reproduces {@code return "BEST_SPEED";}.
func (LZ4WithPresetDictCompressionMode) String() string { return "BEST_SPEED" }

var _ compressing.CompressionMode = LZ4WithPresetDictCompressionMode{}

// ─── LZ4WithPresetDictDecompressor ──────────────────────────────────────────

// lz4WithPresetDictDecompressor mirrors the private static final nested class
// LZ4WithPresetDictCompressionMode.LZ4WithPresetDictDecompressor.
type lz4WithPresetDictDecompressor struct {
	compressedLengths []int32
	buffer            []byte
}

// newLZ4WithPresetDictDecompressor reproduces
//
//	compressedLengths = new int[0];
//	buffer = new byte[0];
func newLZ4WithPresetDictDecompressor() *lz4WithPresetDictDecompressor {
	return &lz4WithPresetDictDecompressor{
		compressedLengths: make([]int32, 0),
		buffer:            make([]byte, 0),
	}
}

// readCompressedLengths reproduces the private helper
//
//	in.readVInt(); // compressed length of the dictionary, unused
//	int totalLength = dictLength;
//	int i = 0;
//	while (totalLength < originalLength) {
//	  compressedLengths = ArrayUtil.grow(compressedLengths, i + 1);
//	  compressedLengths[i++] = in.readVInt();
//	  totalLength += blockLength;
//	}
//	return i;
func (d *lz4WithPresetDictDecompressor) readCompressedLengths(
	in store.DataInput, originalLength, dictLength, blockLength int,
) (int, error) {
	if _, err := in.ReadVInt(); err != nil { // compressed length of the dictionary, unused
		return 0, err
	}
	totalLength := dictLength
	i := 0
	for totalLength < originalLength {
		d.compressedLengths = growInt32(d.compressedLengths, i+1)
		v, err := in.ReadVInt()
		if err != nil {
			return 0, err
		}
		d.compressedLengths[i] = v
		i++
		totalLength += blockLength
	}
	return i, nil
}

// Decompress reproduces LZ4WithPresetDictDecompressor.decompress.
func (d *lz4WithPresetDictDecompressor) Decompress(
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

	numBlocks, err := d.readCompressedLengths(in, originalLength, dictLength, blockLength)
	if err != nil {
		return err
	}

	// LZ4Decompress may write up to lz4DecodePadBytes past the logical end of
	// the window it fills, so the scratch buffer carries that headroom.
	d.buffer = growByte(d.buffer, dictLength+blockLength+lz4DecodePadBytes)
	b.Length = 0
	// Read the dictionary
	wrapped := bstore.NewEndiannessReverserDataInput(in)
	written, err := compress.LZ4Decompress(wrapped, dictLength, d.buffer, 0)
	if err != nil {
		return err
	}
	if written != dictLength {
		return fmt.Errorf("lucene87: Illegal dict length: got %d want %d", written, dictLength)
	}

	offsetInBlock := dictLength
	offsetInBytesRef := offset
	if offset >= dictLength {
		offsetInBytesRef -= dictLength

		// Skip unneeded blocks
		numBytesToSkip := int64(0)
		for i := 0; i < numBlocks && offsetInBlock+blockLength < offset; i++ {
			compressedBlockLength := d.compressedLengths[i]
			numBytesToSkip += int64(compressedBlockLength)
			offsetInBlock += blockLength
			offsetInBytesRef -= blockLength
		}
		if err := in.SkipBytes(numBytesToSkip); err != nil {
			return err
		}
	} else {
		// The dictionary contains some bytes we need, copy its content to the
		// BytesRef
		b.Bytes = growByte(b.Bytes, dictLength)
		copy(b.Bytes[:dictLength], d.buffer[:dictLength])
		b.Length = dictLength
	}

	// Read blocks that intersect with the interval we need
	for offsetInBlock < offset+length {
		bytesToDecompress := blockLength
		if remain := offset + length - offsetInBlock; bytesToDecompress > remain {
			bytesToDecompress = remain
		}
		if _, err := compress.LZ4Decompress(
			bstore.NewEndiannessReverserDataInput(in), bytesToDecompress, d.buffer, dictLength,
		); err != nil {
			return err
		}
		b.Bytes = growByte(b.Bytes, b.Length+bytesToDecompress)
		copy(b.Bytes[b.Length:b.Length+bytesToDecompress],
			d.buffer[dictLength:dictLength+bytesToDecompress])
		b.Length += bytesToDecompress
		offsetInBlock += blockLength
	}

	b.Offset = offsetInBytesRef
	b.Length = length
	return nil
}

// Clone reproduces {@code return new LZ4WithPresetDictDecompressor();}.
func (d *lz4WithPresetDictDecompressor) Clone() compressing.Decompressor {
	return newLZ4WithPresetDictDecompressor()
}

// ─── LZ4WithPresetDictCompressor ────────────────────────────────────────────

// lz4WithPresetDictCompressor mirrors the private static nested class
// LZ4WithPresetDictCompressionMode.LZ4WithPresetDictCompressor.
type lz4WithPresetDictCompressor struct {
	compressed *store.ByteBuffersDataOutput
	hashTable  *compress.FastCompressionHashTable
	buffer     []byte
}

// newLZ4WithPresetDictCompressor reproduces
//
//	compressed = ByteBuffersDataOutput.newResettableInstance();
//	hashTable = new LZ4.FastCompressionHashTable();
//	buffer = BytesRef.EMPTY_BYTES;
func newLZ4WithPresetDictCompressor() *lz4WithPresetDictCompressor {
	return &lz4WithPresetDictCompressor{
		compressed: store.NewByteBuffersDataOutput(),
		hashTable:  compress.NewFastCompressionHashTable(),
		buffer:     make([]byte, 0),
	}
}

// doCompress reproduces the private helper
//
//	long prevCompressedSize = compressed.size();
//	LZ4.compressWithDictionary(bytes, 0, dictLen, len, compressed, hashTable);
//	out.writeVInt(Math.toIntExact(compressed.size() - prevCompressedSize));
func (c *lz4WithPresetDictCompressor) doCompress(b []byte, dictLen, length int, out store.DataOutput) error {
	prevCompressedSize := c.compressed.Size()
	if err := compress.LZ4CompressWithDictionary(b, 0, dictLen, length, c.compressed, c.hashTable); err != nil {
		return err
	}
	diff := c.compressed.Size() - prevCompressedSize
	if diff < 0 || diff > math.MaxInt32 {
		return fmt.Errorf("lucene87: LZ4 block length out of int32 range: %d", diff)
	}
	return out.WriteVInt(int32(diff))
}

// Compress reproduces LZ4WithPresetDictCompressor.compress.
func (c *lz4WithPresetDictCompressor) Compress(b []byte, off, length int, out store.DataOutput) error {
	dictLength := length / (lz4PresetDictNumSubBlocks * lz4PresetDictSizeFactor)
	blockLength := (length - dictLength + lz4PresetDictNumSubBlocks - 1) / lz4PresetDictNumSubBlocks
	c.buffer = growByte(c.buffer, dictLength+blockLength)
	if err := out.WriteVInt(int32(dictLength)); err != nil {
		return err
	}
	if err := out.WriteVInt(int32(blockLength)); err != nil {
		return err
	}
	end := off + length

	c.compressed.Reset()
	// Compress the dictionary first
	copy(c.buffer[:dictLength], b[off:off+dictLength])
	if err := c.doCompress(c.buffer, 0, dictLength, out); err != nil {
		return err
	}

	// And then sub blocks
	for start := off + dictLength; start < end; start += blockLength {
		l := blockLength
		if remain := off + length - start; l > remain {
			l = remain
		}
		copy(c.buffer[dictLength:dictLength+l], b[start:start+l])
		if err := c.doCompress(c.buffer, dictLength, l, out); err != nil {
			return err
		}
	}

	// We only wrote lengths so far, now write compressed data
	return c.compressed.CopyTo(out)
}

// Close is the Java no-op.
func (c *lz4WithPresetDictCompressor) Close() error { return nil }

// ─── ArrayUtil.grow renderings ──────────────────────────────────────────────

// growByte renders org.apache.lucene.util.ArrayUtil#grow(byte[], int): a
// larger array that preserves the existing contents when the current one is
// too small, and the current one otherwise.
func growByte(array []byte, minSize int) []byte {
	if len(array) < minSize {
		grown := make([]byte, util.Oversize(minSize, 1))
		copy(grown, array)
		return grown
	}
	return array
}

// growInt32 renders org.apache.lucene.util.ArrayUtil#grow(int[], int).
func growInt32(array []int32, minSize int) []int32 {
	if len(array) < minSize {
		grown := make([]int32, util.Oversize(minSize, 4))
		copy(grown, array)
		return grown
	}
	return array
}
