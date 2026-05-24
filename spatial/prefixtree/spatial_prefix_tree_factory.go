package prefixtree

// Configuration key constants for SpatialPrefixTreeFactory.
const (
	// PrefixTreeKey selects the tree type ("geohash", "quad", "packedQuad", "s2").
	PrefixTreeKey = "prefixTree"
	// MaxLevelsKey overrides the maximum depth of the tree.
	MaxLevelsKey = "maxLevels"
	// MaxDistErrKey sets the maximum detail distance error in degrees.
	MaxDistErrKey = "maxDistErr"
	// VersionKey sets the Lucene version to mimic.
	VersionKey = "version"
)

// SpatialPrefixTreeFactory builds SpatialPrefixTree instances from a
// configuration map.
//
// Port of org.apache.lucene.spatial.prefix.tree.SpatialPrefixTreeFactory.
//
// Deviation: SpatialContext, DistanceUtils, and Version are not yet ported;
// MakeSPT returns nil until backlog #2693 is resolved.
type SpatialPrefixTreeFactory struct {
	// Args holds the raw configuration parameters.
	Args      map[string]string
	Ctx       interface{} // spatial4j SpatialContext
	MaxLevels *int
}

// NewSpatialPrefixTreeFactory builds an empty factory.
func NewSpatialPrefixTreeFactory() *SpatialPrefixTreeFactory {
	return &SpatialPrefixTreeFactory{Args: make(map[string]string)}
}

// SetParam records a configuration entry.
func (f *SpatialPrefixTreeFactory) SetParam(k, v string) { f.Args[k] = v }

// Init initialises the factory from args and ctx.
func (f *SpatialPrefixTreeFactory) Init(args map[string]string, ctx interface{}) {
	f.Args = args
	f.Ctx = ctx
	// MaxLevels and Version parsing deferred to #2693 (requires SpatialContext/DistanceUtils).
}

// MakeSPT selects the correct factory and builds the tree.
// Returns nil — full implementation deferred to #2693.
func MakeSPT(args map[string]string, _ interface{} /* SpatialContext */) SpatialPrefixTree {
	return nil
}
