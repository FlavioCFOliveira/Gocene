// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// DirectoryTaxonomyWriter is a TaxonomyWriter that writes to a Directory.
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyWriter.
type DirectoryTaxonomyWriter struct {
	// directory is the store directory
	directory store.Directory

	// nextOrdinal is the next available ordinal
	nextOrdinal int

	// isOpen tracks if this writer is open
	isOpen bool

	// cache holds pending category additions
	cache map[string]int
}

// DirectoryTaxonomyWriterOptions contains options for opening a DirectoryTaxonomyWriter.
type DirectoryTaxonomyWriterOptions struct {
	// OpenMode specifies how to open the index
	OpenMode OpenMode
}

// OpenMode specifies how to open an index.
type OpenMode int

const (
	// CREATE creates a new index, removing any existing index.
	CREATE OpenMode = iota
	// APPEND opens an existing index.
	APPEND
	// CREATE_OR_APPEND creates a new index if none exists, otherwise appends.
	CREATE_OR_APPEND
)

// NewDirectoryTaxonomyWriter creates a new DirectoryTaxonomyWriter.
func NewDirectoryTaxonomyWriter(dir store.Directory) (*DirectoryTaxonomyWriter, error) {
	return NewDirectoryTaxonomyWriterWithOptions(dir, nil)
}

// NewDirectoryTaxonomyWriterWithOptions creates a new DirectoryTaxonomyWriter with options.
func NewDirectoryTaxonomyWriterWithOptions(dir store.Directory, opts *DirectoryTaxonomyWriterOptions) (*DirectoryTaxonomyWriter, error) {
	if dir == nil {
		return nil, fmt.Errorf("directory cannot be nil")
	}

	writer := &DirectoryTaxonomyWriter{
		directory:   dir,
		nextOrdinal: 1, // Start at 1, reserve 0 for invalid
		isOpen:      true,
		cache:       make(map[string]int),
	}

	// Handle open mode
	mode := CREATE_OR_APPEND
	if opts != nil {
		mode = opts.OpenMode
	}

	// Initialize based on open mode
	switch mode {
	case CREATE:
		// Clear any existing taxonomy
		if err := writer.clear(); err != nil {
			return nil, fmt.Errorf("clearing existing taxonomy: %w", err)
		}
	case APPEND:
		// Load existing taxonomy
		if err := writer.loadExisting(); err != nil {
			return nil, fmt.Errorf("loading existing taxonomy: %w", err)
		}
	case CREATE_OR_APPEND:
		// Try to load existing, otherwise start fresh
		if err := writer.loadExisting(); err != nil {
			// No existing taxonomy, start fresh
			writer.nextOrdinal = 1
		}
	}

	return writer, nil
}

// OpenDirectoryTaxonomyWriter opens a DirectoryTaxonomyWriter from the given directory.
// This is a convenience function that matches Lucene's API.
func OpenDirectoryTaxonomyWriter(dir store.Directory) (*DirectoryTaxonomyWriter, error) {
	return NewDirectoryTaxonomyWriter(dir)
}

// clear clears any existing taxonomy data.
func (dtw *DirectoryTaxonomyWriter) clear() error {
	// In a full implementation, this would delete all taxonomy files
	dtw.nextOrdinal = 1
	dtw.cache = make(map[string]int)
	return nil
}

// loadExisting loads existing taxonomy data from the directory.
func (dtw *DirectoryTaxonomyWriter) loadExisting() error {
	// In a full implementation, this would:
	// 1. Check if taxonomy files exist
	// 2. Load all category paths and their ordinals
	// 3. Set nextOrdinal appropriately
	return fmt.Errorf("no existing taxonomy found")
}

// AddCategory adds a category to the taxonomy.
// Returns the ordinal of the category.
func (dtw *DirectoryTaxonomyWriter) AddCategory(label *FacetLabel) (int, error) {
	if !dtw.isOpen {
		return -1, fmt.Errorf("writer is closed")
	}

	if label == nil || label.IsEmpty() {
		return -1, fmt.Errorf("label cannot be nil or empty")
	}

	path := label.String()

	// Check cache first
	if ord, ok := dtw.cache[path]; ok {
		return ord, nil
	}

	// In a full implementation, this would:
	// 1. Check if the category already exists in the index
	// 2. If not, add it and assign an ordinal
	// 3. Update parent-child relationships

	// For now, just assign a new ordinal
	ordinal := dtw.nextOrdinal
	dtw.nextOrdinal++
	dtw.cache[path] = ordinal

	return ordinal, nil
}

// AddCategoryPath adds a category path to the taxonomy.
// Returns the ordinal of the category.
func (dtw *DirectoryTaxonomyWriter) AddCategoryPath(components ...string) (int, error) {
	label := NewFacetLabel(components...)
	return dtw.AddCategory(label)
}

// GetSize returns the number of categories in the taxonomy.
func (dtw *DirectoryTaxonomyWriter) GetSize() int {
	return dtw.nextOrdinal - 1
}

