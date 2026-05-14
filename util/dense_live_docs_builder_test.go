// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

func newDenseTestBits(t *testing.T, maxDoc int, livePositions []int) *FixedBitSet {
	t.Helper()
	bits, err := NewFixedBitSet(maxDoc)
	if err != nil {
		t.Fatalf("NewFixedBitSet(%d): %v", maxDoc, err)
	}
	for _, p := range livePositions {
		bits.Set(p)
	}
	return bits
}

func TestDenseLiveDocs_WithDeletedCount_Agrees(t *testing.T) {
	t.Parallel()

	const maxDoc = 8
	liveBits := newDenseTestBits(t, maxDoc, []int{0, 2, 4, 6}) // 4 live, 4 deleted
	d, err := NewDenseLiveDocsBuilder(liveBits, maxDoc).
		WithDeletedCount(4).
		BuildE()
	if err != nil {
		t.Fatalf("BuildE: %v", err)
	}
	if d.DeletedCount() != 4 {
		t.Errorf("DeletedCount = %d, want 4", d.DeletedCount())
	}
	if d.LiveCount() != 4 {
		t.Errorf("LiveCount = %d, want 4", d.LiveCount())
	}
}

func TestDenseLiveDocs_WithDeletedCount_Mismatch(t *testing.T) {
	t.Parallel()

	const maxDoc = 8
	liveBits := newDenseTestBits(t, maxDoc, []int{0, 2, 4, 6}) // actual deleted = 4
	_, err := NewDenseLiveDocsBuilder(liveBits, maxDoc).
		WithDeletedCount(99).
		BuildE()
	if err == nil {
		t.Fatalf("expected mismatch error, got nil")
	}
	// Out-of-range path runs first.
	if !strings.Contains(err.Error(), "outside valid range") {
		t.Errorf("error %q lacks expected substring", err.Error())
	}
}

func TestDenseLiveDocs_WithDeletedCount_MismatchInRange(t *testing.T) {
	t.Parallel()

	const maxDoc = 8
	liveBits := newDenseTestBits(t, maxDoc, []int{0, 2, 4, 6}) // actual deleted = 4
	_, err := NewDenseLiveDocsBuilder(liveBits, maxDoc).
		WithDeletedCount(2).
		BuildE()
	if err == nil {
		t.Fatalf("expected cardinality mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("error %q lacks cardinality-mismatch substring", err.Error())
	}
}

func TestDenseLiveDocs_WithDeletedCount_NegativeRejected(t *testing.T) {
	t.Parallel()

	d := NewDenseLiveDocsBuilder(newDenseTestBits(t, 4, []int{0, 1, 2, 3}), 4).
		WithDeletedCount(-1)
	if _, err := d.BuildE(); err == nil {
		t.Errorf("expected error for negative deletedCount")
	}
}

func TestDenseLiveDocs_Build_DefaultsToCardinality(t *testing.T) {
	t.Parallel()

	const maxDoc = 6
	liveBits := newDenseTestBits(t, maxDoc, []int{0, 5}) // 2 live, 4 deleted
	d := NewDenseLiveDocsBuilder(liveBits, maxDoc).Build()
	if d.DeletedCount() != 4 {
		t.Errorf("DeletedCount = %d, want 4", d.DeletedCount())
	}
}

func TestDenseLiveDocs_MustBuild_PanicsOnError(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustBuild should panic on invalid deletedCount")
		}
	}()
	NewDenseLiveDocsBuilder(newDenseTestBits(t, 4, []int{0}), 4).
		WithDeletedCount(99).
		MustBuild()
}
