// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package lucene90

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

const (
	// lz4PresetDictNumSubBlocks is the number of sub-blocks the input is
	// split into. Matches Lucene's NUM_SUB_BLOCKS = 10.
	lz4PresetDictNumSubBlocks = 10
	// lz4PresetDictSizeFactor controls the dictionary size: roughly
	// blockSize / DICT_SIZE_FACTOR. Matches Lucene's DICT_SIZE_FACTOR = 2.
	lz4PresetDictSizeFactor = 2
	// lz4DecodePadBytes is the trailing padding required by LZ4Decompress so
	// the 8-bytes-at-a-time fast-copy hot path does not read past the end of
	// the destination slice. Mirrors codecs/compressing.lz4PadBytes.
	lz4DecodePadBytes = 7
)

// LZ4WithPresetDictCompressionMode is a CompressionMode that trades
// compression ratio for fast (de)compression. The input is split into a small
// dictionary followed by ~10 sub-blocks; each block is compressed with LZ4
// seeded by the dictionary. Suitable for low-latency reads.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.LZ4WithPresetDictCompressionMode.
type LZ4WithPresetDictCompressionMode struct{}

// NewLZ4WithPresetDictCompressionMode returns the stateless mode value.
func NewLZ4WithPresetDictCompressionMode() LZ4WithPresetDictCompressionMode {
	return LZ4WithPresetDictCompressionMode{}
}

// NewCompressor returns a fresh Compressor configured for LZ4 with
// preset-dictionary framing.
func (LZ4WithPresetDictCompressionMode) NewCompressor() compressing.Compressor {
	return newLZ4WithPresetDictCompressor()
}

// NewDecompressor returns a fresh Decompressor for the preset-dictionary LZ4
// wire format.
func (LZ4WithPresetDictCompressionMode) NewDecompressor() compressing.Decompressor {
	return newLZ4WithPresetDictDecompressor()
}

// String returns the canonical Lucene name of this mode ("BEST_SPEED").
func (LZ4WithPresetDictCompressionMode) String() string { return "BEST_SPEED" }

// Statically assert the value satisfies the CompressionMode contract.
var _ compressing.CompressionMode = LZ4WithPresetDictCompressionMode{}

// -----------------------------------------------------------------------------
// Compressor
// -----------------------------------------------------------------------------

// lz4WithPresetDictCompressor implements the write path. It owns a single
// FastCompressionHashTable that is reused across Compress calls (matching
// Java) and a re-settable ByteBuffersDataOutput that accumulates the body
// payload of every block before it is appended to out (so that all block
// lengths can be written first, then the bodies, exactly like the Java
// reference).
type lz4WithPresetDictCompressor struct {
	compressed *store.ByteBuffersDataOutput
	hashTable  *compress.FastCompressionHashTable
	buffer     []byte
}

func newLZ4WithPresetDictCompressor() *lz4WithPresetDictCompressor {
	return &lz4WithPresetDictCompressor{
		compressed: store.NewByteBuffersDataOutput(),
		hashTable:  compress.NewFastCompressionHashTable(),
		buffer:     nil,
	}
}

// doCompress mirrors the inner Java helper: compress buffer[0:dictLen+len]
// (where bytes[dictLen:dictLen+len] is the payload and bytes[0:dictLen] is
// the preset dictionary), then write the compressed-bytes count as a VInt
// onto out. The compressed body itself is stashed in c.compressed and
// flushed in one shot at the end of Compress.
func (c *lz4WithPresetDictCompressor) doCompress(b []byte, dictLen, length int, out store.DataOutput) error {
	prevSize := c.compressed.Size()
	if err := compress.LZ4CompressWithDictionary(b, 0, dictLen, length, c.compressed, c.hashTable); err != nil {
		return err
	}
	diff := c.compressed.Size() - prevSize
	if diff < 0 || diff > math.MaxInt32 {
		return fmt.Errorf("lucene90: LZ4 block length out of int32 range: %d", diff)
	}
	return writeVInt(out, int32(diff))
}

