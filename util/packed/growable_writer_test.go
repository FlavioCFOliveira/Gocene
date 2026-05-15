// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import "testing"

func TestGrowableWriter_GrowsOnOverflow(t *testing.T) {
	t.Parallel()
	g := NewGrowableWriter(4, 16, 0.0)
	if g.GetBitsPerValue() < 4 {
		t.Fatalf("initial bitsPerValue should be >= 4, got %d", g.GetBitsPerValue())
	}
	// 4-bit width holds 0..15.
	g.Set(0, 15)
	if got := g.Get(0); got != 15 {
		t.Errorf("Get(0) = %d, want 15", got)
	}
	// 16 doesn't fit; the writer must grow.
	g.Set(1, 16)
	if g.GetBitsPerValue() < 5 {
		t.Fatalf("after set(1,16) bitsPerValue should be >= 5, got %d", g.GetBitsPerValue())
	}
	if got := g.Get(0); got != 15 {
		t.Errorf("Get(0) after grow = %d, want 15", got)
	}
	if got := g.Get(1); got != 16 {
		t.Errorf("Get(1) = %d, want 16", got)
	}
}

func TestGrowableWriter_NegativeForces64Bit(t *testing.T) {
	t.Parallel()
	g := NewGrowableWriter(8, 4, 0.0)
	g.Set(0, -1) // any negative value -> 64-bit storage
	if got := g.GetBitsPerValue(); got != 64 {
		t.Errorf("GetBitsPerValue() after negative = %d, want 64", got)
	}
	if got := g.Get(0); got != -1 {
		t.Errorf("Get(0) = %d, want -1", got)
	}
}

func TestGrowableWriter_SetBulkGrowsOnce(t *testing.T) {
	t.Parallel()
	g := NewGrowableWriter(4, 8, 0.0)
	in := []int64{1, 2, 3, 1000} // 1000 needs >=10 bits
	g.SetBulk(0, in, 0, len(in))
	if g.GetBitsPerValue() < 10 {
		t.Errorf("GetBitsPerValue() = %d, want >= 10", g.GetBitsPerValue())
	}
	for i, want := range in {
		if got := g.Get(i); got != want {
			t.Errorf("[%d]: got %d want %d", i, got, want)
		}
	}
}

func TestGrowableWriter_FillGrows(t *testing.T) {
	t.Parallel()
	g := NewGrowableWriter(2, 6, 0.0)
	g.Fill(0, 6, 200) // needs 8 bits
	if g.GetBitsPerValue() < 8 {
		t.Errorf("GetBitsPerValue() = %d, want >= 8", g.GetBitsPerValue())
	}
	for i := 0; i < 6; i++ {
		if got := g.Get(i); got != 200 {
			t.Errorf("[%d]: got %d want 200", i, got)
		}
	}
}

func TestGrowableWriter_ResizePreservesPrefix(t *testing.T) {
	t.Parallel()
	g := NewGrowableWriter(8, 4, 0.0)
	g.Set(0, 10)
	g.Set(1, 20)
	g.Set(2, 30)
	g.Set(3, 40)
	resized := g.Resize(2)
	if resized.Size() != 2 {
		t.Fatalf("resized.Size() = %d, want 2", resized.Size())
	}
	if got := resized.Get(0); got != 10 {
		t.Errorf("resized.Get(0) = %d, want 10", got)
	}
	if got := resized.Get(1); got != 20 {
		t.Errorf("resized.Get(1) = %d, want 20", got)
	}
	// Extend: tail is zero-initialised.
	larger := g.Resize(6)
	if larger.Size() != 6 {
		t.Fatalf("larger.Size() = %d, want 6", larger.Size())
	}
	if got := larger.Get(5); got != 0 {
		t.Errorf("larger.Get(5) = %d, want 0", got)
	}
}

func TestGrowableWriter_RamBytesUsedPositive(t *testing.T) {
	t.Parallel()
	g := NewGrowableWriter(8, 100, 0.0)
	if got := g.RamBytesUsed(); got <= 0 {
		t.Errorf("RamBytesUsed() = %d, want > 0", got)
	}
}
