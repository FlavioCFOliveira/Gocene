// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.store.BaseDataOutputTestCase (Lucene 10.4.0).
//
// Java reference:
//   lucene/core/src/test/org/apache/lucene/store/BaseDataOutputTestCase.java
//
// Shape: the Java original is an abstract JUnit base class parameterised by
// T extends DataOutput. Go has no inheritance, so this file exposes a single
// reusable helper, AssertDataOutputRandomizedWrites, that concrete tests in
// package store invoke with a factory + byte-extractor pair (see, for example,
// TestByteBuffersDataOutput_BaseDataOutputContract in
// byte_buffers_data_output_test.go). The helper lives in a _test.go file, so
// it is visible only to other tests in this package, mirroring the
// test-utility scope of the Java abstract base.
//
// Deviations from the Java original (and why):
//
//  1. The Java test compares the SUT's bytes byte-for-byte against an
//     OutputStreamDataOutput-fed ByteArrayOutputStream by replaying the same
//     PRNG seed twice. Gocene's OutputStreamDataOutput emits LITTLE-endian
//     16/32/64-bit integers while ByteBuffersDataOutput emits BIG-endian
//     (the latter matches Lucene 10.4.0; the former is a pre-existing Gocene
//     divergence). Byte-for-byte comparison would fail on the first short.
//     Additionally, ByteArrayDataInput.ReadShort/Int/Long are themselves
//     little-endian, so even a "round-trip through DataInput" path is
//     inconsistent with the BE writers. The helper therefore reads the raw
//     bytes of fixed-width writes directly and reassembles them as
//     big-endian, matching the Lucene canonical layout that
//     ByteBuffersDataOutput produces.
//
//  2. Java's DataOutput exposes writeZInt/writeZLong; Gocene's DataOutput
//     interface does not. Only *ByteBuffersDataOutput and
//     *util.PagedBytesDataOutput implement them. The zigzag generators are
//     gated through the zigzagDataOutput interface and skipped when the SUT
//     does not satisfy it. Gocene's DataInput likewise has no ReadZInt /
//     ReadZLong, so the helper decodes zigzag inline.
//
//  3. Java's ByteBuffersDataOutput overloads writeBytes(ByteBuffer); Gocene
//     has no such overload, so the second branch of the array/buffer
//     generator collapses to dst.WriteBytes / dst.WriteBytesN.
//
//  4. Java seeds Xoroshiro128PlusRandom from LuceneTestCase.random(); Go
//     uses math/rand/v2.PCG seeded from a uint64 derived from time.Now plus
//     the test name, so seeds are deterministic per run and reported on
//     failure for repro.

package store

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

// zigzagDataOutput is the optional capability for DataOutput implementations
// that also expose Lucene's zig-zag VInt/VLong encoding. Only impls that
// satisfy this interface will have the writeZInt/writeZLong generators
// exercised.
type zigzagDataOutput interface {
	WriteZInt(v int32) error
	WriteZLong(v int64) error
}

// dataOutputFactory constructs a fresh DataOutput for a single test run.
// Returning a concrete pointer (rather than DataOutput) preserves access to
// optional capabilities (zigzag, RAM accounting) via type assertion inside
// the helper.
type dataOutputFactory func() DataOutput

// dataOutputBytes extracts the bytes written to an instance produced by a
// dataOutputFactory. The Java original calls this toBytes(T).
type dataOutputBytes func(DataOutput) []byte

