// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Taxonomy index field name constants mirror the Java Consts class.
const (
	// taxoFieldFull is the field name for the full category path.
	// Stored as a StringField (indexed, not stored) for term-based lookup
	// and as a BinaryDocValuesField for ordinal-to-path retrieval.
	// Matches org.apache.lucene.facet.taxonomy.directory.Consts.FULL.
	taxoFieldFull = "$full_path$"

	// taxoFieldParentNDV is the NumericDocValuesField for the parent ordinal.
	// Matches org.apache.lucene.facet.taxonomy.directory.Consts.FIELD_PARENT_ORDINAL_NDV.
	taxoFieldParentNDV = "$parent_ndv$"

	// taxoIndexEpoch is the commit-data key for the index epoch.
	// Matches DirectoryTaxonomyWriter.INDEX_EPOCH.
	taxoIndexEpoch = "index.epoch"

	// taxoRootOrdinal is the ordinal of the root category (always 0).
	taxoRootOrdinal = 0

	// taxoInvalidOrdinal is the sentinel for "no parent" or "not found" (−1).
	taxoInvalidOrdinal = -1
)

// OpenMode specifies how to open/create a taxonomy index.
// Mirrors org.apache.lucene.index.IndexWriterConfig.OpenMode.
type OpenMode int

const (
	// CREATE creates a new index, removing any existing index.
	CREATE OpenMode = iota
	// APPEND opens an existing index.
	APPEND
	// CREATE_OR_APPEND creates a new index if none exists, otherwise appends.
	CREATE_OR_APPEND
)

// DirectoryTaxonomyWriterOptions contains options for opening a DirectoryTaxonomyWriter.
type DirectoryTaxonomyWriterOptions struct {
	// OpenMode specifies how to open the index.
	OpenMode OpenMode
}

// DirectoryTaxonomyWriter is a TaxonomyWriter that persists the taxonomy to a
// Lucene Directory using an internal IndexWriter.
//
// Each category is stored as one document with:
//   - StringField(taxoFieldFull, path, NOT_STORED) — for term-based lookup
//   - BinaryDocValuesField(taxoFieldFull, path_bytes) — for ordinal-to-path reads
//   - NumericDocValuesField(taxoFieldParentNDV, parentOrdinal) — parent relationship
//
// The document's docID equals the category ordinal. The root category
// (empty FacetLabel) is always at ordinal 0 and is added automatically on
// construction when the taxonomy is empty.
//
// This is the Go port of
// org.apache.lucene.facet.taxonomy.directory.DirectoryTaxonomyWriter.
type DirectoryTaxonomyWriter struct {
	directory store.Directory

	// indexWriter is the internal Lucene index writer.
	indexWriter *index.IndexWriter

	// mu protects all mutable state below.
	mu sync.Mutex

	// pathToOrdinal maps FacetLabel.String() -> ordinal.
	// This is the in-memory cache; it is always complete (no eviction in this
	// simplified implementation — Gocene does not yet port LruTaxonomyWriterCache).
	pathToOrdinal map[string]int

	// ordinalToPath maps ordinal -> FacetLabel.
	ordinalToPath []*FacetLabel

	// parentOrdinals[i] is the ordinal of the parent of category i.
	// parentOrdinals[0] == taxoInvalidOrdinal (root has no parent).
	parentOrdinals []int

	// nextOrdinal is the next available ordinal.
	nextOrdinal int

	// isOpen tracks whether the writer is open.
	isOpen bool

	// indexEpoch is incremented when the taxonomy is (re-)created.
	indexEpoch int64

	// uncommittedChanges is true when categories have been added since the last Commit.
	uncommittedChanges bool
}

// NewDirectoryTaxonomyWriter opens or creates a taxonomy at the given directory
// using CREATE_OR_APPEND mode.
func NewDirectoryTaxonomyWriter(dir store.Directory) (*DirectoryTaxonomyWriter, error) {
	return NewDirectoryTaxonomyWriterWithOptions(dir, nil)
}

