// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/compressing/TestCompressingStoredFieldsFormat.java
// (Apache Lucene 10.5.0).
//
// Blockers: the class extends
// org.apache.lucene.tests.index.BaseStoredFieldsFormatTestCase and its
// getCodec() and testChunkCleanup() use
// org.apache.lucene.tests.codecs.compressing.CompressingCodec; neither
// test-framework class is ported. testChunkCleanup also reads the
// package-private Lucene90CompressingStoredFieldsReader.getNumDirtyDocs() and
// getNumDirtyChunks(), which Gocene lacks. testZFloat, testZDouble and
// testTLong run in full.

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

const (
	compressingSecond = int64(1000)
	compressingHour   = 60 * 60 * compressingSecond
	compressingDay    = 24 * compressingHour
)

func compressingTestRandom(t *testing.T) *rand.Rand {
	t.Helper()
	seed := time.Now().UnixNano()
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewSource(seed))
}

// floatToIntBits renders Float.floatToIntBits: every NaN collapses to the
// canonical 0x7fc00000.
func floatToIntBits(f float32) uint32 {
	if f != f {
		return 0x7fc00000
	}
	return math.Float32bits(f)
}

// doubleToLongBits renders Double.doubleToLongBits: every NaN collapses to the
// canonical 0x7ff8000000000000.
func doubleToLongBits(d float64) uint64 {
	if d != d {
		return 0x7ff8000000000000
	}
	return math.Float64bits(d)
}

// javaFloatMinValue is Float.MIN_VALUE (the smallest positive subnormal).
var javaFloatMinValue = math.Float32frombits(1)

func TestCompressingStoredFieldsFormat_BaseStoredFieldsFormatTestCase(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.index.BaseStoredFieldsFormatTestCase and " +
		"org.apache.lucene.tests.codecs.compressing.CompressingCodec (getCodec()) (not ported)")
}

func TestCompressingStoredFieldsFormat_testZFloat(t *testing.T) {
	buffer := make([]byte, 5) // we never need more than 5 bytes
	out := store.NewByteArrayDataOutput(buffer)
	in := store.NewByteArrayDataInput(buffer)

	roundTrip := func(f float32) {
		t.Helper()
		in.ResetWithSlice(buffer, 0, out.GetPosition())
		g, err := readZFloat(in)
		if err != nil {
			t.Fatalf("readZFloat: %v", err)
		}
		if !in.EOF() {
			t.Fatal("assertTrue(in.eof())")
		}
		if floatToIntBits(f) != floatToIntBits(g) {
			t.Fatalf("f=%v g=%v", f, g)
		}
	}

	// round-trip small integer values
	for i := math.MinInt16; i < math.MaxInt16; i++ {
		f := float32(i)
		if err := writeZFloat(out, f); err != nil {
			t.Fatalf("writeZFloat: %v", err)
		}
		roundTrip(f)

		// check that compression actually works
		if i >= -1 && i <= 123 {
			if out.GetPosition() != 1 {
				t.Fatalf("i=%d: position %d, want 1 (single byte compression)", i, out.GetPosition())
			}
		}
		out.Reset(buffer)
	}

	// round-trip special values
	special := []float32{
		float32(math.Copysign(0, -1)),
		0,
		float32(math.Inf(-1)),
		float32(math.Inf(1)),
		javaFloatMinValue,
		math.MaxFloat32,
		float32(math.NaN()),
	}
	for _, f := range special {
		if err := writeZFloat(out, f); err != nil {
			t.Fatalf("writeZFloat: %v", err)
		}
		roundTrip(f)
		out.Reset(buffer)
	}

	// round-trip random values
	r := compressingTestRandom(t)
	for i := 0; i < 100000; i++ {
		f := r.Float32() * float32(r.Intn(100)-50)
		if err := writeZFloat(out, f); err != nil {
			t.Fatalf("writeZFloat: %v", err)
		}
		limit := 4
		if floatToIntBits(f)>>31 == 1 {
			limit = 5
		}
		if !(out.GetPosition() <= limit) {
			t.Fatalf("length=%d, f=%v", out.GetPosition(), f)
		}
		roundTrip(f)
		out.Reset(buffer)
	}
}

