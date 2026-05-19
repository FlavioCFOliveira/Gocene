// Package tests is the Go port of org.apache.lucene.internal.tests.
//
// It exposes a small registry (TestSecrets) of access objects that grant the
// Lucene test framework controlled visibility into package-private state
// declared inside production packages. Getters may only be invoked from the
// test-framework module (mirroring Java's StackWalker check), and each setter
// may be called at most once (mirroring Java's ensureNull guard).
//
// Source: core/src/java/org/apache/lucene/internal/tests/TestSecrets.java
//
//	(Apache Lucene 10.4.0, commit 9983b7c)
package tests

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
)

// testFrameworkPathPrefix is the canonical Go-side equivalent of Java's
// "org.apache.lucene.tests." caller restriction. Any function whose runtime
// frame belongs to a package under this import-path prefix is recognised as
// the Lucene test framework and is permitted to obtain accessors.
const testFrameworkPathPrefix = "github.com/FlavioCFOliveira/Gocene/lucenetests/"

// ErrTestFrameworkOnly is returned by getters when the immediate caller does
// not belong to the Lucene test-framework module. It mirrors the Java
// UnsupportedOperationException message exactly so tests can assert on the
// string.
var ErrTestFrameworkOnly = errors.New(
	"Lucene TestSecrets can only be used by the test-framework.",
)

// ErrAccessorAlreadySet is returned by setters when an accessor has already
// been registered. It mirrors the Java AssertionError message exactly.
var ErrAccessorAlreadySet = errors.New(
	"The accessor is already set. It can only be called from inside Lucene Core.",
)

// ConcurrentMergeSchedulerAccess is the test-only accessor surface for
// ConcurrentMergeScheduler internals. The full method set is intentionally
// deferred: the Java contract pulls in types from index/, which are still in
// flux. Production code must not depend on this interface.
type ConcurrentMergeSchedulerAccess interface {
	concurrentMergeSchedulerAccess()
}

// FilterIndexInputAccess is the test-only accessor surface for FilterIndexInput
// internals. See [ConcurrentMergeSchedulerAccess] for the deferred-signature
// rationale.
type FilterIndexInputAccess interface {
	filterIndexInputAccess()
}

// IndexPackageAccess is the test-only accessor surface for the index package.
// See [ConcurrentMergeSchedulerAccess] for the deferred-signature rationale.
type IndexPackageAccess interface {
	indexPackageAccess()
}

// IndexWriterAccess is the test-only accessor surface for IndexWriter
// internals. See [ConcurrentMergeSchedulerAccess] for the deferred-signature
// rationale.
type IndexWriterAccess interface {
	indexWriterAccess()
}

// SegmentReaderAccess is the test-only accessor surface for SegmentReader
// internals. See [ConcurrentMergeSchedulerAccess] for the deferred-signature
// rationale.
type SegmentReaderAccess interface {
	segmentReaderAccess()
}

// testSecretsRegistry holds the singleton accessor slots. Java keeps these as
// package-static fields; Go centralises them on a single struct guarded by a
// mutex so concurrent setter/getter calls are race-free. Each slot carries a
// dedicated "set" flag so the first invocation of a setter — even with a nil
// argument — latches the slot, mirroring Java's ensureNull guard which trips
// on the second call regardless of whether the value is null.
type testSecretsRegistry struct {
	mu                        sync.Mutex
	indexPackageAccess        IndexPackageAccess
	indexPackageAccessSet     bool
	cmsAccess                 ConcurrentMergeSchedulerAccess
	cmsAccessSet              bool
	segmentReaderAccess       SegmentReaderAccess
	segmentReaderAccessSet    bool
	indexWriterAccess         IndexWriterAccess
	indexWriterAccessSet      bool
	filterIndexInputAccess    FilterIndexInputAccess
	filterIndexInputAccessSet bool
}

var testSecrets = &testSecretsRegistry{}

// GetIndexPackageAccess returns the registered IndexPackageAccess. It returns
// [ErrTestFrameworkOnly] if invoked from outside the Lucene test-framework
// module.
func GetIndexPackageAccess() (IndexPackageAccess, error) {
	if err := ensureCaller(); err != nil {
		return nil, err
	}
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if !testSecrets.indexPackageAccessSet || testSecrets.indexPackageAccess == nil {
		return nil, errNilAccessor("IndexPackageAccess")
	}
	return testSecrets.indexPackageAccess, nil
}

// GetConcurrentMergeSchedulerAccess returns the registered
// ConcurrentMergeSchedulerAccess.
func GetConcurrentMergeSchedulerAccess() (ConcurrentMergeSchedulerAccess, error) {
	if err := ensureCaller(); err != nil {
		return nil, err
	}
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if !testSecrets.cmsAccessSet || testSecrets.cmsAccess == nil {
		return nil, errNilAccessor("ConcurrentMergeSchedulerAccess")
	}
	return testSecrets.cmsAccess, nil
}

