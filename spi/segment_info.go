// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package spi holds the structural types and interfaces that both the
// index/ package and the codecs/ family operate on. It is a leaf package —
// it depends only on the standard library and on the store/ primitive
// package — so that it can be imported from anywhere in the Gocene tree
// without creating cycles.
//
// spi is the canonical declaration site for SegmentInfo and the sort
// metadata it carries. The index/ package re-exports these types via Go
// type aliases (type X = spi.X) for source-level backward compatibility
// with callers that historically reached for index.X.
//
// This package is part of the SPI unification work tracked under rmp
// #4669.
package spi

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// SegmentInfo stores metadata about a Lucene index segment.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentInfo
// (10.5.0). The field set mirrors the Java class: name, maxDoc, dir,
// isCompoundFile, id, codec, diagnostics, attributes, indexSort, version,
// minVersion, hasBlocks and the file set.
//
// Like the Java class, maxDoc is initialised to -1 and MaxDoc panics when
// it is read before it has been set (Lucene throws
// IllegalStateException). A SegmentInfo created through the convenience
// constructor NewSegmentInfo(name, maxDoc, dir) already has maxDoc set;
// the flush path creates it with -1 and calls SetMaxDoc once the final
// count is known.
type SegmentInfo struct {
	// name is the unique segment name in the directory (e.g. "_0").
	name string

	// maxDoc is the number of docs in the segment, including deleted ones.
	// -1 until set (mirrors Lucene's SegmentInfo.maxDoc).
	maxDoc int

	// Dir is where this segment resides. Public final, mirroring Lucene's
	// public final Directory dir.
	Dir Directory

	// isCompoundFile indicates whether this segment is stored in a compound
	// file.
	isCompoundFile bool

	// id uniquely identifies this segment (16 bytes).
	id []byte

	// codec is the codec used to write this segment. Nil on placeholders
	// created by read paths that have not resolved a codec object; the
	// codec name captured on those paths is kept in codecName.
	codec Codec

	// codecName keeps the codec name read from segments_N when no Codec
	// object is attached (the read path has no codec registry to resolve
	// the name). The segments_N writer serialises CodecName(), so a
	// read-then-write round trip preserves the original bytes.
	codecName string

	// diagnostics holds debugging information about how the segment was
	// created.
	diagnostics map[string]string

	// attributes holds custom per-segment attributes.
	attributes map[string]string

	// indexSort describes how documents are sorted in the segment; nil if
	// documents are not sorted.
	indexSort *Sort

	// version tracks the Lucene version this segment was created with.
	version string

	// minVersion tracks the minimum version that contributed documents to
	// this segment (Lucene's SegmentInfo.minVersion, since 7.0). It is nil
	// when absent: the .si writer then emits the hasMinVersion=0 byte,
	// exactly as Lucene99SegmentInfoFormat does when
	// SegmentInfo.getMinVersion()==null.
	minVersion *string

	// hasBlocks records whether this segment contains document blocks
	// (Lucene's SegmentInfo.getHasBlocks(), serialised as the HasBlocks
	// byte after IsCompoundFile in the .si format).
	hasBlocks bool

	// files is the sorted set of files associated with this segment.
	files []string

	// mu protects mutable fields.
	mu sync.RWMutex
}

// Sort represents a sort specification for documents in a segment.
type Sort struct {
	// fields to sort by
	fields []SortField
}


// NewSortFromFields wraps an existing []SortField slice in a *Sort. The
// slice is taken by reference; callers must not retain or mutate it
// after the call.
func NewSortFromFields(fields []SortField) *Sort {
	return &Sort{fields: fields}
}

// SortedNumericSortField represents a sort field for multi-valued numeric fields.
type SortedNumericSortField struct {
	SortField
	selector string // min, max, etc.
}

// NewSortedNumericSortField creates a new SortedNumericSortField.
func NewSortedNumericSortField(name string, sortType SortFieldType) *SortedNumericSortField {
	return &SortedNumericSortField{
		SortField: SortField{
			Field:    name,
			Type:     sortType,
			Reverse:  false,
			Selector: "min",
		},
	}
}

// SortedSetSortField represents a sort field for multi-valued string fields.
type SortedSetSortField struct {
	SortField
}

// NewSortedSetSortField creates a new SortedSetSortField.
func NewSortedSetSortField(name string, reverse bool) *SortedSetSortField {
	return &SortedSetSortField{
		SortField: SortField{
			Field:    name,
			Type:     SortFieldTypeString,
			Reverse:  reverse,
			Selector: "min",
		},
	}
}

// NewSortField creates a new SortField with the given name and type number.
func NewSortField(field string, sortType SortFieldType) *SortField {
	return &SortField{
		Field:   field,
		Type:    sortType,
		Reverse: false,
	}
}

// NewSortFieldFull creates a new SortField with the given name, type number, and descending flag.
func NewSortFieldFull(field string, sortType SortFieldType, descending bool) *SortField {
	return &SortField{
		Field:   field,
		Type:    sortType,
		Reverse: descending,
	}
}