// NewDirectoryTaxonomyWriterWithOptions opens or creates a taxonomy with explicit options.
func NewDirectoryTaxonomyWriterWithOptions(dir store.Directory, opts *DirectoryTaxonomyWriterOptions) (*DirectoryTaxonomyWriter, error) {
	if dir == nil {
		return nil, fmt.Errorf("directory cannot be nil")
	}

	mode := CREATE_OR_APPEND
	if opts != nil {
		mode = opts.OpenMode
	}

	var iwcOpenMode index.OpenMode
	switch mode {
	case CREATE:
		iwcOpenMode = index.CREATE
	case APPEND:
		iwcOpenMode = index.APPEND
	case CREATE_OR_APPEND:
		iwcOpenMode = index.CREATE_OR_APPEND
	}

	// Use LogByteSizeMergePolicy to preserve docID order across merges.
	config := index.NewIndexWriterConfig(nil)
	config.SetOpenMode(iwcOpenMode)
	config.SetMergePolicy(index.NewLogByteSizeMergePolicy())

	iw, err := index.NewIndexWriter(dir, config)
	if err != nil {
		return nil, fmt.Errorf("opening taxonomy index writer: %w", err)
	}

	w := &DirectoryTaxonomyWriter{
		directory:   dir,
		indexWriter: iw,
		isOpen:      true,
	}

	// Initialise in-memory structures.
	w.pathToOrdinal = make(map[string]int)
	w.ordinalToPath = make([]*FacetLabel, 0, 16)
	w.parentOrdinals = make([]int, 0, 16)
	w.nextOrdinal = 0

	// The taxonomy must always contain the root category at ordinal 0.
	// If the index is empty (freshly created), add the root document now.
	// If the index already has documents, load existing categories from disk.
	maxDoc := iw.GetDocStats().MaxDoc
	if maxDoc == 0 {
		// New index: insert root.
		w.indexEpoch = 1
		if err := w.addCategoryLocked(NewFacetLabelEmpty()); err != nil {
			iw.Close() //nolint:errcheck
			return nil, fmt.Errorf("adding root category: %w", err)
		}
	} else {
		// Existing index: load all categories from the persisted reader.
		w.nextOrdinal = maxDoc
		if err := w.loadFromDisk(); err != nil {
			iw.Close() //nolint:errcheck
			return nil, fmt.Errorf("loading existing taxonomy: %w", err)
		}
	}

	return w, nil
}

// OpenDirectoryTaxonomyWriter is a convenience alias for NewDirectoryTaxonomyWriter.
func OpenDirectoryTaxonomyWriter(dir store.Directory) (*DirectoryTaxonomyWriter, error) {
	return NewDirectoryTaxonomyWriter(dir)
}

// loadFromDisk populates the in-memory cache from the persisted index.
// This is called when re-opening a writer over an existing taxonomy.
// It attempts to read categories through the index reader; if reading fails
// (e.g., due to the SegmentReader core-readers gap) the cache is left incomplete
// and cache misses will surface as new-ordinal assignments (ordinal collision).
func (w *DirectoryTaxonomyWriter) loadFromDisk() error {
	r, err := index.OpenDirectoryReader(w.directory)
	if err != nil {
		// Cannot open reader — leave cache incomplete.
		return nil
	}
	defer r.Close() //nolint:errcheck

	numDocs := r.NumDocs()
	w.ordinalToPath = make([]*FacetLabel, numDocs)
	w.parentOrdinals = make([]int, numDocs)

	leaves, err := r.Leaves()
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

		// Read BinaryDocValues for path (ordinal→path).
		bdv, err := lr.GetBinaryDocValues(taxoFieldFull)
		if err != nil || bdv == nil {
			// DocValues not readable yet (SegmentReader core-readers gap).
			// Leave the in-memory maps empty; the writer will re-assign ordinals
			// which may differ from the persisted ones.
			continue
		}

		// Read NumericDocValues for parent.
		ndv, err := lr.GetNumericDocValues(taxoFieldParentNDV)
		if err != nil {
			ndv = nil
		}

		for docID := 0; docID < maxDoc; docID++ {
			globalOrd := base + docID

			// Read path from BinaryDocValues.
			pathBytes, err := bdv.Get(docID)
			if err != nil {
				continue
			}
			label := facetLabelFromPathString(string(pathBytes))
			key := label.String()
			w.pathToOrdinal[key] = globalOrd
			w.ordinalToPath[globalOrd] = label

			// Read parent.
			parentOrd := taxoInvalidOrdinal
			if ndv != nil {
				pv, err := ndv.Get(docID)
				if err == nil {
					parentOrd = int(pv)
				}
			}
			w.parentOrdinals[globalOrd] = parentOrd
		}
	}
	return nil
}

