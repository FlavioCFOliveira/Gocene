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

// Package lucene90 ports org.apache.lucene.codecs.lucene90.
//
// This file implements DeflateWithPresetDictCompressionMode, a CompressionMode
// that trades speed for compression ratio by combining raw DEFLATE level 6
// with a preset dictionary built from the first slice of the input stream.
// It is the Go port of
// org.apache.lucene.codecs.lucene90.DeflateWithPresetDictCompressionMode.
package lucene90

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	// deflatePresetDictNumSubBlocks is the number of sub-blocks the input is
	// split into. Matches Lucene's NUM_SUB_BLOCKS = 10.
	deflatePresetDictNumSubBlocks = 10
	// deflatePresetDictSizeFactor controls the dictionary size: roughly
	// blockSize / DICT_SIZE_FACTOR. Matches Lucene's DICT_SIZE_FACTOR = 6.
	deflatePresetDictSizeFactor = 6
	// deflatePresetDictLevel is the DEFLATE compression level used by the
	// compressor (Java reference uses 6 — the default; 3 is the highest
	// without lazy match evaluation, and higher than 6 wastes CPU).
	deflatePresetDictLevel = 6
)

// DeflateWithPresetDictCompressionMode is a CompressionMode that trades speed
// for compression ratio. The input is split into a small dictionary followed
// by ~10 sub-blocks; each block is compressed with raw DEFLATE seeded by the
// dictionary. Suitable for indices whose total size dwarfs the OS page cache.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.DeflateWithPresetDictCompressionMode.
type DeflateWithPresetDictCompressionMode struct{}

// NewDeflateWithPresetDictCompressionMode returns the singleton-style mode
// instance. Each call allocates a fresh value-receiver struct; the type carries
// no state and instances are interchangeable, mirroring the Java public
// constructor (which is also stateless).
func NewDeflateWithPresetDictCompressionMode() DeflateWithPresetDictCompressionMode {
	return DeflateWithPresetDictCompressionMode{}
}

// NewCompressor returns a fresh Compressor configured for DEFLATE level 6 with
// preset-dictionary framing.
func (DeflateWithPresetDictCompressionMode) NewCompressor() compressing.Compressor {
	return newDeflateWithPresetDictCompressor(deflatePresetDictLevel)
}

// NewDecompressor returns a fresh Decompressor for the preset-dictionary
// DEFLATE wire format.
func (DeflateWithPresetDictCompressionMode) NewDecompressor() compressing.Decompressor {
	return newDeflateWithPresetDictDecompressor()
}

// String returns the canonical Lucene name of this mode ("BEST_COMPRESSION").
func (DeflateWithPresetDictCompressionMode) String() string { return "BEST_COMPRESSION" }

// Statically assert that the value type satisfies the contract.
var _ compressing.CompressionMode = DeflateWithPresetDictCompressionMode{}

// -----------------------------------------------------------------------------
// Compressor
// -----------------------------------------------------------------------------

// deflateWithPresetDictCompressor implements the write path. The instance owns
// a scratch buffer for the compressed bytes and another for the per-block
// uncompressed window, both reused across Compress calls to avoid per-call
// allocation in the steady state. Matches the Java DeflateWithPresetDictCompressor.
type deflateWithPresetDictCompressor struct {
	level      int
	compressed []byte // scratch for raw-DEFLATE output of one block
	buffer     []byte // scratch for dict || current sub-block uncompressed bytes
	closed     bool
}

func newDeflateWithPresetDictCompressor(level int) *deflateWithPresetDictCompressor {
	return &deflateWithPresetDictCompressor{
		level:      level,
		compressed: make([]byte, 0, 64),
		buffer:     nil,
	}
}

// doCompress mirrors the inner Java helper: compress bytes[off:off+len] using
// the supplied dictionary (may be empty) and emit "VInt(compressedLen) ||
// compressedBytes" to out. Empty windows write only "VInt(0)".
//
// The Go standard library's flate.Writer does not expose a Reset variant that
// re-attaches a preset dictionary, so we allocate a fresh flate.Writer per
// call via flate.NewWriterDict. This is a small per-block overhead and
// preserves the exact wire format produced by java.util.zip.Deflater(level,
// true /* nowrap */) seeded via setDictionary.
func (c *deflateWithPresetDictCompressor) doCompress(b []byte, off, length int, dict []byte, out store.DataOutput) error {
	if length == 0 {
		return writeVInt(out, 0)
	}

	c.compressed = c.compressed[:0]
	buf := bytes.NewBuffer(c.compressed)
	var (
		w   *flate.Writer
		err error
	)
	if len(dict) == 0 {
		w, err = flate.NewWriter(buf, c.level)
	} else {
		w, err = flate.NewWriterDict(buf, c.level, dict)
	}
	if err != nil {
		return fmt.Errorf("lucene90: flate.NewWriter(level=%d): %w", c.level, err)
	}
	if _, err := w.Write(b[off : off+length]); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	c.compressed = buf.Bytes()

	if err := writeVInt(out, int32(len(c.compressed))); err != nil {
		return err
	}
	return out.WriteBytes(c.compressed)
}

