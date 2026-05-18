package util

// ShapeFieldCache stores per-document shape payloads keyed by docID.
// Mirrors org.apache.lucene.spatial.util.ShapeFieldCache.
type ShapeFieldCache struct {
	Shapes map[int][]any
}

// NewShapeFieldCache builds an empty cache.
func NewShapeFieldCache() *ShapeFieldCache {
	return &ShapeFieldCache{Shapes: make(map[int][]any)}
}

// Add registers shape under docID.
func (c *ShapeFieldCache) Add(docID int, shape any) {
	c.Shapes[docID] = append(c.Shapes[docID], shape)
}

// Get returns the cached shapes for docID (nil when absent).
func (c *ShapeFieldCache) Get(docID int) []any { return c.Shapes[docID] }

// ShapeFieldCacheProvider builds ShapeFieldCache instances for a leaf.
// Mirrors org.apache.lucene.spatial.util.ShapeFieldCacheProvider.
type ShapeFieldCacheProvider interface {
	GetCache(leafID int) (*ShapeFieldCache, error)
}

// ShapeFieldCacheDistanceValueSource computes distance against a cached
// shape collection. Mirrors
// org.apache.lucene.spatial.util.ShapeFieldCacheDistanceValueSource.
type ShapeFieldCacheDistanceValueSource struct {
	Provider     ShapeFieldCacheProvider
	DistanceFn   func(docID int, shapes []any) (float64, error)
	currentLeaf  int
	currentCache *ShapeFieldCache
}

// NewShapeFieldCacheDistanceValueSource builds the source.
func NewShapeFieldCacheDistanceValueSource(provider ShapeFieldCacheProvider, fn func(docID int, shapes []any) (float64, error)) *ShapeFieldCacheDistanceValueSource {
	return &ShapeFieldCacheDistanceValueSource{Provider: provider, DistanceFn: fn}
}

// SetLeaf changes the active leaf.
func (s *ShapeFieldCacheDistanceValueSource) SetLeaf(leafID int) error {
	cache, err := s.Provider.GetCache(leafID)
	if err != nil {
		return err
	}
	s.currentLeaf = leafID
	s.currentCache = cache
	return nil
}

// GetValues returns the distance for docID using the active leaf cache.
func (s *ShapeFieldCacheDistanceValueSource) GetValues(docID int) (float64, error) {
	if s.currentCache == nil || s.DistanceFn == nil {
		return 0, nil
	}
	return s.DistanceFn(docID, s.currentCache.Get(docID))
}

var _ DoubleValuesSource = (*ShapeFieldCacheDistanceValueSource)(nil)

// ShapeValuesPredicate decides whether a document's shape satisfies a
// predicate. Mirrors org.apache.lucene.spatial.util.ShapeValuesPredicate.
type ShapeValuesPredicate func(docID int) (bool, error)
