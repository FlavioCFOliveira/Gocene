// Purpose: Shared scaffolding mirror for vectorization tests.
// Lucene reference: org.apache.lucene.internal.vectorization.BaseVectorizationTestCase
//   (lucene/core/src/test/org/apache/lucene/internal/vectorization/BaseVectorizationTestCase.java).
//
// The Java abstract class extends LuceneTestCase, exposes the default and
// (optionally) Panama-accelerated VectorizationProvider singletons, and uses
// JUnit's @BeforeClass + assumeTrue() to skip suites when the JDK vector
// incubator module is absent. Gocene has no Panama equivalent, so the helpers
// below return the default provider as the "maybe-Panama" stand-in and the
// guard helper centralizes the skip so call sites match the Java contract.
//
// This file intentionally lives in _test.go: the Java type is test-only and
// only referenced from other test cases in the same package.

package vectorization

import "testing"

// luceneProvider mirrors BaseVectorizationTestCase.LUCENE_PROVIDER: the scalar
// default VectorizationProvider used as the reference implementation.
//
//nolint:unused // Test scaffolding consumed by future ported vectorization tests.
var luceneProvider = defaultVectorizationProviderForTest()

// panamaProvider mirrors BaseVectorizationTestCase.PANAMA_PROVIDER: the
// SIMD-accelerated provider when present. Gocene has no Panama backend, so this
// resolves to the default provider and [[assumePanamaProviderAvailable]] skips
// the suite to keep the Java semantics.
//
//nolint:unused // Test scaffolding consumed by future ported vectorization tests.
var panamaProvider = maybePanamaProviderForTest()

// defaultVectorizationProviderForTest mirrors defaultProvider() and returns a
// freshly constructed scalar provider. Kept as a function (not an inline var
// initializer) so future ported tests can override it without touching package
// state.
func defaultVectorizationProviderForTest() *DefaultVectorizationProvider {
	return NewDefaultVectorizationProvider()
}

// maybePanamaProviderForTest mirrors maybePanamaProvider() and would resolve to
// the Panama-backed provider when available. The Go port has no Panama
// equivalent yet, so it returns the default provider; pair with
// [[assumePanamaProviderAvailable]] to honour the Java skip contract.
func maybePanamaProviderForTest() *DefaultVectorizationProvider {
	return NewDefaultVectorizationProvider()
}

// assumePanamaProviderAvailable mirrors the @BeforeClass assumeTrue() guard in
// BaseVectorizationTestCase: when the Panama provider is indistinguishable from
// the default provider (always true in Gocene today), it skips the test with
// the same message Lucene emits.
//
//nolint:unused // Helper consumed by future ported vectorization tests.
func assumePanamaProviderAvailable(tb testing.TB) {
	tb.Helper()
	if panamaProvider == nil || luceneProvider == nil {
		tb.Skip("Test only works when JDK's vector incubator module is enabled.")
		return
	}
	// In Gocene the two providers share the same concrete type, so the Java
	// `panama.getClass() != lucene.getClass()` check always fails. Preserve the
	// upstream message verbatim for parity with JUnit test output.
	tb.Skip("Test only works when JDK's vector incubator module is enabled.")
}
