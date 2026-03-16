package facets

import (
	"fmt"
	"strings"
	"sync"
)

// TaxonomyWriter provides write access to the taxonomy index.
// It allows adding new category paths and retrieving their ordinals.
//
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.TaxonomyWriter.
type TaxonomyWriter struct {
	// reader is the associated TaxonomyReader
	reader *TaxonomyReader

	// ordinals maps category paths to their ordinals
	ordinals map[string]int

	// paths maps ordinals back to category paths
	paths map[int]string

	// children maps parent ordinals to their children
	children map[int][]int

	// parent maps child ordinals to their parent
	parent map[int]int

	// nextOrdinal is the next available ordinal
	nextOrdinal int

	// mu protects the maps
	mu sync.RWMutex

	// isOpen indicates if this writer is open
	isOpen bool
}

// NewTaxonomyWriter creates a new TaxonomyWriter.
func NewTaxonomyWriter() *TaxonomyWriter {
	return &TaxonomyWriter{
		ordinals:    make(map[string]int),
		paths:       make(map[int]string),
		children:    make(map[int][]int),
		parent:      make(map[int]int),
		nextOrdinal: 1, // Start at 1, reserve 0 for invalid
		isOpen:      true,
	}
}

// NewTaxonomyWriterWithReader creates a new TaxonomyWriter with an existing reader.
func NewTaxonomyWriterWithReader(reader *TaxonomyReader) *TaxonomyWriter {
	tw := NewTaxonomyWriter()
	tw.reader = reader

	// Copy data from reader
	if reader != nil {
		reader.mu.RLock()
		defer reader.mu.RUnlock()

		for path, ord := range reader.ordinals {
			tw.ordinals[path] = ord
		}
		for ord, path := range reader.paths {
			tw.paths[ord] = path
		}
		for parent, children := range reader.children {
			tw.children[parent] = make([]int, len(children))
			copy(tw.children[parent], children)
		}
		for child, parent := range reader.parent {
			tw.parent[child] = parent
		}
		tw.nextOrdinal = reader.nextOrdinal
	}

	return tw
}

// AddCategory adds a category path to the taxonomy and returns its ordinal.
// If the path already exists, returns the existing ordinal.
func (tw *TaxonomyWriter) AddCategory(path string) (int, error) {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if !tw.isOpen {
		return -1, fmt.Errorf("taxonomy writer is closed")
	}

	// Check if path already exists
	if ord, ok := tw.ordinals[path]; ok {
		return ord, nil
	}

	// Get the parent path
	parentPath := tw.getParentPath(path)
	parentOrdinal := -1

	// Add parent if it exists and is not empty
	if parentPath != "" {
		var err error
		parentOrdinal, err = tw.addCategoryInternal(parentPath)
		if err != nil {
			return -1, err
		}
	}

	// Add the new category
	ordinal := tw.nextOrdinal
	tw.nextOrdinal++

	tw.ordinals[path] = ordinal
	tw.paths[ordinal] = path

	// Set up parent-child relationship
	if parentOrdinal > 0 {
		tw.parent[ordinal] = parentOrdinal
		tw.children[parentOrdinal] = append(tw.children[parentOrdinal], ordinal)
	}

	return ordinal, nil
}

// addCategoryInternal is the internal version that doesn't lock.
func (tw *TaxonomyWriter) addCategoryInternal(path string) (int, error) {
	// Check if path already exists
	if ord, ok := tw.ordinals[path]; ok {
		return ord, nil
	}

	// Get the parent path
	parentPath := tw.getParentPath(path)
	parentOrdinal := -1

	// Add parent if it exists and is not empty
	if parentPath != "" {
		var err error
		parentOrdinal, err = tw.addCategoryInternal(parentPath)
		if err != nil {
			return -1, err
		}
	}

	// Add the new category
	ordinal := tw.nextOrdinal
	tw.nextOrdinal++

	tw.ordinals[path] = ordinal
	tw.paths[ordinal] = path

	// Set up parent-child relationship
	if parentOrdinal > 0 {
		tw.parent[ordinal] = parentOrdinal
		tw.children[parentOrdinal] = append(tw.children[parentOrdinal], ordinal)
	}

	return ordinal, nil
}

// getParentPath returns the parent path for the given path.
func (tw *TaxonomyWriter) getParentPath(path string) string {
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return ""
	}
	return path[:lastSlash]
}

