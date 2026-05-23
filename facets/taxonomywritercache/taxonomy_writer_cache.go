// Package taxonomywritercache implements
// org.apache.lucene.facet.taxonomy.writercache: the contract and concrete
// caches used by DirectoryTaxonomyWriter to dedupe label-to-ordinal lookups.
package taxonomywritercache

// TaxonomyWriterCache is the contract every label-to-ordinal cache must
// satisfy. Mirrors org.apache.lucene.facet.taxonomy.writercache.TaxonomyWriterCache.
//
// The cache does not guarantee to hold all entries that have been Put into it.
// A partial LRU cache may evict entries; when Put returns true the caller must
// flush its pending writes to disk. When Get returns -1 the category may still
// exist on disk — the caller must verify against the index.
//
// Implementations must be safe for concurrent access.
type TaxonomyWriterCache interface {
	// Get returns the ordinal associated with label or -1 when the cache
	// does not know it.
	Get(label string) int

	// Put records the (label, ordinal) mapping. Returns true if the cache
	// cleared some entries and the caller should flush its on-disk index.
	Put(label string, ord int) bool

	// Size returns the number of entries currently cached.
	Size() int

	// Clear empties the cache without releasing underlying resources.
	Clear()

	// IsFull reports whether the cache is at capacity (the next Put may evict).
	IsFull() bool

	// Close releases all resources held by the cache. After Close the cache
	// must not be used. Mirrors TaxonomyWriterCache.close().
	Close()
}
