// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// TestSegmentInfosCloneIsolation is the regression test for rmp #4728:
// SegmentInfos.Clone() used to shallow-copy the []*SegmentCommitInfo slice, so
// mutating a SegmentCommitInfo through the clone (e.g. AdvanceDelGen) also
// mutated the original — a data race with concurrent IndexWriter operations.
// Clone() must now deep-clone each SegmentCommitInfo (matching Lucene's
// SegmentInfos.clone()).
func TestSegmentInfosCloneIsolation(t *testing.T) {
	infos := spi.NewSegmentInfos()
	for i, name := range []string{"_0", "_1"} {
		si := schema.NewSegmentInfo(name, 10*(i+1), nil)
		sci := spi.NewSegmentCommitInfo(si, 0, 0)
		infos.Add(sci)
	}

	clone := infos.Clone()

	if clone.Size() != infos.Size() {
		t.Fatalf("clone size = %d, want %d", clone.Size(), infos.Size())
	}

	// The cloned slice must hold DISTINCT SegmentCommitInfo pointers.
	for i := 0; i < infos.Size(); i++ {
		if clone.Get(i) == infos.Get(i) {
			t.Fatalf("segment %d: clone shares the same *SegmentCommitInfo pointer as the original", i)
		}
	}

	// Mutating the clone must not affect the original.
	origDelCount := infos.Get(0).DelCount()
	origDelGen := infos.Get(0).DelGen()

	clone.Get(0).SetDelCount(origDelCount + 5)
	clone.Get(0).AdvanceDelGen()

	if got := infos.Get(0).DelCount(); got != origDelCount {
		t.Errorf("original DelCount mutated through clone: got %d, want %d", got, origDelCount)
	}
	if got := infos.Get(0).DelGen(); got != origDelGen {
		t.Errorf("original DelGen mutated through clone: got %d, want %d", got, origDelGen)
	}
}

// TestSegmentInfosCloneCarriesInMemoryRefs verifies the rmp #4728 decision to
// carry the read-only Gocene-specific in-memory references into the clone so a
// cloned SegmentInfos stays reader-equivalent to the original.
func TestSegmentInfosCloneCarriesInMemoryRefs(t *testing.T) {
	si := schema.NewSegmentInfo("_0", 1, nil)
	sci := spi.NewSegmentCommitInfo(si, 0, 0)
	fi := schema.NewFieldInfos()
	sci.SetInMemoryFieldInfos(fi)

	infos := spi.NewSegmentInfos()
	infos.Add(sci)

	clone := infos.Clone()

	if got := clone.Get(0).GetInMemoryFieldInfos(); got != fi {
		t.Errorf("clone did not carry the in-memory FieldInfos reference: got %p, want %p", got, fi)
	}
}
