// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/schema"
)

// SegmentCommitInfo wraps SegmentInfo with commit-specific metadata.
// This is the Go port of Lucene's org.apache.lucene.index.SegmentCommitInfo.
//
// SegmentCommitInfo tracks information about a segment at a specific commit
// point, including deletion information and field infos generation.
//
// Lifted from package index to package spi by rmp #4706 so that the canonical
// SegmentInfosFormat interface (and the on-disk segments_N reader/writer)
// no longer require a back-edge into package index.
type SegmentCommitInfo struct {
	// segmentInfo is the wrapped SegmentInfo
	segmentInfo *schema.SegmentInfo

	// delCount is the number of deleted documents in this segment
	delCount int

	// softDelCount is the number of soft-deleted documents
	softDelCount int

	// delGen is the deletion file generation
	// -1 means no deletions
	// >= 0 means deletions exist
	delGen int64

	// fieldInfosGen is the field infos file generation
	// -1 means no separate field infos
	// >= 0 means field infos exist
	fieldInfosGen int64

	// docValuesGen is the doc values file generation
	// -1 means no separate doc values
	// >= 0 means doc values exist
	docValuesGen int64

	// attributes holds custom per-commit attributes
	attributes map[string]string

	// fieldInfosFiles tracks files containing FieldInfo updates
	fieldInfosFiles map[string]struct{}

	// docValuesUpdatesFiles tracks files containing DocValues updates
	// field number -> set of file names
	docValuesUpdatesFiles map[int]map[string]struct{}

	// id is a 16-byte unique identifier for this segment commit
	// -1 if no ID is assigned
	id []byte

	// inMemoryFieldInfos holds the FieldInfos for this segment in memory.
	// This is not persisted to disk; it is populated by IndexWriter during
	// commit and copied by AddIndexes to preserve field metadata across
	// directory boundaries.
	inMemoryFieldInfos *schema.FieldInfos

	// inMemoryFields holds in-memory postings built from DocumentsWriter
	// DWPTs when no codec was wired.  Used by SegmentReader.Terms() to
	// serve search queries without a full codec round-trip.
	inMemoryFields FieldsProducer

	// deletedOrdinals records which document ordinals (0-based within this
	// segment) were deleted.  Used by SegmentReader to build the live-docs
	// bitset without codec infrastructure.  Persisted in the segments file.
	deletedOrdinals []int

	// mu protects mutable fields
	mu sync.RWMutex
}

// NewSegmentCommitInfo creates a new SegmentCommitInfo wrapping a SegmentInfo.
//
// Parameters:
//   - segmentInfo: the SegmentInfo to wrap
//   - delCount: number of deleted documents
//   - delGen: deletion file generation (-1 if no deletions)
func NewSegmentCommitInfo(segmentInfo *schema.SegmentInfo, delCount int, delGen int64) *SegmentCommitInfo {
	return &SegmentCommitInfo{
		segmentInfo:           segmentInfo,
		delCount:              delCount,
		softDelCount:          0,
		delGen:                delGen,
		fieldInfosGen:         -1,
		docValuesGen:          -1,
		attributes:            make(map[string]string),
		fieldInfosFiles:       make(map[string]struct{}),
		docValuesUpdatesFiles: make(map[int]map[string]struct{}),
	}
}

// GetID returns the unique identifier for this segment commit.
func (sci *SegmentCommitInfo) GetID() []byte {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.id
}

// SetID sets the unique identifier for this segment commit.
func (sci *SegmentCommitInfo) SetID(id []byte) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.id = id
}

// FieldInfosFiles returns the set of files for FieldInfo updates.
func (sci *SegmentCommitInfo) FieldInfosFiles() map[string]struct{} {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	copy := make(map[string]struct{}, len(sci.fieldInfosFiles))
	for k, v := range sci.fieldInfosFiles {
		copy[k] = v
	}
	return copy
}

// SetFieldInfosFiles sets the set of files for FieldInfo updates.
func (sci *SegmentCommitInfo) SetFieldInfosFiles(files map[string]struct{}) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.fieldInfosFiles = make(map[string]struct{}, len(files))
	for k, v := range files {
		sci.fieldInfosFiles[k] = v
	}
}

// DocValuesUpdatesFiles returns the map of field numbers to sets of files
// for DocValues updates.
func (sci *SegmentCommitInfo) DocValuesUpdatesFiles() map[int]map[string]struct{} {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	copy := make(map[int]map[string]struct{}, len(sci.docValuesUpdatesFiles))
	for k, v := range sci.docValuesUpdatesFiles {
		innerCopy := make(map[string]struct{}, len(v))
		for k2, v2 := range v {
			innerCopy[k2] = v2
		}
		copy[k] = innerCopy
	}
	return copy
}

