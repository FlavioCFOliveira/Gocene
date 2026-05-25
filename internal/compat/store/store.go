// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package store implements the Sprint 114 T6 store-primitives scenario in Go.
//
// The store-primitives artefact is a single "store-primitives.dat" file
// wrapped in the Lucene CodecUtil index header / footer envelope, containing
// COUNT frames where each frame exercises every primitive serialisation
// method on Lucene's DataOutput contract: vInt, vLong, zInt, zLong, string,
// byte, short, int and long.
//
// The layout MUST be byte-identical to the Java-side implementation in
// tools/lucene-fixtures/.../scenarios/StorePrimitivesScenario.java.
//
//	IndexHeader( codec="GoceneStorePrimitives", version=0, id=16B(seed), suffix="" )
//	vInt    count = 8
//	for i in 0..count-1:
//	    vInt    (int)(seed * (i+1) & 0x7FFFFFFF)
//	    vLong   seed * (long)(i+3)
//	    zInt    (int)(seed * (i+1) - 7)
//	    zLong   seed * (long)(i+1) * -3
//	    string  "frame-" + i + "-seed-" + seed
//	    byte    (byte) i
//	    short   (short)(seed + i)               (LE on disk)
//	    int     (int)(seed * (i+5))             (LE on disk)
//	    long    seed << (i & 0x3F)              (LE on disk)
//	Footer  ( FOOTER_MAGIC, algorithmId=0, CRC32 of preceding bytes )
//
// The helpers have NO build tag so they can be exercised both by the
// compat-tagged tests in this package and (if needed) by downstream Gocene
// code that wants to assert local round-trip correctness without the Java
// harness.
package store

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	// Codec is the codec name embedded in the index header.
	Codec = "GoceneStorePrimitives"
	// Version of the store-primitives artefact.
	Version int32 = 0
	// Count is the number of frames in the payload.
	Count = 8
	// FileName is the file written inside the target directory.
	FileName = "store-primitives.dat"
)

// IDFromSeed returns the deterministic 16-byte identifier embedded in the
// CodecUtil index header. Mirrors Determinism.idBytes on the Java side.
func IDFromSeed(seed int64) []byte {
	id := make([]byte, 16)
	binary.BigEndian.PutUint64(id[0:8], uint64(seed))
	binary.BigEndian.PutUint64(id[8:16], uint64(^seed))
	return id
}

// VIntValue returns the i-th frame's vInt value for the given seed.
func VIntValue(seed int64, i int) int32 {
	return int32(uint64(seed*int64(i+1)) & 0x7FFFFFFF)
}

// VLongValue returns the i-th frame's vLong value for the given seed.
func VLongValue(seed int64, i int) int64 {
	return seed * int64(i+3)
}

// ZIntValue returns the i-th frame's zigzag-int value for the given seed.
func ZIntValue(seed int64, i int) int32 {
	return int32(seed*int64(i+1) - 7)
}

// ZLongValue returns the i-th frame's zigzag-long value for the given seed.
func ZLongValue(seed int64, i int) int64 {
	return seed * int64(i+1) * -3
}

// StringValue returns the i-th frame's string value for the given seed.
func StringValue(seed int64, i int) string {
	return "frame-" + strconv.Itoa(i) + "-seed-" + strconv.FormatInt(seed, 10)
}

// ShortValue returns the i-th frame's int16 value for the given seed.
func ShortValue(seed int64, i int) int16 {
	return int16(seed + int64(i))
}

// IntValue returns the i-th frame's int32 value for the given seed.
func IntValue(seed int64, i int) int32 {
	return int32(seed * int64(i+5))
}

// LongValue returns the i-th frame's int64 value for the given seed.
// Mirrors Java's `seed << i` semantics (shift count masked with 0x3F).
func LongValue(seed int64, i int) int64 {
	return seed << uint(i&0x3F)
}

