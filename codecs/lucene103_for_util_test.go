// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Source: lucene/backward-codecs/src/test/org/apache/lucene/backward_codecs/
//         lucene103/{TestForUtil,TestForDeltaUtil,TestPForUtil}.java
//
// Isolated round-trip tests for the 128-wide ForUtil / ForDeltaUtil / PForUtil
// block primitives that back the read-only Lucene 10.3 postings format. These
// mirror the upstream test contracts (deterministic seed instead of randomized
// testing) and verify byte-exact encode -> decode recovery across every
// bitsPerValue, isolating the primitives from the higher-level postings
// round-trip in lucene103_postings_roundtrip_test.go.

package codecs

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// l103MaxValue returns the maximum unsigned value representable in bpv bits,
// mirroring PackedInts.maxValue.
func l103MaxValue(bpv int) int32 {
	if bpv >= 32 {
		return int32(^uint32(0) >> 1)
	}
	return int32((int64(1) << uint(bpv)) - 1)
}

// l103OpenInput re-opens a previously written file for reading.
func l103OpenInput(t *testing.T, dir store.Directory, name string) store.IndexInput {
	t.Helper()
	in, err := dir.OpenInput(name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput(%q): %v", name, err)
	}
	return in
}

// TestLucene103ForUtil_EncodeDecode mirrors TestForUtil.testEncodeDecode: encode
// many random 128-int blocks at every bitsPerValue in [1,31] and verify exact
// recovery and that the read pointer reaches the written end pointer.
func TestLucene103ForUtil_EncodeDecode(t *testing.T) {
	rng := rand.New(rand.NewSource(0x10331))
	const iterations = 200
	values := make([]int32, iterations*lucene103BlockSizeConst)
	bpvs := make([]int, iterations)

	for i := 0; i < iterations; i++ {
		bpv := 1 + rng.Intn(31) // [1,31]
		bpvs[i] = bpv
		maxV := l103MaxValue(bpv)
		for j := 0; j < lucene103BlockSizeConst; j++ {
			values[i*lucene103BlockSizeConst+j] = int32(rng.Int63n(int64(maxV) + 1))
		}
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("forutil.bin", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	fu := newLucene103ForUtil()
	src := make([]int32, lucene103BlockSizeConst)
	for i := 0; i < iterations; i++ {
		copy(src, values[i*lucene103BlockSizeConst:(i+1)*lucene103BlockSizeConst])
		if err := fu.encode(src, bpvs[i], out); err != nil {
			t.Fatalf("encode[%d] bpv=%d: %v", i, bpvs[i], err)
		}
	}
	endPointer := out.GetFilePointer()
	if err := out.Close(); err != nil {
		t.Fatalf("out.Close: %v", err)
	}

	in := l103OpenInput(t, dir, "forutil.bin")
	defer in.Close()
	fuDec := newLucene103ForUtil()
	restored := make([]int64, lucene103BlockSizeConst)
	for i := 0; i < iterations; i++ {
		for j := range restored {
			restored[j] = -1
		}
		if err := fuDec.decode(bpvs[i], in, restored); err != nil {
			t.Fatalf("decode[%d] bpv=%d: %v", i, bpvs[i], err)
		}
		for j := 0; j < lucene103BlockSizeConst; j++ {
			want := int64(values[i*lucene103BlockSizeConst+j])
			if restored[j] != want {
				t.Fatalf("decode[%d] bpv=%d pos=%d: got %d want %d", i, bpvs[i], j, restored[j], want)
			}
		}
	}
	if in.GetFilePointer() != endPointer {
		t.Fatalf("read end pointer %d != write end pointer %d", in.GetFilePointer(), endPointer)
	}
}

// TestLucene103ForDeltaUtil_EncodeDecode mirrors TestForDeltaUtil: encode random
// strictly-positive delta blocks and verify decodeAndPrefixSum recovers the
// running sum exactly. bpv is restricted to [1, 24] (31-7) as in Lucene.
func TestLucene103ForDeltaUtil_EncodeDecode(t *testing.T) {
	rng := rand.New(rand.NewSource(0x10332))
	const iterations = 200
	values := make([]int32, iterations*lucene103BlockSizeConst)

	for i := 0; i < iterations; i++ {
		bpv := 1 + rng.Intn(31-7) // [1,24]
		maxV := l103MaxValue(bpv)
		for j := 0; j < lucene103BlockSizeConst; j++ {
			// strictly positive deltas in [1, maxV]
			values[i*lucene103BlockSizeConst+j] = int32(1 + rng.Int63n(int64(maxV)))
		}
	}

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("fordelta.bin", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	fdu := newLucene103ForDeltaUtil()
	for i := 0; i < iterations; i++ {
		src := make([]int32, lucene103BlockSizeConst)
		copy(src, values[i*lucene103BlockSizeConst:(i+1)*lucene103BlockSizeConst])
		bpv, err := fdu.bitsRequired(src)
		if err != nil {
			t.Fatalf("bitsRequired[%d]: %v", i, err)
		}
		if err := out.WriteByte(byte(bpv)); err != nil {
			t.Fatalf("writeByte bpv[%d]: %v", i, err)
		}
		// encodeDeltas mutates src in place (collapse) — that is fine here.
		if err := fdu.encodeDeltas(bpv, src, out); err != nil {
			t.Fatalf("encodeDeltas[%d] bpv=%d: %v", i, bpv, err)
		}
	}
	endPointer := out.GetFilePointer()
	if err := out.Close(); err != nil {
		t.Fatalf("out.Close: %v", err)
	}

	in := l103OpenInput(t, dir, "fordelta.bin")
	defer in.Close()
	fduDec := newLucene103ForDeltaUtil()
	restored := make([]int64, lucene103BlockSizeConst)
	const base = 0
	for i := 0; i < iterations; i++ {
		bpvByte, err := in.ReadByte()
		if err != nil {
			t.Fatalf("readByte bpv[%d]: %v", i, err)
		}
		if err := fduDec.decodeAndPrefixSum(int(bpvByte), in, base, restored); err != nil {
			t.Fatalf("decodeAndPrefixSum[%d] bpv=%d: %v", i, bpvByte, err)
		}
		// expected[j] = base + sum(values[0..j])
		var acc int64 = base
		for j := 0; j < lucene103BlockSizeConst; j++ {
			acc += int64(values[i*lucene103BlockSizeConst+j])
			if restored[j] != acc {
				t.Fatalf("decodeAndPrefixSum[%d] bpv=%d pos=%d: got %d want %d", i, bpvByte, j, restored[j], acc)
			}
		}
	}
	if in.GetFilePointer() != endPointer {
		t.Fatalf("read end pointer %d != write end pointer %d", in.GetFilePointer(), endPointer)
	}
}

// TestLucene103PForUtil_EncodeDecode mirrors TestPForUtil: encode blocks that
// include occasional large "exception" values and verify exact recovery via the
// PFOR patch path, plus the all-equal optimisation.
func TestLucene103PForUtil_EncodeDecode(t *testing.T) {
	rng := rand.New(rand.NewSource(0x10333))
	const iterations = 200
	values := make([][]int32, iterations)

	for i := 0; i < iterations; i++ {
		block := make([]int32, lucene103BlockSizeConst)
		// Keep the base width <= 16 so that a patch of up to 8 extra bits stays
		// comfortably within the non-negative int32 range (the PFOR contract
		// requires strictly-positive small integers; patches add <= 8 bits).
		bpv := 1 + rng.Intn(16) // [1,16]
		maxV := l103MaxValue(bpv)
		for j := 0; j < lucene103BlockSizeConst; j++ {
			block[j] = int32(rng.Int63n(int64(maxV) + 1))
		}
		// Inject up to MAX_EXCEPTIONS large values, each adding up to 8 high bits
		// above the base width (the PFOR exception patch is a single byte).
		numExceptions := rng.Intn(lucene103PForMaxExceptions + 1)
		for e := 0; e < numExceptions; e++ {
			idx := rng.Intn(lucene103BlockSizeConst)
			extra := int32(1+rng.Intn(255)) << uint(bpv)
			block[idx] = extra | block[idx]
		}
		values[i] = block
	}
	// Also include an all-equal small block to exercise the bitsPerValue==0
	// optimisation in PForUtil.
	allEqual := make([]int32, lucene103BlockSizeConst)
	for j := range allEqual {
		allEqual[j] = 7
	}
	values[0] = allEqual

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("pfor.bin", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	pf := newLucene103PForUtil()
	for i := 0; i < iterations; i++ {
		src := make([]int32, lucene103BlockSizeConst)
		copy(src, values[i])
		if err := pf.encode(src, out); err != nil {
			t.Fatalf("encode[%d]: %v", i, err)
		}
	}
	endPointer := out.GetFilePointer()
	if err := out.Close(); err != nil {
		t.Fatalf("out.Close: %v", err)
	}

	in := l103OpenInput(t, dir, "pfor.bin")
	defer in.Close()
	pfDec := newLucene103PForUtil()
	restored := make([]int64, lucene103BlockSizeConst)
	for i := 0; i < iterations; i++ {
		for j := range restored {
			restored[j] = -1
		}
		if err := pfDec.decode(in, restored); err != nil {
			t.Fatalf("decode[%d]: %v", i, err)
		}
		for j := 0; j < lucene103BlockSizeConst; j++ {
			if restored[j] != int64(values[i][j]) {
				t.Fatalf("decode[%d] pos=%d: got %d want %d", i, j, restored[j], values[i][j])
			}
		}
	}
	if in.GetFilePointer() != endPointer {
		t.Fatalf("read end pointer %d != write end pointer %d", in.GetFilePointer(), endPointer)
	}
}

// TestLucene103PForUtil_Skip verifies that lucene103PForUtilSkip advances the
// input by exactly one block, leaving the next decode aligned.
func TestLucene103PForUtil_Skip(t *testing.T) {
	rng := rand.New(rand.NewSource(0x10334))
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	block0 := make([]int32, lucene103BlockSizeConst)
	block1 := make([]int32, lucene103BlockSizeConst)
	for j := 0; j < lucene103BlockSizeConst; j++ {
		block0[j] = int32(rng.Intn(1000))
		block1[j] = int32(rng.Intn(1000))
	}

	out, err := dir.CreateOutput("pforskip.bin", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	pf := newLucene103PForUtil()
	if err := pf.encode(append([]int32(nil), block0...), out); err != nil {
		t.Fatalf("encode block0: %v", err)
	}
	if err := pf.encode(append([]int32(nil), block1...), out); err != nil {
		t.Fatalf("encode block1: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("out.Close: %v", err)
	}

	in := l103OpenInput(t, dir, "pforskip.bin")
	defer in.Close()
	if err := lucene103PForUtilSkip(in); err != nil {
		t.Fatalf("skip block0: %v", err)
	}
	pfDec := newLucene103PForUtil()
	restored := make([]int64, lucene103BlockSizeConst)
	if err := pfDec.decode(in, restored); err != nil {
		t.Fatalf("decode block1 after skip: %v", err)
	}
	for j := 0; j < lucene103BlockSizeConst; j++ {
		if restored[j] != int64(block1[j]) {
			t.Fatalf("after skip pos=%d: got %d want %d", j, restored[j], block1[j])
		}
	}
}
