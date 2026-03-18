// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// DirectoryTaxonomyReader is a TaxonomyReader that reads from a Directory.
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyReader.
type DirectoryTaxonomyReader struct {
	*TaxonomyReader

	// directory is the store directory
	directory store.Directory

	// indexEpoch is the epoch of the index when this reader was opened
	indexEpoch int64

	// isOpen tracks if this reader is open
	isOpen bool
}

// DirectoryTaxonomyReaderOptions contains options for opening a DirectoryTaxonomyReader.
type DirectoryTaxonomyReaderOptions struct {
	// ReadOnly indicates if this reader should be read-only
	ReadOnly bool
}

// NewDirectoryTaxonomyReader creates a new DirectoryTaxonomyReader from the given directory.
func NewDirectoryTaxonomyReader(dir store.Directory) (*DirectoryTaxonomyReader, error) {
	return NewDirectoryTaxonomyReaderWithOptions(dir, nil)
}

// NewDirectoryTaxonomyReaderWithOptions creates a new DirectoryTaxonomyReader with options.
func NewDirectoryTaxonomyReaderWithOptions(dir store.Directory, opts *DirectoryTaxonomyReaderOptions) (*DirectoryTaxonomyReader, error) {
	if dir == nil {
		return nil, fmt.Errorf("directory cannot be nil")
	}

	// Create base taxonomy reader
	baseReader := NewTaxonomyReader()

	reader := &DirectoryTaxonomyReader{
		TaxonomyReader: baseReader,
		directory:      dir,
		indexEpoch:     0,
		isOpen:         true,
	}

	// Load taxonomy data from directory
	if err := reader.loadFromDirectory(); err != nil {
		return nil, fmt.Errorf("loading taxonomy from directory: %w", err)
	}

	return reader, nil
}

// OpenDirectoryTaxonomyReader opens a DirectoryTaxonomyReader from the given directory.
// This is a convenience function that matches Lucene's API.
func OpenDirectoryTaxonomyReader(dir store.Directory) (*DirectoryTaxonomyReader, error) {
	return NewDirectoryTaxonomyReader(dir)
}

// loadFromDirectory loads the taxonomy data from the directory.
func (dtr *DirectoryTaxonomyReader) loadFromDirectory() error {
	// In a full implementation, this would:
	// 1. Open the taxonomy index from the directory
	// 2. Read all category paths and their ordinals
	// 3. Build the parent-child relationships
	// 4. Set the next ordinal

	// For now, we just mark it as loaded
	return nil
}

// GetDirectory returns the directory used by this reader.
func (dtr *DirectoryTaxonomyReader) GetDirectory() store.Directory {
	return dtr.directory
}

// GetIndexEpoch returns the index epoch when this reader was opened.
func (dtr *DirectoryTaxonomyReader) GetIndexEpoch() int64 {
	return dtr.indexEpoch
}

// IsOpen returns true if this reader is open.
func (dtr *DirectoryTaxonomyReader) IsOpen() bool {
	return dtr.isOpen
}

// Close closes this reader.
func (dtr *DirectoryTaxonomyReader) Close() error {
	if !dtr.isOpen {
		return nil
	}

	// Close the base taxonomy reader
	if err := dtr.TaxonomyReader.Close(); err != nil {
		return err
	}

	dtr.isOpen = false
	return nil
}

// Refresh refreshes this reader with the latest data from the directory.
// Returns true if the reader was refreshed, false if it was already up to date.
func (dtr *DirectoryTaxonomyReader) Refresh() (bool, error) {
	if !dtr.isOpen {
		return false, fmt.Errorf("reader is closed")
	}

	// In a full implementation, this would:
	// 1. Check if the index has been updated
	// 2. If so, reload the taxonomy data
	// 3. Return true if refreshed, false otherwise

	return false, nil
}

// GetOrdinalFromPath gets the ordinal for a path from the directory.
// This is a convenience method that combines path construction with ordinal lookup.
func (dtr *DirectoryTaxonomyReader) GetOrdinalFromPath(components ...string) int {
	path := NewFacetLabel(components...).String()
	return dtr.GetOrdinal(path)
}

// GetPathComponents returns the path components for the given ordinal.
func (dtr *DirectoryTaxonomyReader) GetPathComponents(ordinal int) []string {
	path := dtr.GetPath(ordinal)
	if path == "" {
		return nil
	}

	// Parse the path string back into components
	// Path format: "/component1/component2"
	// We can't easily parse back, so we just return the path as a single component
	// In a full implementation, this would properly parse the path
	return []string{path}
}

// GetFacetLabel returns the FacetLabel for the given ordinal.
func (dtr *DirectoryTaxonomyReader) GetFacetLabel(ordinal int) *FacetLabel {
	path := dtr.GetPath(ordinal)
	if path == "" {
		return nil
	}

	// Parse the path string into components
	// In a full implementation, this would properly parse the path
	return NewFacetLabel(path)
}

