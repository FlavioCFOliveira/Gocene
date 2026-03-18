// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"os"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// createTempDirectoryForWriter creates a temporary directory for testing.
func createTempDirectoryForWriter(t *testing.T) (store.Directory, func()) {
	tempDir, err := os.MkdirTemp("", "taxonomy_writer_test_")
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

func TestNewDirectoryTaxonomyWriter(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, err := NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writer == nil {
		t.Fatal("expected writer to be created")
	}
	if writer.directory != dir {
		t.Error("expected directory to be set")
	}
	if !writer.isOpen {
		t.Error("expected writer to be open")
	}
	if writer.nextOrdinal != 1 {
		t.Errorf("expected nextOrdinal 1, got %d", writer.nextOrdinal)
	}
}

func TestNewDirectoryTaxonomyWriterNilDirectory(t *testing.T) {
	writer, err := NewDirectoryTaxonomyWriter(nil)
	if err == nil {
		t.Error("expected error for nil directory")
	}
	if writer != nil {
		t.Error("expected nil writer for nil directory")
	}
}

func TestNewDirectoryTaxonomyWriterWithOptions(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	opts := &DirectoryTaxonomyWriterOptions{
		OpenMode: CREATE,
	}

	writer, err := NewDirectoryTaxonomyWriterWithOptions(dir, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writer == nil {
		t.Fatal("expected writer to be created")
	}
}

func TestOpenDirectoryTaxonomyWriter(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, err := OpenDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writer == nil {
		t.Fatal("expected writer to be created")
	}
}

func TestDirectoryTaxonomyWriterAddCategory(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	label := NewFacetLabel("electronics", "phones")
	ordinal, err := writer.AddCategory(label)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ordinal != 1 {
		t.Errorf("expected ordinal 1, got %d", ordinal)
	}

	// Adding same category should return same ordinal
	ordinal2, err := writer.AddCategory(label)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ordinal2 != ordinal {
		t.Errorf("expected same ordinal %d, got %d", ordinal, ordinal2)
	}
}

func TestDirectoryTaxonomyWriterAddCategoryNil(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	_, err := writer.AddCategory(nil)
	if err == nil {
		t.Error("expected error for nil label")
	}
}

func TestDirectoryTaxonomyWriterAddCategoryEmpty(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	emptyLabel := NewFacetLabelEmpty()
	_, err := writer.AddCategory(emptyLabel)
	if err == nil {
		t.Error("expected error for empty label")
	}
}

func TestDirectoryTaxonomyWriterAddCategoryClosed(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	writer.Close()

	label := NewFacetLabel("electronics")
	_, err := writer.AddCategory(label)
	if err == nil {
		t.Error("expected error when adding to closed writer")
	}
}

func TestDirectoryTaxonomyWriterAddCategoryPath(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	ordinal, err := writer.AddCategoryPath("electronics", "phones")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ordinal != 1 {
		t.Errorf("expected ordinal 1, got %d", ordinal)
	}
}

func TestDirectoryTaxonomyWriterGetSize(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// Initially size should be 0
	if writer.GetSize() != 0 {
		t.Errorf("expected size 0, got %d", writer.GetSize())
	}

	// Add a category
	writer.AddCategory(NewFacetLabel("electronics"))

	// Size should be 1 (nextOrdinal - 1 = 2 - 1 = 1)
	if writer.GetSize() != 1 {
		t.Errorf("expected size 1, got %d", writer.GetSize())
	}
}

func TestDirectoryTaxonomyWriterGetNextOrdinal(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	if writer.GetNextOrdinal() != 1 {
		t.Errorf("expected nextOrdinal 1, got %d", writer.GetNextOrdinal())
	}

	writer.AddCategory(NewFacetLabel("electronics"))

	if writer.GetNextOrdinal() != 2 {
		t.Errorf("expected nextOrdinal 2, got %d", writer.GetNextOrdinal())
	}
}

func TestDirectoryTaxonomyWriterGetDirectory(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	gotDir := writer.GetDirectory()
	if gotDir != dir {
		t.Error("expected GetDirectory to return the same directory")
	}
}

func TestDirectoryTaxonomyWriterIsOpen(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)

	if !writer.IsOpen() {
		t.Error("expected IsOpen to be true")
	}

	writer.Close()

	if writer.IsOpen() {
		t.Error("expected IsOpen to be false after close")
	}
}

func TestDirectoryTaxonomyWriterCommit(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// Add some categories
	writer.AddCategory(NewFacetLabel("electronics"))
	writer.AddCategory(NewFacetLabel("books"))

	// Commit
	err := writer.Commit()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Cache should be cleared after commit
	if writer.GetCacheSize() != 0 {
		t.Errorf("expected cache size 0 after commit, got %d", writer.GetCacheSize())
	}
}

func TestDirectoryTaxonomyWriterCommitClosed(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	writer.Close()

	err := writer.Commit()
	if err == nil {
		t.Error("expected error when committing closed writer")
	}
}

func TestDirectoryTaxonomyWriterClose(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)

	err := writer.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Closing again should not error
	err = writer.Close()
	if err != nil {
		t.Errorf("expected no error on second close, got %v", err)
	}
}

