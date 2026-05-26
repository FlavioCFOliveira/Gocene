// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// newInMemoryDirectoryForWriter creates an in-memory ByteBuffersDirectory for testing.
// ByteBuffersDirectory is always writable and avoids the CreateOutput subclass issue
// that FSDirectory (base) has.
func newInMemoryDirectoryForWriter(t *testing.T) store.Directory {
	t.Helper()
	return store.NewByteBuffersDirectory()
}

func TestNewDirectoryTaxonomyWriter(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

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
	// Root is always added at construction; nextOrdinal starts at 1.
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
	dir := newInMemoryDirectoryForWriter(t)

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
	dir := newInMemoryDirectoryForWriter(t)

	writer, err := OpenDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writer == nil {
		t.Fatal("expected writer to be created")
	}
}

func TestDirectoryTaxonomyWriterAddCategory(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// AddCategory("electronics","phones") recursively creates the ancestor
	// "electronics" at ordinal 1 first, then "electronics/phones" at ordinal 2.
	label := NewFacetLabel("electronics", "phones")
	ordinal, err := writer.AddCategory(label)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ordinal != 2 {
		t.Errorf("expected ordinal 2 (ancestor 'electronics' at 1, leaf at 2), got %d", ordinal)
	}

	// Adding same category should return same ordinal.
	ordinal2, err := writer.AddCategory(label)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ordinal2 != ordinal {
		t.Errorf("expected same ordinal %d, got %d", ordinal, ordinal2)
	}
}

func TestDirectoryTaxonomyWriterAddCategoryNil(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	_, err := writer.AddCategory(nil)
	if err == nil {
		t.Error("expected error for nil label")
	}
}

func TestDirectoryTaxonomyWriterAddCategoryEmpty(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// An empty label is the root category (ordinal 0). Lucene returns the root
	// ordinal rather than an error.
	emptyLabel := NewFacetLabelEmpty()
	ordinal, err := writer.AddCategory(emptyLabel)
	if err != nil {
		t.Errorf("expected no error for empty label (root), got %v", err)
	}
	if ordinal != taxoRootOrdinal {
		t.Errorf("expected root ordinal %d, got %d", taxoRootOrdinal, ordinal)
	}
}

func TestDirectoryTaxonomyWriterAddCategoryClosed(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	writer.Close()

	label := NewFacetLabel("electronics")
	_, err := writer.AddCategory(label)
	if err == nil {
		t.Error("expected error when adding to closed writer")
	}
}

func TestDirectoryTaxonomyWriterAddCategoryPath(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// Same as AddCategory("electronics","phones"): ancestor at 1, leaf at 2.
	ordinal, err := writer.AddCategoryPath("electronics", "phones")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ordinal != 2 {
		t.Errorf("expected ordinal 2 (ancestor at 1, leaf at 2), got %d", ordinal)
	}
}

func TestDirectoryTaxonomyWriterGetSize(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// After construction the root is at ordinal 0; GetSize() == 1.
	if writer.GetSize() != 1 {
		t.Errorf("expected initial size 1 (root), got %d", writer.GetSize())
	}

	// Add a top-level category.
	writer.AddCategory(NewFacetLabel("electronics")) //nolint:errcheck

	// Root(0) + electronics(1) = 2.
	if writer.GetSize() != 2 {
		t.Errorf("expected size 2, got %d", writer.GetSize())
	}
}

func TestDirectoryTaxonomyWriterGetNextOrdinal(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// After construction, root is at 0 and nextOrdinal == 1.
	if writer.GetNextOrdinal() != 1 {
		t.Errorf("expected nextOrdinal 1, got %d", writer.GetNextOrdinal())
	}

	writer.AddCategory(NewFacetLabel("electronics")) //nolint:errcheck

	if writer.GetNextOrdinal() != 2 {
		t.Errorf("expected nextOrdinal 2, got %d", writer.GetNextOrdinal())
	}
}

func TestDirectoryTaxonomyWriterGetDirectory(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	gotDir := writer.GetDirectory()
	if gotDir != dir {
		t.Error("expected GetDirectory to return the same directory")
	}
}

func TestDirectoryTaxonomyWriterIsOpen(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

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
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// Add some categories.
	writer.AddCategory(NewFacetLabel("electronics")) //nolint:errcheck
	writer.AddCategory(NewFacetLabel("books"))       //nolint:errcheck

	// Commit.
	err := writer.Commit()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// After commit, uncommitted-changes flag is cleared.
	if writer.HasUncommittedChanges() {
		t.Error("expected no uncommitted changes after commit")
	}

	// In-memory cache is still intact (Gocene does not evict on commit).
	// root(0) + electronics(1) + books(2) → GetCacheSize() == 2 (excludes root).
	if writer.GetCacheSize() != 2 {
		t.Errorf("expected cache size 2 after commit (cache not evicted), got %d", writer.GetCacheSize())
	}
}