// AssertDataOutputRandomizedWrites is the Go peer of
// BaseDataOutputTestCase#testRandomizedWrites. It performs ~50k mixed writes
// against the SUT, then verifies every value round-trips through
// ByteArrayDataInput. Concrete tests in this package call it with their own
// factory / extractor pair.
//
// The helper is safe to call from t.Run sub-tests; each invocation uses an
// independent PRNG seed reported on failure for deterministic repro.
func AssertDataOutputRandomizedWrites(t *testing.T, factory dataOutputFactory, toBytes dataOutputBytes) {
	t.Helper()

	const maxAddCalls = 50_000

	// Derive a per-call seed from time + test name. On failure we report it
	// so the run can be reproduced by hard-coding the same uint64 pair.
	now := uint64(time.Now().UnixNano())
	nameSeed := stringSeed(t.Name())
	rng := rand.New(rand.NewPCG(now, nameSeed))

	dst := factory()

	// Collect read-side verifiers (parallel to Java's IOConsumer<DataInput>
	// list returned from addRandomData).
	verifiers := make([]func(*ByteArrayDataInput) error, 0, maxAddCalls)

	for i := 0; i < maxAddCalls; i++ {
		gen := pickGenerator(rng, dst)
		v, err := gen.fn(dst, rng)
		if err != nil {
			t.Fatalf("seed=(%d,%d) op#%d (%s): write failed: %v",
				now, nameSeed, i, gen.name, err)
		}
		verifiers = append(verifiers, v)
	}

	in := NewByteArrayDataInput(toBytes(dst))
	for i, v := range verifiers {
		if err := v(in); err != nil {
			t.Fatalf("seed=(%d,%d) op#%d: %v", now, nameSeed, i, err)
		}
	}
	if leftover := len(toBytes(dst)) - in.GetPosition(); leftover != 0 {
		t.Fatalf("seed=(%d,%d): %d trailing byte(s) after all reads", now, nameSeed, leftover)
	}
}

// generator pairs a write action with the read-side verifier it produces.
// The name field exists only for failure-report context.
type generator struct {
	name string
	fn   func(DataOutput, *rand.Rand) (func(*ByteArrayDataInput) error, error)
}

func pickGenerator(rng *rand.Rand, dst DataOutput) generator {
	gens := coreGenerators
	if _, ok := dst.(zigzagDataOutput); ok {
		gens = allGenerators
	}
	return gens[rng.IntN(len(gens))]
}

// coreGenerators covers every DataOutput method on the Gocene interface.
// Order matches the Java static initializer.
var coreGenerators = []generator{
	genWriteByte,
	genWriteBytesFull,
	genWriteBytesOffset,
	genWriteInt,
	genWriteLong,
	genWriteShort,
	genWriteVInt,
	genWriteVLong,
	genWriteString,
}

// allGenerators adds the zigzag variants for SUTs that implement them.
var allGenerators = append(append([]generator{}, coreGenerators...),
	genWriteZInt,
	genWriteZLong,
)

// --- generators ---------------------------------------------------------

var genWriteByte = generator{
	name: "writeByte",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		v := byte(rng.Uint32())
		if err := dst.WriteByte(v); err != nil {
			return nil, err
		}
		return func(in *ByteArrayDataInput) error {
			got, err := in.ReadByte()
			if err != nil {
				return fmt.Errorf("readByte: %w", err)
			}
			if got != v {
				return fmt.Errorf("readByte: got %#x, want %#x", got, v)
			}
			return nil
		}, nil
	},
}

var genWriteBytesFull = generator{
	name: "writeBytes(full)",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		b := randomBytes(rng, 0, 100)
		// Java's second branch (ByteBuffer overload) has no Gocene peer;
		// the test alternates between WriteBytes and WriteBytesN instead.
		var err error
		if rng.IntN(2) == 0 {
			err = dst.WriteBytes(b)
		} else {
			err = dst.WriteBytesN(b, len(b))
		}
		if err != nil {
			return nil, err
		}
		return func(in *ByteArrayDataInput) error {
			read := make([]byte, len(b))
			if err := in.ReadBytes(read); err != nil {
				return fmt.Errorf("readBytes: %w", err)
			}
			if !bytesEqual(read, b) {
				return fmt.Errorf("readBytes: got %x, want %x", read, b)
			}
			return nil
		}, nil
	},
}

var genWriteBytesOffset = generator{
	name: "writeBytes(offset+len)",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		b := randomBytes(rng, 0, 100)
		off := 0
		if len(b) > 0 {
			off = rng.IntN(len(b) + 1)
		}
		ln := 0
		if rem := len(b) - off; rem > 0 {
			ln = rng.IntN(rem + 1)
		}
		// Gocene has no WriteBytes(b, off, len) overload; slice in caller.
		if err := dst.WriteBytesN(b[off:off+ln], ln); err != nil {
			return nil, err
		}
		expect := append([]byte(nil), b[off:off+ln]...)
		return func(in *ByteArrayDataInput) error {
			read := make([]byte, ln)
			if err := in.ReadBytes(read); err != nil {
				return fmt.Errorf("readBytes(off): %w", err)
			}
			if !bytesEqual(read, expect) {
				return fmt.Errorf("readBytes(off): got %x, want %x", read, expect)
			}
			return nil
		}, nil
	},
}