// GetSegmentReaderAccess returns the registered SegmentReaderAccess.
func GetSegmentReaderAccess() (SegmentReaderAccess, error) {
	if err := ensureCaller(); err != nil {
		return nil, err
	}
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if !testSecrets.segmentReaderAccessSet || testSecrets.segmentReaderAccess == nil {
		return nil, errNilAccessor("SegmentReaderAccess")
	}
	return testSecrets.segmentReaderAccess, nil
}

// GetIndexWriterAccess returns the registered IndexWriterAccess.
func GetIndexWriterAccess() (IndexWriterAccess, error) {
	if err := ensureCaller(); err != nil {
		return nil, err
	}
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if !testSecrets.indexWriterAccessSet || testSecrets.indexWriterAccess == nil {
		return nil, errNilAccessor("IndexWriterAccess")
	}
	return testSecrets.indexWriterAccess, nil
}

// GetFilterInputIndexAccess returns the registered FilterIndexInputAccess.
// The misspelled "InputIndex" matches the Java method name verbatim.
func GetFilterInputIndexAccess() (FilterIndexInputAccess, error) {
	if err := ensureCaller(); err != nil {
		return nil, err
	}
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if !testSecrets.filterIndexInputAccessSet || testSecrets.filterIndexInputAccess == nil {
		return nil, errNilAccessor("FilterIndexInputAccess")
	}
	return testSecrets.filterIndexInputAccess, nil
}

// SetIndexWriterAccess registers the IndexWriterAccess accessor. It returns
// [ErrAccessorAlreadySet] if the slot has already been latched, even when the
// prior call registered a nil accessor.
func SetIndexWriterAccess(a IndexWriterAccess) error {
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if testSecrets.indexWriterAccessSet {
		return ErrAccessorAlreadySet
	}
	testSecrets.indexWriterAccess = a
	testSecrets.indexWriterAccessSet = true
	return nil
}

// SetIndexPackageAccess registers the IndexPackageAccess accessor.
func SetIndexPackageAccess(a IndexPackageAccess) error {
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if testSecrets.indexPackageAccessSet {
		return ErrAccessorAlreadySet
	}
	testSecrets.indexPackageAccess = a
	testSecrets.indexPackageAccessSet = true
	return nil
}

// SetConcurrentMergeSchedulerAccess registers the
// ConcurrentMergeSchedulerAccess accessor.
func SetConcurrentMergeSchedulerAccess(a ConcurrentMergeSchedulerAccess) error {
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if testSecrets.cmsAccessSet {
		return ErrAccessorAlreadySet
	}
	testSecrets.cmsAccess = a
	testSecrets.cmsAccessSet = true
	return nil
}

// SetSegmentReaderAccess registers the SegmentReaderAccess accessor.
func SetSegmentReaderAccess(a SegmentReaderAccess) error {
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if testSecrets.segmentReaderAccessSet {
		return ErrAccessorAlreadySet
	}
	testSecrets.segmentReaderAccess = a
	testSecrets.segmentReaderAccessSet = true
	return nil
}

// SetFilterInputIndexAccess registers the FilterIndexInputAccess accessor.
// The misspelled "InputIndex" matches the Java method name verbatim.
func SetFilterInputIndexAccess(a FilterIndexInputAccess) error {
	testSecrets.mu.Lock()
	defer testSecrets.mu.Unlock()
	if testSecrets.filterIndexInputAccessSet {
		return ErrAccessorAlreadySet
	}
	testSecrets.filterIndexInputAccess = a
	testSecrets.filterIndexInputAccessSet = true
	return nil
}

// ensureCaller mirrors the Java StackWalker check: the immediate caller of the
// getter (one frame above ensureCaller, since the getter itself sits between
// them) must belong to a package under [testFrameworkPathPrefix]. Test
// binaries built against this package satisfy the check because Go test files
// compile into the package under test; the runtime function name therefore
// preserves the original import path.
func ensureCaller() error {
	// Frame 0: runtime.Callers. Frame 1: ensureCaller. Frame 2: the getter
	// (e.g. GetIndexWriterAccess). Frame 3: the caller we need to inspect.
	var pcs [4]uintptr
	n := runtime.Callers(0, pcs[:])
	if n < 4 {
		return ErrTestFrameworkOnly
	}
	frames := runtime.CallersFrames(pcs[:n])
	// Advance past the three known frames.
	for i := 0; i < 3; i++ {
		_, more := frames.Next()
		if !more {
			return ErrTestFrameworkOnly
		}
	}
	callerFrame, _ := frames.Next()
	if !strings.HasPrefix(callerFrame.Function, testFrameworkPathPrefix) {
		return ErrTestFrameworkOnly
	}
	return nil
}

// errNilAccessor mirrors Java's Objects.requireNonNull(...) NullPointerException
// surface using a descriptive Go error.
func errNilAccessor(name string) error {
	return fmt.Errorf("tests: %s accessor is not registered", name)
}
