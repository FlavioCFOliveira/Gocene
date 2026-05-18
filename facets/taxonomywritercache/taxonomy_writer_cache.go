// Package taxonomywritercache implements
// org.apache.lucene.facet.taxonomy.writercache: the contract and concrete
// caches used by DirectoryTaxonomyWriter to dedupe label-to-ordinal lookups.
package taxonomywritercache

// TaxonomyWriterCache is the contract every label-to-ordinal cache must
// satisfy. Mirrors org.apache.lucene.facet.taxonomy.writercache.TaxonomyWriterCache.
type TaxonomyWriterCache interface {
	// Get returns the ordinal associated with label or -1 when the cache
	// does not know it.
	Get(label string) int

	// Put records the (label, ordinal) mapping. Returns true if the cache
	// is now full and the caller should consider flushing.
	Put(label string, ord int) bool

	// Size returns the number of entries currently cached.
	Size() int

	// Clear empties the cache.
	Clear()

	// IsFull reports whether the cache cannot accept more entries without
	// evicting.
	IsFull() bool
}
