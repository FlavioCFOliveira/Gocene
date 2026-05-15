// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"io"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestBlockPacked_RoundTripSmall(t *testing.T) {
	t.Parallel()
	const blockSize = 64
	out := store.NewByteArrayDataOutput(64)
	w, err := NewBlockPackedWriter(out, blockSize)
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{
		1, 2, 3, 4, 5,
		-7, -7, -7, -7,
		100, 200, 300, 400, 500,
		0, 0, 0, 0,
	}
	for _, v := range want {
		if err := w.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}
	in := store.NewByteArrayDataInput(out.GetBytes())
	r, err := NewBlockPackedReaderIterator(in, VersionCurrent, blockSize, int64(len(want)))
	if err != nil {
		t.Fatal(err)
	}
	for i, exp := range want {
		got, err := r.Next()
		if err != nil {
			t.Fatalf("Next %d: %v", i, err)
		}
		if got != exp {
			t.Errorf("[%d]: got %d want %d", i, got, exp)
		}
	}
	if _, err := r.Next(); err != io.EOF {
		t.Errorf("expected io.EOF after last value, got %v", err)
	}
}

func TestBlockPacked_RoundTripMultiBlock(t *testing.T) {
	t.Parallel()
	const blockSize = 64
	const numValues = 1000
	rng := rand.New(rand.NewSource(31337))
	values := make([]int64, numValues)
	for i := range values {
		values[i] = int64(rng.Intn(1 << 16))
	}

	out := store.NewByteArrayDataOutput(64)
	w, err := NewBlockPackedWriter(out, blockSize)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}

	in := store.NewByteArrayDataInput(out.GetBytes())
	r, err := NewBlockPackedReaderIterator(in, VersionCurrent, blockSize, numValues)
	if err != nil {
		t.Fatal(err)
	}
	for i, exp := range values {
		got, err := r.Next()
		if err != nil {
			t.Fatalf("Next %d: %v", i, err)
		}
		if got != exp {
			t.Fatalf("[%d]: got %d want %d", i, got, exp)
		}
	}
}

func TestBlockPacked_SkipMatchesNext(t *testing.T) {
	t.Parallel()
	const blockSize = 64
	const numValues = 256
	values := make([]int64, numValues)
	for i := range values {
		values[i] = int64(i * 7)
	}

	out := store.NewByteArrayDataOutput(64)
	w, err := NewBlockPackedWriter(out, blockSize)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range values {
		if err := w.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatal(err)
	}

	in := store.NewByteArrayDataInput(out.GetBytes())
	r, err := NewBlockPackedReaderIterator(in, VersionCurrent, blockSize, numValues)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Skip(130); err != nil {
		t.Fatal(err)
	}
	got, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if got != values[130] {
		t.Errorf("after Skip(130).Next() = %d, want %d", got, values[130])
	}
}
