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

package compressing

import (
	"bytes"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

// CompressionMode tells how much effort should be spent on compression and
// decompression of stored fields.
//
// This is the Go port of the abstract class
// org.apache.lucene.codecs.compressing.CompressionMode. Three modes are
// provided as package-level variables: [FAST], [HIGH_COMPRESSION] and
// [FAST_DECOMPRESSION].
type CompressionMode interface {
	// NewCompressor creates a new Compressor instance.
	NewCompressor() Compressor

	// NewDecompressor creates a new Decompressor instance.
	NewDecompressor() Decompressor

	// String returns the canonical name of this mode (FAST,
	// HIGH_COMPRESSION or FAST_DECOMPRESSION).
	String() string
}

// FAST trades compression ratio for speed. Although the compression ratio
// might remain high, compression and decompression are very fast. Use this
// mode with indices that have a high update rate but should be able to load
// documents from disk quickly.
//
// Backed by LZ4 fast compression.
var FAST CompressionMode = fastMode{}

// HIGH_COMPRESSION trades speed for compression ratio. Although compression
// and decompression might be slow, this compression mode should provide a
// good compression ratio. This mode might be interesting if/when your index
// size is much bigger than your OS cache.
//
// Backed by raw DEFLATE level 6 (matching the Java reference which uses
// Deflater(6, true), i.e. nowrap = true).
var HIGH_COMPRESSION CompressionMode = highCompressionMode{}

// FAST_DECOMPRESSION is similar to [FAST] but it spends more time compressing
// in order to improve the compression ratio. Best used with indices that
// have a low update rate but should be able to load documents from disk
// quickly.
//
// Backed by LZ4 high compression on the write path and the same LZ4
// decompressor as [FAST] on the read path.
var FAST_DECOMPRESSION CompressionMode = fastDecompressionMode{}

// fastMode implements [FAST].
type fastMode struct{}

func (fastMode) NewCompressor() Compressor     { return newLZ4FastCompressor() }
func (fastMode) NewDecompressor() Decompressor { return lz4Decompressor{} }
func (fastMode) String() string                { return "FAST" }

// highCompressionMode implements [HIGH_COMPRESSION].
type highCompressionMode struct{}

func (highCompressionMode) NewCompressor() Compressor {
	// 3 is the highest level that doesn't have lazy match evaluation; 6 is
	// the default and matches the Java reference. Higher levels offer
	// diminishing returns at significant CPU cost.
	return newDeflateCompressor(flate.DefaultCompression) // = 6
}
func (highCompressionMode) NewDecompressor() Decompressor { return newDeflateDecompressor() }
func (highCompressionMode) String() string                { return "HIGH_COMPRESSION" }

// fastDecompressionMode implements [FAST_DECOMPRESSION].
type fastDecompressionMode struct{}

func (fastDecompressionMode) NewCompressor() Compressor     { return newLZ4HighCompressor() }
func (fastDecompressionMode) NewDecompressor() Decompressor { return lz4Decompressor{} }
func (fastDecompressionMode) String() string                { return "FAST_DECOMPRESSION" }

// -----------------------------------------------------------------------------
// LZ4 compressors
// -----------------------------------------------------------------------------

// lz4FastCompressor is the LZ4 fast variant used by [FAST]. The hash table
// is owned by the instance and reused across Compress calls.
type lz4FastCompressor struct {
	ht *compress.FastCompressionHashTable
}

func newLZ4FastCompressor() *lz4FastCompressor {
	return &lz4FastCompressor{ht: compress.NewFastCompressionHashTable()}
}

// Compress reads the entire buffersInput payload into a scratch slice and
// emits an LZ4 stream into out. Mirrors LZ4FastCompressor.compress in the
// Java reference (single buffer read, single LZ4.compress call).
func (c *lz4FastCompressor) Compress(buffersInput store.ByteBuffersDataInput, out store.DataOutput) error {
	data, err := readAllBuffersInput(buffersInput)
	if err != nil {
		return err
	}
	return compress.LZ4Compress(data, 0, len(data), out, c.ht)
}

// Close is a no-op for the fast LZ4 compressor (the hash table holds no
// OS-owned resources). Mirrors the Java implementation.
func (c *lz4FastCompressor) Close() error { return nil }

// lz4HighCompressor is the LZ4 high-compression variant used by
// [FAST_DECOMPRESSION].
type lz4HighCompressor struct {
	ht *compress.HighCompressionHashTable
}

func newLZ4HighCompressor() *lz4HighCompressor {
	return &lz4HighCompressor{ht: compress.NewHighCompressionHashTable()}
}

// Compress mirrors LZ4HighCompressor.compress in the Java reference.
func (c *lz4HighCompressor) Compress(buffersInput store.ByteBuffersDataInput, out store.DataOutput) error {
	data, err := readAllBuffersInput(buffersInput)
	if err != nil {
		return err
	}
	return compress.LZ4Compress(data, 0, len(data), out, c.ht)
}

// Close is a no-op for the LZ4 HC compressor.
func (c *lz4HighCompressor) Close() error { return nil }

// -----------------------------------------------------------------------------
// LZ4 decompressor (shared by FAST and FAST_DECOMPRESSION)
// -----------------------------------------------------------------------------

// lz4PadBytes mirrors the +7 padding the Java reference allocates for the
// LZ4 fast-copy code path: copying 8 bytes at a time can read up to 7 bytes
// past the logical end of the buffer, so the destination must have headroom.
const lz4PadBytes = 7

// lz4Decompressor is stateless and shared across goroutines via a singleton
// value; Clone may return the receiver because there is no per-instance
// state. Matches the Java LZ4_DECOMPRESSOR anonymous inner class.
type lz4Decompressor struct{}

// ErrCorruptLengthMismatch is returned when an LZ4 or DEFLATE stream
// decodes to more (or fewer, for DEFLATE) bytes than the originalLength
// recorded by the compressor. This is the Go equivalent of Lucene's
// CorruptIndexException("Lengths mismatch: ...").
var ErrCorruptLengthMismatch = errors.New("compressing: corrupted stream — decompressed length mismatch")

// Decompress decodes the [offset, offset+length) window from the LZ4 stream
// available in in. Mirrors the Java LZ4_DECOMPRESSOR contract: the
// destination buffer is grown with 7 bytes of trailing padding to permit
// the LZ4 fast-copy hot path, and bytes.Offset / bytes.Length are set so
// that bytes.ValidBytes() returns exactly the requested window.
func (lz4Decompressor) Decompress(in store.DataInput, originalLength, offset, length int, dst *util.BytesRef) error {
	if dst == nil {
		return errors.New("compressing: BytesRef destination is nil")
	}
	if offset < 0 || length < 0 || offset+length > originalLength {
		return fmt.Errorf("compressing: invalid window offset=%d length=%d originalLength=%d", offset, length, originalLength)
	}

	// add 7 padding bytes — not strictly necessary but enables the
	// 8-bytes-at-a-time copy fast path inside LZ4Decompress.
	needed := originalLength + lz4PadBytes
	if cap(dst.Bytes) < needed {
		newCap := util.Oversize(needed, 1)
		dst.Bytes = make([]byte, newCap)
	} else {
		dst.Bytes = dst.Bytes[:cap(dst.Bytes)]
	}

	decompressedLen, err := compress.LZ4Decompress(in, offset+length, dst.Bytes, 0)
	if err != nil {
		return err
	}
	if decompressedLen > originalLength {
		return fmt.Errorf("%w: %d > %d", ErrCorruptLengthMismatch, decompressedLen, originalLength)
	}
	dst.Offset = offset
	dst.Length = length
	return nil
}

// Clone returns the receiver: the decompressor is stateless.
func (d lz4Decompressor) Clone() Decompressor { return d }

// -----------------------------------------------------------------------------
// DEFLATE compressor / decompressor
// -----------------------------------------------------------------------------

// deflateCompressor implements [HIGH_COMPRESSION] using raw DEFLATE (no zlib
// header), matching Java's new Deflater(level, true /* nowrap */). The wire
// format is the VInt-prefixed run produced by Lucene 10.4.0.
type deflateCompressor struct {
	level   int
	scratch []byte // reusable destination for the raw DEFLATE bytes
	writer  *flate.Writer
	closed  bool
}

func newDeflateCompressor(level int) *deflateCompressor {
	c := &deflateCompressor{level: level, scratch: make([]byte, 0, 64)}
	// Pre-allocate the flate.Writer once; subsequent calls invoke Reset.
	w, err := flate.NewWriter(io.Discard, level)
	if err != nil {
		// flate.NewWriter only errors on an invalid level; level is
		// validated at construction time so this is unrecoverable.
		panic(fmt.Sprintf("compressing: flate.NewWriter(level=%d) failed: %v", level, err))
	}
	c.writer = w
	return c
}

// Compress mirrors DeflateCompressor.compress in the Java reference:
//   - drain buffersInput into a scratch byte slice,
//   - run a single raw-DEFLATE pass collecting every output byte into a
//     contiguous buffer,
//   - emit the compressed length as a VInt followed by the compressed bytes.
func (c *deflateCompressor) Compress(buffersInput store.ByteBuffersDataInput, out store.DataOutput) error {
	if c.closed {
		return errors.New("compressing: deflate compressor closed")
	}
	data, err := readAllBuffersInput(buffersInput)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		// matches the Java "needsInput" early-return: write a VInt of 0
		// and no payload. The corresponding decompressor must accept this.
		return writeVInt(out, 0)
	}

	// Reset the flate writer and route its output into our scratch buffer.
	// Reusing the writer avoids the per-call allocation cost of a fresh
	// Deflater under the JVM equivalent.
	c.scratch = c.scratch[:0]
	buf := bytes.NewBuffer(c.scratch)
	c.writer.Reset(buf)
	if _, err := c.writer.Write(data); err != nil {
		return err
	}
	if err := c.writer.Close(); err != nil {
		return err
	}
	c.scratch = buf.Bytes()

	if err := writeVInt(out, int32(len(c.scratch))); err != nil {
		return err
	}
	return out.WriteBytes(c.scratch)
}

