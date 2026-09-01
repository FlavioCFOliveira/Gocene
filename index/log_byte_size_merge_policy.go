// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
)

const (
	// DefaultMinMergeMB is the default minimum segment size.
	DefaultMinMergeMB = 16.0
	// DefaultMaxMergeMB is the default maximum segment size.
	DefaultMaxMergeMB = 2048.0
	// DefaultMaxMergeMBForForcedMerge is the default maximum segment size for forced merge.
	DefaultMaxMergeMBForForcedMerge = -1.0 // Use -1 to represent Long.MAX_VALUE
)

// LogByteSizeMergePolicy is a LogMergePolicy that measures size of a segment as the total byte size of the segment's files.
// Mirrors org.apache.lucene.index.LogByteSizeMergePolicy from Apache Lucene 10.5.0.
type LogByteSizeMergePolicy struct {
	LogMergePolicy
}

// NewLogByteSizeMergePolicy constructs a LogByteSizeMergePolicy with default settings.
func NewLogByteSizeMergePolicy() *LogByteSizeMergePolicy {
	p := &LogByteSizeMergePolicy{}
	p.SetMinMergeMB(DefaultMinMergeMB)
	p.SetMaxMergeMB(DefaultMaxMergeMB)
	p.SetMaxMergeMBForForcedMerge(DefaultMaxMergeMBForForcedMerge)
	return p
}

func (p *LogByteSizeMergePolicy) size(info *SegmentCommitInfo, ctx MergeContext) (int64, error) {
	return sizeBytes(info, ctx)
}

func (p *LogByteSizeMergePolicy) SetMaxMergeMB(mb float64) {
	p.maxMergeSize = int64(mb * 1024 * 1024)
}

func (p *LogByteSizeMergePolicy) GetMaxMergeMB() float64 {
	return float64(p.maxMergeSize) / 1024 / 1024
}

func (p *LogByteSizeMergePolicy) SetMaxMergeMBForForcedMerge(mb float64) {
	p.maxMergeSizeForForcedMerge = int64(mb * 1024 * 1024)
}

func (p *LogByteSizeMergePolicy) GetMaxMergeMBForForcedMerge() float64 {
	return float64(p.maxMergeSizeForForcedMerge) / 1024 / 1024
}

func (p *LogByteSizeMergePolicy) SetMinMergeMB(mb float64) {
	p.minMergeSize = int64(mb * 1024 * 1024)
}

func (p *LogByteSizeMergePolicy) GetMinMergeMB() float64 {
	return float64(p.minMergeSize) / 1024 / 1024
}

func (p *LogByteSizeMergePolicy) String() string {
	return fmt.Sprintf("LogByteSizeMergePolicy(min=%f MB, max=%f MB)", p.GetMinMergeMB(), p.GetMaxMergeMB())
}