// WriteStorePrimitives produces the store-primitives artefact at
// targetDir/store-primitives.dat for the given seed.
//
// The output is byte-identical to the artefact produced by the Java
// harness at /tmp/lucene-fixtures.jar gen store-primitives <seed> <targetDir>.
func WriteStorePrimitives(targetDir string, seed int64) error {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("store-primitives: mkdir %s: %w", targetDir, err)
	}
	dir, err := store.NewNIOFSDirectory(targetDir)
	if err != nil {
		return fmt.Errorf("store-primitives: open dir %s: %w", targetDir, err)
	}
	defer dir.Close()

	raw, err := dir.CreateOutput(FileName, store.IOContextDefault)
	if err != nil {
		return fmt.Errorf("store-primitives: create output: %w", err)
	}
	out := store.NewChecksumIndexOutput(raw)

	if err := codecs.WriteIndexHeader(out, Codec, Version, IDFromSeed(seed), ""); err != nil {
		out.Close()
		return fmt.Errorf("store-primitives: write index header: %w", err)
	}
	if err := store.WriteVInt(out, Count); err != nil {
		out.Close()
		return fmt.Errorf("store-primitives: write count: %w", err)
	}
	for i := 0; i < Count; i++ {
		if err := writeFrame(out, seed, i); err != nil {
			out.Close()
			return fmt.Errorf("store-primitives: write frame[%d]: %w", i, err)
		}
	}
	if err := codecs.WriteFooter(out); err != nil {
		out.Close()
		return fmt.Errorf("store-primitives: write footer: %w", err)
	}
	return out.Close()
}

// ReadStorePrimitives parses and validates the store-primitives artefact at
// sourceDir/store-primitives.dat for the given seed.
func ReadStorePrimitives(sourceDir string, seed int64) error {
	dir, err := store.NewNIOFSDirectory(sourceDir)
	if err != nil {
		return fmt.Errorf("store-primitives: open dir %s: %w", sourceDir, err)
	}
	defer dir.Close()

	rawIn, err := dir.OpenInput(FileName, store.IOContextDefault)
	if err != nil {
		return fmt.Errorf("store-primitives: open input: %w", err)
	}
	defer rawIn.Close()

	in := store.NewChecksumIndexInput(rawIn)

	if _, err := codecs.CheckIndexHeader(in, Codec, Version, Version, IDFromSeed(seed), ""); err != nil {
		return fmt.Errorf("store-primitives: check index header: %w", err)
	}
	count, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("store-primitives: read count: %w", err)
	}
	if count != Count {
		return fmt.Errorf("store-primitives: count mismatch, expected %d, got %d", Count, count)
	}
	for i := 0; i < Count; i++ {
		if err := readFrame(in, seed, i); err != nil {
			return fmt.Errorf("store-primitives: read frame[%d]: %w", i, err)
		}
	}
	if _, err := codecs.CheckFooter(in); err != nil {
		return fmt.Errorf("store-primitives: check footer: %w", err)
	}
	return nil
}

// Path returns the store-primitives artefact path inside dir.
func Path(dir string) string {
	return filepath.Join(dir, FileName)
}

// writeFrame emits one deterministic frame. Each multi-byte primitive uses
// the byte order Lucene 10.4.0's DataOutput contract dictates:
//   - vInt / vLong / zInt / zLong: vbyte (endianness-agnostic).
//   - string: vInt length + UTF-8 bytes.
//   - byte: single octet.
//   - short / int / long: LITTLE-ENDIAN on disk (see DataOutput.writeInt
//     in Apache Lucene 10.4.0).
//
// Note on the divergence flagged in the Gocene memory index: Gocene's
// SimpleFS/Checksum IndexOutput.WriteShort/Int/Long are BIG-ENDIAN, so we
// must NOT call them here. Use the LE helpers (or raw byte writes for
// 16-bit since there is no store.WriteInt16LE) instead.
func writeFrame(out store.DataOutput, seed int64, i int) error {
	if err := store.WriteVInt(out, VIntValue(seed, i)); err != nil {
		return fmt.Errorf("vInt: %w", err)
	}
	if err := store.WriteVLong(out, VLongValue(seed, i)); err != nil {
		return fmt.Errorf("vLong: %w", err)
	}
	if err := writeZInt(out, ZIntValue(seed, i)); err != nil {
		return fmt.Errorf("zInt: %w", err)
	}
	if err := writeZLong(out, ZLongValue(seed, i)); err != nil {
		return fmt.Errorf("zLong: %w", err)
	}
	if err := store.WriteString(out, StringValue(seed, i)); err != nil {
		return fmt.Errorf("string: %w", err)
	}
	if err := out.WriteByte(byte(i)); err != nil {
		return fmt.Errorf("byte: %w", err)
	}
	if err := writeShortLE(out, ShortValue(seed, i)); err != nil {
		return fmt.Errorf("short: %w", err)
	}
	if err := store.WriteInt32LE(out, IntValue(seed, i)); err != nil {
		return fmt.Errorf("int: %w", err)
	}
	if err := store.WriteInt64LE(out, LongValue(seed, i)); err != nil {
		return fmt.Errorf("long: %w", err)
	}
	return nil
}

