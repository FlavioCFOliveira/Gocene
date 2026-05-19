// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// newTrackingTmpTestDir builds a SimpleFSDirectory rooted at a temp path.
// SimpleFSDirectory implements CreateTempOutput, so it satisfies the
// optional structural interface used by TrackingTmpOutputDirectoryWrapper.
func newTrackingTmpTestDir(t *testing.T) store.Directory {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	t.Cleanup(func() {
		_ = dir.Close()
	})
	return dir
}

func TestTrackingTmpOutputDirectoryWrapper_CreateOutputAllocatesTempAndTracks(t *testing.T) {
	dir := newTrackingTmpTestDir(t)
	w := NewTrackingTmpOutputDirectoryWrapper(dir)

	out, err := w.CreateOutput("logical-a", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	tmpName := out.GetName()
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if tmpName == "" {
		t.Fatal("expected non-empty temp name from underlying CreateTempOutput")
	}
	if tmpName == "logical-a" {
		t.Fatalf("expected delegate to rename logical-a to a temp file, got %q", tmpName)
	}

	got := w.GetTemporaryFiles()
	if mapped, ok := got["logical-a"]; !ok || mapped != tmpName {
		t.Fatalf("expected mapping logical-a -> %q, got %v", tmpName, got)
	}
	if !dir.FileExists(tmpName) {
		t.Fatalf("expected temp file %q to exist in delegate", tmpName)
	}
}

func TestTrackingTmpOutputDirectoryWrapper_OpenInputResolvesLogicalName(t *testing.T) {
	dir := newTrackingTmpTestDir(t)
	w := NewTrackingTmpOutputDirectoryWrapper(dir)

	out, err := w.CreateOutput("logical-b", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := out.WriteByte(0x42); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := w.OpenInput("logical-b", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput(logical): %v", err)
	}
	defer in.Close()

	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0x42 {
		t.Fatalf("ReadByte = 0x%x, want 0x42", b)
	}
}

func TestTrackingTmpOutputDirectoryWrapper_OpenInputPassesThroughUnknown(t *testing.T) {
	dir := newTrackingTmpTestDir(t)
	w := NewTrackingTmpOutputDirectoryWrapper(dir)

	// Write a file directly through the delegate so it has a known name
	// that the wrapper never observed.
	raw, err := dir.CreateOutput("preexisting", store.IOContextWrite)
	if err != nil {
		t.Fatalf("delegate CreateOutput: %v", err)
	}
	if err := raw.WriteByte(0x7F); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := w.OpenInput("preexisting", store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput pass-through: %v", err)
	}
	defer in.Close()

	b, err := in.ReadByte()
	if err != nil {
		t.Fatalf("ReadByte: %v", err)
	}
	if b != 0x7F {
		t.Fatalf("ReadByte = 0x%x, want 0x7F", b)
	}
}

func TestTrackingTmpOutputDirectoryWrapper_GetTemporaryFilesIsSnapshot(t *testing.T) {
	dir := newTrackingTmpTestDir(t)
	w := NewTrackingTmpOutputDirectoryWrapper(dir)

	out, err := w.CreateOutput("logical-c", store.IOContextWrite)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	_ = out.Close()

	snap := w.GetTemporaryFiles()
	if len(snap) != 1 {
		t.Fatalf("snapshot len = %d, want 1", len(snap))
	}
	snap["mutated"] = "nope"

	again := w.GetTemporaryFiles()
	if _, ok := again["mutated"]; ok {
		t.Fatal("internal map leaked: external mutation reflected in wrapper state")
	}
	if len(again) != 1 {
		t.Fatalf("second snapshot len = %d, want 1", len(again))
	}
}

// nonTempDir is a Directory implementation that intentionally does not
// expose CreateTempOutput; used to exercise the error path on
// TrackingTmpOutputDirectoryWrapper.CreateOutput when the delegate lacks
// temp-output capability.
type nonTempDir struct {
	store.Directory
}

func TestTrackingTmpOutputDirectoryWrapper_CreateOutputErrorsWhenDelegateLacksTempSupport(t *testing.T) {
	dir := newTrackingTmpTestDir(t)
	w := NewTrackingTmpOutputDirectoryWrapper(&nonTempDir{Directory: dir})

	_, err := w.CreateOutput("logical-d", store.IOContextWrite)
	if err == nil {
		t.Fatal("expected error when delegate has no CreateTempOutput, got nil")
	}
	if !strings.Contains(err.Error(), "CreateTempOutput") {
		t.Fatalf("error %v does not mention CreateTempOutput", err)
	}
}
