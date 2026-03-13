// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfo stores metadata about a Lucene index segment.
// This is the Go port of Lucene's org.apache.lucene.index.SegmentInfo.
//
// A segment is a self-contained index that contains documents.
// Multiple segments can be merged together to form a larger index.
type SegmentInfo struct {
	// name is the segment name (e.g., "_0", "_1")
	name string

	// docCount is the number of documents in this segment
	docCount int

	// directory is where the segment files are stored
	directory store.Directory

	// files is the set of files associated with this segment
	files []string

	// version is the Lucene version that created this segment
	version string

	// isCompoundFile indicates if this segment is stored in a compound file
	isCompoundFile bool

	// codec is the codec used to write this segment
	codec string

	// diagnostics holds debugging information about how the segment was created
	diagnostics map[string]string

	// attributes holds custom per-segment attributes
	attributes map[string]string

	// id is a 16-byte unique identifier for this segment
	id [16]byte

	// indexSort describes how documents are sorted in this segment
	// nil if documents are not sorted
	indexSort *Sort

	// mu protects mutable fields
	mu sync.RWMutex
}

// Sort represents a sort specification for documents in a segment.
type Sort struct {
	// fields to sort by
	fields []SortField
}

// SortField represents a single sort field.
type SortField struct {
	// field name to sort by
	field string
	// descending is true for descending order
	descending bool
	// sortType is the type of sorting
	sortType SortType
}

// SortType represents how a field should be sorted.
type SortType int

const (
	// SortTypeString sorts strings
	SortTypeString SortType = iota
	// SortTypeLong sorts as long integers
	SortTypeLong
	// SortTypeInt sorts as integers
	SortTypeInt
	// SortTypeFloat sorts as floats
	SortTypeFloat
	// SortTypeDouble sorts as doubles
	SortTypeDouble
)

// NewSegmentInfo creates a new SegmentInfo.
func NewSegmentInfo(name string, docCount int, dir store.Directory) *SegmentInfo {
	return &SegmentInfo{
		name:        name,
		docCount:    docCount,
		directory:   dir,
		files:       make([]string, 0),
		version:     "10.0.0", // Default version
		codec:       "Lucene104",
		diagnostics: make(map[string]string),
		attributes:  make(map[string]string),
		indexSort:   nil,
	}
}

// Name returns the segment name.
func (si *SegmentInfo) Name() string {
	return si.name
}

// DocCount returns the number of documents in this segment.
func (si *SegmentInfo) DocCount() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.docCount
}

// Directory returns the directory where segment files are stored.
func (si *SegmentInfo) Directory() store.Directory {
	return si.directory
}

// Files returns the files associated with this segment.
// Returns a copy of the file list.
func (si *SegmentInfo) Files() []string {
	si.mu.RLock()
	defer si.mu.RUnlock()

	files := make([]string, len(si.files))
	copy(files, si.files)
	return files
}

// SetFiles sets the files associated with this segment.
func (si *SegmentInfo) SetFiles(files []string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.files = make([]string, len(files))
	copy(si.files, files)
	sort.Strings(si.files)
}

// AddFile adds a file to this segment.
func (si *SegmentInfo) AddFile(file string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.files = append(si.files, file)
	sort.Strings(si.files)
}

// HasFile returns true if the specified file is part of this segment.
func (si *SegmentInfo) HasFile(file string) bool {
	si.mu.RLock()
	defer si.mu.RUnlock()
	for _, f := range si.files {
		if f == file {
			return true
		}
	}
	return false
}

// Version returns the Lucene version that created this segment.
func (si *SegmentInfo) Version() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.version
}

// SetVersion sets the Lucene version.
func (si *SegmentInfo) SetVersion(version string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.version = version
}

// IsCompoundFile returns true if this segment uses a compound file.
func (si *SegmentInfo) IsCompoundFile() bool {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.isCompoundFile
}

// SetCompoundFile sets whether this segment uses a compound file.
func (si *SegmentInfo) SetCompoundFile(compound bool) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.isCompoundFile = compound
}

// Codec returns the codec used for this segment.
func (si *SegmentInfo) Codec() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.codec
}

// SetCodec sets the codec for this segment.
func (si *SegmentInfo) SetCodec(codec string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.codec = codec
}

// GetDiagnostics returns the diagnostic information.
// Returns a copy of the diagnostics map.
func (si *SegmentInfo) GetDiagnostics() map[string]string {
	si.mu.RLock()
	defer si.mu.RUnlock()

	diagnostics := make(map[string]string, len(si.diagnostics))
	for k, v := range si.diagnostics {
		diagnostics[k] = v
	}
	return diagnostics
}

// SetDiagnostics sets the diagnostic information.
func (si *SegmentInfo) SetDiagnostics(diagnostics map[string]string) {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.diagnostics = make(map[string]string, len(diagnostics))
	for k, v := range diagnostics {
		si.diagnostics[k] = v
	}
}

// GetDiagnostic returns a specific diagnostic value.
func (si *SegmentInfo) GetDiagnostic(key string) string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.diagnostics[key]
}

// SetDiagnostic sets a diagnostic value.
func (si *SegmentInfo) SetDiagnostic(key, value string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.diagnostics[key] = value
}

// GetAttributes returns a copy of the attributes map.
func (si *SegmentInfo) GetAttributes() map[string]string {
	si.mu.RLock()
	defer si.mu.RUnlock()

	attributes := make(map[string]string, len(si.attributes))
	for k, v := range si.attributes {
		attributes[k] = v
	}
	return attributes
}