// Compress implements compressing.Compressor. Wire format:
//
//	VInt(dictLength) || VInt(blockLength) ||
//	  VInt(dictCompressedLen) || VInt(block_i_compressedLen)... ||
//	  dictCompressedBytes || block_i_compressedBytes...
//
// All length prefixes appear before any compressed bytes; the bytes are then
// emitted in one CopyTo from the internal accumulator. This is exactly the
// Java framing.
func (c *lz4WithPresetDictCompressor) Compress(buffersInput store.ByteBuffersDataInput, out store.DataOutput) error {
	remaining := buffersInput.Length() - buffersInput.Position()
	if remaining < 0 || remaining > math.MaxInt32 {
		return fmt.Errorf("lucene90: buffersInput remaining length out of int32 range: %d", remaining)
	}
	length := int(remaining)

	// dictLength = min(MAX_DISTANCE, length / (NUM_SUB_BLOCKS * DICT_SIZE_FACTOR))
	var dictLength int
	if length > 0 {
		dictLength = length / (lz4PresetDictNumSubBlocks * lz4PresetDictSizeFactor)
		if dictLength > compress.MaxDistance {
			dictLength = compress.MaxDistance
		}
	}
	var blockLength int
	if length > dictLength {
		blockLength = (length - dictLength + lz4PresetDictNumSubBlocks - 1) / lz4PresetDictNumSubBlocks
	}

	// Grow buffer once to hold dict || one sub-block of payload.
	if need := dictLength + blockLength; cap(c.buffer) < need {
		c.buffer = make([]byte, need)
	} else {
		c.buffer = c.buffer[:need]
	}

	if err := writeVInt(out, int32(dictLength)); err != nil {
		return err
	}
	if err := writeVInt(out, int32(blockLength)); err != nil {
		return err
	}

	c.compressed.Reset()

	// Compress the dictionary first (no preset dict — dictLen=0).
	if dictLength > 0 {
		if err := buffersInput.ReadBytes(c.buffer[:dictLength]); err != nil {
			return err
		}
	}
	if err := c.doCompress(c.buffer, 0, dictLength, out); err != nil {
		return err
	}

	// Then each sub-block, seeded with the dictionary.
	for start := dictLength; start < length; start += blockLength {
		l := blockLength
		if l > length-start {
			l = length - start
		}
		if l > 0 {
			if err := buffersInput.ReadBytes(c.buffer[dictLength : dictLength+l]); err != nil {
				return err
			}
		}
		if err := c.doCompress(c.buffer, dictLength, l, out); err != nil {
			return err
		}
	}

	// We only wrote lengths so far; now flush the compressed bodies.
	return c.compressed.CopyTo(out)
}

// Close is a no-op for this mode — matches the Java reference.
func (c *lz4WithPresetDictCompressor) Close() error { return nil }

// -----------------------------------------------------------------------------
// Decompressor
// -----------------------------------------------------------------------------

// lz4WithPresetDictDecompressor implements the read path. It owns scratch
// state across Decompress calls so the steady state allocates nothing beyond
// the destination BytesRef growth.
type lz4WithPresetDictDecompressor struct {
	compressedLengths []int32
	buffer            []byte
}

func newLZ4WithPresetDictDecompressor() *lz4WithPresetDictDecompressor {
	return &lz4WithPresetDictDecompressor{
		compressedLengths: nil,
		buffer:            nil,
	}
}

// readCompressedLengths reads the dictionary's compressed-length VInt
// (discarded) and then the per-block compressed-length VInts, returning the
// number of blocks. Mirrors the Java helper.
func (d *lz4WithPresetDictDecompressor) readCompressedLengths(in store.DataInput, originalLength, dictLength, blockLength int) (int, error) {
	if _, err := readVInt(in); err != nil { // compressed length of the dictionary, unused
		return 0, err
	}
	cap1 := originalLength/blockLength + 1
	if cap(d.compressedLengths) < cap1 {
		d.compressedLengths = make([]int32, cap1)
	} else {
		d.compressedLengths = d.compressedLengths[:cap1]
	}
	totalLength := dictLength
	i := 0
	for totalLength < originalLength {
		v, err := readVInt(in)
		if err != nil {
			return 0, err
		}
		d.compressedLengths[i] = v
		i++
		totalLength += blockLength
	}
	return i, nil
}

