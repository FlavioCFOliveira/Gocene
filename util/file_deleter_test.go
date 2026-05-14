// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"io/fs"
	"slices"
	"sort"
	"testing"
)

// fakeDir is a minimal FileDeleterDirectory that records deletes and
// can be primed to fail.
type fakeDir struct {
	deletes  []string
	missing  map[string]bool
	failWith map[string]error
}

func newFakeDir() *fakeDir {
	return &fakeDir{
		missing:  make(map[string]bool),
		failWith: make(map[string]error),
	}
}

func (f *fakeDir) DeleteFile(name string) error {
	if err, ok := f.failWith[name]; ok {
		return err
	}
	if f.missing[name] {
		return fs.ErrNotExist
	}
	f.deletes = append(f.deletes, name)
	return nil
}

func TestFileDeleter_IncRef_DecRef_Lifecycle(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	fd := NewFileDeleter(dir, nil)

	fd.IncRefAll([]string{"a", "b"})
	if got := fd.GetRefCount("a"); got != 1 {
		t.Errorf("RefCount(a) = %d, want 1", got)
	}
	if !fd.Exists("a") {
		t.Errorf("Exists(a) = false")
	}

	if err := fd.DecRefAll([]string{"a"}); err != nil {
		t.Errorf("DecRefAll error: %v", err)
	}
	if got := fd.GetRefCount("a"); got != 0 {
		t.Errorf("RefCount(a) after DecRefAll = %d, want 0", got)
	}
	if !slices.Contains(dir.deletes, "a") {
		t.Errorf("expected a in deletes, got %v", dir.deletes)
	}
}

func TestFileDeleter_DecRefAll_UnderflowReturnsError(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	fd := NewFileDeleter(dir, nil)
	if err := fd.DecRefAll([]string{"never"}); err == nil {
		t.Errorf("expected underflow error")
	}
}

func TestFileDeleter_SegmentsBeforeOthers(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	fd := NewFileDeleter(dir, nil)
	files := []string{"_0.dat", "segments_2", "_1.dat", "segments_3"}
	fd.IncRefAll(files)
	if err := fd.DecRefAll(files); err != nil {
		t.Fatalf("DecRefAll error: %v", err)
	}
	// First two deletes must be segments_*, in some order.
	if len(dir.deletes) != 4 {
		t.Fatalf("deletes = %v, want 4 entries", dir.deletes)
	}
	for i, name := range dir.deletes[:2] {
		if name != "segments_2" && name != "segments_3" {
			t.Errorf("deletes[%d] = %q, want segments_*", i, name)
		}
	}
	for i, name := range dir.deletes[2:] {
		if name == "segments_2" || name == "segments_3" {
			t.Errorf("deletes[%d] = %q, segments must come first", i+2, name)
		}
	}
}

func TestFileDeleter_InitRefCount_AndUnrefed(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	fd := NewFileDeleter(dir, nil)
	fd.InitRefCount("touched")
	if got := fd.GetRefCount("touched"); got != 0 {
		t.Errorf("RefCount(touched) = %d, want 0", got)
	}
	if fd.Exists("touched") {
		t.Errorf("Exists(touched) = true")
	}
	got := fd.UnrefedFiles()
	if len(got) != 1 || got[0] != "touched" {
		t.Errorf("UnrefedFiles = %v, want [touched]", got)
	}
}

func TestFileDeleter_AllFiles(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	fd := NewFileDeleter(dir, nil)
	fd.IncRefAll([]string{"a", "b", "c"})
	got := fd.AllFiles()
	sort.Strings(got)
	if !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Errorf("AllFiles = %v, want [a b c]", got)
	}
}

func TestFileDeleter_ForceDelete(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	fd := NewFileDeleter(dir, nil)
	fd.IncRef("a")
	if err := fd.ForceDelete("a"); err != nil {
		t.Fatalf("ForceDelete error: %v", err)
	}
	if fd.GetRefCount("a") != 0 {
		t.Errorf("RefCount(a) after ForceDelete = %d, want 0", fd.GetRefCount("a"))
	}
	if !slices.Contains(dir.deletes, "a") {
		t.Errorf("expected a in deletes")
	}
}

func TestFileDeleter_DeleteFileIfNoRef(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	fd := NewFileDeleter(dir, nil)
	// File with refs: should NOT be deleted.
	fd.IncRef("kept")
	if err := fd.DeleteFileIfNoRef("kept"); err != nil {
		t.Errorf("DeleteFileIfNoRef(kept) error: %v", err)
	}
	if slices.Contains(dir.deletes, "kept") {
		t.Errorf("file with refs was deleted")
	}
	// Untracked file: should be deleted.
	if err := fd.DeleteFileIfNoRef("orphan"); err != nil {
		t.Errorf("DeleteFileIfNoRef(orphan) error: %v", err)
	}
	if !slices.Contains(dir.deletes, "orphan") {
		t.Errorf("expected orphan in deletes")
	}
}

func TestFileDeleter_TolerateMissingOnDelete(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	dir.missing["gone"] = true
	fd := NewFileDeleter(dir, nil)
	fd.TolerateMissingOnDelete = true
	fd.IncRef("gone")
	if err := fd.DecRefAll([]string{"gone"}); err != nil {
		t.Errorf("DecRefAll error: %v", err)
	}
}

func TestFileDeleter_TolerateMissingOnDelete_False(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	dir.missing["gone"] = true
	fd := NewFileDeleter(dir, nil)
	fd.TolerateMissingOnDelete = false
	fd.IncRef("gone")
	err := fd.DecRefAll([]string{"gone"})
	if err == nil {
		t.Errorf("expected error when TolerateMissingOnDelete is false")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}
}

func TestFileDeleter_MessengerInvoked(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	var msgs []FileDeleterMsgType
	fd := NewFileDeleter(dir, func(typ FileDeleterMsgType, _ string) {
		msgs = append(msgs, typ)
	})
	fd.IncRef("a")
	_ = fd.DecRefAll([]string{"a"})
	hasRef := false
	hasFile := false
	for _, m := range msgs {
		if m == FileDeleterMsgRef {
			hasRef = true
		}
		if m == FileDeleterMsgFile {
			hasFile = true
		}
	}
	if !hasRef || !hasFile {
		t.Errorf("expected both REF and FILE messages, got %v", msgs)
	}
}

func TestFileDeleter_CustomSegmentsPrefix(t *testing.T) {
	t.Parallel()

	dir := newFakeDir()
	fd := NewFileDeleter(dir, nil)
	fd.SegmentsPrefix = "META_"
	files := []string{"data.bin", "META_001", "data2.bin"}
	fd.IncRefAll(files)
	_ = fd.DecRefAll(files)
	if dir.deletes[0] != "META_001" {
		t.Errorf("first delete = %q, want META_001", dir.deletes[0])
	}
}
