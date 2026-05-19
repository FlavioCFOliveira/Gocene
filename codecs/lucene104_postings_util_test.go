// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene104PostingsUtil_IntegerOverflow ports
// org.apache.lucene.codecs.lucene104.TestPostingsUtil#testIntegerOverflow,
// the regression test for https://github.com/apache/lucene/issues/13373:
// a doc delta of 1<<30 must survive the left-shift-by-one applied by
// writeVIntBlock without overflowing into a negative int.
//
// Lucene's BLOCK_SIZE is 256; the Java test picks one size from [1,3) and
// one from [4, BLOCK_SIZE) to exercise both the regular-VInt-first-value
// branch and the group-VInt-first-value branch.
func TestLucene104PostingsUtil_IntegerOverflow(t *testing.T) {
	const blockSize = 256
	rng := rand.New(rand.NewSource(0xC0FFEE))
	size1 := 1 + rng.Intn(2)           // [1, 3)
	size2 := 4 + rng.Intn(blockSize-4) // [4, BLOCK_SIZE)
	for _, size := range []int{size1, size2} {
		size := size
		t.Run("", func(t *testing.T) {
			doTestLucene104IntegerOverflow(t, size)
		})
	}
}

func doTestLucene104IntegerOverflow(t *testing.T, size int) {
	t.Helper()
	docDeltaBuffer := make([]int32, size)
	freqBuffer := make([]int32, size)
	const delta int32 = 1 << 30
	docDeltaBuffer[0] = delta

	out := store.NewByteArrayDataOutput(64)
	if err := writeLucene104VIntBlock(out, docDeltaBuffer, freqBuffer, size, true); err != nil {
		t.Fatalf("writeVIntBlock: %v", err)
	}

	restoredDocs := make([]int32, size)
	restoredFreqs := make([]int32, size)
	in := store.NewByteArrayDataInput(out.GetBytes())
	if err := readLucene104VIntBlock(in, restoredDocs, restoredFreqs, size, true, true); err != nil {
		t.Fatalf("readVIntBlock: %v", err)
	}
	if restoredDocs[0] != delta {
		t.Fatalf("restored doc delta = %d, want %d", restoredDocs[0], delta)
	}
}

// TestLucene104PostingsUtil_RoundTripFreqs exercises the freqBuffer round
// trip across both freq==1 (omitted on wire) and freq!=1 (emitted as VInt)
// branches, and the indexHasFreq=false / decodeFreq=false skip paths.
func TestLucene104PostingsUtil_RoundTripFreqs(t *testing.T) {
	docs := []int32{10, 20, 30, 40, 50, 60, 70, 80, 90}
	freqs := []int32{1, 5, 1, 1, 9, 2, 1, 17, 1}
	num := len(docs)
	docBuf := append([]int32(nil), docs...)
	freqBuf := append([]int32(nil), freqs...)

	out := store.NewByteArrayDataOutput(64)
	if err := writeLucene104VIntBlock(out, docBuf, freqBuf, num, true); err != nil {
		t.Fatalf("writeVIntBlock: %v", err)
	}
	gotDocs := make([]int32, num)
	gotFreqs := make([]int32, num)
	in := store.NewByteArrayDataInput(out.GetBytes())
	if err := readLucene104VIntBlock(in, gotDocs, gotFreqs, num, true, true); err != nil {
		t.Fatalf("readVIntBlock: %v", err)
	}
	for i := 0; i < num; i++ {
		if gotDocs[i] != docs[i] {
			t.Fatalf("doc[%d] = %d, want %d", i, gotDocs[i], docs[i])
		}
		if gotFreqs[i] != freqs[i] {
			t.Fatalf("freq[%d] = %d, want %d", i, gotFreqs[i], freqs[i])
		}
	}

	// decodeFreq=false must still strip the low bit from each doc.
	in2 := store.NewByteArrayDataInput(out.GetBytes())
	gotDocs2 := make([]int32, num)
	if err := readLucene104VIntBlock(in2, gotDocs2, nil, num, true, false); err != nil {
		t.Fatalf("readVIntBlock decodeFreq=false: %v", err)
	}
	for i := 0; i < num; i++ {
		if gotDocs2[i] != docs[i] {
			t.Fatalf("doc[%d] decodeFreq=false = %d, want %d", i, gotDocs2[i], docs[i])
		}
	}
}

// TestLucene104PostingsUtil_NoFreqs verifies the indexHasFreq=false path:
// docs are written verbatim (no shift) by writeGroupVInts and read back
// unchanged by readGroupVInts.
func TestLucene104PostingsUtil_NoFreqs(t *testing.T) {
	docs := []int32{1, 2, 3, 4, 5, 6, 7, 8}
	num := len(docs)
	docBuf := append([]int32(nil), docs...)

	out := store.NewByteArrayDataOutput(32)
	if err := writeLucene104VIntBlock(out, docBuf, nil, num, false); err != nil {
		t.Fatalf("writeVIntBlock: %v", err)
	}
	gotDocs := make([]int32, num)
	in := store.NewByteArrayDataInput(out.GetBytes())
	if err := readLucene104VIntBlock(in, gotDocs, nil, num, false, false); err != nil {
		t.Fatalf("readVIntBlock: %v", err)
	}
	for i := 0; i < num; i++ {
		if gotDocs[i] != docs[i] {
			t.Fatalf("doc[%d] = %d, want %d", i, gotDocs[i], docs[i])
		}
	}
}
