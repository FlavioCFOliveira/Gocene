package prefixtree

// SpatialPrefixTreeFactory builds SpatialPrefixTree instances from a
// configuration map. Mirrors
// org.apache.lucene.spatial.prefix.tree.SpatialPrefixTreeFactory.
type SpatialPrefixTreeFactory struct {
	Params map[string]string
}

// NewSpatialPrefixTreeFactory builds an empty factory.
func NewSpatialPrefixTreeFactory() *SpatialPrefixTreeFactory {
	return &SpatialPrefixTreeFactory{Params: make(map[string]string)}
}

// SetParam records a configuration entry.
func (f *SpatialPrefixTreeFactory) SetParam(k, v string) { f.Params[k] = v }