var genWriteInt = generator{
	name: "writeInt",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		v := int32(rng.Uint32())
		if err := dst.WriteInt(v); err != nil {
			return nil, err
		}
		// Lucene's DataOutput.writeInt is little-endian (low byte first); the
		// SUTs now emit LE to match (rmp #4786). Consume the 4 raw bytes
		// directly and reassemble per Lucene's canonical LE layout.
		return func(in *ByteArrayDataInput) error {
			var raw [4]byte
			for i := range raw {
				b, err := in.ReadByte()
				if err != nil {
					return fmt.Errorf("readInt byte %d: %w", i, err)
				}
				raw[i] = b
			}
			got := int32(uint32(raw[0]) | uint32(raw[1])<<8 |
				uint32(raw[2])<<16 | uint32(raw[3])<<24)
			if got != v {
				return fmt.Errorf("readInt: got %d (raw %x), want %d", got, raw, v)
			}
			return nil
		}, nil
	},
}

var genWriteLong = generator{
	name: "writeLong",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		v := int64(rng.Uint64())
		if err := dst.WriteLong(v); err != nil {
			return nil, err
		}
		// Lucene's DataOutput.writeLong is little-endian; the SUTs now emit LE
		// to match (rmp #4786). Reassemble per Lucene's canonical LE layout.
		return func(in *ByteArrayDataInput) error {
			var raw [8]byte
			for i := range raw {
				b, err := in.ReadByte()
				if err != nil {
					return fmt.Errorf("readLong byte %d: %w", i, err)
				}
				raw[i] = b
			}
			var u uint64
			for i := 0; i < 8; i++ {
				u |= uint64(raw[i]) << (8 * i)
			}
			got := int64(u)
			if got != v {
				return fmt.Errorf("readLong: got %d (raw %x), want %d", got, raw, v)
			}
			return nil
		}, nil
	},
}

var genWriteShort = generator{
	name: "writeShort",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		v := int16(rng.Uint32())
		if err := dst.WriteShort(v); err != nil {
			return nil, err
		}
		// Lucene's DataOutput.writeShort is little-endian (low byte first); the
		// SUTs now emit LE to match (rmp #4786). Consume the two bytes directly
		// and reassemble per Lucene's canonical LE layout.
		return func(in *ByteArrayDataInput) error {
			lo, err := in.ReadByte()
			if err != nil {
				return fmt.Errorf("readShort lo: %w", err)
			}
			hi, err := in.ReadByte()
			if err != nil {
				return fmt.Errorf("readShort hi: %w", err)
			}
			got := int16(uint16(lo) | uint16(hi)<<8)
			if got != v {
				return fmt.Errorf("readShort: got %d (raw %02x%02x), want %d", got, lo, hi, v)
			}
			return nil
		}, nil
	},
}

var genWriteVInt = generator{
	name: "writeVInt",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		v := int32(rng.Uint32())
		vo, ok := dst.(VariableLengthOutput)
		if !ok {
			return func(*ByteArrayDataInput) error { return nil }, nil
		}
		if err := vo.WriteVInt(v); err != nil {
			return nil, err
		}
		return func(in *ByteArrayDataInput) error {
			got, err := in.ReadVInt()
			if err != nil {
				return fmt.Errorf("readVInt: %w", err)
			}
			if got != v {
				return fmt.Errorf("readVInt: got %d, want %d", got, v)
			}
			return nil
		}, nil
	},
}

var genWriteVLong = generator{
	name: "writeVLong",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		// Java masks to non-negative via `& (-1L >>> 1)`; mirror that since
		// VLong is unsigned on the wire and Lucene rejects negative VLongs.
		v := int64(rng.Uint64() & (^uint64(0) >> 1))
		vo, ok := dst.(VariableLengthOutput)
		if !ok {
			return func(*ByteArrayDataInput) error { return nil }, nil
		}
		if err := vo.WriteVLong(v); err != nil {
			return nil, err
		}
		return func(in *ByteArrayDataInput) error {
			got, err := in.ReadVLong()
			if err != nil {
				return fmt.Errorf("readVLong: %w", err)
			}
			if got != v {
				return fmt.Errorf("readVLong: got %d, want %d", got, v)
			}
			return nil
		}, nil
	},
}