func TestDirectoryTaxonomyWriterCommitClosed(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	writer.Close()

	err := writer.Commit()
	if err == nil {
		t.Error("expected error when committing closed writer")
	}
}

func TestDirectoryTaxonomyWriterClose(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)

	err := writer.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Closing again should not error.
	err = writer.Close()
	if err != nil {
		t.Errorf("expected no error on second close, got %v", err)
	}
}

func TestDirectoryTaxonomyWriterRollback(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)

	// Add some categories.
	writer.AddCategory(NewFacetLabel("electronics")) //nolint:errcheck
	writer.AddCategory(NewFacetLabel("books"))       //nolint:errcheck

	// Cache has 2 non-root categories.
	if writer.GetCacheSize() != 2 {
		t.Errorf("expected cache size 2, got %d", writer.GetCacheSize())
	}

	// Rollback mirrors Lucene semantics: closes the writer without committing.
	err := writer.Rollback()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// After rollback, the writer is closed.
	if writer.IsOpen() {
		t.Error("expected writer to be closed after Rollback (Lucene semantics)")
	}
}

func TestDirectoryTaxonomyWriterRollbackClosed(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	writer.Close()

	err := writer.Rollback()
	if err == nil {
		t.Error("expected error when rolling back closed writer")
	}
}

func TestDirectoryTaxonomyWriterHasUncommittedChanges(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// Root is added internally at construction but does not count as uncommitted.
	if writer.HasUncommittedChanges() {
		t.Error("expected no uncommitted changes initially (root is internal)")
	}

	// Add a category.
	writer.AddCategory(NewFacetLabel("electronics")) //nolint:errcheck

	// Now there should be uncommitted changes.
	if !writer.HasUncommittedChanges() {
		t.Error("expected uncommitted changes after adding category")
	}

	// Commit.
	writer.Commit() //nolint:errcheck

	// No more uncommitted changes.
	if writer.HasUncommittedChanges() {
		t.Error("expected no uncommitted changes after commit")
	}
}

func TestDirectoryTaxonomyWriterGetCacheSize(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	writer, _ := NewDirectoryTaxonomyWriter(dir)
	defer writer.Close()

	// GetCacheSize excludes the root; initially 0.
	if writer.GetCacheSize() != 0 {
		t.Errorf("expected cache size 0, got %d", writer.GetCacheSize())
	}

	writer.AddCategory(NewFacetLabel("electronics")) //nolint:errcheck

	if writer.GetCacheSize() != 1 {
		t.Errorf("expected cache size 1, got %d", writer.GetCacheSize())
	}
}

func TestNewDirectoryTaxonomyWriterFactory(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	if factory == nil {
		t.Fatal("expected factory to be created")
	}
	if factory.directory != dir {
		t.Error("expected directory to be set")
	}
}

func TestNewDirectoryTaxonomyWriterFactoryWithOptions(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

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
	dir := newInMemoryDirectoryForWriter(t)

	factory := NewDirectoryTaxonomyWriterFactory(dir)

	writer, err := factory.Open()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writer == nil {
		t.Fatal("expected writer to be created")
	}
	writer.Close() //nolint:errcheck
}

func TestNewDirectoryTaxonomyWriterManager(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

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

	manager.Close() //nolint:errcheck
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
	dir := newInMemoryDirectoryForWriter(t)

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	manager, _ := NewDirectoryTaxonomyWriterManager(factory)
	defer manager.Close() //nolint:errcheck

	writer := manager.Acquire()
	if writer == nil {
		t.Error("expected to acquire writer")
	}
}

func TestDirectoryTaxonomyWriterManagerCommit(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	manager, _ := NewDirectoryTaxonomyWriterManager(factory)
	defer manager.Close() //nolint:errcheck

	err := manager.Commit()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDirectoryTaxonomyWriterManagerCommitClosed(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

	factory := NewDirectoryTaxonomyWriterFactory(dir)
	manager, _ := NewDirectoryTaxonomyWriterManager(factory)
	manager.Close() //nolint:errcheck

	err := manager.Commit()
	if err == nil {
		t.Error("expected error when committing closed manager")
	}
}

func TestDirectoryTaxonomyWriterManagerClose(t *testing.T) {
	dir := newInMemoryDirectoryForWriter(t)

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

	// Closing again should not error.
	err = manager.Close()
	if err != nil {
		t.Errorf("expected no error on second close, got %v", err)
	}
}