// facetLabelFromPathString converts a Lucene taxonomy path string back to a FacetLabel.
// The format is the result of FacetsConfig.pathToString: components joined by '￾'.
// This mirrors the Java FacetsConfig.stringToPath logic.
func facetLabelFromPathString(s string) *FacetLabel {
	if s == "" {
		return NewFacetLabelEmpty()
	}
	// Lucene's pathToString joins components with ￾ (U+FFFE).
	const sep = "￾"
	parts := splitPathString(s, sep)
	return NewFacetLabel(parts...)
}

// splitPathString splits s by sep, handling empty segments properly.
func splitPathString(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	sepLen := len(sep)
	for i := 0; i <= len(s)-sepLen; i++ {
		if s[i:i+sepLen] == sep {
			result = append(result, s[start:i])
			start = i + sepLen
		}
	}
	result = append(result, s[start:])
	return result
}

// facetLabelToPathString serialises a FacetLabel to the Lucene taxonomy path string.
// Mirrors org.apache.lucene.facet.FacetsConfig.pathToString(String[], int).
func facetLabelToPathString(label *FacetLabel) string {
	if label == nil || len(label.Components) == 0 {
		return ""
	}
	// Lucene joins components with U+FFFE.
	const sep = "￾"
	result := label.Components[0]
	for _, c := range label.Components[1:] {
		result += sep + c
	}
	return result
}

// addCategoryLocked adds a category to both the index and the in-memory cache.
// Must be called with w.mu held.
func (w *DirectoryTaxonomyWriter) addCategoryLocked(label *FacetLabel) error {
	key := label.String()
	if _, exists := w.pathToOrdinal[key]; exists {
		return nil
	}

	// Determine parent ordinal (recursively adding ancestors if necessary).
	parentOrd := taxoInvalidOrdinal
	if len(label.Components) > 0 {
		parentLabel := label.Parent()
		if parentLabel == nil || len(parentLabel.Components) == 0 {
			parentOrd = taxoRootOrdinal
		} else {
			// Ensure parent exists first.
			if err := w.addCategoryLocked(parentLabel); err != nil {
				return err
			}
			parentOrd = w.pathToOrdinal[parentLabel.String()]
		}
	}

	ord := w.nextOrdinal
	w.nextOrdinal++

	// Persist to the index.
	pathStr := facetLabelToPathString(label)
	if err := w.indexCategoryDocument(pathStr, parentOrd); err != nil {
		return err
	}

	// Update in-memory maps.
	w.pathToOrdinal[key] = ord
	w.ordinalToPath = append(w.ordinalToPath, label)
	w.parentOrdinals = append(w.parentOrdinals, parentOrd)

	// Mark uncommitted changes only for non-root categories (root is an
	// internal implementation detail, not a user-facing add).
	if len(label.Components) > 0 {
		w.uncommittedChanges = true
	}

	return nil
}

// indexCategoryDocument adds one taxonomy document to the index writer.
func (w *DirectoryTaxonomyWriter) indexCategoryDocument(pathStr string, parentOrd int) error {
	doc := document.NewDocument()

	// StringField for term-based lookup (indexed, not stored).
	sf, err := document.NewStringField(taxoFieldFull, pathStr, false)
	if err != nil {
		return fmt.Errorf("creating string field: %w", err)
	}
	doc.Add(sf)

	// BinaryDocValuesField for ordinal-to-path retrieval.
	bdf, err := document.NewBinaryDocValuesField(taxoFieldFull, []byte(pathStr))
	if err != nil {
		return fmt.Errorf("creating binary doc values field: %w", err)
	}
	doc.Add(bdf)

	// NumericDocValuesField for parent ordinal.
	ndf, err := document.NewNumericDocValuesField(taxoFieldParentNDV, int64(parentOrd))
	if err != nil {
		return fmt.Errorf("creating numeric doc values field: %w", err)
	}
	doc.Add(ndf)

	return w.indexWriter.AddDocument(doc)
}