// GetNextOrdinal returns the next available ordinal.
func (dtw *DirectoryTaxonomyWriter) GetNextOrdinal() int {
	return dtw.nextOrdinal
}

// GetDirectory returns the directory used by this writer.
func (dtw *DirectoryTaxonomyWriter) GetDirectory() store.Directory {
	return dtw.directory
}

// IsOpen returns true if this writer is open.
func (dtw *DirectoryTaxonomyWriter) IsOpen() bool {
	return dtw.isOpen
}

// Commit commits all pending changes to the directory.
func (dtw *DirectoryTaxonomyWriter) Commit() error {
	if !dtw.isOpen {
		return fmt.Errorf("writer is closed")
	}

	// In a full implementation, this would:
	// 1. Write all cached categories to the directory
	// 2. Update the taxonomy index
	// 3. Sync the directory

	// Clear the cache after commit
	dtw.cache = make(map[string]int)

	return nil
}

// Close closes this writer.
func (dtw *DirectoryTaxonomyWriter) Close() error {
	if !dtw.isOpen {
		return nil
	}

	// Commit any pending changes
	if err := dtw.Commit(); err != nil {
		return fmt.Errorf("committing on close: %w", err)
	}

	dtw.isOpen = false
	dtw.cache = nil
	return nil
}

// Rollback rolls back any pending changes.
func (dtw *DirectoryTaxonomyWriter) Rollback() error {
	if !dtw.isOpen {
		return fmt.Errorf("writer is closed")
	}

	// Clear the cache, discarding pending changes
	dtw.cache = make(map[string]int)

	return nil
}

// HasUncommittedChanges returns true if there are uncommitted changes.
func (dtw *DirectoryTaxonomyWriter) HasUncommittedChanges() bool {
	return len(dtw.cache) > 0
}

// GetCacheSize returns the number of cached categories.
func (dtw *DirectoryTaxonomyWriter) GetCacheSize() int {
	return len(dtw.cache)
}

// DirectoryTaxonomyWriterFactory creates DirectoryTaxonomyWriter instances.
type DirectoryTaxonomyWriterFactory struct {
	// directory is the store directory
	directory store.Directory

	// options are the writer options
	options *DirectoryTaxonomyWriterOptions
}

// NewDirectoryTaxonomyWriterFactory creates a new factory.
func NewDirectoryTaxonomyWriterFactory(dir store.Directory) *DirectoryTaxonomyWriterFactory {
	return &DirectoryTaxonomyWriterFactory{
		directory: dir,
		options:   nil,
	}
}

// NewDirectoryTaxonomyWriterFactoryWithOptions creates a new factory with options.
func NewDirectoryTaxonomyWriterFactoryWithOptions(dir store.Directory, opts *DirectoryTaxonomyWriterOptions) *DirectoryTaxonomyWriterFactory {
	return &DirectoryTaxonomyWriterFactory{
		directory: dir,
		options:   opts,
	}
}

// Open opens a DirectoryTaxonomyWriter.
func (f *DirectoryTaxonomyWriterFactory) Open() (*DirectoryTaxonomyWriter, error) {
	return NewDirectoryTaxonomyWriterWithOptions(f.directory, f.options)
}

// DirectoryTaxonomyWriterManager manages DirectoryTaxonomyWriter instances.
type DirectoryTaxonomyWriterManager struct {
	// factory is the writer factory
	factory *DirectoryTaxonomyWriterFactory

	// current is the current writer
	current *DirectoryTaxonomyWriter

	// isOpen tracks if this manager is open
	isOpen bool
}

// NewDirectoryTaxonomyWriterManager creates a new manager.
func NewDirectoryTaxonomyWriterManager(factory *DirectoryTaxonomyWriterFactory) (*DirectoryTaxonomyWriterManager, error) {
	if factory == nil {
		return nil, fmt.Errorf("factory cannot be nil")
	}

	// Open initial writer
	writer, err := factory.Open()
	if err != nil {
		return nil, fmt.Errorf("opening initial writer: %w", err)
	}

	return &DirectoryTaxonomyWriterManager{
		factory: factory,
		current: writer,
		isOpen:  true,
	}, nil
}

// Acquire returns the current writer.
func (m *DirectoryTaxonomyWriterManager) Acquire() *DirectoryTaxonomyWriter {
	return m.current
}

// Commit commits the current writer.
func (m *DirectoryTaxonomyWriterManager) Commit() error {
	if !m.isOpen {
		return fmt.Errorf("manager is closed")
	}
	return m.current.Commit()
}

// Close closes this manager.
func (m *DirectoryTaxonomyWriterManager) Close() error {
	if !m.isOpen {
		return nil
	}

	if m.current != nil {
		if err := m.current.Close(); err != nil {
			return err
		}
		m.current = nil
	}

	m.isOpen = false
	return nil
}