// SetDocValuesUpdatesFiles sets the map of field numbers to sets of files
// for DocValues updates.
func (sci *SegmentCommitInfo) SetDocValuesUpdatesFiles(files map[int]map[string]struct{}) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.docValuesUpdatesFiles = make(map[int]map[string]struct{}, len(files))
	for k, v := range files {
		innerCopy := make(map[string]struct{}, len(v))
		for k2, v2 := range v {
			innerCopy[k2] = v2
		}
		sci.docValuesUpdatesFiles[k] = innerCopy
	}
}

// SegmentInfo returns the wrapped SegmentInfo.
func (sci *SegmentCommitInfo) SegmentInfo() *schema.SegmentInfo {
	return sci.segmentInfo
}

// DelCount returns the number of deleted documents.
func (sci *SegmentCommitInfo) DelCount() int {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.delCount
}

// SetDelCount sets the number of deleted documents.
func (sci *SegmentCommitInfo) SetDelCount(count int) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.delCount = count
}

// IncrDelCount atomically increments the deletion count by delta.
// Used by IndexWriter.Commit to apply committed-segment deletions
// that were generated by UpdateDocument calls.
func (sci *SegmentCommitInfo) IncrDelCount(delta int) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.delCount += delta
}

// SoftDelCount returns the number of soft-deleted documents.
func (sci *SegmentCommitInfo) SoftDelCount() int {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.softDelCount
}

// SetSoftDelCount sets the number of soft-deleted documents.
func (sci *SegmentCommitInfo) SetSoftDelCount(count int) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.softDelCount = count
}

// DelGen returns the deletion file generation.
// Returns -1 if there are no deletions.
func (sci *SegmentCommitInfo) DelGen() int64 {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.delGen
}

// SetDelGen sets the deletion file generation.
func (sci *SegmentCommitInfo) SetDelGen(gen int64) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.delGen = gen
}

// HasDeletions returns true if this segment has deletions.
// Returns true when delGen >= 0 (on-disk deletion file exists) or when
// deletedOrdinals are recorded in memory (Gocene in-memory deletion tracking).
func (sci *SegmentCommitInfo) HasDeletions() bool {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.delGen >= 0 || len(sci.deletedOrdinals) > 0
}

// FieldInfosGen returns the field infos file generation.
// Returns -1 if there is no separate field infos file.
func (sci *SegmentCommitInfo) FieldInfosGen() int64 {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.fieldInfosGen
}

// SetFieldInfosGen sets the field infos file generation.
func (sci *SegmentCommitInfo) SetFieldInfosGen(gen int64) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.fieldInfosGen = gen
}

// HasFieldInfosGen returns true if this segment has a separate field infos file.
func (sci *SegmentCommitInfo) HasFieldInfosGen() bool {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.fieldInfosGen >= 0
}

// DocValuesGen returns the doc values file generation.
// Returns -1 if there is no separate doc values file.
func (sci *SegmentCommitInfo) DocValuesGen() int64 {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.docValuesGen
}

// SetDocValuesGen sets the doc values file generation.
func (sci *SegmentCommitInfo) SetDocValuesGen(gen int64) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.docValuesGen = gen
}

// HasDocValuesGen returns true if this segment has a separate doc values file.
func (sci *SegmentCommitInfo) HasDocValuesGen() bool {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.docValuesGen >= 0
}

// GetAttribute returns a custom attribute value.
func (sci *SegmentCommitInfo) GetAttribute(key string) string {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.attributes[key]
}

// SetAttribute sets a custom attribute value.
func (sci *SegmentCommitInfo) SetAttribute(key, value string) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.attributes[key] = value
}

// GetAttributes returns a copy of all custom attributes.
func (sci *SegmentCommitInfo) GetAttributes() map[string]string {
	sci.mu.RLock()
	defer sci.mu.RUnlock()

	copy := make(map[string]string, len(sci.attributes))
	for k, v := range sci.attributes {
		copy[k] = v
	}
	return copy
}

// Name returns the segment name (delegates to SegmentInfo).
func (sci *SegmentCommitInfo) Name() string {
	return sci.segmentInfo.Name()
}

// DocCount returns the total document count (delegates to SegmentInfo).
func (sci *SegmentCommitInfo) DocCount() int {
	return sci.segmentInfo.DocCount()
}

// NumDocs returns the number of live documents (docCount - delCount - softDelCount).
func (sci *SegmentCommitInfo) NumDocs() int {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.segmentInfo.DocCount() - sci.delCount - sci.softDelCount
}

// MaxDoc returns the maximum document ID (docCount - 1).
func (sci *SegmentCommitInfo) MaxDoc() int {
	return sci.segmentInfo.DocCount() - 1
}

// GetGeneration returns the segment generation (delegates to SegmentInfo).
func (sci *SegmentCommitInfo) GetGeneration() int64 {
	return sci.segmentInfo.GetGeneration()
}