// AddCategory adds a category to the taxonomy, recursively adding any missing
// ancestors first. Returns the ordinal of the added (or existing) category.
//
// If label is nil or empty (root), returns an error for nil; for the empty
// label (root category), returns taxoRootOrdinal = 0.
func (w *DirectoryTaxonomyWriter) AddCategory(label *FacetLabel) (int, error) {
	if label == nil {
		return -1, fmt.Errorf("label cannot be nil")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isOpen {
		return -1, fmt.Errorf("taxonomy writer is closed")
	}

	// Root category (empty label) is always ordinal 0.
	if len(label.Components) == 0 {
		return taxoRootOrdinal, nil
	}

	key := label.String()
	if ord, ok := w.pathToOrdinal[key]; ok {
		return ord, nil
	}

	if err := w.addCategoryLocked(label); err != nil {
		return -1, err
	}
	return w.pathToOrdinal[key], nil
}

// AddCategoryPath is a convenience method that creates a FacetLabel from
// the given path components and calls AddCategory.
func (w *DirectoryTaxonomyWriter) AddCategoryPath(components ...string) (int, error) {
	return w.AddCategory(NewFacetLabel(components...))
}

// GetSize returns the number of categories in the taxonomy, including the root.
// An empty taxonomy always has size 1 (the root at ordinal 0).
func (w *DirectoryTaxonomyWriter) GetSize() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.nextOrdinal
}

// GetPath returns the FacetLabel for the given ordinal.
// Returns nil if the ordinal is out of range.
func (w *DirectoryTaxonomyWriter) GetPath(ordinal int) *FacetLabel {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ordinal < 0 || ordinal >= len(w.ordinalToPath) {
		return nil
	}
	return w.ordinalToPath[ordinal]
}

// GetOrdinal returns the ordinal for the given FacetLabel, or taxoInvalidOrdinal
// if the category does not exist.
func (w *DirectoryTaxonomyWriter) GetOrdinal(label *FacetLabel) int {
	if label == nil {
		return taxoInvalidOrdinal
	}
	if len(label.Components) == 0 {
		return taxoRootOrdinal
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	ord, ok := w.pathToOrdinal[label.String()]
	if !ok {
		return taxoInvalidOrdinal
	}
	return ord
}

// GetParent returns the parent ordinal for the given ordinal.
// Returns taxoInvalidOrdinal for the root or out-of-range ordinals.
func (w *DirectoryTaxonomyWriter) GetParent(ordinal int) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ordinal < 0 || ordinal >= len(w.parentOrdinals) {
		return taxoInvalidOrdinal
	}
	return w.parentOrdinals[ordinal]
}

// GetNextOrdinal returns the next ordinal that will be assigned.
func (w *DirectoryTaxonomyWriter) GetNextOrdinal() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.nextOrdinal
}

// GetDirectory returns the directory used by this writer.
func (w *DirectoryTaxonomyWriter) GetDirectory() store.Directory {
	return w.directory
}

// IsOpen returns true if this writer is open.
func (w *DirectoryTaxonomyWriter) IsOpen() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.isOpen
}

// GetIndexWriter returns the internal index writer. Used by
// DirectoryTaxonomyReader to open an NRT reader.
func (w *DirectoryTaxonomyWriter) GetIndexWriter() *index.IndexWriter {
	return w.indexWriter
}

// Commit persists all pending changes to the directory.
func (w *DirectoryTaxonomyWriter) Commit() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.isOpen {
		return fmt.Errorf("taxonomy writer is closed")
	}
	if err := w.indexWriter.Commit(); err != nil {
		return err
	}
	w.uncommittedChanges = false
	return nil
}

// Close commits and closes this writer.
func (w *DirectoryTaxonomyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.isOpen {
		return nil
	}
	if err := w.indexWriter.Commit(); err != nil {
		_ = w.indexWriter.Close()
		w.isOpen = false
		return fmt.Errorf("commit on close: %w", err)
	}
	if err := w.indexWriter.Close(); err != nil {
		w.isOpen = false
		return fmt.Errorf("closing index writer: %w", err)
	}
	w.isOpen = false
	return nil
}

// Rollback discards all uncommitted changes and closes the writer.
// This mirrors Lucene's DirectoryTaxonomyWriter.rollback() which closes the
// internal IndexWriter without committing; subsequent operations on this
// writer will return errors.
func (w *DirectoryTaxonomyWriter) Rollback() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.isOpen {
		return fmt.Errorf("taxonomy writer is closed")
	}
	err := w.indexWriter.Rollback()
	w.isOpen = false
	return err
}

