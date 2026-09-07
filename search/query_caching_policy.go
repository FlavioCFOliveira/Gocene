package search

// QueryCachingPolicy defines which filters should be cached.
// Implementations of this interface must be thread-safe.
type QueryCachingPolicy interface {
	// OnUse is called every time that a cached filter is used.
	// This is typically useful if the policy wants to track usage statistics in order to make decisions.
	OnUse(query Query)

	// ShouldCache determines whether the given Query is worth caching.
	// This method will be called by the QueryCache to know whether to cache.
	// It will first attempt to load a DocIdSet from the cache.
	// If it is not cached yet and this method returns true then a cache entry will be generated.
	// Otherwise an uncached scorer will be returned.
	ShouldCache(query Query) (bool, error)
}