// NewSort creates a new Sort from multiple SortField pointers.
func NewSort(fields ...*SortField) *Sort {
	sfields := make([]SortField, len(fields))
	for i, f := range fields {
		sfields[i] = *f
	}
	return &Sort{fields: sfields}
}

// Fields returns the slice of SortFields in this Sort.
func (s *Sort) Fields() []*SortField {
	fields := make([]*SortField, len(s.fields))
	for i := range s.fields {
		fields[i] = &s.fields[i]
	}
	return fields
}

// SortRELEVANCE is a special sort that sorts by relevance (score).
// This cannot be used as an index sort.
var SortRELEVANCE = &Sort{fields: nil}

// NewSegmentInfo creates a new SegmentInfo with the given name, maxDoc and
// directory. The version defaults to "10.0.0" (this port's current
// version); read paths immediately overwrite it with the value parsed
// from disk via SetVersion. codec is intentionally left nil; callers that
// flush real codec files must call SetCodec explicitly after the flush
// succeeds, and read paths without a codec registry capture the codec
// name via SetCodecName instead.
func NewSegmentInfo(name string, maxDoc int, dir Directory) *SegmentInfo {
	return &SegmentInfo{
		name:        name,
		maxDoc:      maxDoc,
		Dir:         dir,
		version:     "10.0.0",
		diagnostics: make(map[string]string),
		attributes:  make(map[string]string),
	}
}

// Name returns the segment's unique name. Mirrors Lucene's public final
// SegmentInfo.name.
func (si *SegmentInfo) Name() string {
	return si.name
}

// MaxDoc returns the number of documents in this segment, including
// deleted ones. Panics if maxDoc has not been set yet, mirroring Lucene's
// SegmentInfo.maxDoc() IllegalStateException.
func (si *SegmentInfo) MaxDoc() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	if si.maxDoc == -1 {
		panic("maxDoc isn't set yet")
	}
	return si.maxDoc
}

// SetMaxDoc sets the max document count. Mirrors Lucene's package-private
// SegmentInfo.setMaxDoc: it panics if maxDoc was already set to a
// different value.
func (si *SegmentInfo) SetMaxDoc(maxDoc int) {
	si.mu.Lock()
	defer si.mu.Unlock()
	if si.maxDoc != -1 && si.maxDoc != maxDoc {
		panic(fmt.Sprintf("maxDoc was already set: this.maxDoc=%d vs maxDoc=%d", si.maxDoc, maxDoc))
	}
	si.maxDoc = maxDoc
}

// DocCount is an alias for MaxDoc, kept for the accessor name several
// callers historically used (for a SegmentInfo the two values are the
// same: maxDoc counts documents including deleted ones).
func (si *SegmentInfo) DocCount() int {
	return si.MaxDoc()
}

// SetDocCount unconditionally sets maxDoc. It exists for read paths that
// restore the value from disk after the placeholder was created with a
// provisional count; unlike SetMaxDoc it does not panic on reassignment.
func (si *SegmentInfo) SetDocCount(count int) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.maxDoc = count
}

// Directory returns the directory where segment files are stored.
func (si *SegmentInfo) Directory() Directory {
	return si.Dir
}

// Files returns a copy of the files associated with this segment.
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

// MinVersion returns the minimum Lucene version that contributed documents
// to this segment, or the empty string with ok=false when no minVersion is
// set. This mirrors Lucene's SegmentInfo.getMinVersion() (which returns
// null when absent); the .si format serialises the hasMinVersion sentinel
// byte from it.
func (si *SegmentInfo) MinVersion() (version string, ok bool) {
	si.mu.RLock()
	defer si.mu.RUnlock()
	if si.minVersion == nil {
		return "", false
	}
	return *si.minVersion, true
}

// SetMinVersion sets the minimum Lucene version ("major.minor.bugfix")
// that contributed documents to this segment. Passing the empty string
// clears it back to absent (hasMinVersion=0 on disk).
func (si *SegmentInfo) SetMinVersion(version string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	if version == "" {
		si.minVersion = nil
		return
	}
	v := version
	si.minVersion = &v
}

// GetHasBlocks reports whether this segment contains document blocks
// (Lucene's SegmentInfo.getHasBlocks()).
func (si *SegmentInfo) GetHasBlocks() bool {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.hasBlocks
}

// SetHasBlocks sets whether this segment contains document blocks.
func (si *SegmentInfo) SetHasBlocks(hasBlocks bool) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.hasBlocks = hasBlocks
}

// IsCompoundFile returns true if this segment uses a compound file.
// Mirrors Lucene's SegmentInfo.getUseCompoundFile().
func (si *SegmentInfo) IsCompoundFile() bool {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.isCompoundFile
}

// GetUseCompoundFile is an alias for IsCompoundFile, mirroring Lucene's
// method name.
func (si *SegmentInfo) GetUseCompoundFile() bool {
	return si.IsCompoundFile()
}

// SetCompoundFile sets whether this segment uses a compound file.
func (si *SegmentInfo) SetCompoundFile(compound bool) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.isCompoundFile = compound
}

// SetUseCompoundFile is an alias for SetCompoundFile, mirroring Lucene's
// method name.
func (si *SegmentInfo) SetUseCompoundFile(compound bool) {
	si.SetCompoundFile(compound)
}