// Close releases the underlying flate writer. Subsequent Compress calls
// fail. Idempotent per the Closeable contract.
func (c *deflateCompressor) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	// flate.Writer has no Close-without-flush; the writer is already
	// flushed in Compress, so there is nothing to release here beyond
	// allowing the GC to reclaim it.
	c.writer = nil
	return nil
}

// deflateDecompressor implements decompression for [HIGH_COMPRESSION]. The
// instance owns a scratch buffer for the compressed bytes; Clone allocates
// a fresh instance with its own scratch.
type deflateDecompressor struct {
	compressed []byte
}

func newDeflateDecompressor() *deflateDecompressor {
	return &deflateDecompressor{compressed: make([]byte, 0)}
}

// Decompress mirrors DeflateDecompressor.decompress in the Java reference:
//   - read the VInt-prefixed compressed payload,
//   - inflate it (raw DEFLATE, no zlib header),
//   - verify the decompressed length matches originalLength,
//   - position bytes.Offset / bytes.Length onto the requested window.
//
// Unlike the Java reference, Go's flate reader does not require a trailing
// dummy zero byte (that requirement is a peculiarity of java.util.zip's
// nowrap Inflater); the compressed-bytes count we read from the VInt is
// already the exact wire length emitted by the Java writer.
func (d *deflateDecompressor) Decompress(in store.DataInput, originalLength, offset, length int, dst *util.BytesRef) error {
	if dst == nil {
		return errors.New("compressing: BytesRef destination is nil")
	}
	if offset < 0 || length < 0 || offset+length > originalLength {
		return fmt.Errorf("compressing: invalid window offset=%d length=%d originalLength=%d", offset, length, originalLength)
	}

	if length == 0 {
		// fast path matching Java: still consume the (zero-byte) compressed
		// payload from the stream before returning. Java skips the read
		// entirely; we do the same to preserve byte-stream alignment.
		dst.Length = 0
		return nil
	}

	compressedLength, err := readVInt(in)
	if err != nil {
		return err
	}
	if compressedLength < 0 {
		return fmt.Errorf("compressing: invalid negative compressedLength=%d", compressedLength)
	}

	if cap(d.compressed) < int(compressedLength) {
		d.compressed = make([]byte, compressedLength)
	} else {
		d.compressed = d.compressed[:compressedLength]
	}
	if compressedLength > 0 {
		if err := in.ReadBytes(d.compressed); err != nil {
			return err
		}
	}

	if cap(dst.Bytes) < originalLength {
		dst.Bytes = make([]byte, originalLength)
	} else {
		dst.Bytes = dst.Bytes[:originalLength]
	}

	r := flate.NewReader(bytes.NewReader(d.compressed))
	defer r.Close()
	n, err := io.ReadFull(r, dst.Bytes)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	if n != originalLength {
		return fmt.Errorf("%w: %d != %d", ErrCorruptLengthMismatch, n, originalLength)
	}
	dst.Offset = offset
	dst.Length = length
	return nil
}