func TestDirectoryTaxonomyWriterRollback(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// Add some categories
	writer.AddCategory(NewFacetLabel("electronics"))
	writer.AddCategory(NewFacetLabel("books"))

	// Cache should have 2 items
	if writer.GetCacheSize() != 2 {
		t.Errorf("expected cache size 2, got %d", writer.GetCacheSize())
	}

	// Rollback
	err := writer.Rollback()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Cache should be cleared after rollback
	if writer.GetCacheSize() != 0 {
		t.Errorf("expected cache size 0 after rollback, got %d", writer.GetCacheSize())
	}
}

func TestDirectoryTaxonomyWriterRollbackClosed(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	writer.Close()

	err := writer.Rollback()
	if err == nil {
		t.Error("expected error when rolling back closed writer")
	}
}

func TestDirectoryTaxonomyWriterHasUncommittedChanges(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// Initially no uncommitted changes
	if writer.HasUncommittedChanges() {
		t.Error("expected no uncommitted changes initially")
	}

	// Add a category
	writer.AddCategory(NewFacetLabel("electronics"))

	// Now there should be uncommitted changes
	if !writer.HasUncommittedChanges() {
		t.Error("expected uncommitted changes after adding category")
	}

	// Commit
	writer.Commit()

	// No more uncommitted changes
	if writer.HasUncommittedChanges() {
		t.Error("expected no uncommitted changes after commit")
	}
}

func TestDirectoryTaxonomyWriterGetCacheSize(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	if writer.GetCacheSize() != 0 {
		t.Errorf("expected cache size 0, got %d", writer.GetCacheSize())
	}

	writer.AddCategory(NewFacetLabel("electronics"))

	if writer.GetCacheSize() != 1 {
		t.Errorf("expected cache size 1, got %d", writer.GetCacheSize())
	}
}

func TestNewDirectoryTaxonomyWriterFactory(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	if factory == nil {
		t.Fatal("expected factory to be created")
	}
	if factory.directory != dir {
		t.Error("expected directory to be set")
	}
}

func TestNewDirectoryTaxonomyWriterFactoryWithOptions(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	opts := &DirectoryTaxonomyWriterOptions{OpenMode: CREATE}
	factory := NewDirectoryTaxonomyWriterFactoryWithOptions(dir, opts)

	if factory == nil {
		t.Fatal("expected factory to be created")
	}
	if factory.options != opts {
		t.Error("expected options to be set")
	}
}

func TestDirectoryTaxonomyWriterFactoryOpen(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	factory := NewDirectoryTaxonomyWriterFactory(dir)

	writer, err := factory.Open()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writer == nil {
		t.Fatal("expected writer to be created")
	}
	writer.Close()
}

func TestNewDirectoryTaxonomyWriterManager(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	factory := NewDirectoryTaxonomyWriterFactory(dir)

	manager, err := NewDirectoryTaxonomyWriterManager(factory)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if manager == nil {
		t.Fatal("expected manager to be created")
	}
	if manager.current == nil {
		t.Error("expected current writer to be set")
	}
	if !manager.isOpen {
		t.Error("expected manager to be open")
	}

	manager.Close()
}

func TestNewDirectoryTaxonomyWriterManagerNilFactory(t *testing.T) {
	manager, err := NewDirectoryTaxonomyWriterManager(nil)
	if err == nil {
		t.Error("expected error for nil factory")
	}
	if manager != nil {
		t.Error("expected nil manager for nil factory")
	}
}

func TestDirectoryTaxonomyWriterManagerAcquire(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	manager, _ := NewDirectoryTaxonomyWriterManager(factory)
	defer manager.Close()

	writer := manager.Acquire()
	if writer == nil {
		t.Error("expected to acquire writer")
	}
}

func TestDirectoryTaxonomyWriterManagerCommit(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	manager, _ := NewDirectoryTaxonomyWriterManager(factory)
	defer manager.Close()

	err := manager.Commit()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDirectoryTaxonomyWriterManagerCommitClosed(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	manager, _ := NewDirectoryTaxonomyWriterManager(factory)
	manager.Close()

	err := manager.Commit()
	if err == nil {
		t.Error("expected error when committing closed manager")
	}
}

func TestDirectoryTaxonomyWriterManagerClose(t *testing.T) {
	dir, cleanup := createTempDirectoryForWriter(t)
	defer cleanup()

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	manager, _ := NewDirectoryTaxonomyWriterManager(factory)

	err := manager.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if manager.isOpen {
		t.Error("expected manager to be closed")
	}
	if manager.current != nil {
		t.Error("expected current writer to be nil after close")
	}

	// Closing again should not error
	err = manager.Close()
	if err != nil {
		t.Errorf("expected no error on second close, got %v", err)
	}
}
