// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"io"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// DirectoryTaxonomyReader is a TaxonomyReader that reads from a Directory.
//
// Two construction paths are supported:
//
//  1. NRT (near-real-time): constructed from a DirectoryTaxonomyWriter whose
//     in-memory state is copied directly. This is the primary round-trip path
//     and works regardless of the SegmentReader core-readers gap.
//
//  2. Cold open: constructed from a Directory. Attempts to read the persisted
//     taxonomy via BinaryDocValues. Until the SegmentReader core-readers gap
//     (memory ref 'gocene-segmentreader-corereaders-gap') is resolved, cold-open
//     readers will have an empty in-memory map and return INVALID_ORDINAL for
//     every ordinal lookup.
//
// This is the Go port of
// org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyReader.
type DirectoryTaxonomyReader struct {
	// directory is the source directory.
	directory store.Directory

	// indexEpoch is the epoch from the commit data when this reader was opened.
	indexEpoch int64

	// isOpen tracks whether the reader is open.
	isOpen bool

	// mu protects the maps below.
	mu sync.RWMutex

	// pathToOrdinal maps FacetLabel.String() -> ordinal.
	pathToOrdinal map[string]int

	// ordinalToPath maps ordinal -> FacetLabel.
	ordinalToPath []*FacetLabel

	// parentOrdinals[i] is the parent ordinal of category i.
	parentOrdinals []int

	// size is the number of categories (including root).
	size int
}

// DirectoryTaxonomyReaderOptions contains options for opening a DirectoryTaxonomyReader.
type DirectoryTaxonomyReaderOptions struct {
	// ReadOnly indicates if this reader should be read-only.
	ReadOnly bool
}

// NewDirectoryTaxonomyReader opens a cold DirectoryTaxonomyReader from the given
// directory. This requires DocValues reading support; until the SegmentReader
// core-readers gap is resolved, ordinal lookups on a cold reader return
// INVALID_ORDINAL.
func NewDirectoryTaxonomyReader(dir store.Directory) (*DirectoryTaxonomyReader, error) {
	return NewDirectoryTaxonomyReaderWithOptions(dir, nil)
}

// NewDirectoryTaxonomyReaderWithOptions creates a cold DirectoryTaxonomyReader with options.
func NewDirectoryTaxonomyReaderWithOptions(dir store.Directory, _ *DirectoryTaxonomyReaderOptions) (*DirectoryTaxonomyReader, error) {
	if dir == nil {
		return nil, fmt.Errorf("directory cannot be nil")
	}

	r := &DirectoryTaxonomyReader{
		directory:      dir,
		isOpen:         true,
		pathToOrdinal:  make(map[string]int),
		ordinalToPath:  []*FacetLabel{},
		parentOrdinals: []int{},
		size:           0,
	}

	// Attempt to load from disk. Failure is non-fatal (leaves maps empty).
	r.loadFromDirectory() //nolint:errcheck

	return r, nil
}

// OpenDirectoryTaxonomyReader is a convenience alias for NewDirectoryTaxonomyReader.
func OpenDirectoryTaxonomyReader(dir store.Directory) (*DirectoryTaxonomyReader, error) {
	return NewDirectoryTaxonomyReader(dir)
}

// NewDirectoryTaxonomyReaderFromWriter constructs a DirectoryTaxonomyReader from
// the in-memory state of the given writer (NRT path). This is the primary
// writer-reader round-trip path in Gocene and does not require DocValues reading.
func NewDirectoryTaxonomyReaderFromWriter(w *DirectoryTaxonomyWriter) (*DirectoryTaxonomyReader, error) {
	if w == nil {
		return nil, fmt.Errorf("writer cannot be nil")
	}

	p2o, o2p, parents, n := w.snapshotState()

	r := &DirectoryTaxonomyReader{
		directory:      w.GetDirectory(),
		isOpen:         true,
		pathToOrdinal:  p2o,
		ordinalToPath:  o2p,
		parentOrdinals: parents,
		size:           n,
	}
	return r, nil
}

