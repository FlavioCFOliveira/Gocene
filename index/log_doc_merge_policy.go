//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
)

// DefaultMinMergeDocs is the default minimum segment size for LogDocMergePolicy.
const DefaultMinMergeDocs = 1000

// LogDocMergePolicy merges segments based on their document count.
// This is the Go port of Lucene's org.apache.lucene.index.LogDocMergePolicy.
//
// This implements GC-638: LogDocMergePolicy
type LogDocMergePolicy struct {
	*LogMergePolicy
}

// NewLogDocMergePolicy creates a new LogDocMergePolicy.
// This implements GC-638: LogDocMergePolicy constructor
func NewLogDocMergePolicy() *LogDocMergePolicy {
	base := NewLogMergePolicy()
	// LogDocMergePolicy measures segment size in documents, not bytes.
	base.minMergeSize = DefaultMinMergeDocs
	// Byte-size caps are irrelevant for a document-count policy.
	base.maxMergeSize = math.MaxInt64
	base.maxMergeSizeForForcedMerge = math.MaxInt64
	base.sizeCalculator = func(info *SegmentCommitInfo, mergeContext MergeContext) int64 {
		docCount := int64(info.SegmentInfo().DocCount())
		if base.calibrateSizeByDeletes && mergeContext != nil {
			delCount := mergeContext.NumDeletesToMerge(info)
			if docCount > 0 && delCount > 0 {
				delRatio := float64(delCount) / float64(docCount)
				if delRatio > 1.0 {
					delRatio = 1.0
				}
				docCount = int64(float64(docCount) * (1.0 - delRatio))
			}
		}
		return docCount
	}
	return &LogDocMergePolicy{
		LogMergePolicy: base,
	}
}

// GetMinMergeDocs returns the minimum segment document count for the lowest
// level of merges.
func (p *LogDocMergePolicy) GetMinMergeDocs() int {
	return int(p.minMergeSize)
}

// SetMinMergeDocs sets the minimum segment document count for the lowest
// level of merges. Segments with fewer docs than this value are treated as
// candidates for full-flush merges.
func (p *LogDocMergePolicy) SetMinMergeDocs(v int) {
	p.minMergeSize = int64(v)
}

// String returns a string representation of the policy.
func (p *LogDocMergePolicy) String() string {
	return fmt.Sprintf("[LogDocMergePolicy: mergeFactor=%d, maxMergeDocs=%d]",
		p.GetMergeFactor(), p.GetMaxMergeDocs())
}

// Ensure interface is implemented
var _ MergePolicy = (*LogDocMergePolicy)(nil)
