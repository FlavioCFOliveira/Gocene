// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene50CompoundReader_Constants verifies the compound-format constants.
func TestLucene50CompoundReader_Constants(t *testing.T) {
	if compoundDataExtension != "cfs" {
		t.Errorf("compoundDataExtension: got %q want %q", compoundDataExtension, "cfs")
	}
	if compoundEntriesExtension != "cfe" {
		t.Errorf("compoundEntriesExtension: got %q want %q", compoundEntriesExtension, "cfe")
	}
	if compoundDataCodec != "Lucene50CompoundData" {
		t.Errorf("compoundDataCodec: got %q want %q", compoundDataCodec, "Lucene50CompoundData")
	}
	if compoundEntryCodec != "Lucene50CompoundEntries" {
		t.Errorf("compoundEntryCodec: got %q want %q", compoundEntryCodec, "Lucene50CompoundEntries")
	}
	if compoundVersionStart != 0 {
		t.Errorf("compoundVersionStart: got %d want 0", compoundVersionStart)
	}
	if compoundVersionCurrent != compoundVersionStart {
		t.Errorf("compoundVersionCurrent: got %d want %d", compoundVersionCurrent, compoundVersionStart)
	}
}

// TestLucene50CompoundReader_CompileTimeAssertions verifies that
// *Lucene50CompoundReader satisfies codecs.CompoundDirectory at compile time.
// (The var _ assertion in the source file would cause a build error if it did not.)
func TestLucene50CompoundReader_CompileTimeAssertions(t *testing.T) {
	var _ codecs.CompoundDirectory = (*Lucene50CompoundReader)(nil)
}

// TestLucene50CompoundReader_ReadOnlyMethods verifies that mutating Directory
// operations return errors rather than panicking.
func TestLucene50CompoundReader_ReadOnlyMethods(t *testing.T) {
	// Construct a half-initialised reader: no handle, no entries.
	// We only test the mutating-method guard, not the actual file I/O.
	r := &Lucene50CompoundReader{}

	_, err := r.CreateOutput("x", store.IOContext{})
	if err == nil {
		t.Error("CreateOutput: expected error")
	}
	if ferr := r.DeleteFile("x"); ferr == nil {
		t.Error("DeleteFile: expected error")
	}
	_, err = r.ObtainLock("x")
	if err == nil {
		t.Error("ObtainLock: expected error")
	}
}

// TestLucene50CompoundReader_EnsureOpen verifies that operations on a closed
// reader return an error.
func TestLucene50CompoundReader_EnsureOpen(t *testing.T) {
	r := &Lucene50CompoundReader{}
	// Simulate close without a real handle.
	r.closed.Store(true)

	if _, err := r.ListAll(); err == nil {
		t.Error("ListAll on closed reader: expected error")
	}
	if _, err := r.FileLength("x"); err == nil {
		t.Error("FileLength on closed reader: expected error")
	}
	if _, err := r.OpenInput("x", store.IOContext{}); err == nil {
		t.Error("OpenInput on closed reader: expected error")
	}
	if err := r.CheckIntegrity(); err == nil {
		t.Error("CheckIntegrity on closed reader: expected error")
	}
}

// TestLucene50CompoundReader_FileExists_Closed verifies that FileExists
// returns false for a closed reader without panicking.
func TestLucene50CompoundReader_FileExists_Closed(t *testing.T) {
	r := &Lucene50CompoundReader{}
	r.closed.Store(true)
	if r.FileExists("_0.cfs") {
		t.Error("FileExists on closed reader: expected false")
	}
}

// TestLucene50CompoundReader_CloseIdempotent verifies that calling Close
// twice is safe.
func TestLucene50CompoundReader_CloseIdempotent(t *testing.T) {
	// Use a noopIndexInput whose Close() can be called multiple times.
	r := &Lucene50CompoundReader{
		handle: &noopCloser{},
	}
	if err := r.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close (idempotent): %v", err)
	}
}

// TestLucene50CompoundReader_OpenMissingFile verifies that OpenInput for an
// unknown file returns an error rather than panicking.
func TestLucene50CompoundReader_OpenMissingFile(t *testing.T) {
	r := &Lucene50CompoundReader{
		segmentName: "_0",
		entries:     map[string]compoundFileEntry{},
	}
	_, err := r.OpenInput("_0.xyz", store.IOContext{})
	if err == nil {
		t.Error("OpenInput missing file: expected error")
	}
}

