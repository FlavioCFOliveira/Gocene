// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import "testing"

func TestPagedMutable_RoundTrip(t *testing.T) {
	t.Parallel()
	const size = int64(1000)
	const pageSize = 64
	pm, err := NewPagedMutable(size, pageSize, 8, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	for i := int64(0); i < size; i++ {
		pm.Set(i, i&0xFF)
	}
	for i := int64(0); i < size; i++ {
		if got := pm.Get(i); got != (i & 0xFF) {
			t.Fatalf("[%d]: got %d want %d", i, got, i&0xFF)
		}
	}
	if pm.Size() != size {
		t.Errorf("Size() = %d, want %d", pm.Size(), size)
	}
}

func TestPagedMutable_Resize(t *testing.T) {
	t.Parallel()
	pm, err := NewPagedMutable(200, 64, 8, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	for i := int64(0); i < 200; i++ {
		pm.Set(i, i)
	}
	resized := pm.Resize(150)
	if resized.Size() != 150 {
		t.Errorf("Size() = %d, want 150", resized.Size())
	}
	for i := int64(0); i < 150; i++ {
		if got := resized.Get(i); got != i {
			t.Fatalf("resized.Get(%d) = %d, want %d", i, got, i)
		}
	}
	larger := pm.Resize(400)
	if larger.Size() != 400 {
		t.Errorf("Size() = %d, want 400", larger.Size())
	}
	if got := larger.Get(399); got != 0 {
		t.Errorf("larger.Get(399) = %d, want 0", got)
	}
}

func TestPagedMutable_GrowExtendsAtLeastMinSize(t *testing.T) {
	t.Parallel()
	pm, err := NewPagedMutable(100, 64, 8, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	larger := pm.Grow(500)
	if larger.Size() < 500 {
		t.Errorf("Grow(500).Size() = %d, want >= 500", larger.Size())
	}
}

func TestPagedGrowableWriter_GrowsBitsPerValuePerPage(t *testing.T) {
	t.Parallel()
	pg, err := NewPagedGrowableWriter(256, 64, 4, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	// First page: small values that fit in 4 bits.
	for i := int64(0); i < 64; i++ {
		pg.Set(i, i&0xF)
	}
	// Third page: large values that force a per-page grow.
	for i := int64(128); i < 192; i++ {
		pg.Set(i, 1<<16+i)
	}
	for i := int64(0); i < 64; i++ {
		if got := pg.Get(i); got != (i & 0xF) {
			t.Fatalf("page0[%d]: got %d want %d", i, got, i&0xF)
		}
	}
	for i := int64(128); i < 192; i++ {
		if got := pg.Get(i); got != 1<<16+i {
			t.Fatalf("page2[%d]: got %d want %d", i, got, 1<<16+i)
		}
	}
}

func TestPagedMutable_RamBytesUsedPositive(t *testing.T) {
	t.Parallel()
	pm, err := NewPagedMutable(256, 64, 8, 0.0)
	if err != nil {
		t.Fatal(err)
	}
	if got := pm.RamBytesUsed(); got <= 0 {
		t.Errorf("RamBytesUsed() = %d, want > 0", got)
	}
}