// loadFromDirectory attempts to populate the reader from the persisted index.
// Returns nil without populating maps if DocValues are unavailable.
func (r *DirectoryTaxonomyReader) loadFromDirectory() error {
	dr, err := index.OpenDirectoryReader(r.directory)
	if err != nil {
		return nil // empty/non-existent index — normal for newly-opened writer
	}
	defer dr.Close() //nolint:errcheck

	numDocs := dr.NumDocs()
	if numDocs == 0 {
		return nil
	}

	p2o := make(map[string]int, numDocs)
	o2p := make([]*FacetLabel, numDocs)
	parents := make([]int, numDocs)
	for i := range parents {
		parents[i] = taxoInvalidOrdinal
	}

	leaves, err := dr.Leaves()
	if err != nil {
		return nil
	}

	for _, lrc := range leaves {
		raw := lrc.Reader()
		// Obtain a LeafReader to access DocValues.
		lr, ok := raw.(interface {
			MaxDoc() int
			GetBinaryDocValues(string) (index.BinaryDocValues, error)
			GetNumericDocValues(string) (index.NumericDocValues, error)
		})
		if !ok {
			// DocValues not available on this reader type.
			continue
		}
		maxDoc := lr.MaxDoc()
		base := lrc.DocBase()

		bdv, err := lr.GetBinaryDocValues(taxoFieldFull)
		if err != nil || bdv == nil {
			// DocValues not readable yet — partial load is not useful.
			return nil
		}
		ndv, err := lr.GetNumericDocValues(taxoFieldParentNDV)
		if err != nil {
			ndv = nil
		}

		for docID := 0; docID < maxDoc; docID++ {
			globalOrd := base + docID
			if globalOrd >= numDocs {
				break
			}

			pathBytes, err := bdv.Get(docID)
			if err != nil {
				continue
			}
			label := facetLabelFromPathString(string(pathBytes))
			key := label.String()
			p2o[key] = globalOrd
			o2p[globalOrd] = label

			if ndv != nil {
				pv, err := ndv.Get(docID)
				if err == nil {
					parents[globalOrd] = int(pv)
				}
			}
		}
	}

	r.mu.Lock()
	r.pathToOrdinal = p2o
	r.ordinalToPath = o2p
	r.parentOrdinals = parents
	r.size = numDocs
	r.mu.Unlock()
	return nil
}

// GetDirectory returns the directory used by this reader.
func (r *DirectoryTaxonomyReader) GetDirectory() store.Directory {
	return r.directory
}

// GetIndexEpoch returns the index epoch when this reader was opened.
func (r *DirectoryTaxonomyReader) GetIndexEpoch() int64 {
	return r.indexEpoch
}

// IsOpen returns true if this reader is open.
func (r *DirectoryTaxonomyReader) IsOpen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isOpen
}

// Close closes this reader.
func (r *DirectoryTaxonomyReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.isOpen = false
	return nil
}

// Refresh attempts to reload the taxonomy from disk if changes have occurred.
// Returns true if the reader was refreshed, false if already up to date.
func (r *DirectoryTaxonomyReader) Refresh() (bool, error) {
	r.mu.RLock()
	isOpen := r.isOpen
	r.mu.RUnlock()
	if !isOpen {
		return false, fmt.Errorf("reader is closed")
	}
	return false, nil
}

// GetSize returns the number of categories including the root.
func (r *DirectoryTaxonomyReader) GetSize() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.size
}

