// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestMonotonicBlockPacked_RoundTripLinear(t *testing.T) {
	t.Parallel()
	values := linearSequence(0, 5, 256)
	out := store.NewByteArrayDataOutput(64)
	w, err := NewMonotonicBlockPackedWriter(out, 64)
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
	r, err := NewMonotonicBlockPackedReader(in, VersionCurrent, 64, int64(len(values)))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range values {
		if got := r.Get(int64(i)); got != want {
			t.Fatalf("[%d]: got %d want %d", i, got, want)
		}
	}
}

func TestMonotonicBlockPacked_RoundTripNoisy(t *testing.T) {
	t.Parallel()
	values := noisySequence(1000, 11, 0.4, 320, 99)
	out := store.NewByteArrayDataOutput(64)
	w, err := NewMonotonicBlockPackedWriter(out, 64)
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
	r, err := NewMonotonicBlockPackedReader(in, VersionCurrent, 64, int64(len(values)))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range values {
		if got := r.Get(int64(i)); got != want {
			t.Fatalf("[%d]: got %d want %d", i, got, want)
		}
	}
}

func TestMonotonicBlockPacked_RejectsNegative(t *testing.T) {
	t.Parallel()
	out := store.NewByteArrayDataOutput(64)
	w, err := NewMonotonicBlockPackedWriter(out, 64)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Add(-1); err == nil {
		t.Error("expected error for negative value, got nil")
	}
}
