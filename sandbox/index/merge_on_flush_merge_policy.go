// Package index implements org.apache.lucene.sandbox.index.
package index

// MergeOnFlushMergePolicy is the merge policy that runs a merge concurrently
// with every flush. Mirrors
// org.apache.lucene.sandbox.index.MergeOnFlushMergePolicy.
type MergeOnFlushMergePolicy struct {
	MaxSmallSegmentSize int64
}

// NewMergeOnFlushMergePolicy builds the policy.
func NewMergeOnFlushMergePolicy(maxSmallSegmentSize int64) *MergeOnFlushMergePolicy {
	if maxSmallSegmentSize < 1 {
		maxSmallSegmentSize = 50 * 1024 * 1024
	}
	return &MergeOnFlushMergePolicy{MaxSmallSegmentSize: maxSmallSegmentSize}
}