// Clone returns a fresh deflateDecompressor with its own scratch buffer.
func (d *deflateDecompressor) Clone() Decompressor { return newDeflateDecompressor() }

// -----------------------------------------------------------------------------
// shared helpers
// -----------------------------------------------------------------------------

// readAllBuffersInput drains every remaining byte from buffersInput into a
// freshly allocated slice. Mirrors the Java idiom
// `byte[] b = new byte[(int) buffersInput.length()];
//
//	buffersInput.readBytes(b, 0, len);` with explicit overflow checking on
//
// the int64 -> int cast.
func readAllBuffersInput(buffersInput store.ByteBuffersDataInput) ([]byte, error) {
	length64 := buffersInput.Length()
	if length64 < 0 || length64 > math.MaxInt32 {
		return nil, fmt.Errorf("compressing: buffersInput length out of int32 range: %d", length64)
	}
	length := int(length64)
	if length == 0 {
		return nil, nil
	}
	data := make([]byte, length)
	if err := buffersInput.ReadBytes(data); err != nil {
		return nil, err
	}
	return data, nil
}

// writeVInt writes a 32-bit value using Lucene's variable-length integer
// encoding. We re-implement it locally instead of casting out to
// store.VariableLengthOutput because not every DataOutput value the
// Compressor receives implements that segregated interface; the on-disk
// byte sequence is identical to the canonical encoder.
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

// readVInt reads a 32-bit value previously written by writeVInt (or any
// equivalent Lucene encoder). The maximum encoded length is 5 bytes.
func readVInt(in store.DataInput) (int32, error) {
	var result uint32
	var shift uint
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
	return 0, errors.New("compressing: invalid VInt — more than 5 bytes")
}