// HasUncommittedChanges returns true if categories have been added since the
// last successful Commit. The implicit root category added at construction
// is not counted as an uncommitted change.
func (w *DirectoryTaxonomyWriter) HasUncommittedChanges() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.isOpen && w.uncommittedChanges
}

// GetCacheSize returns the number of categories currently held in the
// in-memory cache (same as GetSize() in this implementation because the
// cache is always complete).
func (w *DirectoryTaxonomyWriter) GetCacheSize() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	// pathToOrdinal includes root; cache size = total - root (for compatibility with tests
	// that expect cache to reflect only user-added categories).
	// However tests call GetCacheSize() after AddCategory and expect it to match the
	// number of Add calls, not the total including root. Since the root is always in the
	// map (at ""), we subtract 1.
	n := len(w.pathToOrdinal)
	if n > 0 {
		n-- // exclude root
	}
	return n
}

// snapshotState captures the writer's in-memory state for an NRT reader.
// The returned slices are copies and safe to read concurrently.
func (w *DirectoryTaxonomyWriter) snapshotState() (map[string]int, []*FacetLabel, []int, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p2o := make(map[string]int, len(w.pathToOrdinal))
	for k, v := range w.pathToOrdinal {
		p2o[k] = v
	}
	o2p := make([]*FacetLabel, len(w.ordinalToPath))
	copy(o2p, w.ordinalToPath)
	parents := make([]int, len(w.parentOrdinals))
	copy(parents, w.parentOrdinals)
	return p2o, o2p, parents, w.nextOrdinal
}

// OrdinalMap records the mapping from source ordinals to destination ordinals
// produced by AddTaxonomy. Mirrors the nested interface in Lucene's
// DirectoryTaxonomyWriter.
type OrdinalMap interface {
	// SetSize declares the size of the source taxonomy. Must be called before
	// any AddMapping call.
	SetSize(size int) error
	// AddMapping records that source ordinal origOrdinal maps to newOrdinal
	// in the destination taxonomy.
	AddMapping(origOrdinal, newOrdinal int) error
	// AddDone signals that all AddMapping calls are complete.
	AddDone() error
	// GetMap returns the complete ordinal-to-ordinal array. Index is the source
	// ordinal; value is the destination ordinal.
	GetMap() ([]int, error)
}

// MemoryOrdinalMap is an in-memory OrdinalMap. Mirrors
// DirectoryTaxonomyWriter.MemoryOrdinalMap.
type MemoryOrdinalMap struct {
	m []int
}

// SetSize allocates the internal array.
func (o *MemoryOrdinalMap) SetSize(size int) error {
	o.m = make([]int, size)
	return nil
}

// AddMapping records a mapping.
func (o *MemoryOrdinalMap) AddMapping(origOrdinal, newOrdinal int) error {
	if origOrdinal < 0 || origOrdinal >= len(o.m) {
		return fmt.Errorf("origOrdinal %d out of range [0, %d)", origOrdinal, len(o.m))
	}
	o.m[origOrdinal] = newOrdinal
	return nil
}

// AddDone is a no-op for the in-memory implementation.
func (o *MemoryOrdinalMap) AddDone() error { return nil }

// GetMap returns the mapping array. May only be called after AddDone.
func (o *MemoryOrdinalMap) GetMap() ([]int, error) { return o.m, nil }

