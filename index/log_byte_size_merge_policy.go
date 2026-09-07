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
	DefaultMinMergeMB = 16.0
	// DefaultMaxMergeMB is the default maximum segment size.
	DefaultMaxMergeMB = 2048.0
	// DefaultMaxMergeMBForForcedMerge is the default maximum segment size for forced merges.
	DefaultMaxMergeMBForForcedMerge = float64(math.MaxInt64)
)

// LogByteSizeMergePolicy is a LogMergePolicy that measures size of a segment
// as the total byte size of the segment's files.
//
// This is the Go port of org.apache.lucene.index.LogByteSizeMergePolicy.
type LogByteSizeMergePolicy struct {
	*LogMergePolicy
}

// NewLogByteSizeMergePolicy creates a new LogByteSizeMergePolicy with default settings.
func NewLogByteSizeMergePolicy() *LogByteSizeMergePolicy {
	p := NewLogMergePolicy()
	p.SetMinMergeMB(DefaultMinMergeMB)
	p.SetMaxMergeMB(DefaultMaxMergeMB)
	p.SetMaxMergeMBForForcedMerge(DefaultMaxMergeMBForForcedMerge)
	return &LogByteSizeMergePolicy{
		LogMergePolicy: p,
	}
}

// SetMaxMergeMB determines the largest segment (measured by total byte size of the segment's files, in MB)
// that may be merged with other segments.
func (p *LogByteSizeMergePolicy) SetMaxMergeMB(mb float64) {
	p.LogMergePolicy.SetMaxMergeMB(mb)
}

// GetMaxMergeMB returns the largest segment (measured by total byte size of the segment's files, in MB)
// that may be merged with other segments.
func (p *LogByteSizeMergePolicy) GetMaxMergeMB() float64 {
	return p.LogMergePolicy.GetMaxMergeMB()
}

// SetMaxMergeMBForForcedMerge determines the largest segment (measured by total byte size of the segment's files, in MB)
// that may be merged with other segments during forceMerge.
func (p *LogByteSizeMergePolicy) SetMaxMergeMBForForcedMerge(mb float64) {
	p.LogMergePolicy.SetMaxMergeMBForForcedMerge(mb)
}

// GetMaxMergeMBForForcedMerge returns the largest segment (measured by total byte size of the segment's files, in MB)
// that may be merged with other segments during forceMerge.
func (p *LogByteSizeMergePolicy) GetMaxMergeMBForForcedMerge() float64 {
	return p.LogMergePolicy.GetMaxMergeMBForForcedMerge()
}

// SetMinMergeMB sets the minimum size for the lowest level segments.
func (p *LogByteSizeMergePolicy) SetMinMergeMB(mb float64) {
	p.LogMergePolicy.SetMinMergeMB(mb)
}

// GetMinMergeMB returns the minimum size for a segment to remain un-merged.
func (p *LogByteSizeMergePolicy) GetMinMergeMB() float64 {
	return p.LogMergePolicy.GetMinMergeMB()
}

// String returns a string representation of the policy.
func (p *LogByteSizeMergePolicy) String() string {
	return fmt.Sprintf("[LogByteSizeMergePolicy: minMergeMB=%.1f, maxMergeMB=%.1f, mergeFactor=%d, maxMergeDocs=%d]",
		p.GetMinMergeMB(), p.GetMaxMergeMB(), p.GetMergeFactor(), p.GetMaxMergeDocs())
}
