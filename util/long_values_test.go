// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import "testing"

func TestLongValues_Identity(t *testing.T) {
	for _, i := range []int64{0, 1, 42, 1_000_000_000, -7} {
		got, err := IdentityLongValues.Get(i)
		if err != nil {
			t.Fatalf("IdentityLongValues.Get(%d): unexpected error: %v", i, err)
		}
		if got != i {
			t.Fatalf("IdentityLongValues.Get(%d)=%d", i, got)
		}
	}
}

func TestLongValues_Zeroes(t *testing.T) {
	for _, i := range []int64{0, 1, -1, 1 << 62} {
		got, err := ZeroLongValues.Get(i)
		if err != nil {
			t.Fatalf("ZeroLongValues.Get(%d): unexpected error: %v", i, err)
		}
		if got != 0 {
			t.Fatalf("ZeroLongValues.Get(%d)=%d want 0", i, got)
		}
	}
}

func TestLongValues_Func(t *testing.T) {
	doubled := LongValuesFunc(func(i int64) (int64, error) { return i * 2, nil })
	for _, i := range []int64{0, 3, -4, 1 << 30} {
		got, err := doubled.Get(i)
		if err != nil {
			t.Fatalf("doubled.Get(%d): unexpected error: %v", i, err)
		}
		if got != i*2 {
			t.Fatalf("doubled.Get(%d)=%d want %d", i, got, i*2)
		}
	}
}

func TestLongValues_InterfaceUsage(t *testing.T) {
	// Confirm both globals satisfy the LongValues interface.
	var _ LongValues = IdentityLongValues
	var _ LongValues = ZeroLongValues
	var _ LongValues = LongValuesFunc(func(int64) (int64, error) { return 0, nil })
}
