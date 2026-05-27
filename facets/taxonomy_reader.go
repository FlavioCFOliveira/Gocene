package facets

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// TaxonomyReader provides read access to the taxonomy index.
// The taxonomy index maps category paths to unique ordinals (integers).
//
// This is the Go port of Lucene's org.apache.lucene.facet.taxonomy.TaxonomyReader.
type TaxonomyReader struct {
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
}

// NewTaxonomyReader creates a new TaxonomyReader.
func NewTaxonomyReader() *TaxonomyReader {
	return &TaxonomyReader{
		ordinals:    make(map[string]int),
		paths:       make(map[int]string),
		children:    make(map[int][]int),
		parent:      make(map[int]int),
		nextOrdinal: 1, // Start at 1, reserve 0 for invalid
	}
}

// GetOrdinal returns the ordinal for the given category path.
// Returns -1 if the path is not found.
func (tr *TaxonomyReader) GetOrdinal(path string) int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if ord, ok := tr.ordinals[path]; ok {
		return ord
	}
	return -1
}

// GetPath returns the category path for the given ordinal.
// Returns empty string if the ordinal is not found.
func (tr *TaxonomyReader) GetPath(ordinal int) string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if path, ok := tr.paths[ordinal]; ok {
		return path
	}
	return ""
}

// GetParent returns the parent ordinal for the given ordinal.
// Returns -1 if the ordinal has no parent (root level).
func (tr *TaxonomyReader) GetParent(ordinal int) int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if parent, ok := tr.parent[ordinal]; ok {
		return parent
	}
	return -1
}

// GetChildren returns the child ordinals for the given parent ordinal.
// Returns empty slice if the parent has no children.
func (tr *TaxonomyReader) GetChildren(parentOrdinal int) []int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	if children, ok := tr.children[parentOrdinal]; ok {
		result := make([]int, len(children))
		copy(result, children)
		return result
	}
	return []int{}
}

// GetSize returns the number of categories in the taxonomy.
func (tr *TaxonomyReader) GetSize() int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return len(tr.ordinals)
}

// GetNextOrdinal returns the next available ordinal.
func (tr *TaxonomyReader) GetNextOrdinal() int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	return tr.nextOrdinal
}

// GetAllPaths returns all category paths in the taxonomy.
func (tr *TaxonomyReader) GetAllPaths() []string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	paths := make([]string, 0, len(tr.ordinals))
	for path := range tr.ordinals {
		paths = append(paths, path)
	}
	return paths
}

// GetDimensions returns all top-level dimensions (root categories).
func (tr *TaxonomyReader) GetDimensions() []string {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	dims := make([]string, 0)
	for ord, path := range tr.paths {
		if tr.parent[ord] == 0 || tr.parent[ord] == -1 {
			// This is a root - extract the dimension name
			dims = append(dims, path)
		}
	}
	return dims
}

// GetRootOrdinals returns all root-level ordinals (those with no parent).
func (tr *TaxonomyReader) GetRootOrdinals() []int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	roots := make([]int, 0)
	for ord := range tr.paths {
		if tr.parent[ord] == 0 || tr.parent[ord] == -1 {
			roots = append(roots, ord)
		}
	}
	return roots
}

// GetSiblings returns the sibling ordinals for the given ordinal.
// Siblings share the same parent.
func (tr *TaxonomyReader) GetSiblings(ordinal int) []int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	parent := tr.parent[ordinal]
	if parent == 0 || parent == -1 {
		return []int{}
	}

	if children, ok := tr.children[parent]; ok {
		siblings := make([]int, 0, len(children)-1)
		for _, child := range children {
			if child != ordinal {
				siblings = append(siblings, child)
			}
		}
		return siblings
	}
	return []int{}
}

// GetDepth returns the depth of the given ordinal in the taxonomy tree.
// Root categories have depth 0.
func (tr *TaxonomyReader) GetDepth(ordinal int) int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	depth := 0
	current := ordinal
	for current > 0 {
		if parent, ok := tr.parent[current]; ok && parent > 0 {
			depth++
			current = parent
		} else {
			break
		}
	}
	return depth
}

// GetAncestors returns all ancestor ordinals for the given ordinal.
// The first element is the immediate parent, the last is the root.
func (tr *TaxonomyReader) GetAncestors(ordinal int) []int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	ancestors := make([]int, 0)
	current := ordinal
	for current > 0 {
		if parent, ok := tr.parent[current]; ok && parent > 0 {
			ancestors = append(ancestors, parent)
			current = parent
		} else {
			break
		}
	}
	return ancestors
}

// GetDescendants returns all descendant ordinals for the given ordinal.
func (tr *TaxonomyReader) GetDescendants(ordinal int) []int {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	descendants := make([]int, 0)
	tr.getDescendantsRecursive(ordinal, &descendants)
	return descendants
}

// getDescendantsRecursive recursively collects descendants.
func (tr *TaxonomyReader) getDescendantsRecursive(ordinal int, descendants *[]int) {
	if children, ok := tr.children[ordinal]; ok {
		for _, child := range children {
			*descendants = append(*descendants, child)
			tr.getDescendantsRecursive(child, descendants)
		}
	}
}

