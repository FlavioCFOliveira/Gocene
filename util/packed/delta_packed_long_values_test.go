// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"math"
	"math/rand"
	"testing"
)

// TestDeltaPackedLongValues_DefaultBuilderRoundTrip exercises
// DeltaPackedBuilderDefault end-to-end on a clustered sequence.
func TestDeltaPackedLongValues_DefaultBuilderRoundTrip(t *testing.T) {
	t.Parallel()
	b, err := DeltaPackedBuilderDefault(64, 0.0)
	if err != nil {
		t.Fatalf("DeltaPackedBuilderDefault: %v", err)
	}
	base := int64(1 << 40)
	values := make([]int64, 300)
	rng := rand.New(rand.NewSource(13))
	for i := range values {
		values[i] = base + int64(rng.Intn(1024))
	}
	for _, v := range values {
		if err := b.Add(v); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	plv := b.Build()
	if plv.Size() != int64(len(values)) {
		t.Fatalf("Size = %d, want %d", plv.Size(), len(values))
	}
	for i, want := range values {
		if got := plv.Get(int64(i)); got != want {
			t.Fatalf("[%d]: got %d want %d", i, got, want)
		}
	}
}

// TestDeltaPackedLongValues_IteratorMatchesGet verifies the iterator and
// random access return identical values across a delta-packed sequence.
func TestDeltaPackedLongValues_IteratorMatchesGet(t *testing.T) {
	t.Parallel()
	b, err := DeltaPackedBuilderDefault(64, 0.0)
	if err != nil {
		t.Fatalf("DeltaPackedBuilderDefault: %v", err)
	}
	base := int64(-50_000)
	for i := 0; i < 257; i++ {
		if err := b.Add(base + int64(i%37)); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	plv := b.Build()
	it := plv.Iterator()
	for i := int64(0); i < plv.Size(); i++ {
		if !it.HasNext() {
			t.Fatalf("iterator exhausted at %d", i)
		}
		want := plv.Get(i)
		if got := it.Next(); got != want {
			t.Fatalf("[%d]: iter=%d Get=%d", i, got, want)
		}
	}
	if it.HasNext() {
		t.Fatal("iterator should be exhausted")
	}
}

// TestDeltaPackedLongValues_ExtremesAndZeroBlock covers extreme values
// (math.MinInt64 / math.MaxInt64 within a block, and an all-zero block
// which the underlying packed strategy compresses to a NullReader).
func TestDeltaPackedLongValues_ExtremesAndZeroBlock(t *testing.T) {
	t.Parallel()
	b, err := DeltaPackedBuilderDefault(64, 0.0)
	if err != nil {
		t.Fatalf("DeltaPackedBuilderDefault: %v", err)
	}
	values := make([]int64, 0, 128)
	values = append(values, math.MinInt64, math.MinInt64+1, math.MinInt64+42)
	for i := 0; i < 64; i++ {
		values = append(values, 0)
	}
	values = append(values, math.MaxInt64-3, math.MaxInt64-2, math.MaxInt64-1, math.MaxInt64)
	for _, v := range values {
		if err := b.Add(v); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	plv := b.Build()
	for i, want := range values {
		if got := plv.Get(int64(i)); got != want {
			t.Fatalf("[%d]: got %d want %d", i, got, want)
		}
	}
}

// TestDeltaPackedLongValues_HelperMatchesStrategy locks down the
// documentation helper deltaPackedGet against the live strategy path.
func TestDeltaPackedLongValues_HelperMatchesStrategy(t *testing.T) {
	t.Parallel()
	b, err := DeltaPackedBuilderDefault(64, 0.0)
	if err != nil {
		t.Fatalf("DeltaPackedBuilderDefault: %v", err)
	}
	for i := 0; i < 200; i++ {
		if err := b.Add(int64(1000 + i)); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	plv := b.Build()
	for i := int64(0); i < plv.Size(); i++ {
		block := int(i >> uint(plv.pageShift))
		element := int(i) & plv.pageMask
		want := plv.Get(i)
		if got := deltaPackedGet(plv, block, element); got != want {
			t.Fatalf("[%d]: helper=%d Get=%d", i, got, want)
		}
	}
}
