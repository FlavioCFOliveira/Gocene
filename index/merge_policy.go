// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"math"
)

// MergePolicy determines when and how merges should be performed.
// This is the Go port of Lucene's org.apache.lucene.index.MergePolicy.
//
// MergePolicy defines how merges are selected and when they should be executed.
// The two main implementations are:
//   - TieredMergePolicy: Groups segments by size into tiers, merging similar-sized segments
//   - LogMergePolicy: Merges the smallest segments (legacy, less efficient)
type MergePolicy interface {
	// FindMerges finds merges needed for the given segment infos.
	// Returns a MergeSpecification containing the merges to perform, or nil if no merges are needed.
	FindMerges(trigger MergeTrigger, infos *SegmentInfos) (*MergeSpecification, error)

	// FindForcedMerges finds forced merges (e.g., for optimizing the index).
	// This is used to reduce the number of segments to maxSegmentCount or fewer.
	FindForcedMerges(infos *SegmentInfos, maxSegmentCount int) (*MergeSpecification, error)

	// FindForcedDeletesMerges finds merges necessary to expunge deleted documents.
	FindForcedDeletesMerges(infos *SegmentInfos) (*MergeSpecification, error)

	// UseCompoundFile returns true if segments should use compound files.
	// Compound files pack all segment files into a single .cfs/.cfe file pair.
	UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool

	// GetMaxMergeDocs returns the maximum number of documents that can be merged.
	GetMaxMergeDocs() int

	// SetMaxMergeDocs sets the maximum number of documents that can be merged.
	SetMaxMergeDocs(maxMergeDocs int)

	// GetMaxMergedSegmentBytes returns the maximum size of a merged segment in bytes.
	GetMaxMergedSegmentBytes() int64

	// SetMaxMergedSegmentBytes sets the maximum size of a merged segment in bytes.
	SetMaxMergedSegmentBytes(maxMergedSegmentBytes int64)
}

// MergeTrigger indicates what triggered a merge.
type MergeTrigger int

const (
	// SEGMENT_FLUSH is triggered by segment flush (adding documents).
	SEGMENT_FLUSH MergeTrigger = iota
	// CLOSED_WRITER is triggered by writer close.
	CLOSED_WRITER
	// EXPLICIT is triggered by explicit merge call from user.
	EXPLICIT
	// MERGE_FINISHED is triggered when a merge finishes.
	MERGE_FINISHED
	// COMMIT is triggered by commit.
	COMMIT
	// FULL_FLUSH is triggered by full flush.
	FULL_FLUSH
	// GET_READER is triggered by get reader (NRT - near real-time).
	GET_READER
)

// String returns the string representation of the MergeTrigger.
func (t MergeTrigger) String() string {
	switch t {
	case SEGMENT_FLUSH:
		return "SEGMENT_FLUSH"
	case CLOSED_WRITER:
		return "CLOSED_WRITER"
	case EXPLICIT:
		return "EXPLICIT"
	case MERGE_FINISHED:
		return "MERGE_FINISHED"
	case COMMIT:
		return "COMMIT"
	case FULL_FLUSH:
		return "FULL_FLUSH"
	case GET_READER:
		return "GET_READER"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", t)
	}
}

// MergeSpecification holds a set of merges to perform.
// This is the Go port of Lucene's org.apache.lucene.index.MergePolicy.MergeSpecification.
type MergeSpecification struct {
	// Merges is the list of merges to perform.
	Merges []*OneMerge
}

// NewMergeSpecification creates a new MergeSpecification.
func NewMergeSpecification() *MergeSpecification {
	return &MergeSpecification{
		Merges: make([]*OneMerge, 0),
	}
}

// Add adds a merge to the specification.
func (ms *MergeSpecification) Add(merge *OneMerge) {
	ms.Merges = append(ms.Merges, merge)
}

// Size returns the number of merges in the specification.
func (ms *MergeSpecification) Size() int {
	return len(ms.Merges)
}

// String returns a string representation of the MergeSpecification.
func (ms *MergeSpecification) String() string {
	return fmt.Sprintf("MergeSpecification(merges=%d)", len(ms.Merges))
}

// OneMerge represents a single merge operation.
// This is the Go port of Lucene's org.apache.lucene.index.MergePolicy.OneMerge.
type OneMerge struct {
	// Segments are the segments to be merged.
	Segments []*SegmentCommitInfo

	// MaxNumDocs is the maximum number of documents in the merged segment.
	MaxNumDocs int

	// TotalDocCount is the total document count (including deleted docs).
	TotalDocCount int

	// TotalNumDocs is the total live document count.
	TotalNumDocs int
}

