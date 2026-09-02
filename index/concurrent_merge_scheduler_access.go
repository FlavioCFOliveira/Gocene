package index

// ConcurrentMergeSchedulerAccess is the test-only accessor surface for
// ConcurrentMergeScheduler internals.
//
// Source: core/src/java/org/apache/lucene/internal/tests/ConcurrentMergeSchedulerAccess.java
// (Apache Lucene 10.4.0, commit 9983b7c)
type ConcurrentMergeSchedulerAccess interface {
	SetSuppressExceptions(cms *ConcurrentMergeScheduler)
}