// GetInMemoryFieldInfos returns the in-memory FieldInfos for this segment.
// May be nil if no documents have been added.
func (sci *SegmentCommitInfo) GetInMemoryFieldInfos() *schema.FieldInfos {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.inMemoryFieldInfos
}

// SetInMemoryFieldInfos sets the in-memory FieldInfos for this segment.
func (sci *SegmentCommitInfo) SetInMemoryFieldInfos(fi *schema.FieldInfos) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.inMemoryFieldInfos = fi
}

// GetInMemoryFields returns the in-memory FieldsProducer for this segment.
// Non-nil only when IndexWriter committed without a codec (test-only path).
func (sci *SegmentCommitInfo) GetInMemoryFields() FieldsProducer {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return sci.inMemoryFields
}

// SetInMemoryFields sets the in-memory FieldsProducer for this segment.
func (sci *SegmentCommitInfo) SetInMemoryFields(fp FieldsProducer) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	sci.inMemoryFields = fp
}

// GetDeletedOrdinals returns the sorted slice of deleted document ordinals
// (0-based within this segment).  Used by SegmentReader to build the live-docs
// bitset without codec infrastructure.
func (sci *SegmentCommitInfo) GetDeletedOrdinals() []int {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	if len(sci.deletedOrdinals) == 0 {
		return nil
	}
	cp := make([]int, len(sci.deletedOrdinals))
	copy(cp, sci.deletedOrdinals)
	return cp
}

// SetDeletedOrdinals sets the deleted document ordinals for this segment.
func (sci *SegmentCommitInfo) SetDeletedOrdinals(ordinals []int) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	if len(ordinals) == 0 {
		sci.deletedOrdinals = nil
		return
	}
	sci.deletedOrdinals = make([]int, len(ordinals))
	copy(sci.deletedOrdinals, ordinals)
}

// String returns a string representation of SegmentCommitInfo.
func (sci *SegmentCommitInfo) String() string {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	return fmt.Sprintf("SegmentCommitInfo(name=%s, delCount=%d, delGen=%d, fieldInfosGen=%d)",
		sci.segmentInfo.Name(), sci.delCount, sci.delGen, sci.fieldInfosGen)
}

// Clone creates a copy of this SegmentCommitInfo.
func (sci *SegmentCommitInfo) Clone() *SegmentCommitInfo {
	sci.mu.RLock()
	defer sci.mu.RUnlock()

	clone := &SegmentCommitInfo{
		segmentInfo:           sci.segmentInfo,
		delCount:              sci.delCount,
		softDelCount:          sci.softDelCount,
		delGen:                sci.delGen,
		fieldInfosGen:         sci.fieldInfosGen,
		docValuesGen:          sci.docValuesGen,
		attributes:            make(map[string]string, len(sci.attributes)),
		fieldInfosFiles:       make(map[string]struct{}, len(sci.fieldInfosFiles)),
		docValuesUpdatesFiles: make(map[int]map[string]struct{}, len(sci.docValuesUpdatesFiles)),
		id:                    append([]byte(nil), sci.id...),
		// inMemoryFieldInfos / inMemoryFields are Gocene-specific, set once at
		// commit and treated as read-only thereafter. Carry the pointers so a
		// cloned SegmentInfos stays reader-equivalent to the original (the
		// no-codec/in-RAM reader path can still resolve fields). Sharing
		// read-only references introduces no race.
		inMemoryFieldInfos: sci.inMemoryFieldInfos,
		inMemoryFields:     sci.inMemoryFields,
	}

	if len(sci.deletedOrdinals) > 0 {
		clone.deletedOrdinals = make([]int, len(sci.deletedOrdinals))
		copy(clone.deletedOrdinals, sci.deletedOrdinals)
	}

	for k, v := range sci.attributes {
		clone.attributes[k] = v
	}
	for k, v := range sci.fieldInfosFiles {
		clone.fieldInfosFiles[k] = v
	}
	for k, v := range sci.docValuesUpdatesFiles {
		inner := make(map[string]struct{}, len(v))
		for k2, v2 := range v {
			inner[k2] = v2
		}
		clone.docValuesUpdatesFiles[k] = inner
	}

	return clone
}

// AdvanceDelGen advances the deletion generation.
// Returns the new generation.
func (sci *SegmentCommitInfo) AdvanceDelGen() int64 {
	sci.mu.Lock()
	defer sci.mu.Unlock()

	if sci.delGen < 0 {
		sci.delGen = 1
	} else {
		sci.delGen++
	}
	return sci.delGen
}

