// Package tests stubs the org.apache.lucene.internal.tests access surface
// that production code is forbidden from touching. The Go port surfaces the
// type names so any code that referenced them at compile time keeps building;
// test-only behaviour lives in dedicated _test.go files elsewhere.
package tests

// ConcurrentMergeSchedulerAccess is the test-only accessor for
// ConcurrentMergeScheduler internals.
type ConcurrentMergeSchedulerAccess struct{}

// FilterIndexInputAccess is the test-only accessor for FilterIndexInput
// internals.
type FilterIndexInputAccess struct{}

// IndexPackageAccess is the test-only accessor for the index package.
type IndexPackageAccess struct{}

// IndexWriterAccess is the test-only accessor for IndexWriter internals.
type IndexWriterAccess struct{}

// SegmentReaderAccess is the test-only accessor for SegmentReader internals.
type SegmentReaderAccess struct{}

// TestSecrets holds the Java-side "Lookup" registry that exposes the access
// objects above. In Go test code can reach internals directly so the
// registry is intentionally empty.
type TestSecrets struct{}