var genWriteZInt = generator{
	name: "writeZInt",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		v := int32(rng.Uint32())
		if err := dst.(zigzagDataOutput).WriteZInt(v); err != nil {
			return nil, err
		}
		return func(in *ByteArrayDataInput) error {
			raw, err := in.ReadVInt()
			if err != nil {
				return fmt.Errorf("readZInt: %w", err)
			}
			// Decode zigzag inline: (raw >>> 1) ^ -(raw & 1).
			got := int32(uint32(raw)>>1) ^ -(raw & 1)
			if got != v {
				return fmt.Errorf("readZInt: got %d, want %d", got, v)
			}
			return nil
		}, nil
	},
}

var genWriteZLong = generator{
	name: "writeZLong",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		v := int64(rng.Uint64())
		if err := dst.(zigzagDataOutput).WriteZLong(v); err != nil {
			return nil, err
		}
		return func(in *ByteArrayDataInput) error {
			raw, err := in.ReadVLong()
			if err != nil {
				return fmt.Errorf("readZLong: %w", err)
			}
			got := int64(uint64(raw)>>1) ^ -(raw & 1)
			if got != v {
				return fmt.Errorf("readZLong: got %d, want %d", got, v)
			}
			return nil
		}, nil
	},
}

var genWriteString = generator{
	name: "writeString",
	fn: func(dst DataOutput, rng *rand.Rand) (func(*ByteArrayDataInput) error, error) {
		var v string
		if rng.IntN(50) == 0 {
			// Occasional large blob, mirroring Java.
			v = randomUnicode(rng, 2048+rng.IntN(2049))
		} else {
			v = randomUnicode(rng, rng.IntN(11))
		}
		if err := dst.WriteString(v); err != nil {
			return nil, err
		}
		return func(in *ByteArrayDataInput) error {
			got, err := in.ReadString()
			if err != nil {
				return fmt.Errorf("readString: %w", err)
			}
			if got != v {
				return fmt.Errorf("readString: got %q, want %q", got, v)
			}
			return nil
		}, nil
	},
}

// --- random helpers -----------------------------------------------------

// randomBytes returns a byte slice of length in [minLen, maxLen]. Both bounds
// are inclusive, matching com.carrotsearch.randomizedtesting.RandomBytes.
func randomBytes(rng *rand.Rand, minLen, maxLen int) []byte {
	n := minLen
	if maxLen > minLen {
		n += rng.IntN(maxLen - minLen + 1)
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(rng.Uint32())
	}
	return out
}

// randomUnicode produces a string whose UTF-16 representation has roughly
// `units` code units, mirroring RandomStrings.randomUnicodeOfLength. The
// surrogate range U+D800..U+DFFF is avoided so the output is always a valid
// Go string.
func randomUnicode(rng *rand.Rand, units int) string {
	if units <= 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(units * 2)
	produced := 0
	for produced < units {
		var r rune
		switch rng.IntN(5) {
		case 0:
			r = rune(rng.IntN(0x80))
		case 1:
			r = rune(0x80 + rng.IntN(0x800-0x80))
		case 2:
			r = rune(0x800 + rng.IntN(0xD800-0x800))
		case 3:
			r = rune(0xE000 + rng.IntN(0x10000-0xE000))
		default:
			r = rune(0x10000 + rng.IntN(0x110000-0x10000))
		}
		// Skip lone surrogates defensively.
		if utf16.IsSurrogate(r) {
			continue
		}
		b.WriteRune(r)
		if r > 0xFFFF {
			produced += 2
		} else {
			produced++
		}
	}
	return b.String()
}

// bytesEqual avoids pulling in the slices package for a single comparison.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// stringSeed mixes a string into a uint64 via FNV-1a, providing a
// deterministic test-name-derived seed component.
func stringSeed(s string) uint64 {
	const (
		offset uint64 = 1469598103934665603
		prime  uint64 = 1099511628211
	)
	h := offset
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime
	}
	return h
}

// TestBaseDataOutputContract_ByteBuffersDataOutput pins the helper against
// the canonical Lucene-compatible implementation in this package, exercising
// every generator (including the zigzag pair). Other concrete DataOutput
// impls in the tree should add their own counterpart calling
// AssertDataOutputRandomizedWrites.
func TestBaseDataOutputContract_ByteBuffersDataOutput(t *testing.T) {
	AssertDataOutputRandomizedWrites(
		t,
		func() DataOutput { return NewByteBuffersDataOutput() },
		func(d DataOutput) []byte { return d.(*ByteBuffersDataOutput).ToArrayCopy() },
	)
}
