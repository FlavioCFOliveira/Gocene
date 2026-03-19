// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
)

// TestXXHash32_Basic tests basic xxHash32 functionality
func TestXXHash32_Basic(t *testing.T) {
	h := NewXXHash32()

	// Test empty hash
	hash := h.Sum32()
	if hash == 0 {
		t.Error("expected non-zero hash for empty input")
	}

	// Reset and test with data
	h.Reset()
	h.Write([]byte("hello"))
	hash1 := h.Sum32()

	h.Reset()
	h.Write([]byte("hello"))
	hash2 := h.Sum32()

	if hash1 != hash2 {
		t.Error("expected same hash for same input")
	}
}

// TestXXHash32_DifferentInputs tests that different inputs produce different hashes
func TestXXHash32_DifferentInputs(t *testing.T) {
	h1 := NewXXHash32()
	h1.Write([]byte("hello"))
	hash1 := h1.Sum32()

	h2 := NewXXHash32()
	h2.Write([]byte("world"))
	hash2 := h2.Sum32()

	if hash1 == hash2 {
		t.Error("expected different hashes for different inputs")
	}
}

// TestXXHash32_Sum tests the Sum method
func TestXXHash32_Sum(t *testing.T) {
	h := NewXXHash32()
	h.Write([]byte("test"))

	sum := h.Sum(nil)
	if len(sum) != 4 {
		t.Errorf("expected 4 bytes, got %d", len(sum))
	}
}

// TestXXHash32_Size tests the Size method
func TestXXHash32_Size(t *testing.T) {
	h := NewXXHash32()
	if h.Size() != 4 {
		t.Errorf("expected size 4, got %d", h.Size())
	}
}

// TestXXHash32_BlockSize tests the BlockSize method
func TestXXHash32_BlockSize(t *testing.T) {
	h := NewXXHash32()
	if h.BlockSize() != 4 {
		t.Errorf("expected block size 4, got %d", h.BlockSize())
	}
}

// TestXXHash32_LongData tests hashing long data
func TestXXHash32_LongData(t *testing.T) {
	h := NewXXHash32()
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	h.Write(data)
	hash := h.Sum32()

	if hash == 0 {
		t.Error("expected non-zero hash for long data")
	}
}

// TestXXHash64_Basic tests basic xxHash64 functionality
func TestXXHash64_Basic(t *testing.T) {
	h := NewXXHash64()

	// Test empty hash
	hash := h.Sum64()
	if hash == 0 {
		t.Error("expected non-zero hash for empty input")
	}

	// Reset and test with data
	h.Reset()
	h.Write([]byte("hello"))
	hash1 := h.Sum64()

	h.Reset()
	h.Write([]byte("hello"))
	hash2 := h.Sum64()

	if hash1 != hash2 {
		t.Error("expected same hash for same input")
	}
}

// TestXXHash64_DifferentInputs tests that different inputs produce different hashes
func TestXXHash64_DifferentInputs(t *testing.T) {
	h1 := NewXXHash64()
	h1.Write([]byte("hello"))
	hash1 := h1.Sum64()

	h2 := NewXXHash64()
	h2.Write([]byte("world"))
	hash2 := h2.Sum64()

	if hash1 == hash2 {
		t.Error("expected different hashes for different inputs")
	}
}

// TestXXHash64_Sum tests the Sum method
func TestXXHash64_Sum(t *testing.T) {
	h := NewXXHash64()
	h.Write([]byte("test"))

	sum := h.Sum(nil)
	if len(sum) != 8 {
		t.Errorf("expected 8 bytes, got %d", len(sum))
	}
}

// TestXXHash64_Size tests the Size method
func TestXXHash64_Size(t *testing.T) {
	h := NewXXHash64()
	if h.Size() != 8 {
		t.Errorf("expected size 8, got %d", h.Size())
	}
}