// TestLucene50CompoundReader_FileLengthMissing verifies that FileLength for
// an unknown file returns an error.
func TestLucene50CompoundReader_FileLengthMissing(t *testing.T) {
	r := &Lucene50CompoundReader{
		segmentName: "_0",
		entries:     map[string]compoundFileEntry{},
	}
	_, err := r.FileLength("_0.xyz")
	if err == nil {
		t.Error("FileLength missing file: expected error")
	}
}

// TestLucene50CompoundReader_ListAll verifies that ListAll prepends the
// segment name to each entry.
func TestLucene50CompoundReader_ListAll(t *testing.T) {
	r := &Lucene50CompoundReader{
		segmentName: "_0",
		entries: map[string]compoundFileEntry{
			".tim": {offset: 0, length: 10},
			".tip": {offset: 10, length: 5},
		},
	}
	files, err := r.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("ListAll: got %d files want 2", len(files))
	}
	// ListAll sorts output.
	if files[0] != "_0.tim" {
		t.Errorf("ListAll[0]: got %q want %q", files[0], "_0.tim")
	}
	if files[1] != "_0.tip" {
		t.Errorf("ListAll[1]: got %q want %q", files[1], "_0.tip")
	}
}

// TestLucene50CompoundReader_FileLength verifies FileLength via the entries map.
func TestLucene50CompoundReader_FileLength(t *testing.T) {
	r := &Lucene50CompoundReader{
		segmentName: "_0",
		entries: map[string]compoundFileEntry{
			".tim": {offset: 0, length: 42},
		},
	}
	l, err := r.FileLength("_0.tim")
	if err != nil {
		t.Fatalf("FileLength: %v", err)
	}
	if l != 42 {
		t.Errorf("FileLength: got %d want 42", l)
	}
}

// TestLucene50CompoundReader_FileExists verifies the existence check.
func TestLucene50CompoundReader_FileExists(t *testing.T) {
	r := &Lucene50CompoundReader{
		segmentName: "_0",
		entries: map[string]compoundFileEntry{
			".tim": {offset: 0, length: 42},
		},
	}
	if !r.FileExists("_0.tim") {
		t.Error("FileExists(_0.tim): expected true")
	}
	if r.FileExists("_0.xyz") {
		t.Error("FileExists(_0.xyz): expected false")
	}
}

// TestLucene50CompoundReader_String verifies the string representation.
func TestLucene50CompoundReader_String(t *testing.T) {
	r := &Lucene50CompoundReader{segmentName: "_0"}
	s := r.String()
	if s == "" {
		t.Error("String(): expected non-empty")
	}
}

// TestLucene50CompoundReader_GetDirectory verifies that GetDirectory returns
// self.
func TestLucene50CompoundReader_GetDirectory(t *testing.T) {
	r := &Lucene50CompoundReader{}
	if r.GetDirectory() != r {
		t.Error("GetDirectory: expected self")
	}
}

// TestLucene50CompoundReader_OpenOnMissingFilesReturnsError verifies that
// constructing the reader with a directory that has no .cfe file returns an
// error.
func TestLucene50CompoundReader_OpenOnMissingFilesReturnsError(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	si := index.NewSegmentInfo("_0", 0, dir)
	if err := si.SetID(make([]byte, 16)); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	_, err := NewLucene50CompoundReader(dir, si)
	if err == nil {
		t.Error("NewLucene50CompoundReader with missing .cfe: expected error")
	}
}

// TestLucene50CompoundReader_GetPendingDeletions verifies the method returns
// nil without error.
func TestLucene50CompoundReader_GetPendingDeletions(t *testing.T) {
	r := &Lucene50CompoundReader{}
	dels, err := r.GetPendingDeletions()
	if err != nil {
		t.Fatalf("GetPendingDeletions: %v", err)
	}
	if dels != nil {
		t.Errorf("GetPendingDeletions: got %v want nil", dels)
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// noopCloser implements the store.IndexInput interface minimally.
// Only Close() is used in CloseIdempotent tests.
type noopCloser struct {
	store.IndexInput
}

func (n *noopCloser) Close() error { return nil }

// Verify errReadOnlyCompound50 is not nil (sanity).
func TestLucene50CompoundReader_ErrReadOnly(t *testing.T) {
	if errReadOnlyCompound50 == nil {
		t.Error("errReadOnlyCompound50 must not be nil")
	}
	_ = errors.Is(errReadOnlyCompound50, errReadOnlyCompound50) // exercising errors package
}
