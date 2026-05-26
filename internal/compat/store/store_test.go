// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestStorePrimitives_RoundTrip exercises the Go-only round-trip: Write
// produces store-primitives.dat, Read parses it back and validates every
// frame, with no Java involvement. Gates the byte-determinism contract
// before any cross-engine compat test is run.
func TestStorePrimitives_RoundTrip(t *testing.T) {
	t.Parallel()
	seeds := []int64{
		0,
		1,
		12648430, // 0xC0FFEE
		912559,   // 0xDECAF
	}
	for _, seed := range seeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if err := WriteStorePrimitives(dir, seed); err != nil {
				t.Fatalf("WriteStorePrimitives(seed=%d): %v", seed, err)
			}
			if err := ReadStorePrimitives(dir, seed); err != nil {
				t.Fatalf("ReadStorePrimitives(seed=%d): %v", seed, err)
			}
		})
	}
}

// TestStorePrimitives_Determinism asserts that two writes with the same seed
// produce byte-identical output, for the two mandated canary seeds.
func TestStorePrimitives_Determinism(t *testing.T) {
	t.Parallel()
	seeds := []int64{
		12648430, // 0xC0FFEE
		912559,   // 0xDECAF
	}
	for _, seed := range seeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			t.Parallel()
			a := t.TempDir()
			b := t.TempDir()
			if err := WriteStorePrimitives(a, seed); err != nil {
				t.Fatalf("write A: %v", err)
			}
			if err := WriteStorePrimitives(b, seed); err != nil {
				t.Fatalf("write B: %v", err)
			}
			bytesA, err := os.ReadFile(filepath.Join(a, FileName))
			if err != nil {
				t.Fatalf("read A: %v", err)
			}
			bytesB, err := os.ReadFile(filepath.Join(b, FileName))
			if err != nil {
				t.Fatalf("read B: %v", err)
			}
			if !bytes.Equal(bytesA, bytesB) {
				t.Fatalf("two writes with seed=%d produced different bytes\n  A (%d B) = %x\n  B (%d B) = %x",
					seed, len(bytesA), bytesA, len(bytesB), bytesB)
			}
		})
	}
}

// TestStorePrimitives_ReadSeedMismatch asserts that ReadStorePrimitives
// rejects a fixture written with a different seed (header id mismatch).
func TestStorePrimitives_ReadSeedMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := WriteStorePrimitives(dir, 12648430); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := ReadStorePrimitives(dir, 912559); err == nil {
		t.Fatal("expected error on seed mismatch, got nil")
	}
}