// AdvanceFieldInfosGen advances the field infos generation.
// Returns the new generation.
func (sci *SegmentCommitInfo) AdvanceFieldInfosGen() int64 {
	sci.mu.Lock()
	defer sci.mu.Unlock()

	if sci.fieldInfosGen < 0 {
		sci.fieldInfosGen = 1
	} else {
		sci.fieldInfosGen++
	}
	return sci.fieldInfosGen
}

// AdvanceDocValuesGen advances the doc values generation.
// Returns the new generation.
func (sci *SegmentCommitInfo) AdvanceDocValuesGen() int64 {
	sci.mu.Lock()
	defer sci.mu.Unlock()

	if sci.docValuesGen < 0 {
		sci.docValuesGen = 1
	} else {
		sci.docValuesGen++
	}
	return sci.docValuesGen
}

// GetDelFileName returns the deletion file name for this generation.
// Returns empty string if there are no deletions.
func (sci *SegmentCommitInfo) GetDelFileName() string {
	sci.mu.RLock()
	defer sci.mu.RUnlock()

	if sci.delGen < 0 {
		return ""
	}
	return fmt.Sprintf("_%s_%d.del", sci.segmentInfo.Name()[1:], sci.delGen)
}

// GetFieldInfosFileName returns the field infos file name for this generation.
// Returns empty string if there is no separate field infos file.
func (sci *SegmentCommitInfo) GetFieldInfosFileName() string {
	sci.mu.RLock()
	defer sci.mu.RUnlock()

	if sci.fieldInfosGen < 0 {
		return ""
	}
	return fmt.Sprintf("_%s_%d.fnm", sci.segmentInfo.Name()[1:], sci.fieldInfosGen)
}

// GetDocValuesFileName returns the doc values file name for this generation.
// Returns empty string if there is no separate doc values file.
func (sci *SegmentCommitInfo) GetDocValuesFileName() string {
	sci.mu.RLock()
	defer sci.mu.RUnlock()

	if sci.docValuesGen < 0 {
		return ""
	}
	return fmt.Sprintf("_%s_%d.dvd", sci.segmentInfo.Name()[1:], sci.docValuesGen)
}

// GetFiles returns all files associated with this segment commit.
//
// This includes the segment files plus any deletion/field-infos/doc-values
// sidecar files. Crucially, every per-field doc-values update file recorded in
// docValuesUpdatesFiles is returned so that a segment which updates only some
// of its fields still protects the unchanged fields' original generation files.
// Mirroring org.apache.lucene.index.SegmentCommitInfo.files().
func (sci *SegmentCommitInfo) GetFiles() []string {
	sci.mu.RLock()
	defer sci.mu.RUnlock()

	seen := make(map[string]struct{}, 16)
	out := make([]string, 0, 16)

	add := func(name string) {
		if _, dup := seen[name]; dup {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	for _, f := range sci.segmentInfo.Files() {
		add(f)
	}
	if sci.delGen >= 0 {
		// Lucene 10.4.0 live-docs extension is "liv" (org.apache.lucene.codecs.
		// lucene90.Lucene90LiveDocsFormat.EXTENSION). The legacy "del" name
		// was a Gocene stub that no longer matches the on-disk file.
		add(fmt.Sprintf("_%s_%d.liv", sci.segmentInfo.Name()[1:], sci.delGen))
	}
	if sci.fieldInfosGen >= 0 {
		add(fmt.Sprintf("_%s_%d.fnm", sci.segmentInfo.Name()[1:], sci.fieldInfosGen))
	}
	if sci.docValuesGen >= 0 {
		add(fmt.Sprintf("_%s_%d.dvd", sci.segmentInfo.Name()[1:], sci.docValuesGen))
	}
	for f := range sci.fieldInfosFiles {
		add(f)
	}
	for _, inner := range sci.docValuesUpdatesFiles {
		for f := range inner {
			add(f)
		}
	}

	return out
}

// SegmentCommitInfoList represents a list of SegmentCommitInfo.
type SegmentCommitInfoList []*SegmentCommitInfo

// Size returns the number of segments.
func (list SegmentCommitInfoList) Size() int {
	return len(list)
}

// TotalDocCount returns the total document count across all segments.
func (list SegmentCommitInfoList) TotalDocCount() int {
	total := 0
	for _, sci := range list {
		total += sci.DocCount()
	}
	return total
}

// TotalNumDocs returns the total number of live documents across all segments.
func (list SegmentCommitInfoList) TotalNumDocs() int {
	total := 0
	for _, sci := range list {
		total += sci.NumDocs()
	}
	return total
}

// TotalDelCount returns the total number of deleted documents across all segments.
func (list SegmentCommitInfoList) TotalDelCount() int {
	total := 0
	for _, sci := range list {
		total += sci.DelCount()
	}
	return total
}

// String returns a string representation of the list.
func (list SegmentCommitInfoList) String() string {
	return fmt.Sprintf("SegmentCommitInfoList(count=%d)", len(list))
}