// TestXXHash64_BlockSize tests the BlockSize method
func TestXXHash64_BlockSize(t *testing.T) {
	h := NewXXHash64()
	if h.BlockSize() != 8 {
		t.Errorf("expected block size 8, got %d", h.BlockSize())
	}
}

// TestXXHash64_LongData tests hashing long data
func TestXXHash64_LongData(t *testing.T) {
	h := NewXXHash64()
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	h.Write(data)
	hash := h.Sum64()

	if hash == 0 {
		t.Error("expected non-zero hash for long data")
	}
}

// TestXXHash32WithSeed tests xxHash32 with different seeds
func TestXXHash32WithSeed(t *testing.T) {
	h1 := NewXXHash32WithSeed(0)
	h1.Write([]byte("test"))
	hash1 := h1.Sum32()

	h2 := NewXXHash32WithSeed(12345)
	h2.Write([]byte("test"))
	hash2 := h2.Sum32()

	if hash1 == hash2 {
		t.Error("expected different hashes for different seeds")
	}
}

// TestXXHash64WithSeed tests xxHash64 with different seeds
func TestXXHash64WithSeed(t *testing.T) {
	h1 := NewXXHash64WithSeed(0)
	h1.Write([]byte("test"))
	hash1 := h1.Sum64()

	h2 := NewXXHash64WithSeed(12345)
	h2.Write([]byte("test"))
	hash2 := h2.Sum64()

	if hash1 == hash2 {
		t.Error("expected different hashes for different seeds")
	}
}

// TestXXHash32_IncrementalWrite tests incremental writing
func TestXXHash32_IncrementalWrite(t *testing.T) {
	h1 := NewXXHash32()
	h1.Write([]byte("hello"))
	h1.Write([]byte(" "))
	h1.Write([]byte("world"))
	hash1 := h1.Sum32()

	h2 := NewXXHash32()
	h2.Write([]byte("hello world"))
	hash2 := h2.Sum32()

	if hash1 != hash2 {
		t.Error("expected same hash for same data written incrementally")
	}
}

// TestXXHash64_IncrementalWrite tests incremental writing
func TestXXHash64_IncrementalWrite(t *testing.T) {
	h1 := NewXXHash64()
	h1.Write([]byte("hello"))
	h1.Write([]byte(" "))
	h1.Write([]byte("world"))
	hash1 := h1.Sum64()

	h2 := NewXXHash64()
	h2.Write([]byte("hello world"))
	hash2 := h2.Sum64()

	if hash1 != hash2 {
		t.Error("expected same hash for same data written incrementally")
	}
}

// TestXXHash32_Interface tests that xxHash32 implements hash.Hash32
func TestXXHash32_Interface(t *testing.T) {
	h := NewXXHash32()

	// Test that it implements the interface
	var _ interface{} = h

	// Write data
	n, err := h.Write([]byte("test"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != 4 {
		t.Errorf("expected to write 4 bytes, wrote %d", n)
	}

	// Get sum
	sum := h.Sum(nil)
	if len(sum) != 4 {
		t.Errorf("expected sum length 4, got %d", len(sum))
	}
}

// TestXXHash64_Interface tests that xxHash64 implements hash.Hash64
func TestXXHash64_Interface(t *testing.T) {
	h := NewXXHash64()

	// Test that it implements the interface
	var _ interface{} = h

	// Write data
	n, err := h.Write([]byte("test"))
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	if n != 4 {
		t.Errorf("expected to write 4 bytes, wrote %d", n)
	}

	// Get sum
	sum := h.Sum(nil)
	if len(sum) != 8 {
		t.Errorf("expected sum length 8, got %d", len(sum))
	}
}

// BenchmarkXXHash32 benchmarks xxHash32
func BenchmarkXXHash32(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := NewXXHash32()
		h.Write(data)
		h.Sum32()
	}
}

// BenchmarkXXHash64 benchmarks xxHash64
func BenchmarkXXHash64(b *testing.B) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h := NewXXHash64()
		h.Write(data)
		h.Sum64()
	}
}
