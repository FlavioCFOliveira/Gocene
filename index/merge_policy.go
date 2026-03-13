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
	// The mergeContext provides information about currently merging segments and delete counts.
	FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error)

	// FindForcedMerges finds forced merges (e.g., for optimizing the index).
	// This is used to reduce the number of segments to maxSegmentCount or fewer.
	// segmentsToMerge maps segments to whether they must be merged (true for original segments).
	FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error)

	// FindForcedDeletesMerges finds merges necessary to expunge deleted documents.
	FindForcedDeletesMerges(infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error)

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

	// NumDeletesToMerge returns the number of deletes that a merge would claim on the given segment.
	// This method will by default return the sum of the del count on disk and the pending delete count.
	NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int

	// KeepFullyDeletedSegment returns true if the segment should be kept even if fully deleted.
	KeepFullyDeletedSegment(info *SegmentCommitInfo) bool
}

// MergeTrigger indicates what triggered a merge.
// This is the Go port of Lucene's org.apache.lucene.index.MergeTrigger.
type MergeTrigger int

const (
	// SEGMENT_FLUSH is triggered by a segment flush.
	SEGMENT_FLUSH MergeTrigger = iota
	// FULL_FLUSH is triggered by a full flush. Full flushes can be caused by a commit,
	// NRT reader reopen or a close call on the index writer.
	FULL_FLUSH
	// EXPLICIT is triggered explicitly by the user.
	EXPLICIT
	// MERGE_FINISHED is triggered by a successfully finished merge.
	MERGE_FINISHED
	// CLOSING is triggered by a closing IndexWriter.
	CLOSING
	// COMMIT is triggered on commit.
	COMMIT
	// GET_READER is triggered on opening NRT readers.
	GET_READER
	// ADD_INDEXES is triggered by an IndexWriter.addIndexes operation.
	ADD_INDEXES
	// CLOSED_WRITER is deprecated: use CLOSING instead.
	// This is kept for backward compatibility.
	CLOSED_WRITER
)

