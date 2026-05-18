package taxonomy

import "sync"

// SearcherAndTaxonomy bundles the IndexSearcher/TaxonomyReader pair returned
// by SearcherTaxonomyManager.Acquire(). The Go port uses opaque interface
// values so callers can supply whichever concrete searcher and reader types
// live in their codebase.
type SearcherAndTaxonomy struct {
	Searcher any
	Taxonomy any
}

// SearcherTaxonomyManager coordinates the lifecycle of a paired
// IndexSearcher/TaxonomyReader so callers can refresh them atomically.
// Mirrors org.apache.lucene.facet.taxonomy.SearcherTaxonomyManager.
type SearcherTaxonomyManager struct {
	mu      sync.RWMutex
	current SearcherAndTaxonomy
	refresh func() (SearcherAndTaxonomy, error)
}

// NewSearcherTaxonomyManager builds the manager with the supplied initial pair
// and refresh callback.
func NewSearcherTaxonomyManager(initial SearcherAndTaxonomy, refresh func() (SearcherAndTaxonomy, error)) *SearcherTaxonomyManager {
	return &SearcherTaxonomyManager{current: initial, refresh: refresh}
}

// Acquire returns the latest (searcher, taxonomy) pair. The Java port also
// increments a refcount; here the caller is expected to manage lifetimes
// externally.
func (m *SearcherTaxonomyManager) Acquire() SearcherAndTaxonomy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// MaybeRefresh invokes the refresh callback and atomically swaps the held
// pair on success.
func (m *SearcherTaxonomyManager) MaybeRefresh() error {
	if m.refresh == nil {
		return nil
	}
	updated, err := m.refresh()
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.current = updated
	m.mu.Unlock()
	return nil
}