// GetSize returns the size of the taxonomy (number of categories).
func (dtr *DirectoryTaxonomyReader) GetSize() int {
	return dtr.TaxonomyReader.GetSize()
}

// GetParent returns the parent ordinal for the given ordinal.
func (dtr *DirectoryTaxonomyReader) GetParent(ordinal int) int {
	return dtr.TaxonomyReader.GetParent(ordinal)
}

// GetChildren returns the child ordinals for the given parent ordinal.
func (dtr *DirectoryTaxonomyReader) GetChildren(ordinal int) []int {
	return dtr.TaxonomyReader.GetChildren(ordinal)
}

// GetSiblings returns the sibling ordinals for the given ordinal.
func (dtr *DirectoryTaxonomyReader) GetSiblings(ordinal int) []int {
	return dtr.TaxonomyReader.GetSiblings(ordinal)
}

// GetDescendants returns all descendant ordinals for the given ordinal.
func (dtr *DirectoryTaxonomyReader) GetDescendants(ordinal int) []int {
	return dtr.TaxonomyReader.GetDescendants(ordinal)
}

// Ensure Openable interface is implemented
var _ io.Closer = (*DirectoryTaxonomyReader)(nil)

// DirectoryTaxonomyReaderFactory creates DirectoryTaxonomyReader instances.
type DirectoryTaxonomyReaderFactory struct {
	// directory is the store directory
	directory store.Directory

	// options are the reader options
	options *DirectoryTaxonomyReaderOptions
}

// NewDirectoryTaxonomyReaderFactory creates a new factory.
func NewDirectoryTaxonomyReaderFactory(dir store.Directory) *DirectoryTaxonomyReaderFactory {
	return &DirectoryTaxonomyReaderFactory{
		directory: dir,
		options:   nil,
	}
}

// NewDirectoryTaxonomyReaderFactoryWithOptions creates a new factory with options.
func NewDirectoryTaxonomyReaderFactoryWithOptions(dir store.Directory, opts *DirectoryTaxonomyReaderOptions) *DirectoryTaxonomyReaderFactory {
	return &DirectoryTaxonomyReaderFactory{
		directory: dir,
		options:   opts,
	}
}

// Open opens a DirectoryTaxonomyReader.
func (f *DirectoryTaxonomyReaderFactory) Open() (*DirectoryTaxonomyReader, error) {
	return NewDirectoryTaxonomyReaderWithOptions(f.directory, f.options)
}

// OpenIfChanged opens a new reader if the index has changed.
// Returns the new reader and true if changed, or the current reader and false if unchanged.
func (f *DirectoryTaxonomyReaderFactory) OpenIfChanged(oldReader *DirectoryTaxonomyReader) (*DirectoryTaxonomyReader, bool, error) {
	if oldReader == nil {
		reader, err := f.Open()
		return reader, true, err
	}

	// Check if the index has changed
	// In a full implementation, this would compare epochs
	return oldReader, false, nil
}

// DirectoryTaxonomyReaderManager manages DirectoryTaxonomyReader instances.
type DirectoryTaxonomyReaderManager struct {
	// factory is the reader factory
	factory *DirectoryTaxonomyReaderFactory

	// current is the current reader
	current *DirectoryTaxonomyReader

	// isOpen tracks if this manager is open
	isOpen bool
}

// NewDirectoryTaxonomyReaderManager creates a new manager.
func NewDirectoryTaxonomyReaderManager(factory *DirectoryTaxonomyReaderFactory) (*DirectoryTaxonomyReaderManager, error) {
	if factory == nil {
		return nil, fmt.Errorf("factory cannot be nil")
	}

	// Open initial reader
	reader, err := factory.Open()
	if err != nil {
		return nil, fmt.Errorf("opening initial reader: %w", err)
	}

	return &DirectoryTaxonomyReaderManager{
		factory: factory,
		current: reader,
		isOpen:  true,
	}, nil
}

// Acquire returns the current reader.
func (m *DirectoryTaxonomyReaderManager) Acquire() *DirectoryTaxonomyReader {
	return m.current
}

// MaybeRefresh refreshes the reader if the index has changed.
func (m *DirectoryTaxonomyReaderManager) MaybeRefresh() error {
	if !m.isOpen {
		return fmt.Errorf("manager is closed")
	}

	newReader, changed, err := m.factory.OpenIfChanged(m.current)
	if err != nil {
		return err
	}

	if changed {
		// Close old reader and use new one
		if m.current != nil {
			m.current.Close()
		}
		m.current = newReader
	}

	return nil
}

// Close closes this manager.
func (m *DirectoryTaxonomyReaderManager) Close() error {
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