// NewOneMerge creates a new OneMerge.
func NewOneMerge(segments []*SegmentCommitInfo) *OneMerge {
	merge := &OneMerge{
		Segments:      segments,
		TotalDocCount: 0,
		TotalNumDocs:  0,
	}

	// Calculate totals
	for _, seg := range segments {
		merge.TotalDocCount += seg.DocCount()
		merge.TotalNumDocs += seg.NumDocs()
	}

	return merge
}

// String returns a string representation of the OneMerge.
func (om *OneMerge) String() string {
	return fmt.Sprintf("OneMerge(segments=%d, totalDocs=%d)", len(om.Segments), om.TotalDocCount)
}

// SegmentsSize returns the number of segments in this merge.
func (om *OneMerge) SegmentsSize() int {
	return len(om.Segments)
}

// EstimateMergeBytes estimates the total size of the merge in bytes.
func (om *OneMerge) EstimateMergeBytes() int64 {
	var total int64
	for _, seg := range om.Segments {
		total += seg.SegmentInfo().SizeInBytes()
	}
	return total
}

// BaseMergePolicy provides common functionality for merge policies.
// This is the Go port of Lucene's org.apache.lucene.index.MergePolicy.BaseMergePolicy.
type BaseMergePolicy struct {
	maxMergeDocs          int
	maxMergedSegmentBytes int64
}

// NewBaseMergePolicy creates a new BaseMergePolicy.
func NewBaseMergePolicy() *BaseMergePolicy {
	return &BaseMergePolicy{
		maxMergeDocs:          math.MaxInt32,
		maxMergedSegmentBytes: 5 * 1024 * 1024 * 1024, // 5GB default
	}
}

// GetMaxMergeDocs returns the maximum number of documents that can be merged.
func (p *BaseMergePolicy) GetMaxMergeDocs() int {
	return p.maxMergeDocs
}

// SetMaxMergeDocs sets the maximum number of documents that can be merged.
func (p *BaseMergePolicy) SetMaxMergeDocs(maxMergeDocs int) {
	p.maxMergeDocs = maxMergeDocs
}

// GetMaxMergedSegmentBytes returns the maximum size of a merged segment in bytes.
func (p *BaseMergePolicy) GetMaxMergedSegmentBytes() int64 {
	return p.maxMergedSegmentBytes
}

// SetMaxMergedSegmentBytes sets the maximum size of a merged segment in bytes.
func (p *BaseMergePolicy) SetMaxMergedSegmentBytes(maxMergedSegmentBytes int64) {
	p.maxMergedSegmentBytes = maxMergedSegmentBytes
}

// FindMerges finds merges (must be implemented by subclasses).
func (p *BaseMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos) (*MergeSpecification, error) {
	return nil, fmt.Errorf("FindMerges not implemented")
}

// FindForcedMerges finds forced merges (must be implemented by subclasses).
func (p *BaseMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int) (*MergeSpecification, error) {
	return nil, fmt.Errorf("FindForcedMerges not implemented")
}

// FindForcedDeletesMerges finds forced deletes merges (must be implemented by subclasses).
func (p *BaseMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos) (*MergeSpecification, error) {
	return nil, fmt.Errorf("FindForcedDeletesMerges not implemented")
}

// UseCompoundFile returns whether to use compound files (default: false).
func (p *BaseMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	return false
}

// sizeToMB converts bytes to megabytes as int64.
func sizeToMB(bytes int64) int64 {
	return bytes / (1024 * 1024)
}

// mbToBytes converts megabytes to bytes.
func mbToBytes(mb int64) int64 {
	return mb * 1024 * 1024
}

// MergePolicyConfig holds configuration for merge policies.
type MergePolicyConfig struct {
	// MaxMergeAtOnce is the maximum number of segments to merge at once.
	MaxMergeAtOnce int

	// MaxMergeAtOnceExplicit is the maximum number of segments to merge at once for explicit merges.
	MaxMergeAtOnceExplicit int

	// MaxMergedSegmentMB is the maximum size of a merged segment in MB.
	MaxMergedSegmentMB int64

	// FloorSegmentMB is the minimum segment size to consider for merging in MB.
	FloorSegmentMB int64

	// MaxMergeDocs is the maximum number of documents to merge.
	MaxMergeDocs int

	// NoCFSRatio is the ratio of non-compound file segments.
	NoCFSRatio float64
}

// DefaultMergePolicyConfig returns a default MergePolicyConfig.
func DefaultMergePolicyConfig() MergePolicyConfig {
	return MergePolicyConfig{
		MaxMergeAtOnce:         10,
		MaxMergeAtOnceExplicit: 30,
		MaxMergedSegmentMB:     5120, // 5GB
		FloorSegmentMB:         2,
		MaxMergeDocs:           math.MaxInt32,
		NoCFSRatio:             0.0,
	}
}