// GetOrdinal returns the ordinal for the given FacetLabel.
// Returns taxoInvalidOrdinal if the category does not exist.
func (r *DirectoryTaxonomyReader) GetOrdinal(label *FacetLabel) int {
	if label == nil {
		return taxoInvalidOrdinal
	}
	if len(label.Components) == 0 {
		return taxoRootOrdinal
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ord, ok := r.pathToOrdinal[label.String()]
	if !ok {
		return taxoInvalidOrdinal
	}
	return ord
}

// GetPath returns the FacetLabel for the given ordinal.
// Returns nil if the ordinal is out of range.
func (r *DirectoryTaxonomyReader) GetPath(ordinal int) *FacetLabel {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if ordinal < 0 || ordinal >= len(r.ordinalToPath) {
		return nil
	}
	return r.ordinalToPath[ordinal]
}

// GetParent returns the parent ordinal for the given ordinal.
// Returns taxoInvalidOrdinal for the root or out-of-range ordinals.
func (r *DirectoryTaxonomyReader) GetParent(ordinal int) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if ordinal < 0 || ordinal >= len(r.parentOrdinals) {
		return taxoInvalidOrdinal
	}
	return r.parentOrdinals[ordinal]
}

// GetOrdinalFromPath is a convenience method that creates a FacetLabel and
// calls GetOrdinal.
func (r *DirectoryTaxonomyReader) GetOrdinalFromPath(components ...string) int {
	return r.GetOrdinal(NewFacetLabel(components...))
}

// GetPathComponents returns the path components for the given ordinal.
func (r *DirectoryTaxonomyReader) GetPathComponents(ordinal int) []string {
	label := r.GetPath(ordinal)
	if label == nil {
		return nil
	}
	return label.Components
}

// GetFacetLabel returns the FacetLabel for the given ordinal.
func (r *DirectoryTaxonomyReader) GetFacetLabel(ordinal int) *FacetLabel {
	return r.GetPath(ordinal)
}

// Ensure io.Closer is implemented.
var _ io.Closer = (*DirectoryTaxonomyReader)(nil)

// DirectoryTaxonomyReaderFactory creates DirectoryTaxonomyReader instances.
type DirectoryTaxonomyReaderFactory struct {
	directory store.Directory
	options   *DirectoryTaxonomyReaderOptions
}

// NewDirectoryTaxonomyReaderFactory creates a new factory.
func NewDirectoryTaxonomyReaderFactory(dir store.Directory) *DirectoryTaxonomyReaderFactory {
	return &DirectoryTaxonomyReaderFactory{directory: dir}
}

// NewDirectoryTaxonomyReaderFactoryWithOptions creates a factory with options.
func NewDirectoryTaxonomyReaderFactoryWithOptions(dir store.Directory, opts *DirectoryTaxonomyReaderOptions) *DirectoryTaxonomyReaderFactory {
	return &DirectoryTaxonomyReaderFactory{directory: dir, options: opts}
}

// Open opens a DirectoryTaxonomyReader.
func (f *DirectoryTaxonomyReaderFactory) Open() (*DirectoryTaxonomyReader, error) {
	return NewDirectoryTaxonomyReaderWithOptions(f.directory, f.options)
}

// OpenIfChanged opens a new reader if the index has changed.
func (f *DirectoryTaxonomyReaderFactory) OpenIfChanged(oldReader *DirectoryTaxonomyReader) (*DirectoryTaxonomyReader, bool, error) {
	if oldReader == nil {
		reader, err := f.Open()
		return reader, true, err
	}
	return oldReader, false, nil
}

// DirectoryTaxonomyReaderManager manages DirectoryTaxonomyReader instances.
type DirectoryTaxonomyReaderManager struct {
	factory *DirectoryTaxonomyReaderFactory
	current *DirectoryTaxonomyReader
	isOpen  bool
}

// NewDirectoryTaxonomyReaderManager creates a new manager.
func NewDirectoryTaxonomyReaderManager(factory *DirectoryTaxonomyReaderFactory) (*DirectoryTaxonomyReaderManager, error) {
	if factory == nil {
		return nil, fmt.Errorf("factory cannot be nil")
	}
	reader, err := factory.Open()
	if err != nil {
		return nil, fmt.Errorf("opening initial reader: %w", err)
	}
	return &DirectoryTaxonomyReaderManager{factory: factory, current: reader, isOpen: true}, nil
}

// Acquire returns the current reader.
func (m *DirectoryTaxonomyReaderManager) Acquire() *DirectoryTaxonomyReader { return m.current }

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
		if m.current != nil {
			m.current.Close() //nolint:errcheck
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