// Compress implements compressing.Compressor. The wire format mirrors Lucene:
//
//	VInt(dictLength) || VInt(blockLength) ||
//	  doCompress(dict, dict=nil) || doCompress(block_i, dict=dict) ...
//
// All length fields are 32-bit VInts so a chunk's total uncompressed length
// must fit in int32, matching the Java reference's explicit
// `(int) buffersInput.length()` cast.
func (c *deflateWithPresetDictCompressor) Compress(buffersInput store.ByteBuffersDataInput, out store.DataOutput) error {
	if c.closed {
		return errors.New("lucene90: deflate compressor closed")
	}
	remaining := buffersInput.Length() - buffersInput.Position()
	if remaining < 0 || remaining > math.MaxInt32 {
		return fmt.Errorf("lucene90: buffersInput remaining length out of int32 range: %d", remaining)
	}
	length := int(remaining)

	var dictLength int
	if length > 0 {
		dictLength = length / (deflatePresetDictNumSubBlocks * deflatePresetDictSizeFactor)
	}
	var blockLength int
	if length > dictLength {
		// ceil((length - dictLength) / NUM_SUB_BLOCKS)
		blockLength = (length - dictLength + deflatePresetDictNumSubBlocks - 1) / deflatePresetDictNumSubBlocks
	}

	if err := writeVInt(out, int32(dictLength)); err != nil {
		return err
	}
	if err := writeVInt(out, int32(blockLength)); err != nil {
		return err
	}

	// Grow the buffer once to hold dict || current block.
	if need := dictLength + blockLength; cap(c.buffer) < need {
		c.buffer = make([]byte, need)
	} else {
		c.buffer = c.buffer[:need]
	}

	// Compress the dictionary first (no preset dict).
	if dictLength > 0 {
		if err := buffersInput.ReadBytes(c.buffer[:dictLength]); err != nil {
			return err
		}
	}
	if err := c.doCompress(c.buffer, 0, dictLength, nil, out); err != nil {
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
		if err := c.doCompress(c.buffer, dictLength, l, c.buffer[:dictLength], out); err != nil {
			return err
		}
	}
	return nil
}

// Close marks the compressor unusable. Idempotent.
func (c *deflateWithPresetDictCompressor) Close() error {
	c.closed = true
	return nil
}

// -----------------------------------------------------------------------------
// Decompressor
// -----------------------------------------------------------------------------

// deflateWithPresetDictDecompressor implements the read path. The instance owns
// a scratch buffer for compressed bytes; Clone allocates a fresh peer with its
// own state. Matches the Java DeflateWithPresetDictDecompressor.
type deflateWithPresetDictDecompressor struct {
	compressed []byte
}

func newDeflateWithPresetDictDecompressor() *deflateWithPresetDictDecompressor {
	return &deflateWithPresetDictDecompressor{compressed: make([]byte, 0)}
}

// readVIntFromIn reads a VInt from a DataInput. The Java reference uses the
// same canonical encoding everywhere.
func readVIntFromIn(in store.DataInput) (int32, error) { return readVInt(in) }

// doDecompress mirrors the inner Java helper: read a length-prefixed compressed
// block, inflate it (optionally seeded with the supplied preset dictionary),
// and APPEND the result into bytes (extending bytes.Length).
//
// Unlike the Java reference we do not need to append a "dummy zero" byte: the
// Go flate.Reader is satisfied with the exact compressed-length payload, so
// the explicit padding (`paddedLength = compressedLength + 1`) in the Java
// source is purely a workaround for java.util.zip.Inflater(true)'s historical
// requirement and has no equivalent here.
func (d *deflateWithPresetDictDecompressor) doDecompress(in store.DataInput, dict []byte, dst *util.BytesRef) error {
	compressedLength, err := readVIntFromIn(in)
	if err != nil {
		return err
	}
	if compressedLength == 0 {
		return nil
	}
	if compressedLength < 0 {
		return fmt.Errorf("lucene90: invalid negative compressedLength=%d", compressedLength)
	}

	if cap(d.compressed) < int(compressedLength) {
		d.compressed = make([]byte, compressedLength)
	} else {
		d.compressed = d.compressed[:compressedLength]
	}
	if err := in.ReadBytes(d.compressed); err != nil {
		return err
	}

	var r io.ReadCloser
	if len(dict) == 0 {
		r = flate.NewReader(bytes.NewReader(d.compressed))
	} else {
		r = flate.NewReaderDict(bytes.NewReader(d.compressed), dict)
	}
	defer r.Close()

	// Inflate into dst.Bytes[dst.Length:cap(dst.Bytes)], advancing dst.Length.
	// The Java reference relies on the caller having pre-grown bytes.bytes
	// large enough; we honour that contract.
	available := cap(dst.Bytes) - dst.Length
	if available <= 0 {
		// Caller did not pre-grow the slice; for safety we extend it now.
		return errors.New("lucene90: BytesRef has no capacity left for inflate output")
	}
	dst.Bytes = dst.Bytes[:cap(dst.Bytes)] // expose the trailing capacity for ReadFull
	n, err := io.ReadFull(r, dst.Bytes[dst.Length:dst.Length+available])
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	dst.Length += n
	return nil
}

