// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"os"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// createTempDirectory creates a temporary directory for testing.
func createTempDirectory(t *testing.T) (store.Directory, func()) {
	tempDir, err := os.MkdirTemp("", "taxonomy_test_")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dir, err := store.NewFSDirectory(tempDir)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create FSDirectory: %v", err)
	}

	cleanup := func() {
		dir.Close()
		os.RemoveAll(tempDir)
	}

	return dir, cleanup
}

func TestNewDirectoryTaxonomyReader(t *testing.T) {
	// Create a mock directory
	dir, cleanup := createTempDirectory(t)
	defer cleanup()

	reader, err := NewDirectoryTaxonomyReader(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if reader == nil {
		t.Fatal("expected reader to be created")
	}
	if reader.directory != dir {
		t.Error("expected directory to be set")
	}
	if !reader.isOpen {
		t.Error("expected reader to be open")
	}
}

func TestNewDirectoryTaxonomyReaderNilDirectory(t *testing.T) {
	reader, err := NewDirectoryTaxonomyReader(nil)
	if err == nil {
		t.Error("expected error for nil directory")
	}
	if reader != nil {
		t.Error("expected nil reader for nil directory")
	}
}

func TestNewDirectoryTaxonomyReaderWithOptions(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	opts := &DirectoryTaxonomyReaderOptions{
		ReadOnly: true,
	}

	reader, err := NewDirectoryTaxonomyReaderWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if reader == nil {
		t.Fatal("expected reader to be created")
	}
}

func TestOpenDirectoryTaxonomyReader(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()

	reader, err := OpenDirectoryTaxonomyReader(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if reader == nil {
		t.Fatal("expected reader to be created")
	}
}

func TestDirectoryTaxonomyReaderGetDirectory(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)

	gotDir := reader.GetDirectory()
	if gotDir != dir {
		t.Error("expected GetDirectory to return the same directory")
	}
}

func TestDirectoryTaxonomyReaderGetIndexEpoch(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)

	epoch := reader.GetIndexEpoch()
	if epoch != 0 {
		t.Errorf("expected epoch 0, got %d", epoch)
	}
}

func TestDirectoryTaxonomyReaderIsOpen(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)

	if !reader.IsOpen() {
		t.Error("expected IsOpen to be true")
	}

	reader.Close()

	if reader.IsOpen() {
		t.Error("expected IsOpen to be false after close")
	}
}

func TestDirectoryTaxonomyReaderClose(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)

	err := reader.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Closing again should not error
	err = reader.Close()
	if err != nil {
		t.Errorf("expected no error on second close, got %v", err)
	}
}

func TestDirectoryTaxonomyReaderRefresh(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close()

	changed, err := reader.Refresh()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if changed {
		t.Error("expected no change on initial refresh")
	}
}

func TestDirectoryTaxonomyReaderRefreshClosed(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)
	reader.Close()

	_, err := reader.Refresh()
	if err == nil {
		t.Error("expected error when refreshing closed reader")
	}
}

func TestDirectoryTaxonomyReaderGetOrdinalFromPath(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close()

	// Initially should return -1 (not found)
	ord := reader.GetOrdinalFromPath("electronics", "phones")
	if ord != -1 {
		t.Errorf("expected -1 for non-existent path, got %d", ord)
	}
}

func TestDirectoryTaxonomyReaderGetPathComponents(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close()

	// Initially should return nil for non-existent ordinal
	comps := reader.GetPathComponents(1)
	if comps != nil {
		t.Error("expected nil for non-existent ordinal")
	}
}

func TestDirectoryTaxonomyReaderGetFacetLabel(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close()

	// Initially should return nil for non-existent ordinal
	label := reader.GetFacetLabel(1)
	if label != nil {
		t.Error("expected nil for non-existent ordinal")
	}
}

func TestDirectoryTaxonomyReaderGetSize(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close()

	size := reader.GetSize()
	if size != 0 {
		t.Errorf("expected size 0 for empty reader, got %d", size)
	}
}

func TestNewDirectoryTaxonomyReaderFactory(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	if factory == nil {
		t.Fatal("expected factory to be created")
	}
	if factory.directory != dir {
		t.Error("expected directory to be set")
	}
}

func TestNewDirectoryTaxonomyReaderFactoryWithOptions(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	opts := &DirectoryTaxonomyReaderOptions{ReadOnly: true}
	factory := NewDirectoryTaxonomyReaderFactoryWithOptions(dir, opts)

	if factory == nil {
		t.Fatal("expected factory to be created")
	}
	if factory.options != opts {
		t.Error("expected options to be set")
	}
}

func TestDirectoryTaxonomyReaderFactoryOpen(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	reader, err := factory.Open()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if reader == nil {
		t.Fatal("expected reader to be created")
	}
	reader.Close()
}

func TestDirectoryTaxonomyReaderFactoryOpenIfChanged(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	// Open initial reader
	oldReader, _ := factory.Open()
	defer oldReader.Close()

	// Try to open if changed
	newReader, changed, err := factory.OpenIfChanged(oldReader)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if changed {
		t.Error("expected no change")
	}
	if newReader != oldReader {
		t.Error("expected same reader when no change")
	}
}

func TestDirectoryTaxonomyReaderFactoryOpenIfChangedNil(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	// Open with nil old reader
	reader, changed, err := factory.OpenIfChanged(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !changed {
		t.Error("expected changed to be true for nil old reader")
	}
	if reader == nil {
		t.Fatal("expected reader to be created")
	}
	reader.Close()
}

func TestNewDirectoryTaxonomyReaderManager(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	manager, err := NewDirectoryTaxonomyReaderManager(factory)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if manager == nil {
		t.Fatal("expected manager to be created")
	}
	if manager.current == nil {
		t.Error("expected current reader to be set")
	}
	if !manager.isOpen {
		t.Error("expected manager to be open")
	}

	manager.Close()
}

func TestNewDirectoryTaxonomyReaderManagerNilFactory(t *testing.T) {
	manager, err := NewDirectoryTaxonomyReaderManager(nil)
	if err == nil {
		t.Error("expected error for nil factory")
	}
	if manager != nil {
		t.Error("expected nil manager for nil factory")
	}
}

func TestDirectoryTaxonomyReaderManagerAcquire(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)
	manager, _ := NewDirectoryTaxonomyReaderManager(factory)
	defer manager.Close()

	reader := manager.Acquire()
	if reader == nil {
		t.Error("expected to acquire reader")
	}
}

func TestDirectoryTaxonomyReaderManagerMaybeRefresh(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)
	manager, _ := NewDirectoryTaxonomyReaderManager(factory)
	defer manager.Close()

	err := manager.MaybeRefresh()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDirectoryTaxonomyReaderManagerMaybeRefreshClosed(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)
	manager, _ := NewDirectoryTaxonomyReaderManager(factory)
	manager.Close()

	err := manager.MaybeRefresh()
	if err == nil {
		t.Error("expected error when refreshing closed manager")
	}
}

func TestDirectoryTaxonomyReaderManagerClose(t *testing.T) {
	dir, cleanup := createTempDirectory(t)
	defer cleanup()
	factory := NewDirectoryTaxonomyReaderFactory(dir)
	manager, _ := NewDirectoryTaxonomyReaderManager(factory)

	err := manager.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if manager.isOpen {
		t.Error("expected manager to be closed")
	}
	if manager.current != nil {
		t.Error("expected current reader to be nil after close")
	}

	// Closing again should not error
	err = manager.Close()
	if err != nil {
		t.Errorf("expected no error on second close, got %v", err)
	}
}