func TestCompressingStoredFieldsFormat_testZDouble(t *testing.T) {
	buffer := make([]byte, 9) // we never need more than 9 bytes
	out := store.NewByteArrayDataOutput(buffer)
	in := store.NewByteArrayDataInput(buffer)

	roundTrip := func(x float64) {
		t.Helper()
		in.ResetWithSlice(buffer, 0, out.GetPosition())
		y, err := readZDouble(in)
		if err != nil {
			t.Fatalf("readZDouble: %v", err)
		}
		if !in.EOF() {
			t.Fatal("assertTrue(in.eof())")
		}
		if doubleToLongBits(x) != doubleToLongBits(y) {
			t.Fatalf("x=%v y=%v", x, y)
		}
	}

	// round-trip small integer values
	for i := math.MinInt16; i < math.MaxInt16; i++ {
		x := float64(i)
		if err := writeZDouble(out, x); err != nil {
			t.Fatalf("writeZDouble: %v", err)
		}
		roundTrip(x)

		// check that compression actually works
		if i >= -1 && i <= 124 {
			if out.GetPosition() != 1 {
				t.Fatalf("i=%d: position %d, want 1 (single byte compression)", i, out.GetPosition())
			}
		}
		out.Reset(buffer)
	}

	// round-trip special values
	special := []float64{
		math.Copysign(0, -1),
		0,
		math.Inf(-1),
		math.Inf(1),
		math.SmallestNonzeroFloat64,
		math.MaxFloat64,
		math.NaN(),
	}
	for _, x := range special {
		if err := writeZDouble(out, x); err != nil {
			t.Fatalf("writeZDouble: %v", err)
		}
		roundTrip(x)
		out.Reset(buffer)
	}

	// round-trip random values
	r := compressingTestRandom(t)
	for i := 0; i < 100000; i++ {
		x := r.Float64() * float64(r.Intn(100)-50)
		if err := writeZDouble(out, x); err != nil {
			t.Fatalf("writeZDouble: %v", err)
		}
		limit := 8
		if x < 0 {
			limit = 9
		}
		if !(out.GetPosition() <= limit) {
			t.Fatalf("length=%d, d=%v", out.GetPosition(), x)
		}
		roundTrip(x)
		out.Reset(buffer)
	}

	// same with floats
	for i := 0; i < 100000; i++ {
		x := float64(r.Float32() * float32(r.Intn(100)-50))
		if err := writeZDouble(out, x); err != nil {
			t.Fatalf("writeZDouble: %v", err)
		}
		if !(out.GetPosition() <= 5) {
			t.Fatalf("length=%d, d=%v", out.GetPosition(), x)
		}
		roundTrip(x)
		out.Reset(buffer)
	}
}

func TestCompressingStoredFieldsFormat_testTLong(t *testing.T) {
	buffer := make([]byte, 10) // we never need more than 10 bytes
	out := store.NewByteArrayDataOutput(buffer)
	in := store.NewByteArrayDataInput(buffer)

	roundTrip := func(l1 int64) {
		t.Helper()
		in.ResetWithSlice(buffer, 0, out.GetPosition())
		l2, err := readTLong(in)
		if err != nil {
			t.Fatalf("readTLong: %v", err)
		}
		if !in.EOF() {
			t.Fatal("assertTrue(in.eof())")
		}
		if l1 != l2 {
			t.Fatalf("l1=%d l2=%d", l1, l2)
		}
	}

	// round-trip small integer values
	for i := math.MinInt16; i < math.MaxInt16; i++ {
		for _, mul := range []int64{compressingSecond, compressingHour, compressingDay} {
			l1 := int64(i) * mul
			if err := writeTLong(out, l1); err != nil {
				t.Fatalf("writeTLong: %v", err)
			}
			roundTrip(l1)

			// check that compression actually works
			if i >= -16 && i <= 15 {
				if out.GetPosition() != 1 {
					t.Fatalf("i=%d mul=%d: position %d, want 1 (single byte compression)", i, mul, out.GetPosition())
				}
			}
			out.Reset(buffer)
		}
	}

	// round-trip random values
	r := compressingTestRandom(t)
	for i := 0; i < 100000; i++ {
		numBits := uint(r.Intn(65))
		// (1L << numBits) - 1 with Java's shift-distance masking (numBits & 63).
		l1 := int64(r.Uint64()) & ((int64(1) << (numBits & 63)) - 1)
		switch r.Intn(4) {
		case 0:
			l1 *= compressingSecond
		case 1:
			l1 *= compressingHour
		case 2:
			l1 *= compressingDay
		}
		if err := writeTLong(out, l1); err != nil {
			t.Fatalf("writeTLong: %v", err)
		}
		roundTrip(l1)
		out.Reset(buffer)
	}
}

// writes some tiny segments with incomplete compressed blocks, and ensures merge recompresses
// them.
func TestCompressingStoredFieldsFormat_testChunkCleanup(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.codecs.compressing.CompressingCodec.randomInstance(Random, int, int, " +
		"boolean, int) and Lucene90CompressingStoredFieldsReader.getNumDirtyDocs()/getNumDirtyChunks() (not ported)")
}