// readFrame parses one frame and validates it against the deterministic
// expectation derived from seed.
func readFrame(in store.DataInput, seed int64, i int) error {
	v, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("vInt: %w", err)
	}
	if exp := VIntValue(seed, i); v != exp {
		return fmt.Errorf("vInt mismatch: got %d, want %d", v, exp)
	}
	vl, err := store.ReadVLong(in)
	if err != nil {
		return fmt.Errorf("vLong: %w", err)
	}
	if exp := VLongValue(seed, i); vl != exp {
		return fmt.Errorf("vLong mismatch: got %d, want %d", vl, exp)
	}
	zi, err := readZInt(in)
	if err != nil {
		return fmt.Errorf("zInt: %w", err)
	}
	if exp := ZIntValue(seed, i); zi != exp {
		return fmt.Errorf("zInt mismatch: got %d, want %d", zi, exp)
	}
	zl, err := readZLong(in)
	if err != nil {
		return fmt.Errorf("zLong: %w", err)
	}
	if exp := ZLongValue(seed, i); zl != exp {
		return fmt.Errorf("zLong mismatch: got %d, want %d", zl, exp)
	}
	s, err := store.ReadString(in)
	if err != nil {
		return fmt.Errorf("string: %w", err)
	}
	if exp := StringValue(seed, i); s != exp {
		return fmt.Errorf("string mismatch: got %q, want %q", s, exp)
	}
	b, err := in.ReadByte()
	if err != nil {
		return fmt.Errorf("byte: %w", err)
	}
	if exp := byte(i); b != exp {
		return fmt.Errorf("byte mismatch: got %d, want %d", b, exp)
	}
	sh, err := readShortLE(in)
	if err != nil {
		return fmt.Errorf("short: %w", err)
	}
	if exp := ShortValue(seed, i); sh != exp {
		return fmt.Errorf("short mismatch: got %d, want %d", sh, exp)
	}
	ii, err := store.ReadInt32LE(in)
	if err != nil {
		return fmt.Errorf("int: %w", err)
	}
	if exp := IntValue(seed, i); ii != exp {
		return fmt.Errorf("int mismatch: got %d, want %d", ii, exp)
	}
	ll, err := store.ReadInt64LE(in)
	if err != nil {
		return fmt.Errorf("long: %w", err)
	}
	if exp := LongValue(seed, i); ll != exp {
		return fmt.Errorf("long mismatch: got %d, want %d", ll, exp)
	}
	return nil
}

// writeZInt is the free-function form of Lucene's DataOutput.writeZInt.
// Gocene only exposes this as a method on ByteBuffersDataOutput, so we
// inline the standard (v<<1)^(v>>31) zigzag encoding here for any
// DataOutput.
func writeZInt(out store.DataOutput, v int32) error {
	return store.WriteVInt(out, (v<<1)^(v>>31))
}

// writeZLong is the free-function form of Lucene's DataOutput.writeZLong.
func writeZLong(out store.DataOutput, v int64) error {
	return store.WriteVLong(out, (v<<1)^(v>>63))
}

// readZInt mirrors Lucene's DataInput.readZInt: (raw >>> 1) ^ -(raw & 1).
func readZInt(in store.DataInput) (int32, error) {
	raw, err := store.ReadVInt(in)
	if err != nil {
		return 0, err
	}
	return int32(uint32(raw)>>1) ^ -(raw & 1), nil
}

// readZLong mirrors Lucene's DataInput.readZLong.
func readZLong(in store.DataInput) (int64, error) {
	raw, err := store.ReadVLong(in)
	if err != nil {
		return 0, err
	}
	return int64(uint64(raw)>>1) ^ -(raw & 1), nil
}

// writeShortLE writes a 16-bit signed integer in little-endian byte order.
// Lucene's DataOutput.writeShort is little-endian; Gocene's IndexOutput
// methods are big-endian (a known divergence), so we emit raw LE bytes
// here to match the wire format.
func writeShortLE(out store.DataOutput, v int16) error {
	return out.WriteBytes([]byte{byte(v), byte(v >> 8)})
}

// readShortLE reads a 16-bit signed integer in little-endian byte order.
func readShortLE(in store.DataInput) (int16, error) {
	b0, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	b1, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	return int16(uint16(b0) | uint16(b1)<<8), nil
}
