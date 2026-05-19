// Purpose: Unit tests for [[DefaultVectorizationProvider]] (GOC-3422).
// Lucene reference: no upstream test peer (the Java type is package-private
// and exercised indirectly through VectorizationProvider). These tests pin the
// observable contract the Go port advertises.

package vectorization

import "testing"

func TestDefaultVectorizationProvider_Names(t *testing.T) {
	t.Parallel()

	if got := DefaultVectorizationProviderName; got != "DefaultVectorizationProvider" {
		t.Fatalf("provider name = %q, want %q", got, "DefaultVectorizationProvider")
	}
	if got := DefaultFlatVectorsScorerName; got != "DefaultFlatVectorScorer" {
		t.Fatalf("flat scorer name = %q, want %q", got, "DefaultFlatVectorScorer")
	}
	if got := Lucene99ScalarQuantizedVectorsScorerName; got != "Lucene99ScalarQuantizedVectorScorer" {
		t.Fatalf("quantized scorer name = %q, want %q", got, "Lucene99ScalarQuantizedVectorScorer")
	}
}

func TestDefaultVectorizationProvider_Contract(t *testing.T) {
	t.Parallel()

	p := NewDefaultVectorizationProvider()
	if p == nil {
		t.Fatal("NewDefaultVectorizationProvider returned nil")
	}
	if p.GetVectorUtilSupport() == nil {
		t.Fatal("VectorUtilSupport should be initialised")
	}
	if got := p.GetLucene99FlatVectorsScorerName(); got != DefaultFlatVectorsScorerName {
		t.Fatalf("flat scorer = %q, want %q", got, DefaultFlatVectorsScorerName)
	}
	if got := p.GetLucene99ScalarQuantizedVectorsScorerName(); got != Lucene99ScalarQuantizedVectorsScorerName {
		t.Fatalf("quantized scorer = %q, want %q", got, Lucene99ScalarQuantizedVectorsScorerName)
	}
	if u := p.NewPostingDecodingUtil(nil); u == nil {
		t.Fatal("NewPostingDecodingUtil returned nil")
	}
}

func TestDefaultVectorizationProvider_EmbedsProvider(t *testing.T) {
	t.Parallel()

	p := NewDefaultVectorizationProvider()
	// The embedded VectorizationProvider must remain swappable via SetActive
	// so codec tests can install a stub support without rebuilding the
	// default provider.
	override := &VectorUtilSupport{}
	p.SetActive(override)
	if p.GetVectorUtilSupport() != override {
		t.Fatal("SetActive override not visible through embedded provider")
	}
}
