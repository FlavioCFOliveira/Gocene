// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// index_input_primitives_compat_test.go drives the Sprint 114 T6 three-class
// gate for the store-primitives scenario. The test depends on the Java fixture
// harness under tools/lucene-fixtures/; the LUCENE_FIXTURES_JAR environment
// variable (or a built target/lucene-fixtures.jar) must be reachable.
//
//	(a) Read-fixture     : Lucene writes  -> Gocene reads & validates.
//	(b) Write-and-verify : Gocene writes  -> Lucene verifies.
//	(c) Full round-trip  : Lucene writes dirA, Gocene reads dirA, Gocene
//	                       writes dirB, Lucene verifies dirB, bytes(dirA) ==
//	                       bytes(dirB).
package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/internal/compat"
)

// compatSeeds is the seed sweep the three-class gate runs. Includes the two
// task-mandated canary seeds (0xC0FFEE and 0xDECAF) plus boundary values.
var compatSeeds = []int64{
	0,
	1,
	12648430, // 0xC0FFEE
	912559,   // 0xDECAF
}

func requireHarness(t *testing.T) {
	t.Helper()
	if _, err := compat.Locate(); err != nil {
		if errors.Is(err, compat.ErrHarnessMissing) {
			t.Skipf("skip: %v", err)
		}
		t.Fatalf("locate harness: %v", err)
	}
}

// TestStorePrimitives_LuceneToGocene_Read exercises class (a): Lucene
// produces the fixture, Gocene parses every primitive and asserts equality
// against the deterministic expectation.
func TestStorePrimitives_LuceneToGocene_Read(t *testing.T) {
	requireHarness(t)
	for _, seed := range compatSeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := t.TempDir()
			if err := compat.GenerateInto("store-primitives", seed, dir); err != nil {
				t.Fatalf("harness gen: %v", err)
			}
			if err := ReadStorePrimitives(dir, seed); err != nil {
				t.Fatalf("Gocene read of Lucene-written fixture (seed=%d): %v", seed, err)
			}
		})
	}
}

// TestStorePrimitives_GoceneToLucene_Verify exercises class (b): Gocene
// produces the fixture, Lucene parses and asserts every primitive matches.
func TestStorePrimitives_GoceneToLucene_Verify(t *testing.T) {
	requireHarness(t)
	for _, seed := range compatSeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := t.TempDir()
			if err := WriteStorePrimitives(dir, seed); err != nil {
				t.Fatalf("Gocene write (seed=%d): %v", seed, err)
			}
			if err := compat.Verify("store-primitives", seed, dir); err != nil {
				t.Fatalf("Lucene verify of Gocene-written fixture (seed=%d): %v", seed, err)
			}
		})
	}
}

// TestStorePrimitives_FullRoundTrip exercises class (c): the full
// Lucene->Gocene->Gocene->Lucene loop, plus byte-equality of the two
// engines' outputs.
func TestStorePrimitives_FullRoundTrip(t *testing.T) {
	requireHarness(t)
	for _, seed := range compatSeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dirA := t.TempDir() // Lucene-written
			dirB := t.TempDir() // Gocene-written

			// 1. Lucene writes dirA/store-primitives.dat.
			if err := compat.GenerateInto("store-primitives", seed, dirA); err != nil {
				t.Fatalf("Lucene gen: %v", err)
			}
			// 2. Gocene reads dirA and validates every primitive.
			if err := ReadStorePrimitives(dirA, seed); err != nil {
				t.Fatalf("Gocene read: %v", err)
			}
			// 3. Gocene writes dirB/store-primitives.dat.
			if err := WriteStorePrimitives(dirB, seed); err != nil {
				t.Fatalf("Gocene write: %v", err)
			}
			// 4. Lucene verifies dirB.
			if err := compat.Verify("store-primitives", seed, dirB); err != nil {
				t.Fatalf("Lucene verify: %v", err)
			}
			// 5. Byte equality of the two engines' outputs.
			bA, err := os.ReadFile(filepath.Join(dirA, FileName))
			if err != nil {
				t.Fatalf("read dirA: %v", err)
			}
			bB, err := os.ReadFile(filepath.Join(dirB, FileName))
			if err != nil {
				t.Fatalf("read dirB: %v", err)
			}
			if !bytes.Equal(bA, bB) {
				t.Fatalf("byte mismatch between Lucene and Gocene outputs (seed=%d)\n"+
					"  Lucene size = %d\n  Gocene size = %d", seed, len(bA), len(bB))
			}
		})
	}
}
