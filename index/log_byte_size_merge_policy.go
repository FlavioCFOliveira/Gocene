// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
)

const (
	// DefaultMinMergeMB is the default minimum segment size.
	// Mirrors LogByteSizeMergePolicy.DEFAULT_MIN_MERGE_MB.
	DefaultMinMergeMB = 16.0
	// DefaultMaxMergeMB is the default maximum segment size. A segment of
	// this size or larger will never be merged.
	// Mirrors LogByteSizeMergePolicy.DEFAULT_MAX_MERGE_MB.
	DefaultMaxMergeMB = 2048.0
	// DefaultMaxMergeMBForForcedMerge is the default maximum segment size for
	// forced merges.
	// Mirrors LogByteSizeMergePolicy.DEFAULT_MAX_MERGE_MB_FOR_FORCED_MERGE.
	DefaultMaxMergeMBForForcedMerge = float64(math.MaxInt64)
)

// LogByteSizeMergePolicy is a LogMergePolicy that measures size of a segment
// as the total byte size of the segment's files.
//
// This is the Go port of org.apache.lucene.index.LogByteSizeMergePolicy.
type LogByteSizeMergePolicy struct {
	*LogMergePolicy
}

// megabytesToBytes converts a size in megabytes to a size in bytes with the
// same saturation behaviour as Java's narrowing (long) cast of a double: a
// value above math.MaxInt64 saturates to math.MaxInt64, a value below
// math.MinInt64 saturates to math.MinInt64, and NaN becomes 0. Lucene relies
// on this explicitly for DEFAULT_MAX_MERGE_MB_FOR_FORCED_MERGE (see the note
// in LogByteSizeMergePolicy's constructor).
func megabytesToBytes(mb float64) int64 {
	bytes := mb * 1024 * 1024
	switch {
	case math.IsNaN(bytes):
		return 0
	case bytes >= math.MaxInt64:
		return math.MaxInt64
	case bytes <= math.MinInt64:
		return math.MinInt64
	default:
		return int64(bytes)
	}
}

// NewLogByteSizeMergePolicy creates a new LogByteSizeMergePolicy with default settings.
func NewLogByteSizeMergePolicy() *LogByteSizeMergePolicy {
	p := NewLogMergePolicy()
	p.minMergeSize = megabytesToBytes(DefaultMinMergeMB)
	p.maxMergeSize = megabytesToBytes(DefaultMaxMergeMB)
	// NOTE: in Java, casting a too-large double to long yields Long.MAX_VALUE;
	// megabytesToBytes reproduces that saturation.
	p.maxMergeSizeForForcedMerge = megabytesToBytes(DefaultMaxMergeMBForForcedMerge)
	// Lucene overrides size(SegmentCommitInfo, MergeContext) to return
	// sizeBytes; Gocene expresses the override through the size calculator
	// hook LogMergePolicy consults from Size.
	p.sizeCalculator = p.sizeBytes
	return &LogByteSizeMergePolicy{
		LogMergePolicy: p,
	}
}

// SetMaxMergeMB determines the largest segment (measured by total byte size of
// the segment's files, in MB) that may be merged with other segments. Small
// values (e.g., less than 50 MB) are best for interactive indexing, as this
// limits the length of pauses while indexing to a few seconds. Larger values
// are best for batched indexing and speedier searches.
//
// Note that SetMaxMergeDocs is also used to check whether a segment is too
// large for merging (it is either or).
func (p *LogByteSizeMergePolicy) SetMaxMergeMB(mb float64) {
	p.LogMergePolicy.maxMergeSize = megabytesToBytes(mb)
}

// GetMaxMergeMB returns the largest segment (measured by total byte size of the segment's files, in MB)
// that may be merged with other segments.
func (p *LogByteSizeMergePolicy) GetMaxMergeMB() float64 {
	return float64(p.LogMergePolicy.maxMergeSize) / 1024 / 1024
}

// SetMaxMergeMBForForcedMerge determines the largest segment (measured by total
// byte size of the segment's files, in MB) that may be merged with other
// segments during forceMerge. Setting it low will leave the index with more
// than one segment, even if ForceMerge is called.
func (p *LogByteSizeMergePolicy) SetMaxMergeMBForForcedMerge(mb float64) {
	p.LogMergePolicy.maxMergeSizeForForcedMerge = megabytesToBytes(mb)
}

// GetMaxMergeMBForForcedMerge returns the largest segment (measured by total byte size of the segment's files, in MB)
// that may be merged with other segments during forceMerge.
func (p *LogByteSizeMergePolicy) GetMaxMergeMBForForcedMerge() float64 {
	return float64(p.LogMergePolicy.maxMergeSizeForForcedMerge) / 1024 / 1024
}

// SetMinMergeMB sets the minimum size for the lowest level segments. Any
// segments below this size are candidates for full-flush merges and are merged
// more aggressively in order to avoid a long tail of small segments.
func (p *LogByteSizeMergePolicy) SetMinMergeMB(mb float64) {
	p.LogMergePolicy.minMergeSize = megabytesToBytes(mb)
}

// GetMinMergeMB returns the minimum size for a segment to remain un-merged.
func (p *LogByteSizeMergePolicy) GetMinMergeMB() float64 {
	return float64(p.LogMergePolicy.minMergeSize) / 1024 / 1024
}

// String returns a string representation of the policy.
func (p *LogByteSizeMergePolicy) String() string {
	return fmt.Sprintf("[LogByteSizeMergePolicy: minMergeMB=%.1f, maxMergeMB=%.1f, mergeFactor=%d, maxMergeDocs=%d]",
		p.GetMinMergeMB(), p.GetMaxMergeMB(), p.GetMergeFactor(), p.GetMaxMergeDocs())
}