// Decompress implements compressing.Decompressor.
func (d *lz4WithPresetDictDecompressor) Decompress(in store.DataInput, originalLength, offset, length int, dst *util.BytesRef) error {
	if dst == nil {
		return errors.New("lucene90: BytesRef destination is nil")
	}
	if offset < 0 || length < 0 || offset+length > originalLength {
		return fmt.Errorf("lucene90: invalid window offset=%d length=%d originalLength=%d", offset, length, originalLength)
	}
	if length == 0 {
		dst.Length = 0
		return nil
	}

	dictLength32, err := readVInt(in)
	if err != nil {
		return err
	}
	blockLength32, err := readVInt(in)
	if err != nil {
		return err
	}
	dictLength := int(dictLength32)
	blockLength := int(blockLength32)
	if dictLength < 0 || blockLength <= 0 {
		return fmt.Errorf("lucene90: invalid dictLength=%d blockLength=%d", dictLength, blockLength)
	}

	numBlocks, err := d.readCompressedLengths(in, originalLength, dictLength, blockLength)
	if err != nil {
		return err
	}

	// Scratch buffer holds dict || current sub-block decompressed bytes;
	// LZ4Decompress writes into buffer[dictLength:dictLength+l]. The decoder
	// reads up to 7 bytes past the logical end of the destination during its
	// fast 8-bytes-at-a-time copy hot path, so we pad the allocation
	// accordingly. Matches the +lz4PadBytes idiom in codecs/compressing.
	if need := dictLength + blockLength + lz4DecodePadBytes; cap(d.buffer) < need {
		d.buffer = make([]byte, need)
	} else {
		d.buffer = d.buffer[:need]
	}
	dst.Length = 0

	// Read the dictionary first.
	if dictLength > 0 {
		written, err := compress.LZ4Decompress(in, dictLength, d.buffer, 0)
		if err != nil {
			return err
		}
		if written != dictLength {
			return fmt.Errorf("lucene90: illegal dict length: got %d want %d", written, dictLength)
		}
	}

	offsetInBlock := dictLength
	offsetInBytesRef := offset

	if offset >= dictLength {
		offsetInBytesRef -= dictLength

		// Skip unneeded blocks.
		numBytesToSkip := int64(0)
		for i := 0; i < numBlocks && offsetInBlock+blockLength < offset; i++ {
			numBytesToSkip += int64(d.compressedLengths[i])
			offsetInBlock += blockLength
			offsetInBytesRef -= blockLength
		}
		if err := skipBytes(in, numBytesToSkip); err != nil {
			return err
		}
	} else {
		// The dictionary contains bytes we need; copy them into dst.
		if cap(dst.Bytes) < dictLength {
			dst.Bytes = make([]byte, dictLength)
		} else {
			dst.Bytes = dst.Bytes[:dictLength]
		}
		copy(dst.Bytes, d.buffer[:dictLength])
		dst.Length = dictLength
	}

	// Pre-grow dst to fit everything we will append.
	if offsetInBlock < offset+length {
		need := dst.Length + offset + length - offsetInBlock
		if cap(dst.Bytes) < need {
			newCap := util.Oversize(need, 1)
			grown := make([]byte, newCap)
			copy(grown, dst.Bytes[:dst.Length])
			dst.Bytes = grown
		} else {
			dst.Bytes = dst.Bytes[:need]
		}
	}

	for offsetInBlock < offset+length {
		bytesToDecompress := blockLength
		if remain := offset + length - offsetInBlock; bytesToDecompress > remain {
			bytesToDecompress = remain
		}
		// Decompress into buffer[dictLength:dictLength+bytesToDecompress]
		// using the dictionary at buffer[0:dictLength].
		if _, err := compress.LZ4Decompress(in, bytesToDecompress, d.buffer, dictLength); err != nil {
			return err
		}
		// Make sure dst has room (the pre-grow above may have set Bytes to
		// a length-slice; expose its full capacity).
		if cap(dst.Bytes) < dst.Length+bytesToDecompress {
			newCap := util.Oversize(dst.Length+bytesToDecompress, 1)
			grown := make([]byte, newCap)
			copy(grown, dst.Bytes[:dst.Length])
			dst.Bytes = grown
		}
		copy(dst.Bytes[dst.Length:dst.Length+bytesToDecompress], d.buffer[dictLength:dictLength+bytesToDecompress])
		dst.Length += bytesToDecompress
		offsetInBlock += blockLength
	}

	dst.Offset = offsetInBytesRef
	dst.Length = length
	return nil
}

// Clone returns an independent decompressor with its own scratch state.
func (d *lz4WithPresetDictDecompressor) Clone() compressing.Decompressor {
	return newLZ4WithPresetDictDecompressor()
}