// String returns the string representation of the MergeTrigger.
func (t MergeTrigger) String() string {
	switch t {
	case SEGMENT_FLUSH:
		return "SEGMENT_FLUSH"
	case FULL_FLUSH:
		return "FULL_FLUSH"
	case EXPLICIT:
		return "EXPLICIT"
	case MERGE_FINISHED:
		return "MERGE_FINISHED"
	case CLOSING:
		return "CLOSING"
	case COMMIT:
		return "COMMIT"
	case GET_READER:
		return "GET_READER"
	case ADD_INDEXES:
		return "ADD_INDEXES"
	case CLOSED_WRITER:
		return "CLOSED_WRITER"
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

	// EstimatedMergeBytes is the estimated size in bytes of the merged segment.
	EstimatedMergeBytes int64

	// TotalMergeBytes is the sum of sizeInBytes of all SegmentInfos.
	TotalMergeBytes int64

	// Info is the resulting segment info (set after merge).
	Info *SegmentCommitInfo

	// RegisterDone is true if the merge was registered with the IndexWriter.
	RegisterDone bool

	// MergeGen is the generation number for this merge.
	MergeGen int64

	// IsExternal is true if this is an external merge (from addIndexes).
	IsExternal bool

	// MaxNumSegments is the max number of segments for forced merges (-1 for non-forced).
	MaxNumSegments int

	// UsesPooledReaders is true if pooled readers are used.
	UsesPooledReaders bool

	// MergeStartNS is the start time of the merge in nanoseconds.
	MergeStartNS int64

	// Progress controls pause/stop/resume for the merge thread.
	Progress *OneMergeProgress

	// Error holds any error that occurred during the merge.
	Error error
}

// NewOneMerge creates a new OneMerge.
func NewOneMerge(segments []*SegmentCommitInfo) *OneMerge {
	if len(segments) == 0 {
		// Allow empty segments list for addIndexes operations
	}

	merge := &OneMerge{
		Segments:         segments,
		TotalDocCount:    0,
		TotalNumDocs:     0,
		MaxNumSegments:   -1,
		Progress:         NewOneMergeProgress(),
		UsesPooledReaders: true,
	}

	// Calculate totals
	for _, seg := range segments {
		merge.TotalDocCount += seg.DocCount()
		merge.TotalNumDocs += seg.NumDocs()
	}

	return merge
}

// NewOneMergeFromReaders creates a OneMerge from CodecReaders.
// Used for addIndexes operations.
func NewOneMergeFromReaders() *OneMerge {
	return &OneMerge{
		Segments:          make([]*SegmentCommitInfo, 0),
		TotalDocCount:     0,
		TotalNumDocs:      0,
		MaxNumSegments:    -1,
		Progress:          NewOneMergeProgress(),
		UsesPooledReaders: true,
	}
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
	if om.EstimatedMergeBytes > 0 {
		return om.EstimatedMergeBytes
	}

	var total int64
	for _, seg := range om.Segments {
		total += seg.SegmentInfo().SizeInBytes()
	}
	om.EstimatedMergeBytes = total
	return total
}

// Abort aborts the merge at the next possible moment.
func (om *OneMerge) Abort() {
	if om.Progress != nil {
		om.Progress.Abort()
	}
}

// IsAborted returns true if the merge has been aborted.
func (om *OneMerge) IsAborted() bool {
	if om.Progress == nil {
		return false
	}
	return om.Progress.IsAborted()
}

// CheckAborted returns an error if the merge has been aborted.
func (om *OneMerge) CheckAborted() error {
	if om.Progress == nil {
		return nil
	}
	return om.Progress.CheckAborted()
}

// GetProgress returns the merge progress.
func (om *OneMerge) GetProgress() *OneMergeProgress {
	return om.Progress
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
func (p *BaseMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	return nil, fmt.Errorf("FindMerges not implemented")
}

// FindForcedMerges finds forced merges (must be implemented by subclasses).
func (p *BaseMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int, segmentsToMerge map[*SegmentCommitInfo]bool, mergeContext MergeContext) (*MergeSpecification, error) {
	return nil, fmt.Errorf("FindForcedMerges not implemented")
}

// FindForcedDeletesMerges finds forced deletes merges (must be implemented by subclasses).
func (p *BaseMergePolicy) FindForcedDeletesMerges(infos *SegmentInfos, mergeContext MergeContext) (*MergeSpecification, error) {
	return nil, fmt.Errorf("FindForcedDeletesMerges not implemented")
}

// NumDeletesToMerge returns the number of deletes that a merge would claim on the given segment.
// By default, this returns the delCount unchanged.
func (p *BaseMergePolicy) NumDeletesToMerge(info *SegmentCommitInfo, delCount int) int {
	return delCount
}

// KeepFullyDeletedSegment returns false by default (don't keep fully deleted segments).
func (p *BaseMergePolicy) KeepFullyDeletedSegment(info *SegmentCommitInfo) bool {
	return false
}

// Size returns the byte size of the segment, pro-rated by percentage of non-deleted documents.
// This is the Go port of Lucene's MergePolicy.size().
func (p *BaseMergePolicy) Size(info *SegmentCommitInfo, mergeContext MergeContext) int64 {
	byteSize := info.SegmentInfo().SizeInBytes()
	delCount := mergeContext.NumDeletesToMerge(info)
	maxDoc := info.SegmentInfo().DocCount()

	if maxDoc <= 0 {
		return byteSize
	}

	delRatio := float64(delCount) / float64(maxDoc)
	if delRatio > 1.0 {
		delRatio = 1.0
	}

	return int64(float64(byteSize) * (1.0 - delRatio))
}

// IsMerged returns true if this segment is already fully merged (has no pending deletes,
// is in the same directory as the writer, and matches the current compound file setting).
func (p *BaseMergePolicy) IsMerged(infos *SegmentInfos, info *SegmentCommitInfo, mergeContext MergeContext) bool {
	if mergeContext == nil {
		return false
	}
	delCount := mergeContext.NumDeletesToMerge(info)
	return delCount == 0
}

// Message prints a debug message to the info stream if enabled.
func (p *BaseMergePolicy) Message(message string, mergeContext MergeContext) {
	if mergeContext != nil {
		if infoStream := mergeContext.GetInfoStream(); infoStream != nil {
			if infoStream.IsEnabled("MP") {
				infoStream.Message("MP", message)
			}
		}
	}
}

// Verbose returns true if the info stream is in verbose mode.
func (p *BaseMergePolicy) Verbose(mergeContext MergeContext) bool {
	if mergeContext == nil {
		return false
	}
	infoStream := mergeContext.GetInfoStream()
	if infoStream == nil {
		return false
	}
	return infoStream.IsEnabled("MP")
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
