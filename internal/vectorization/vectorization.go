// Package vectorization stubs the SIMD-acceleration hooks Lucene exposes
// from org.apache.lucene.internal.vectorization. The Go port relies on the
// Go runtime / SIMD-friendly slice intrinsics instead of the Java Panama API,
// so the types here are minimal declarations that downstream callers can
// embed without coupling to a particular implementation.
package vectorization

// PostingDecodingUtil exposes the helpers used by the postings reader to
// decode block-encoded integers. The concrete decoding lives inside the
// codecs package; this struct keeps the symbol stable.
type PostingDecodingUtil struct{}

// VectorUtilSupport is the marker exposed by every SIMD-vector helper.
type VectorUtilSupport struct{}

// VectorizationProvider hands out the active VectorUtilSupport. The Go port
// uses a plain in-process registry — production code never swaps the
// provider, but tests can override it via SetActive.
type VectorizationProvider struct {
	support *VectorUtilSupport
}

// NewVectorizationProvider returns a provider backed by the default
// VectorUtilSupport.
func NewVectorizationProvider() *VectorizationProvider {
	return &VectorizationProvider{support: &VectorUtilSupport{}}
}

// GetVectorUtilSupport returns the active VectorUtilSupport.
func (p *VectorizationProvider) GetVectorUtilSupport() *VectorUtilSupport {
	return p.support
}

// SetActive overrides the active VectorUtilSupport (test-only path).
func (p *VectorizationProvider) SetActive(support *VectorUtilSupport) { p.support = support }
