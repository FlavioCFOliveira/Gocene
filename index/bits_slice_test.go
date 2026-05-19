// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubBits is a minimal util.Bits implementation backed by a bool slice.
type stubBits struct {
	bits []bool
}

func (s *stubBits) Get(i int) bool { return s.bits[i] }
func (s *stubBits) Length() int    { return len(s.bits) }

func TestBitsSlice_GetMapsRelativeIndexToParent(t *testing.T) {
	t.Parallel()

	parent := &stubBits{bits: []bool{false, true, false, true, true, false, true}}
	slice := NewBitsSlice(parent, ReaderSlice{Start: 2, Length: 4, ReaderIndex: 0})

	if got, want := slice.Length(), 4; got != want {
		t.Fatalf("Length() = %d, want %d", got, want)
	}

	tests := []struct {
		idx  int
		want bool
	}{
		{0, false}, // parent[2]
		{1, true},  // parent[3]
		{2, true},  // parent[4]
		{3, false}, // parent[5]
	}
	for _, tc := range tests {
		if got := slice.Get(tc.idx); got != tc.want {
			t.Errorf("Get(%d) = %v, want %v", tc.idx, got, tc.want)
		}
	}
}

func TestBitsSlice_GetOutOfBoundsPanics(t *testing.T) {
	t.Parallel()

	parent := &stubBits{bits: []bool{true, true, true}}
	slice := NewBitsSlice(parent, ReaderSlice{Start: 0, Length: 2})

	cases := []int{-1, 2, 99}
	for _, idx := range cases {
		idx := idx
		t.Run("", func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatalf("Get(%d) did not panic", idx)
				}
			}()
			_ = slice.Get(idx)
		})
	}
}

func TestNewBitsSlice_NegativeLengthPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for negative length")
		}
	}()
	_ = NewBitsSlice(&stubBits{}, ReaderSlice{Start: 0, Length: -1})
}

func TestBitsSlice_ZeroLength(t *testing.T) {
	t.Parallel()

	parent := &stubBits{bits: []bool{true}}
	slice := NewBitsSlice(parent, ReaderSlice{Start: 0, Length: 0})

	if got := slice.Length(); got != 0 {
		t.Fatalf("Length() = %d, want 0", got)
	}
}

// Ensure BitsSlice satisfies util.Bits.
var _ util.Bits = (*BitsSlice)(nil)
