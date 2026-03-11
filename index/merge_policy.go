// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// MergePolicy controls segment merging.
type MergePolicy interface {
	// FindMerges finds merges needed for the given segment infos.
	FindMerges(mergeTrigger MergeTrigger, infos *SegmentInfos) (*MergeSpecification, error)

	// FindForcedMerges finds forced merges (e.g., for optimizing the index).
	FindForcedMerges(infos *SegmentInfos, maxSegmentCount int) (*MergeSpecification, error)

	// UseCompoundFile returns true if segments should use compound files.
	UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool
}

// MergeTrigger indicates what triggered a merge.
type MergeTrigger int

const (
	// SEGMENT_FLUSH is triggered by segment flush.
	SEGMENT_FLUSH MergeTrigger = iota
	// CLOSED_WRITER is triggered by writer close.
	CLOSED_WRITER
	// EXPLICIT is triggered by explicit merge call.
	EXPLICIT
	// MERGE_FINISHED is triggered when a merge finishes.
	MERGE_FINISHED
	// COMMIT is triggered by commit.
	COMMIT
	// FULL_FLUSH is triggered by full flush.
	FULL_FLUSH
	// GET_READER is triggered by get reader.
	GET_READER
)

// MergeSpecification holds a set of merges to perform.
type MergeSpecification struct {
	Merges []*OneMerge
}

// OneMerge represents a single merge operation.
type OneMerge struct {
	Segments []*SegmentCommitInfo
}

// BaseMergePolicy provides common functionality.
type BaseMergePolicy struct{}

// FindMerges finds merges.
func (p *BaseMergePolicy) FindMerges(trigger MergeTrigger, infos *SegmentInfos) (*MergeSpecification, error) {
	return nil, nil
}

// FindForcedMerges finds forced merges.
func (p *BaseMergePolicy) FindForcedMerges(infos *SegmentInfos, maxSegmentCount int) (*MergeSpecification, error) {
	return nil, nil
}

// UseCompoundFile returns whether to use compound files.
func (p *BaseMergePolicy) UseCompoundFile(infos *SegmentInfos, mergedSegmentInfo *SegmentInfo) bool {
	return false
}