// AddCategoryPath adds a hierarchical category path and returns the leaf ordinal.
func (tw *TaxonomyWriter) AddCategoryPath(components []string) (int, error) {
	if len(components) == 0 {
		return -1, fmt.Errorf("cannot add empty path")
	}

	path := strings.Join(components, "/")
	return tw.AddCategory(path)
}

// GetOrdinal returns the ordinal for the given category path.
// Returns -1 if the path is not found.
func (tw *TaxonomyWriter) GetOrdinal(path string) int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()

	if ord, ok := tw.ordinals[path]; ok {
		return ord
	}
	return -1
}

// GetPath returns the category path for the given ordinal.
// Returns empty string if the ordinal is not found.
func (tw *TaxonomyWriter) GetPath(ordinal int) string {
	tw.mu.RLock()
	defer tw.mu.RUnlock()

	if path, ok := tw.paths[ordinal]; ok {
		return path
	}
	return ""
}

// GetSize returns the number of categories in the taxonomy.
func (tw *TaxonomyWriter) GetSize() int {
	tw.mu.RLock()
	defer tw.mu.RUnlock()

	return len(tw.ordinals)
}

// Commit commits the changes to the taxonomy.
func (tw *TaxonomyWriter) Commit() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if !tw.isOpen {
		return fmt.Errorf("taxonomy writer is closed")
	}

	// In a full implementation, this would write to the index
	// For now, just sync with the reader if one exists
	if tw.reader != nil {
		tw.syncToReader()
	}

	return nil
}

// syncToReader syncs the writer's data to the reader.
func (tw *TaxonomyWriter) syncToReader() {
	if tw.reader == nil {
		return
	}

	tw.reader.mu.Lock()
	defer tw.reader.mu.Unlock()

	// Copy data to reader
	for path, ord := range tw.ordinals {
		tw.reader.ordinals[path] = ord
	}
	for ord, path := range tw.paths {
		tw.reader.paths[ord] = path
	}
	for parent, children := range tw.children {
		tw.reader.children[parent] = make([]int, len(children))
		copy(tw.reader.children[parent], children)
	}
	for child, parent := range tw.parent {
		tw.reader.parent[child] = parent
	}
	tw.reader.nextOrdinal = tw.nextOrdinal
}

// Close closes this taxonomy writer.
func (tw *TaxonomyWriter) Close() error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if !tw.isOpen {
		return nil
	}

	// Commit any pending changes
	tw.syncToReader()

	tw.isOpen = false
	return nil
}

// IsOpen returns true if this writer is open.
func (tw *TaxonomyWriter) IsOpen() bool {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return tw.isOpen
}

// GetReader returns a TaxonomyReader for this taxonomy.
func (tw *TaxonomyWriter) GetReader() (*TaxonomyReader, error) {
	tw.mu.RLock()
	defer tw.mu.RUnlock()

	if !tw.isOpen {
		return nil, fmt.Errorf("taxonomy writer is closed")
	}

	// Create a new reader with the current state
	reader := NewTaxonomyReader()

	// Copy data to reader
	for path, ord := range tw.ordinals {
		reader.ordinals[path] = ord
	}
	for ord, path := range tw.paths {
		reader.paths[ord] = path
	}
	for parent, children := range tw.children {
		reader.children[parent] = make([]int, len(children))
		copy(reader.children[parent], children)
	}
	for child, parent := range tw.parent {
		reader.parent[child] = parent
	}
	reader.nextOrdinal = tw.nextOrdinal

	return reader, nil
}

// TaxonomyWriterFactory creates TaxonomyWriter instances.
type TaxonomyWriterFactory struct {
	// directory is where to store the taxonomy
	directory string
}

// NewTaxonomyWriterFactory creates a new TaxonomyWriterFactory.
func NewTaxonomyWriterFactory(directory string) *TaxonomyWriterFactory {
	return &TaxonomyWriterFactory{
		directory: directory,
	}
}

// Open opens a TaxonomyWriter.
func (twf *TaxonomyWriterFactory) Open() (*TaxonomyWriter, error) {
	return NewTaxonomyWriter(), nil
}

// OpenWithReader opens a TaxonomyWriter with an existing reader.
func (twf *TaxonomyWriterFactory) OpenWithReader(reader *TaxonomyReader) (*TaxonomyWriter, error) {
	return NewTaxonomyWriterWithReader(reader), nil
}