// GetCodec returns the codec used for this segment, or nil when the
// segment was read back without a codec registry (see CodecName).
// Mirrors Lucene's SegmentInfo.getCodec().
func (si *SegmentInfo) GetCodec() Codec {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.codec
}

// Codec is an alias for GetCodec.
func (si *SegmentInfo) Codec() Codec {
	return si.GetCodec()
}

// SetCodec sets the codec for this segment, mirroring Lucene's
// SegmentInfo.setCodec. A nil codec panics, as in Lucene.
func (si *SegmentInfo) SetCodec(codec Codec) {
	if codec == nil {
		panic("codec must be non-null")
	}
	si.mu.Lock()
	defer si.mu.Unlock()
	si.codec = codec
	si.codecName = codec.Name()
}

// CodecName returns the codec name for this segment: the attached codec's
// name when available, otherwise the name captured from segments_N by the
// read path.
func (si *SegmentInfo) CodecName() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	if si.codec != nil {
		return si.codec.Name()
	}
	return si.codecName
}

// SetCodecName captures the codec name read from disk on read paths that
// have no codec registry to resolve the name to a Codec object.
func (si *SegmentInfo) SetCodecName(name string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.codecName = name
}

// GetDiagnostics returns a copy of the diagnostic information.
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

// AddDiagnostics adds or modifies this segment's diagnostics, mirroring
// Lucene's SegmentInfo.addDiagnostics.
func (si *SegmentInfo) AddDiagnostics(diagnostics map[string]string) {
	si.mu.Lock()
	defer si.mu.Unlock()

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

// PutAttribute sets a custom attribute value and returns the previous
// value, mirroring Lucene's SegmentInfo.putAttribute.
func (si *SegmentInfo) PutAttribute(key, value string) string {
	si.mu.Lock()
	defer si.mu.Unlock()
	oldValue := si.attributes[key]
	si.attributes[key] = value
	return oldValue
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

// String returns a string representation of the segment, mirroring
// Lucene's SegmentInfo.toString().
func (si *SegmentInfo) String() string {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.toStringLocked(0)
}

// toStringLocked renders the Lucene toString(delCount) form; the caller
// must hold mu.
func (si *SegmentInfo) toStringLocked(delCount int) string {
	version := si.version
	if version == "" {
		version = "?"
	}
	cfs := 'C'
	if si.isCompoundFile {
		cfs = 'c'
	}
	s := fmt.Sprintf("%s(%s):%c%d", si.name, version, cfs, si.maxDoc)
	if delCount != 0 {
		s += fmt.Sprintf("/%d", delCount)
	}
	return s
}

// Clone creates a deep copy of this SegmentInfo.
func (si *SegmentInfo) Clone() *SegmentInfo {
	si.mu.RLock()
	defer si.mu.RUnlock()

	clone := &SegmentInfo{
		name:           si.name,
		maxDoc:         si.maxDoc,
		Dir:            si.Dir,
		isCompoundFile: si.isCompoundFile,
		codec:          si.codec,
		codecName:      si.codecName,
		version:        si.version,
		hasBlocks:      si.hasBlocks,
		indexSort:      si.indexSort,
		files:          make([]string, len(si.files)),
		diagnostics:    make(map[string]string, len(si.diagnostics)),
		attributes:     make(map[string]string, len(si.attributes)),
		id:             make([]byte, len(si.id)),
	}
	copy(clone.files, si.files)
	copy(clone.id, si.id)
	for k, v := range si.diagnostics {
		clone.diagnostics[k] = v
	}
	for k, v := range si.attributes {
		clone.attributes[k] = v
	}
	if si.minVersion != nil {
		v := *si.minVersion
		clone.minVersion = &v
	}
	return clone
}

// GetID returns a copy of the unique identifier for this segment.
// Mirrors Lucene's SegmentInfo.getId().
func (si *SegmentInfo) GetID() []byte {
	si.mu.RLock()
	defer si.mu.RUnlock()
	id := make([]byte, len(si.id))
	copy(id, si.id)
	return id
}

// SetID sets the unique identifier for this segment (16 bytes, as
// written in the segments_N and .si formats).
func (si *SegmentInfo) SetID(id []byte) error {
	if len(id) != 16 {
		return fmt.Errorf("invalid id length: %d (expected 16)", len(id))
	}
	si.mu.Lock()
	defer si.mu.Unlock()
	si.id = make([]byte, 16)
	copy(si.id, id)
	return nil
}

// GetGeneration returns the segment generation parsed from the name.
// Segment names are typically "_N" where N is the generation.
func (si *SegmentInfo) GetGeneration() int64 {
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
		desc += f.Field
		if f.Reverse {
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

// SizeInBytes estimates the size of this segment in bytes by summing the
// lengths of its files.
func (si *SegmentInfo) SizeInBytes() int64 {
	si.mu.RLock()
	defer si.mu.RUnlock()

	var size int64
	for _, file := range si.files {
		if si.Dir != nil {
			if fileLength, err := si.Dir.FileLength(file); err == nil {
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
		total += si.MaxDoc()
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
