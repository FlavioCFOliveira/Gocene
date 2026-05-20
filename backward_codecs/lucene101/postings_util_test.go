// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene101

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func roundtripVInt(t *testing.T, docs, freqs []int32, num int, writeFreqs bool) ([]int32, []int32) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })

	out, err := dir.CreateOutput("p.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	dc := append([]int32(nil), docs[:num]...)
	fc := append([]int32(nil), freqs[:num]...)
	if err := WriteVIntBlock(out, dc, fc, num, writeFreqs); err != nil {
		t.Fatalf("WriteVIntBlock: %v", err)
	}
	_ = out.Close()

	in, err := dir.OpenInput("p.dat", store.IOContext{})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	t.Cleanup(func() { _ = in.Close() })

	rd := make([]int32, num)
	rf := make([]int32, num)
	if err := ReadVIntBlock(in, rd, rf, num, writeFreqs, writeFreqs); err != nil {
		t.Fatalf("ReadVIntBlock: %v", err)
	}
	return rd, rf
}

// TestPostingsUtil_IntegerOverflow ports TestPostingsUtil.testIntegerOverflow.
func TestPostingsUtil_IntegerOverflow(t *testing.T) {
	const delta = 1 << 30
	for _, size := range []int{1, 2, 4, BlockSize / 2} {
		docs := make([]int32, size)
		freqs := make([]int32, size)
		docs[0] = delta
		rd, _ := roundtripVInt(t, docs, freqs, size, true)
		if rd[0] != delta {
			t.Errorf("size=%d: docs[0] got %d want %d", size, rd[0], delta)
		}
	}
}

// TestPostingsUtil_Roundtrip verifies full roundtrip with freqs.
func TestPostingsUtil_Roundtrip(t *testing.T) {
	n := 6
	docs := []int32{5, 10, 3, 7, 1, 2}
	freqs := []int32{1, 2, 1, 3, 1, 4}
	rd, rf := roundtripVInt(t, docs, freqs, n, true)
	for i := 0; i < n; i++ {
		if rd[i] != docs[i] {
			t.Errorf("[%d] doc: got %d want %d", i, rd[i], docs[i])
		}
		if rf[i] != freqs[i] {
			t.Errorf("[%d] freq: got %d want %d", i, rf[i], freqs[i])
		}
	}
}

// TestPostingsUtil_NegativeNum verifies error on negative num.
func TestPostingsUtil_NegativeNum(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	t.Cleanup(func() { _ = dir.Close() })
	out, _ := dir.CreateOutput("e.dat", store.IOContext{})
	defer func() { _ = out.Close() }()
	if err := WriteVIntBlock(out, []int32{}, []int32{}, -1, false); err == nil {
		t.Error("WriteVIntBlock(-1): expected error")
	}
	in, _ := dir.OpenInput("e.dat", store.IOContext{})
	defer func() { _ = in.Close() }()
	if err := ReadVIntBlock(in, []int32{}, []int32{}, -1, false, false); err == nil {
		t.Error("ReadVIntBlock(-1): expected error")
	}
}