// Decompress decodes the [offset, offset+length) window from the wire format
// emitted by deflateWithPresetDictCompressor. Implements
// compressing.Decompressor.
func (d *deflateWithPresetDictDecompressor) Decompress(in store.DataInput, originalLength, offset, length int, dst *util.BytesRef) error {
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

	dictLength, err := readVIntFromIn(in)
	if err != nil {
		return err
	}
	blockLength, err := readVIntFromIn(in)
	if err != nil {
		return err
	}
	if dictLength < 0 || blockLength <= 0 {
		return fmt.Errorf("lucene90: invalid dictLength=%d blockLength=%d", dictLength, blockLength)
	}

	// Pre-grow the destination to at least dictLength so the dictionary fits.
	if cap(dst.Bytes) < int(dictLength) {
		dst.Bytes = make([]byte, dictLength)
	} else {
		dst.Bytes = dst.Bytes[:cap(dst.Bytes)]
	}
	dst.Offset = 0
	dst.Length = 0

	// Read the dictionary first (no preset dict).
	if err := d.doDecompress(in, nil, dst); err != nil {
		return err
	}
	if int32(dst.Length) != dictLength {
		return fmt.Errorf("lucene90: unexpected dict length: got %d want %d", dst.Length, dictLength)
	}

	dict := make([]byte, dictLength)
	copy(dict, dst.Bytes[:dictLength])

	offsetInBlock := int(dictLength)
	offsetInBytesRef := offset

	// Skip blocks entirely before the requested window.
	for offsetInBlock+int(blockLength) < offset {
		compressedLength, err := readVIntFromIn(in)
		if err != nil {
			return err
		}
		if err := skipBytes(in, int64(compressedLength)); err != nil {
			return err
		}
		offsetInBlock += int(blockLength)
		offsetInBytesRef -= int(blockLength)
	}

	// Read every block that intersects the requested window.
	for offsetInBlock < offset+length {
		need := dst.Length + int(blockLength)
		if cap(dst.Bytes) < need {
			// Grow with some slack: doubling matches ArrayUtil.grow semantics
			// closely enough for our purposes.
			newCap := util.Oversize(need, 1)
			grown := make([]byte, newCap)
			copy(grown, dst.Bytes[:dst.Length])
			dst.Bytes = grown
		}
		if err := d.doDecompress(in, dict, dst); err != nil {
			return err
		}
		offsetInBlock += int(blockLength)
	}

	dst.Offset = offsetInBytesRef
	dst.Length = length
	return nil
}

// Clone returns an independent decompressor with its own scratch buffer.
func (d *deflateWithPresetDictDecompressor) Clone() compressing.Decompressor {
	return newDeflateWithPresetDictDecompressor()
}

// -----------------------------------------------------------------------------
// VInt helpers (private; the canonical Lucene encoding)
// -----------------------------------------------------------------------------

// writeVInt is the canonical Lucene variable-length 32-bit integer encoder.
// Re-implemented locally for the same reason as in codecs/compressing: not
// every DataOutput we receive implements the segregated VInt interface.
func writeVInt(out store.DataOutput, v int32) error {
	uv := uint32(v)
	for uv >= 0x80 {
		if err := out.WriteByte(byte(uv | 0x80)); err != nil {
			return err
		}
		uv >>= 7
	}
	return out.WriteByte(byte(uv))
}

// readVInt is the canonical Lucene variable-length 32-bit integer decoder.
// Max encoded length is 5 bytes.
func readVInt(in store.DataInput) (int32, error) {
	var (
		result uint32
		shift  uint
	)
	for i := 0; i < 5; i++ {
		b, err := in.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= uint32(b&0x7F) << shift
		if b&0x80 == 0 {
			return int32(result), nil
		}
		shift += 7
	}
	return 0, errors.New("lucene90: invalid VInt — more than 5 bytes")
}

// skipBytes discards exactly n bytes from in. The Lucene DataInput
// abstraction does not require a fast Skip primitive; we read-and-throw-away
// in 8KB windows so we never allocate more than necessary even for large
// skipped blocks.
func skipBytes(in store.DataInput, n int64) error {
	if n <= 0 {
		return nil
	}
	const chunk = 8192
	scratch := make([]byte, chunk)
	for n > 0 {
		take := int64(chunk)
		if take > n {
			take = n
		}
		if err := in.ReadBytes(scratch[:take]); err != nil {
			return err
		}
		n -= take
	}
	return nil
}
