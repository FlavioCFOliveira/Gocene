// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

// TestStressRamUsageEstimator validates the basic RamUsageEstimator
// functions. This is a lightweight replacement for the Java
// TestStressRamUsageEstimator @Nightly monster suite which allocates
// millions of objects. The core estimation code is exercised here
// without requiring GC observation or multi-hundred-MiB allocations.
func TestStressRamUsageEstimator(t *testing.T) {
	// ShallowSizeOf on a simple int should return at least 8 bytes
	// (the platform word size).
	sz := ShallowSizeOf(42)
	if sz <= 0 {
		t.Fatalf("ShallowSizeOf(42) = %d, want > 0", sz)
	}
	// SizeOfIntSlice
	slice := make([]int, 100)
	sz = SizeOfIntSlice(slice)
	if sz <= 0 {
		t.Fatalf("SizeOfIntSlice(100) = %d, want > 0", sz)
	}
	// SizeOfByteSlice
	bs := make([]byte, 1024)
	sz = SizeOfByteSlice(bs)
	if sz <= 0 {
		t.Fatalf("SizeOfByteSlice(1024) = %d, want > 0", sz)
	}
	// SizeOfString
	sz = SizeOfString("hello")
	if sz <= 0 {
		t.Fatalf("SizeOfString(...) = %d, want > 0", sz)
	}
	// AlignObjectSize must always be a multiple of 8.
	for _, v := range []int64{0, 1, 7, 8, 9, 100} {
		aligned := AlignObjectSize(v)
		if aligned%8 != 0 {
			t.Fatalf("AlignObjectSize(%d) = %d, not 8-byte aligned", v, aligned)
		}
	}
	// HumanReadableUnits smoke test.
	hr := HumanReadableUnits(1024)
	if len(hr) == 0 {
		t.Fatal("HumanReadableUnits(1024) returned empty string")
	}
	// SizeOfAccountable on an Accountable value.
	sz = SizeOfAccountable(NamedAccountableBytes("test", 42))
	if sz <= 0 {
		t.Fatalf("SizeOfAccountable(NamedAccountableBytes) = %d, want > 0", sz)
	}
}
