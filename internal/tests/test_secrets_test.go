// Source: core/src/test/org/apache/lucene/internal/tests/TestTestSecrets.java
//
//	(Apache Lucene 10.4.0, commit 9983b7c)
package tests

import (
	"errors"
	"testing"
)

// TestCallerOfGetter mirrors TestTestSecrets#testCallerOfGetter: invoking a
// getter from outside the test-framework package prefix must surface
// [ErrTestFrameworkOnly] with the canonical message.
func TestCallerOfGetter(t *testing.T) {
	t.Parallel()
	_, err := illegalCaller()
	if !errors.Is(err, ErrTestFrameworkOnly) {
		t.Fatalf("expected ErrTestFrameworkOnly, got %v", err)
	}
	const want = "Lucene TestSecrets can only be used by the test-framework."
	if got := err.Error(); got != want {
		t.Fatalf("unexpected message: got %q, want %q", got, want)
	}
}

// illegalCaller funnels the call through a helper inside this (non
// test-framework) package, matching the Java helper of the same name.
func illegalCaller() (IndexWriterAccess, error) {
	return GetIndexWriterAccess()
}

// TestCannotSet mirrors TestTestSecrets#testCannotSet: every setter must
// refuse a second registration with [ErrAccessorAlreadySet].
//
// Java seeds the singletons via the test-framework's static initialiser;
// the Go test seeds them locally and then asserts the second call fails.
func TestCannotSet(t *testing.T) {
	// Cannot use t.Parallel(): the registry is package-global singleton state.
	resetTestSecrets(t)

	if err := SetIndexWriterAccess(nil); err != nil {
		t.Fatalf("first SetIndexWriterAccess must succeed, got %v", err)
	}
	if err := SetIndexWriterAccess(nil); !errors.Is(err, ErrAccessorAlreadySet) {
		t.Fatalf("expected ErrAccessorAlreadySet for IndexWriterAccess, got %v", err)
	}

	if err := SetConcurrentMergeSchedulerAccess(nil); err != nil {
		t.Fatalf("first SetConcurrentMergeSchedulerAccess must succeed, got %v", err)
	}
	if err := SetConcurrentMergeSchedulerAccess(nil); !errors.Is(err, ErrAccessorAlreadySet) {
		t.Fatalf("expected ErrAccessorAlreadySet for ConcurrentMergeSchedulerAccess, got %v", err)
	}

	if err := SetIndexPackageAccess(nil); err != nil {
		t.Fatalf("first SetIndexPackageAccess must succeed, got %v", err)
	}
	if err := SetIndexPackageAccess(nil); !errors.Is(err, ErrAccessorAlreadySet) {
		t.Fatalf("expected ErrAccessorAlreadySet for IndexPackageAccess, got %v", err)
	}

	if err := SetSegmentReaderAccess(nil); err != nil {
		t.Fatalf("first SetSegmentReaderAccess must succeed, got %v", err)
	}
	if err := SetSegmentReaderAccess(nil); !errors.Is(err, ErrAccessorAlreadySet) {
		t.Fatalf("expected ErrAccessorAlreadySet for SegmentReaderAccess, got %v", err)
	}

	// Cover FilterInputIndexAccess as well — the Java peer omits it because
	// FilterIndexInput predates the rest of the surface, but the Go contract
	// includes the same single-set guarantee.
	if err := SetFilterInputIndexAccess(nil); err != nil {
		t.Fatalf("first SetFilterInputIndexAccess must succeed, got %v", err)
	}
	if err := SetFilterInputIndexAccess(nil); !errors.Is(err, ErrAccessorAlreadySet) {
		t.Fatalf("expected ErrAccessorAlreadySet for FilterIndexInputAccess, got %v", err)
	}
}

// resetTestSecrets restores the singleton registry to its zero state and
// re-installs the prior state after the test exits, so neighbouring tests
// observe a clean slate. Confined to _test.go to keep the production surface
// immutable.
func resetTestSecrets(t *testing.T) {
	t.Helper()
	testSecrets.mu.Lock()
	prevIndexPackage := testSecrets.indexPackageAccess
	prevIndexPackageSet := testSecrets.indexPackageAccessSet
	prevCMS := testSecrets.cmsAccess
	prevCMSSet := testSecrets.cmsAccessSet
	prevSegmentReader := testSecrets.segmentReaderAccess
	prevSegmentReaderSet := testSecrets.segmentReaderAccessSet
	prevIndexWriter := testSecrets.indexWriterAccess
	prevIndexWriterSet := testSecrets.indexWriterAccessSet
	prevFilterIndexInput := testSecrets.filterIndexInputAccess
	prevFilterIndexInputSet := testSecrets.filterIndexInputAccessSet
	testSecrets.indexPackageAccess = nil
	testSecrets.indexPackageAccessSet = false
	testSecrets.cmsAccess = nil
	testSecrets.cmsAccessSet = false
	testSecrets.segmentReaderAccess = nil
	testSecrets.segmentReaderAccessSet = false
	testSecrets.indexWriterAccess = nil
	testSecrets.indexWriterAccessSet = false
	testSecrets.filterIndexInputAccess = nil
	testSecrets.filterIndexInputAccessSet = false
	testSecrets.mu.Unlock()
	t.Cleanup(func() {
		testSecrets.mu.Lock()
		testSecrets.indexPackageAccess = prevIndexPackage
		testSecrets.indexPackageAccessSet = prevIndexPackageSet
		testSecrets.cmsAccess = prevCMS
		testSecrets.cmsAccessSet = prevCMSSet
		testSecrets.segmentReaderAccess = prevSegmentReader
		testSecrets.segmentReaderAccessSet = prevSegmentReaderSet
		testSecrets.indexWriterAccess = prevIndexWriter
		testSecrets.indexWriterAccessSet = prevIndexWriterSet
		testSecrets.filterIndexInputAccess = prevFilterIndexInput
		testSecrets.filterIndexInputAccessSet = prevFilterIndexInputSet
		testSecrets.mu.Unlock()
	})
}