// IsDescendantOf checks if childOrdinal is a descendant of ancestorOrdinal.
func (tr *TaxonomyReader) IsDescendantOf(childOrdinal, ancestorOrdinal int) bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	current := childOrdinal
	for current > 0 {
		if current == ancestorOrdinal {
			return true
		}
		if parent, ok := tr.parent[current]; ok {
			current = parent
		} else {
			break
		}
	}
	return false
}

// Close closes this taxonomy reader.
func (tr *TaxonomyReader) Close() error {
	// Nothing to close for in-memory implementation
	return nil
}

// TaxonomyReaderLoader populates a TaxonomyReader from an underlying index.
// It is the hook that bridges TaxonomyReaderFactory.Open to a concrete
// index-backed source (typically DirectoryTaxonomyReader, ported in Sprint
// 116 task #4662). When no loader is installed, Open returns an empty
// in-memory reader, preserving the original behaviour of this lightweight
// adapter type.
type TaxonomyReaderLoader func(reader *index.IndexReader, dst *TaxonomyReader) error

// TaxonomyReaderRefresher checks the underlying index for newer taxonomy
// commits and updates the supplied reader in place. The refresher MUST be
// safe to call repeatedly and MUST not mutate the reader unless changes are
// detected. Returning a non-nil error aborts MaybeRefresh.
type TaxonomyReaderRefresher func(current *TaxonomyReader) error

// TaxonomyReaderFactory creates TaxonomyReader instances.
type TaxonomyReaderFactory struct {
	// reader is the index reader to use
	reader *index.IndexReader

	// loader, when non-nil, populates the freshly created TaxonomyReader from
	// the index. Without a loader, Open returns an empty reader.
	loader TaxonomyReaderLoader
}

// NewTaxonomyReaderFactory creates a new TaxonomyReaderFactory.
func NewTaxonomyReaderFactory(reader *index.IndexReader) *TaxonomyReaderFactory {
	return &TaxonomyReaderFactory{
		reader: reader,
	}
}

// SetLoader installs a loader callback used by Open to populate the returned
// reader. Passing nil restores the empty-reader behaviour.
func (trf *TaxonomyReaderFactory) SetLoader(loader TaxonomyReaderLoader) {
	trf.loader = loader
}

// Open opens a TaxonomyReader from the index.
//
// If a loader is installed, it is invoked to populate the new reader from the
// underlying index. Without a loader, Open returns an empty in-memory reader;
// callers that need an index-backed reader should either install a loader
// here or use DirectoryTaxonomyReader directly.
func (trf *TaxonomyReaderFactory) Open() (*TaxonomyReader, error) {
	r := NewTaxonomyReader()
	if trf.loader != nil {
		if err := trf.loader(trf.reader, r); err != nil {
			return nil, fmt.Errorf("loading taxonomy: %w", err)
		}
	}
	return r, nil
}

// TaxonomyReaderManager manages TaxonomyReader instances.
type TaxonomyReaderManager struct {
	// current is the current TaxonomyReader
	current *TaxonomyReader

	// refresher, when non-nil, is invoked by MaybeRefresh to update current.
	refresher TaxonomyReaderRefresher

	// mu protects current and refresher
	mu sync.RWMutex
}

// NewTaxonomyReaderManager creates a new TaxonomyReaderManager.
func NewTaxonomyReaderManager(reader *TaxonomyReader) *TaxonomyReaderManager {
	return &TaxonomyReaderManager{
		current: reader,
	}
}

// SetRefresher installs a refresher callback used by MaybeRefresh. Passing
// nil restores the no-op behaviour.
func (trm *TaxonomyReaderManager) SetRefresher(r TaxonomyReaderRefresher) {
	trm.mu.Lock()
	defer trm.mu.Unlock()
	trm.refresher = r
}

// Acquire returns the current TaxonomyReader.
func (trm *TaxonomyReaderManager) Acquire() *TaxonomyReader {
	trm.mu.RLock()
	defer trm.mu.RUnlock()
	return trm.current
}

// MaybeRefresh refreshes the reader if a refresher is installed.
//
// The installed TaxonomyReaderRefresher decides whether the current reader
// needs to be updated; this manager does not impose its own change-detection
// policy (see DirectoryTaxonomyReader for the disk-backed variant). Without
// a refresher, MaybeRefresh is a no-op and returns nil.
func (trm *TaxonomyReaderManager) MaybeRefresh() error {
	trm.mu.RLock()
	current := trm.current
	refresher := trm.refresher
	trm.mu.RUnlock()
	if refresher == nil || current == nil {
		return nil
	}
	if err := refresher(current); err != nil {
		return fmt.Errorf("refreshing taxonomy reader: %w", err)
	}
	return nil
}

// Close closes this manager.
func (trm *TaxonomyReaderManager) Close() error {
	trm.mu.Lock()
	defer trm.mu.Unlock()

	if trm.current != nil {
		return trm.current.Close()
	}
	return nil
}
