// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math/rand"
	"testing"
)

func TestPackedLongValues_PackedRoundTrip(t *testing.T) {
	t.Parallel()
	b, err := PackedBuilder(64, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	values := []int64{0, 1, 2, 3, 4, 5, 100, 200, 0, 0, 0, 65535, 9999, 4096, 1, 1}
	for _, v := range values {
		if err := b.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	plv := b.Build()
	if plv.Size() != int64(len(values)) {
		t.Errorf("Size() = %d, want %d", plv.Size(), len(values))
	}
	for i, want := range values {
		if got := plv.Get(int64(i)); got != want {
			t.Errorf("[%d]: got %d want %d", i, got, want)
		}
	}
}

func TestPackedLongValues_DeltaRoundTrip(t *testing.T) {
	t.Parallel()
	b, err := DeltaPackedBuilder(64, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	// Values close to each other → delta-packing should be effective.
	base := int64(1_000_000)
	values := make([]int64, 200)
	rng := rand.New(rand.NewSource(7))
	for i := range values {
		values[i] = base + int64(rng.Intn(16))
	}
	for _, v := range values {
		if err := b.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	plv := b.Build()
	for i, want := range values {
		if got := plv.Get(int64(i)); got != want {
			t.Fatalf("[%d]: got %d want %d", i, got, want)
		}
	}
}

func TestPackedLongValues_Iterator(t *testing.T) {
	t.Parallel()
	b, _ := PackedBuilder(64, 0.0)
	for i := int64(0); i < 200; i++ {
		_ = b.Add(i * 2)
	}
	plv := b.Build()
	it := plv.Iterator()
	for i := int64(0); i < 200; i++ {
		if !it.HasNext() {
			t.Fatalf("iterator exhausted at %d", i)
		}
		if got := it.Next(); got != i*2 {
			t.Errorf("[%d]: got %d want %d", i, got, i*2)
		}
	}
	if it.HasNext() {
		t.Error("iterator should be exhausted")
	}
}

func TestPackedLongValuesBuilder_AddAfterBuildFails(t *testing.T) {
	t.Parallel()
	b, _ := PackedBuilder(64, 0.0)
	_ = b.Add(1)
	b.Build()
	if err := b.Add(2); err == nil {
		t.Error("Add after Build() should fail, got nil")
	}
}

func TestPackedLongValues_RamBytesUsedPositive(t *testing.T) {
	t.Parallel()
	b, _ := PackedBuilder(64, 0.0)
	for i := int64(0); i < 128; i++ {
		_ = b.Add(i)
	}
	plv := b.Build()
	if got := plv.RamBytesUsed(); got <= 0 {
		t.Errorf("RamBytesUsed() = %d, want > 0", got)
	}
}
