// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// newInMemoryDirectoryForReader creates an in-memory ByteBuffersDirectory for testing.
func newInMemoryDirectoryForReader(t *testing.T) store.Directory {
	t.Helper()
	return store.NewByteBuffersDirectory()
}

func TestNewDirectoryTaxonomyReader(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)

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
	dir := newInMemoryDirectoryForReader(t)
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
	dir := newInMemoryDirectoryForReader(t)

	reader, err := OpenDirectoryTaxonomyReader(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if reader == nil {
		t.Fatal("expected reader to be created")
	}
}

func TestDirectoryTaxonomyReaderGetDirectory(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)

	gotDir := reader.GetDirectory()
	if gotDir != dir {
		t.Error("expected GetDirectory to return the same directory")
	}
}

func TestDirectoryTaxonomyReaderGetIndexEpoch(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)

	epoch := reader.GetIndexEpoch()
	if epoch != 0 {
		t.Errorf("expected epoch 0, got %d", epoch)
	}
}

func TestDirectoryTaxonomyReaderIsOpen(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)

	if !reader.IsOpen() {
		t.Error("expected IsOpen to be true")
	}

	reader.Close() //nolint:errcheck

	if reader.IsOpen() {
		t.Error("expected IsOpen to be false after close")
	}
}

func TestDirectoryTaxonomyReaderClose(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)

	err := reader.Close()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Closing again should not error.
	err = reader.Close()
	if err != nil {
		t.Errorf("expected no error on second close, got %v", err)
	}
}

func TestDirectoryTaxonomyReaderRefresh(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close() //nolint:errcheck

	changed, err := reader.Refresh()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if changed {
		t.Error("expected no change on initial refresh")
	}
}

func TestDirectoryTaxonomyReaderRefreshClosed(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)
	reader.Close() //nolint:errcheck

	_, err := reader.Refresh()
	if err == nil {
		t.Error("expected error when refreshing closed reader")
	}
}

func TestDirectoryTaxonomyReaderGetOrdinalFromPath(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close() //nolint:errcheck

	// Cold reader opened on empty directory: no categories loaded.
	ord := reader.GetOrdinalFromPath("electronics", "phones")
	if ord != -1 {
		t.Errorf("expected -1 for non-existent path, got %d", ord)
	}
}

func TestDirectoryTaxonomyReaderGetPathComponents(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close() //nolint:errcheck

	// Cold reader on empty directory: no categories.
	comps := reader.GetPathComponents(1)
	if comps != nil {
		t.Error("expected nil for non-existent ordinal")
	}
}

func TestDirectoryTaxonomyReaderGetFacetLabel(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close() //nolint:errcheck

	// Cold reader on empty directory: no categories.
	label := reader.GetFacetLabel(1)
	if label != nil {
		t.Error("expected nil for non-existent ordinal")
	}
}

func TestDirectoryTaxonomyReaderGetSize(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	reader, _ := NewDirectoryTaxonomyReader(dir)
	defer reader.Close() //nolint:errcheck

	// Cold reader opened on empty directory: size == 0 (no taxonomy written).
	size := reader.GetSize()
	if size != 0 {
		t.Errorf("expected size 0 for empty reader, got %d", size)
	}
}

func TestNewDirectoryTaxonomyReaderFactory(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	if factory == nil {
		t.Fatal("expected factory to be created")
	}
	if factory.directory != dir {
		t.Error("expected directory to be set")
	}
}

func TestNewDirectoryTaxonomyReaderFactoryWithOptions(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
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
	dir := newInMemoryDirectoryForReader(t)
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	reader, err := factory.Open()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if reader == nil {
		t.Fatal("expected reader to be created")
	}
	reader.Close() //nolint:errcheck
}

func TestDirectoryTaxonomyReaderFactoryOpenIfChanged(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	// Open initial reader.
	oldReader, _ := factory.Open()
	defer oldReader.Close() //nolint:errcheck

	// Try to open if changed.
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
	dir := newInMemoryDirectoryForReader(t)
	factory := NewDirectoryTaxonomyReaderFactory(dir)

	// Open with nil old reader.
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
	reader.Close() //nolint:errcheck
}

