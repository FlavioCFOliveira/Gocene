// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene103

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// roundtripVIntBlock writes docBuffer/freqBuffer via WriteVIntBlock then reads
// them back via ReadVIntBlock and returns the restored slices.
func roundtripVIntBlock(t *testing.T, docBuffer, freqBuffer []int32, num int, writeFreqs bool) ([]int32, []int32) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	// Write.
	out, err := dir.CreateOutput("postings.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	// Preserve input buffers; WriteVIntBlock mutates docBuffer in-place.
	docCopy := append([]int32(nil), docBuffer[:num]...)
	freqCopy := append([]int32(nil), freqBuffer[:num]...)
	if err := WriteVIntBlock(out, docCopy, freqCopy, num, writeFreqs); err != nil {
		t.Fatalf("WriteVIntBlock: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	// Read.
	in, err := dir.OpenInput("postings.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })

	restoredDocs := make([]int32, num)
	restoredFreqs := make([]int32, num)
	if err := ReadVIntBlock(in, restoredDocs, restoredFreqs, num, writeFreqs, writeFreqs); err != nil {
		t.Fatalf("ReadVIntBlock: %v", err)
	}
	return restoredDocs, restoredFreqs
}

// TestPostingsUtil_IntegerOverflow is the direct port of
// TestPostingsUtil.testIntegerOverflow from the Java reference.
// It exercises the integer-overflow bug fixed in
// https://github.com/apache/lucene/issues/13373 for both a
// small size (1–3, first value is a regular vint) and a
// larger size (4–BlockSize, first value is a group vint).
func TestPostingsUtil_IntegerOverflow(t *testing.T) {
	const delta = 1 << 30

	// Sizes chosen to cover both encoding paths:
	//   size=1  → falls through to vint encoding
	//   size=4  → enters group-vint encoding
	sizes := []int{1, 2, 4, BlockSize / 2}
	for _, size := range sizes {
		docDeltaBuffer := make([]int32, size)
		freqBuffer := make([]int32, size)
		docDeltaBuffer[0] = delta

		restoredDocs, _ := roundtripVIntBlock(t, docDeltaBuffer, freqBuffer, size, true)
		if restoredDocs[0] != delta {
			t.Errorf("size=%d: docs[0] got %d want %d", size, restoredDocs[0], delta)
		}
	}
}

// TestPostingsUtil_WithFreqs verifies a full roundtrip of both doc deltas and
// frequencies.
func TestPostingsUtil_WithFreqs(t *testing.T) {
	n := 8
	docs := []int32{10, 5, 3, 7, 1, 2, 8, 4}
	freqs := []int32{1, 2, 1, 5, 1, 3, 1, 10}

	restoredDocs, restoredFreqs := roundtripVIntBlock(t, docs, freqs, n, true)

	for i := 0; i < n; i++ {
		if restoredDocs[i] != docs[i] {
			t.Errorf("[%d] doc: got %d want %d", i, restoredDocs[i], docs[i])
		}
		if restoredFreqs[i] != freqs[i] {
			t.Errorf("[%d] freq: got %d want %d", i, restoredFreqs[i], freqs[i])
		}
	}
}

// TestPostingsUtil_NoFreqs verifies roundtrip when the index has no freq info.
func TestPostingsUtil_NoFreqs(t *testing.T) {
	n := 6
	docs := []int32{1, 2, 3, 4, 5, 6}
	freqs := make([]int32, n)

	// writeFreqs=false: freqs are not written nor read back.
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	out, err := dir.CreateOutput("nofreq.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	docCopy := append([]int32(nil), docs...)
	if err := WriteVIntBlock(out, docCopy, freqs, n, false); err != nil {
		t.Fatalf("WriteVIntBlock: %v", err)
	}
	_ = out.Close()

	in, err := dir.OpenInput("nofreq.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })

	restoredDocs := make([]int32, n)
	restoredFreqs := make([]int32, n)
	if err := ReadVIntBlock(in, restoredDocs, restoredFreqs, n, false, false); err != nil {
		t.Fatalf("ReadVIntBlock: %v", err)
	}
	for i := 0; i < n; i++ {
		if restoredDocs[i] != docs[i] {
			t.Errorf("[%d] doc: got %d want %d", i, restoredDocs[i], docs[i])
		}
	}
}

// TestPostingsUtil_HasFreqNoDecodeFreq verifies that when indexHasFreq=true but
// decodeFreq=false, doc deltas are right-shifted (stripped of the freq bit)
// but freqBuffer is not populated.
func TestPostingsUtil_HasFreqNoDecodeFreq(t *testing.T) {
	n := 4
	// Encode: docBuffer[i] = (docDelta << 1) | freqFlag.
	// Use delta=10, freq=1 → encoded=21 (bit0=1 means freq=1).
	// Use delta=20, freq=3 → encoded=40 (bit0=0 means read extra vint).
	docs := []int32{10, 20, 5, 15}
	freqs := []int32{1, 3, 1, 2}

	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	out, err := dir.CreateOutput("hasfreq.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	docCopy := append([]int32(nil), docs...)
	freqCopy := append([]int32(nil), freqs...)
	if err := WriteVIntBlock(out, docCopy, freqCopy, n, true); err != nil {
		t.Fatalf("WriteVIntBlock: %v", err)
	}
	_ = out.Close()

	in, err := dir.OpenInput("hasfreq.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })

	restoredDocs := make([]int32, n)
	restoredFreqs := make([]int32, n)
	// indexHasFreq=true, decodeFreq=false: doc bits are shifted but freqs discarded.
	if err := ReadVIntBlock(in, restoredDocs, restoredFreqs, n, true, false); err != nil {
		t.Fatalf("ReadVIntBlock: %v", err)
	}
	for i := 0; i < n; i++ {
		if restoredDocs[i] != docs[i] {
			t.Errorf("[%d] doc: got %d want %d", i, restoredDocs[i], docs[i])
		}
	}
}

// TestPostingsUtil_BlockSize verifies a full-block roundtrip.
func TestPostingsUtil_BlockSize(t *testing.T) {
	docs := make([]int32, BlockSize)
	freqs := make([]int32, BlockSize)
	for i := range docs {
		docs[i] = int32(i + 1)
		freqs[i] = 1
	}

	restoredDocs, restoredFreqs := roundtripVIntBlock(t, docs, freqs, BlockSize, true)

	for i := 0; i < BlockSize; i++ {
		if restoredDocs[i] != docs[i] {
			t.Errorf("[%d] doc: got %d want %d", i, restoredDocs[i], docs[i])
		}
		if restoredFreqs[i] != freqs[i] {
			t.Errorf("[%d] freq: got %d want %d", i, restoredFreqs[i], freqs[i])
		}
	}
}

// TestPostingsUtil_NegativeNum verifies that negative num returns an error.
func TestPostingsUtil_NegativeNum(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, _ := dir.CreateOutput("err.dat", store.IOContext{})
	defer func() { _ = out.Close() }()

	if err := WriteVIntBlock(out, []int32{}, []int32{}, -1, false); err == nil {
		t.Error("WriteVIntBlock(-1): expected error")
	}

	in, _ := dir.OpenInput("err.dat", store.IOContext{})
	defer func() { _ = in.Close() }()

	if err := ReadVIntBlock(in, []int32{}, []int32{}, -1, false, false); err == nil {
		t.Error("ReadVIntBlock(-1): expected error")
	}
}
