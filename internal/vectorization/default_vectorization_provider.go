// Purpose: Default scalar VectorizationProvider mirror.
// Lucene reference: org.apache.lucene.internal.vectorization.DefaultVectorizationProvider
//   (lucene/core/src/java/org/apache/lucene/internal/vectorization/DefaultVectorizationProvider.java).
//
// The Java original is package-private and overrides VectorizationProvider's
// scalar hooks. The Go port keeps the provider plumbing in [[vectorization]]
// minimal and avoids importing codecs/* (which would create a cycle), so this
// shim records the scorer identities by stable name. Wiring into the codec
// graph is owned by the codecs packages themselves.

package vectorization

// DefaultVectorizationProviderName is the symbolic identity of the default,
// scalar VectorizationProvider exposed by Lucene. It is fixed by the upstream
// class name and is used by codec wiring to disambiguate provider variants
// without forcing a cross-package import.
const DefaultVectorizationProviderName = "DefaultVectorizationProvider"

// DefaultFlatVectorsScorerName mirrors the singleton identity of
// org.apache.lucene.codecs.hnsw.DefaultFlatVectorScorer.INSTANCE that the Java
// DefaultVectorizationProvider returns from getLucene99FlatVectorsScorer().
const DefaultFlatVectorsScorerName = "DefaultFlatVectorScorer"

// Lucene99ScalarQuantizedVectorsScorerName mirrors the class identity of
// org.apache.lucene.codecs.lucene99.Lucene99ScalarQuantizedVectorScorer that
// the Java DefaultVectorizationProvider constructs from
// getLucene99ScalarQuantizedVectorsScorer(), wrapping the default flat scorer.
const Lucene99ScalarQuantizedVectorsScorerName = "Lucene99ScalarQuantizedVectorScorer"

// DefaultVectorizationProvider is the scalar VectorizationProvider used when
// no SIMD-accelerated implementation is selected. It mirrors the
// package-private Java type by name and exposes the same observable surface:
// a VectorUtilSupport, named flat-vector scorers, and a PostingDecodingUtil
// factory that wraps the provided input.
type DefaultVectorizationProvider struct {
	VectorizationProvider
}

// NewDefaultVectorizationProvider builds a DefaultVectorizationProvider backed
// by the default VectorUtilSupport, matching the Java no-arg constructor.
func NewDefaultVectorizationProvider() *DefaultVectorizationProvider {
	return &DefaultVectorizationProvider{
		VectorizationProvider: VectorizationProvider{support: &VectorUtilSupport{}},
	}
}

// GetLucene99FlatVectorsScorerName returns the identity of the flat-vectors
// scorer that the Java provider would hand back from
// getLucene99FlatVectorsScorer(). Returning the identity keeps the
// [[vectorization]] package free of codecs/* imports while preserving the
// public contract for callers that need to resolve the concrete scorer.
func (p *DefaultVectorizationProvider) GetLucene99FlatVectorsScorerName() string {
	return DefaultFlatVectorsScorerName
}

// GetLucene99ScalarQuantizedVectorsScorerName returns the identity of the
// quantized scorer the Java provider would build from
// getLucene99ScalarQuantizedVectorsScorer(). See
// [[GetLucene99FlatVectorsScorerName]] for the rationale behind exposing the
// identity rather than a concrete type.
func (p *DefaultVectorizationProvider) GetLucene99ScalarQuantizedVectorsScorerName() string {
	return Lucene99ScalarQuantizedVectorsScorerName
}

// NewPostingDecodingUtil mirrors newPostingDecodingUtil(IndexInput). The
// concrete decoding lives inside the codecs package; this shim returns the
// stable placeholder so callers compile against a real factory point.
//
// The input argument is intentionally [[any]]: the Gocene IndexInput interface
// lives in the store package, and importing it here would invert the codec
// dependency direction. The current factory ignores the input — the decoder
// is wired by the codec when it consumes the returned struct.
func (p *DefaultVectorizationProvider) NewPostingDecodingUtil(_ any) *PostingDecodingUtil {
	return &PostingDecodingUtil{}
}