func TestNewDirectoryTaxonomyReaderManager(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
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

	manager.Close() //nolint:errcheck
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
	dir := newInMemoryDirectoryForReader(t)
	factory := NewDirectoryTaxonomyReaderFactory(dir)
	manager, _ := NewDirectoryTaxonomyReaderManager(factory)
	defer manager.Close() //nolint:errcheck

	reader := manager.Acquire()
	if reader == nil {
		t.Error("expected to acquire reader")
	}
}

func TestDirectoryTaxonomyReaderManagerMaybeRefresh(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	factory := NewDirectoryTaxonomyReaderFactory(dir)
	manager, _ := NewDirectoryTaxonomyReaderManager(factory)
	defer manager.Close() //nolint:errcheck

	err := manager.MaybeRefresh()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestDirectoryTaxonomyReaderManagerMaybeRefreshClosed(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
	factory := NewDirectoryTaxonomyReaderFactory(dir)
	manager, _ := NewDirectoryTaxonomyReaderManager(factory)
	manager.Close() //nolint:errcheck

	err := manager.MaybeRefresh()
	if err == nil {
		t.Error("expected error when refreshing closed manager")
	}
}

func TestDirectoryTaxonomyReaderManagerClose(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)
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

	// Closing again should not error.
	err = manager.Close()
	if err != nil {
		t.Errorf("expected no error on second close, got %v", err)
	}
}

// TestDirectoryTaxonomyReaderNRTRoundTrip verifies the NRT writer→reader path:
// categories added to the writer are immediately visible in an NRT reader opened
// from the writer's in-memory state, without requiring DocValues disk reads.
func TestDirectoryTaxonomyReaderNRTRoundTrip(t *testing.T) {
	dir := newInMemoryDirectoryForReader(t)

	// Open a writer and add some categories.
	tw, err := NewDirectoryTaxonomyWriter(dir)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyWriter: %v", err)
	}
	defer tw.Close() //nolint:errcheck

	ordElec, err := tw.AddCategory(NewFacetLabel("electronics"))
	if err != nil {
		t.Fatalf("AddCategory(electronics): %v", err)
	}
	ordPhones, err := tw.AddCategory(NewFacetLabel("electronics", "phones"))
	if err != nil {
		t.Fatalf("AddCategory(electronics/phones): %v", err)
	}

	// Open NRT reader from writer.
	tr, err := NewDirectoryTaxonomyReaderFromWriter(tw)
	if err != nil {
		t.Fatalf("NewDirectoryTaxonomyReaderFromWriter: %v", err)
	}
	defer tr.Close() //nolint:errcheck

	// Root at 0, electronics at 1, phones at 2.
	if ordElec != 1 {
		t.Errorf("ordElec: want 1, got %d", ordElec)
	}
	if ordPhones != 2 {
		t.Errorf("ordPhones: want 2, got %d", ordPhones)
	}

	// GetSize: root + electronics + phones = 3.
	if tr.GetSize() != 3 {
		t.Errorf("GetSize: want 3, got %d", tr.GetSize())
	}

	// GetOrdinal round-trips.
	if got := tr.GetOrdinal(NewFacetLabel("electronics")); got != ordElec {
		t.Errorf("GetOrdinal(electronics): want %d, got %d", ordElec, got)
	}
	if got := tr.GetOrdinal(NewFacetLabel("electronics", "phones")); got != ordPhones {
		t.Errorf("GetOrdinal(electronics/phones): want %d, got %d", ordPhones, got)
	}

	// GetPath round-trips.
	pathElec := tr.GetPath(ordElec)
	if pathElec == nil || pathElec.String() != NewFacetLabel("electronics").String() {
		t.Errorf("GetPath(%d): unexpected %v", ordElec, pathElec)
	}
	pathPhones := tr.GetPath(ordPhones)
	if pathPhones == nil || pathPhones.String() != NewFacetLabel("electronics", "phones").String() {
		t.Errorf("GetPath(%d): unexpected %v", ordPhones, pathPhones)
	}

	// GetParent: electronics' parent is root (0); phones' parent is electronics (1).
	if p := tr.GetParent(ordElec); p != taxoRootOrdinal {
		t.Errorf("GetParent(electronics): want %d, got %d", taxoRootOrdinal, p)
	}
	if p := tr.GetParent(ordPhones); p != ordElec {
		t.Errorf("GetParent(phones): want %d (electronics), got %d", ordElec, p)
	}
}