// SetAttributes sets the attributes.
func (si *SegmentInfo) SetAttributes(attributes map[string]string) {
	si.mu.Lock()
	defer si.mu.Unlock()

	si.attributes = make(map[string]string, len(attributes))
	for k, v := range attributes {
		si.attributes[k] = v
	}
}

// GetAttribute returns a custom attribute value.
func (si *SegmentInfo) GetAttribute(key string) string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.attributes[key]
}

// SetAttribute sets a custom attribute value.
func (si *SegmentInfo) SetAttribute(key, value string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.attributes[key] = value
}

// IndexSort returns the sort specification for this segment.
func (si *SegmentInfo) IndexSort() *Sort {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.indexSort
}

// SetIndexSort sets the sort specification for this segment.
func (si *SegmentInfo) SetIndexSort(sort *Sort) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.indexSort = sort
}

// SetDocCount sets the document count.
// This should be called when documents are deleted.
func (si *SegmentInfo) SetDocCount(count int) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.docCount = count
}

// String returns a string representation of SegmentInfo.
func (si *SegmentInfo) String() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return fmt.Sprintf("SegmentInfo(name=%s, docCount=%d, version=%s, codec=%s)",
		si.name, si.docCount, si.version, si.codec)
}

// Clone creates a copy of this SegmentInfo.
func (si *SegmentInfo) Clone() *SegmentInfo {
	si.mu.RLock()
	defer si.mu.RUnlock()

	clone := &SegmentInfo{
		name:           si.name,
		docCount:       si.docCount,
		directory:      si.directory,
		files:          make([]string, len(si.files)),
		version:        si.version,
		isCompoundFile: si.isCompoundFile,
		codec:          si.codec,
		diagnostics:    make(map[string]string, len(si.diagnostics)),
		attributes:     make(map[string]string, len(si.attributes)),
		indexSort:      si.indexSort,
	}

	copy(clone.files, si.files)
	for k, v := range si.diagnostics {
		clone.diagnostics[k] = v
	}
	for k, v := range si.attributes {
		clone.attributes[k] = v
	}

	return clone
}

// GetID returns a unique identifier for this segment.
func (si *SegmentInfo) GetID() []byte {
	si.mu.RLock()
	defer si.mu.RUnlock()
	id := make([]byte, 16)
	copy(id, si.id[:])
	return id
}

// SetID sets the unique identifier for this segment.
func (si *SegmentInfo) SetID(id []byte) error {
	if len(id) != 16 {
		return fmt.Errorf("invalid id length: %d (expected 16)", len(id))
	}
	si.mu.Lock()
	defer si.mu.Unlock()
	copy(si.id[:], id)
	return nil
}

// GetGeneration returns the segment generation from the name.
// Segment names are typically "_N" where N is the generation.
func (si *SegmentInfo) GetGeneration() int64 {
	// Parse generation from name like "_0", "_1", etc.
	var gen int64
	if len(si.name) > 1 && si.name[0] == '_' {
		fmt.Sscanf(si.name[1:], "%d", &gen)
	}
	return gen
}

// GetIndexSortDescription returns a description of the index sort.
func (si *SegmentInfo) GetIndexSortDescription() string {
	si.mu.RLock()
	defer si.mu.RUnlock()

	if si.indexSort == nil || len(si.indexSort.fields) == 0 {
		return "<not sorted>"
	}

	desc := ""
	for i, f := range si.indexSort.fields {
		if i > 0 {
			desc += ", "
		}
		desc += f.field
		if f.descending {
			desc += " DESC"
		} else {
			desc += " ASC"
		}
	}
	return desc
}

// AddDiagnosticsFromMerge adds diagnostic information when this segment
// was created from a merge.
func (si *SegmentInfo) AddDiagnosticsFromMerge(mergedSegments []*SegmentInfo) {
	si.SetDiagnostic("merge", "true")
	si.SetDiagnostic("mergeTime", time.Now().Format(time.RFC3339))
	si.SetDiagnostic("mergedSegmentCount", fmt.Sprintf("%d", len(mergedSegments)))

	for i, seg := range mergedSegments {
		key := fmt.Sprintf("sourceSegment%d", i)
		si.SetDiagnostic(key, seg.Name())
	}
}

// SizeInBytes returns an estimate of the size of this segment in bytes.
func (si *SegmentInfo) SizeInBytes() int64 {
	si.mu.RLock()
	defer si.mu.RUnlock()

	var size int64
	for _, file := range si.files {
		if si.directory != nil {
			if fileLength, err := si.directory.FileLength(file); err == nil {
				size += fileLength
			}
		}
	}
	return size
}

// SegmentInfoList represents a list of SegmentInfo objects.
type SegmentInfoList []*SegmentInfo

// TotalDocCount returns the total document count across all segments.
func (list SegmentInfoList) TotalDocCount() int {
	total := 0
	for _, si := range list {
		total += si.DocCount()
	}
	return total
}

// String returns a string representation of the list.
func (list SegmentInfoList) String() string {
	return fmt.Sprintf("SegmentInfoList(count=%d)", len(list))
}

// GetMaxDoc returns the maximum document ID (total docs - 1).
func (list SegmentInfoList) GetMaxDoc() int {
	return list.TotalDocCount() - 1
}
