// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package packed

import (
	"testing"
)

// TestNewLegacyDirectMonotonicReader verifies the constructor sets Name and Version.
func TestNewLegacyDirectMonotonicReader(t *testing.T) {
	r := NewLegacyDirectMonotonicReader("1.0")
	if r.Name != "LegacyDirectMonotonicReader" {
		t.Errorf("Name: got %q want %q", r.Name, "LegacyDirectMonotonicReader")
	}
	if r.Version != "1.0" {
		t.Errorf("Version: got %q want %q", r.Version, "1.0")
	}
}

// TestLegacyDirectMonotonicReader_VersionVariants verifies version field.
func TestLegacyDirectMonotonicReader_VersionVariants(t *testing.T) {
	versions := []string{"", "0.9", "1.0", "2.0.0"}
	for _, v := range versions {
		r := NewLegacyDirectMonotonicReader(v)
		if r.Version != v {
			t.Errorf("Version: got %q want %q", r.Version, v)
		}
	}
}

// TestNewLegacyDirectMonotonicWriter verifies the constructor sets Name and Version.
func TestNewLegacyDirectMonotonicWriter(t *testing.T) {
	w := NewLegacyDirectMonotonicWriter("1.0")
	if w.Name != "LegacyDirectMonotonicWriter" {
		t.Errorf("Name: got %q want %q", w.Name, "LegacyDirectMonotonicWriter")
	}
	if w.Version != "1.0" {
		t.Errorf("Version: got %q want %q", w.Version, "1.0")
	}
}

// TestNewLegacyDirectMonotonicMeta verifies the Meta constructor computes the
// correct number of blocks and initialises all slices.
func TestNewLegacyDirectMonotonicMeta(t *testing.T) {
	tests := []struct {
		name       string
		numValues  int64
		blockShift int
		wantBlocks int
	}{
		{"empty, shift 3", 0, 3, 0},
		{"single value, shift 3", 1, 3, 1},
		{"exactly one block, shift 3", 8, 3, 1},
		{"one block + 1, shift 3", 9, 3, 2},
		{"shift 7 (128 values/block)", 128, 7, 1},
		{"shift 7, fractional", 200, 7, 2},
	}
	for _, tc := range tests {
		m := NewLegacyDirectMonotonicMeta(tc.numValues, tc.blockShift)
		if m.BlockShift != tc.blockShift {
			t.Errorf("%s: BlockShift got %d want %d", tc.name, m.BlockShift, tc.blockShift)
		}
		if m.NumBlocks != tc.wantBlocks {
			t.Errorf("%s: NumBlocks got %d want %d", tc.name, m.NumBlocks, tc.wantBlocks)
		}
		if len(m.Mins) != tc.wantBlocks {
			t.Errorf("%s: len(Mins) got %d want %d", tc.name, len(m.Mins), tc.wantBlocks)
		}
		if len(m.Avgs) != tc.wantBlocks {
			t.Errorf("%s: len(Avgs) got %d want %d", tc.name, len(m.Avgs), tc.wantBlocks)
		}
		if len(m.BPVs) != tc.wantBlocks {
			t.Errorf("%s: len(BPVs) got %d want %d", tc.name, len(m.BPVs), tc.wantBlocks)
		}
		if len(m.Offsets) != tc.wantBlocks {
			t.Errorf("%s: len(Offsets) got %d want %d", tc.name, len(m.Offsets), tc.wantBlocks)
		}
	}
}

// TestNewLegacyDirectMonotonicMeta_FieldsMutable verifies that the slices are
// writable.
func TestNewLegacyDirectMonotonicMeta_FieldsMutable(t *testing.T) {
	m := NewLegacyDirectMonotonicMeta(64, 3) // 8 blocks
	m.Mins[0] = 42
	m.Avgs[0] = 3.14
	m.BPVs[0] = 4
	m.Offsets[0] = 12345
	if m.Mins[0] != 42 {
		t.Errorf("Mins[0] expected 42 got %d", m.Mins[0])
	}
	if m.Avgs[0] != 3.14 {
		t.Errorf("Avgs[0] expected 3.14 got %f", m.Avgs[0])
	}
	if m.BPVs[0] != 4 {
		t.Errorf("BPVs[0] expected 4 got %d", m.BPVs[0])
	}
	if m.Offsets[0] != 12345 {
		t.Errorf("Offsets[0] expected 12345 got %d", m.Offsets[0])
	}
}

// TestNewLegacyDirectMonotonicMeta_UniqueInstances verifies each call returns a
// distinct Meta instance.
func TestNewLegacyDirectMonotonicMeta_UniqueInstances(t *testing.T) {
	a := NewLegacyDirectMonotonicMeta(8, 3)
	b := NewLegacyDirectMonotonicMeta(8, 3)
	if a == b {
		t.Error("NewLegacyDirectMonotonicMeta must return distinct instances")
	}
	if &a.Mins[0] == &b.Mins[0] {
		t.Error("Mins slices must be independent")
	}
}