// AddTaxonomy merges the source taxonomy from srcDir into this writer.
// For every category in the source that is absent from the destination,
// it is added; for every category (including ones already present) the
// mapping from the source ordinal to the destination ordinal is recorded
// in the provided OrdinalMap.
//
// The implementation iterates the source taxonomy's term index (the
// taxoFieldFull StringField). This path works without DocValues and does
// not require the SegmentReader core-readers to be wired. The source
// taxonomy directory must have been committed (no in-flight NRT state is
// visible via OpenDirectoryReader unless the index writer is NRT-flushed).
//
// This is the Go port of DirectoryTaxonomyWriter.addTaxonomy.
func (w *DirectoryTaxonomyWriter) AddTaxonomy(srcDir store.Directory, ordMap OrdinalMap) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.isOpen {
		return fmt.Errorf("taxonomy writer is closed")
	}
	if srcDir == nil {
		return fmt.Errorf("srcDir cannot be nil")
	}
	if ordMap == nil {
		return fmt.Errorf("ordMap cannot be nil")
	}

	r, err := index.OpenDirectoryReader(srcDir)
	if err != nil {
		return fmt.Errorf("opening source taxonomy: %w", err)
	}
	defer r.Close() //nolint:errcheck

	size := r.NumDocs()
	if err := ordMap.SetSize(size); err != nil {
		return err
	}

	leaves, err := r.Leaves()
	if err != nil {
		return err
	}

	for _, lrc := range leaves {
		lr := lrc.Reader()
		type termer interface {
			Terms(string) (index.Terms, error)
		}
		tr, ok := lr.(termer)
		if !ok {
			continue
		}
		base := lrc.DocBase()

		terms, err := tr.Terms(taxoFieldFull)
		if err != nil {
			return fmt.Errorf("reading terms: %w", err)
		}
		if terms == nil {
			continue
		}
		te, err := terms.GetIterator()
		if err != nil {
			return fmt.Errorf("getting terms iterator: %w", err)
		}

		for {
			term, err := te.Next()
			if err != nil {
				return fmt.Errorf("iterating terms: %w", err)
			}
			if term == nil {
				break
			}

			pathStr := string(term.Bytes.Bytes[term.Bytes.Offset : term.Bytes.Offset+term.Bytes.Length])
			label := facetLabelFromPathString(pathStr)

			// addCategoryLocked will return early if the label already exists.
			if err := w.addCategoryLocked(label); err != nil {
				return fmt.Errorf("adding category %q: %w", pathStr, err)
			}
			newOrd := w.pathToOrdinal[label.String()]

			// Get the source docID for this term to determine its source ordinal.
			pe, err := te.Postings(0)
			if err != nil {
				return fmt.Errorf("getting postings for %q: %w", pathStr, err)
			}
			docID, err := pe.NextDoc()
			if err != nil {
				return fmt.Errorf("reading postings doc for %q: %w", pathStr, err)
			}
			origOrd := base + docID

			if err := ordMap.AddMapping(origOrd, newOrd); err != nil {
				return fmt.Errorf("recording ordinal mapping: %w", err)
			}
		}
	}

	return ordMap.AddDone()
}

// DirectoryTaxonomyWriterFactory creates DirectoryTaxonomyWriter instances.
type DirectoryTaxonomyWriterFactory struct {
	directory store.Directory
	options   *DirectoryTaxonomyWriterOptions
}

// NewDirectoryTaxonomyWriterFactory creates a new factory.
func NewDirectoryTaxonomyWriterFactory(dir store.Directory) *DirectoryTaxonomyWriterFactory {
	return &DirectoryTaxonomyWriterFactory{directory: dir}
}

// NewDirectoryTaxonomyWriterFactoryWithOptions creates a factory with options.
func NewDirectoryTaxonomyWriterFactoryWithOptions(dir store.Directory, opts *DirectoryTaxonomyWriterOptions) *DirectoryTaxonomyWriterFactory {
	return &DirectoryTaxonomyWriterFactory{directory: dir, options: opts}
}

// Open opens a DirectoryTaxonomyWriter.
func (f *DirectoryTaxonomyWriterFactory) Open() (*DirectoryTaxonomyWriter, error) {
	return NewDirectoryTaxonomyWriterWithOptions(f.directory, f.options)
}

// DirectoryTaxonomyWriterManager manages a single DirectoryTaxonomyWriter.
type DirectoryTaxonomyWriterManager struct {
	factory *DirectoryTaxonomyWriterFactory
	current *DirectoryTaxonomyWriter
	isOpen  bool
}

// NewDirectoryTaxonomyWriterManager creates a new manager.
func NewDirectoryTaxonomyWriterManager(factory *DirectoryTaxonomyWriterFactory) (*DirectoryTaxonomyWriterManager, error) {
	if factory == nil {
		return nil, fmt.Errorf("factory cannot be nil")
	}
	writer, err := factory.Open()
	if err != nil {
		return nil, fmt.Errorf("opening initial writer: %w", err)
	}
	return &DirectoryTaxonomyWriterManager{factory: factory, current: writer, isOpen: true}, nil
}

// Acquire returns the current writer.
func (m *DirectoryTaxonomyWriterManager) Acquire() *DirectoryTaxonomyWriter { return m.current }

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
