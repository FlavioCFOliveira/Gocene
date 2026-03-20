// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SpatialIndexFormat handles encoding/decoding of spatial index data.
// This is the Go port of Lucene's spatial index format functionality.
//
// The spatial index format stores spatial data using various strategies
// (point vector, bounding box, prefix tree, etc.) for efficient geospatial queries.
type SpatialIndexFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsWriter returns a writer for writing spatial index data.
	// The caller should close the returned writer when done.
	FieldsWriter(state *index.SegmentWriteState) (SpatialIndexFormatWriter, error)

	// FieldsReader returns a reader for reading spatial index data.
	// The caller should close the returned reader when done.
	FieldsReader(state *index.SegmentReadState) (SpatialIndexFormatReader, error)
}

// SpatialIndexFormatWriter is the interface for spatial index writers.
type SpatialIndexFormatWriter interface {
	// RegisterStrategy registers a spatial strategy for a field.
	RegisterStrategy(fieldName string, strategy SpatialStrategy) error

	// StartDocument starts writing a new document.
	StartDocument() error

	// FinishDocument finishes writing the current document.
	FinishDocument() error

	// WriteSpatialField writes a spatial field to the index.
	WriteSpatialField(fieldName string, shape Shape) ([]document.IndexableField, error)

	// Close releases all resources used by this writer.
	Close() error
}

// SpatialIndexFormatReader is the interface for spatial index readers.
type SpatialIndexFormatReader interface {
	// RegisterStrategy registers a spatial strategy for a field.
	RegisterStrategy(fieldName string, strategy SpatialStrategy) error

	// MakeQuery creates a spatial query for the given field, operation, and shape.
	MakeQuery(fieldName string, operation SpatialOperation, shape Shape) (search.Query, error)

	// Close releases all resources used by this reader.
	Close() error
}

// BaseSpatialIndexFormat provides common functionality for SpatialIndexFormat implementations.
type BaseSpatialIndexFormat struct {
	name string
}

// NewBaseSpatialIndexFormat creates a new BaseSpatialIndexFormat.
func NewBaseSpatialIndexFormat(name string) *BaseSpatialIndexFormat {
	return &BaseSpatialIndexFormat{name: name}
}

// Name returns the format name.
func (f *BaseSpatialIndexFormat) Name() string {
	return f.name
}

// FieldsWriter returns a fields writer (must be implemented by subclasses).
func (f *BaseSpatialIndexFormat) FieldsWriter(state *index.SegmentWriteState) (SpatialIndexFormatWriter, error) {
	return nil, fmt.Errorf("FieldsWriter not implemented")
}

// FieldsReader returns a fields reader (must be implemented by subclasses).
func (f *BaseSpatialIndexFormat) FieldsReader(state *index.SegmentReadState) (SpatialIndexFormatReader, error) {
	return nil, fmt.Errorf("FieldsReader not implemented")
}

// SpatialIndexFormatImpl is a concrete implementation of SpatialIndexFormat.
// It provides reading and writing of spatial index data using the standard format.
type SpatialIndexFormatImpl struct {
	*BaseSpatialIndexFormat
}

// NewSpatialIndexFormat creates a new SpatialIndexFormatImpl with the default name.
func NewSpatialIndexFormat() *SpatialIndexFormatImpl {
	return &SpatialIndexFormatImpl{
		BaseSpatialIndexFormat: NewBaseSpatialIndexFormat("SpatialIndexFormat"),
	}
}

// NewSpatialIndexFormatWithName creates a new SpatialIndexFormatImpl with a custom name.
func NewSpatialIndexFormatWithName(name string) *SpatialIndexFormatImpl {
	return &SpatialIndexFormatImpl{
		BaseSpatialIndexFormat: NewBaseSpatialIndexFormat(name),
	}
}

// FieldsWriter returns a writer for writing spatial index data.
func (f *SpatialIndexFormatImpl) FieldsWriter(state *index.SegmentWriteState) (SpatialIndexFormatWriter, error) {
	if state == nil {
		return nil, fmt.Errorf("state cannot be nil")
	}
	if state.Directory == nil {
		return nil, fmt.Errorf("state.Directory cannot be nil")
	}
	if state.SegmentInfo == nil {
		return nil, fmt.Errorf("state.SegmentInfo cannot be nil")
	}
	if state.FieldInfos == nil {
		return nil, fmt.Errorf("state.FieldInfos cannot be nil")
	}

	return NewSpatialIndexWriter(state.Directory, state.SegmentInfo, state.FieldInfos)
}

// FieldsReader returns a reader for reading spatial index data.
func (f *SpatialIndexFormatImpl) FieldsReader(state *index.SegmentReadState) (SpatialIndexFormatReader, error) {
	if state == nil {
		return nil, fmt.Errorf("state cannot be nil")
	}
	if state.Directory == nil {
		return nil, fmt.Errorf("state.Directory cannot be nil")
	}
	if state.SegmentInfo == nil {
		return nil, fmt.Errorf("state.SegmentInfo cannot be nil")
	}
	if state.FieldInfos == nil {
		return nil, fmt.Errorf("state.FieldInfos cannot be nil")
	}

	return NewSpatialIndexReader(state.Directory, state.SegmentInfo, state.FieldInfos)
}

// SpatialIndexFileNames provides file name utilities for spatial index files.
type SpatialIndexFileNames struct {
	segmentName string
}

// NewSpatialIndexFileNames creates a new SpatialIndexFileNames for the given segment.
func NewSpatialIndexFileNames(segmentName string) *SpatialIndexFileNames {
	return &SpatialIndexFileNames{segmentName: segmentName}
}

// GetIndexFileName returns the name of the spatial index file for the segment.
func (n *SpatialIndexFileNames) GetIndexFileName() string {
	return fmt.Sprintf("%s.spatial", n.segmentName)
}

// GetMetadataFileName returns the name of the spatial metadata file for the segment.
func (n *SpatialIndexFileNames) GetMetadataFileName() string {
	return fmt.Sprintf("%s.spatial_meta", n.segmentName)
}

// GetFileNames returns all spatial index file names for the segment.
func (n *SpatialIndexFileNames) GetFileNames() []string {
	return []string{
		n.GetIndexFileName(),
		n.GetMetadataFileName(),
	}
}

// SpatialIndexFileHeader represents the header of a spatial index file.
type SpatialIndexFileHeader struct {
	Magic   uint32
	Version uint32
}

// SpatialIndexFileHeaderCurrentVersion is the current version of the spatial index file format.
const SpatialIndexFileHeaderCurrentVersion = 1

// SpatialIndexFileMagic is the magic number for spatial index files (SP = Spatial).
const SpatialIndexFileMagic = 0x53500000

// WriteSpatialIndexFileHeader writes the spatial index file header.
func WriteSpatialIndexFileHeader(out store.IndexOutput) error {
	// Write magic number
	if err := store.WriteUint32(out, SpatialIndexFileMagic); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(out, SpatialIndexFileHeaderCurrentVersion); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// ReadSpatialIndexFileHeader reads and validates the spatial index file header.
func ReadSpatialIndexFileHeader(in store.IndexInput) (*SpatialIndexFileHeader, error) {
	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != SpatialIndexFileMagic {
		return nil, fmt.Errorf("invalid magic number: expected 0x%08x, got 0x%08x", SpatialIndexFileMagic, magic)
	}

	// Read version
	version, err := store.ReadUint32(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	if version != SpatialIndexFileHeaderCurrentVersion {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	return &SpatialIndexFileHeader{
		Magic:   magic,
		Version: version,
	}, nil
}

// SpatialIndexMetadata represents metadata for a spatial index.
type SpatialIndexMetadata struct {
	// NumFields is the number of spatial fields in the index.
	NumFields int

	// FieldNames is the list of spatial field names.
	FieldNames []string

	// DocCount is the number of documents with spatial data.
	DocCount int
}

// WriteSpatialIndexMetadata writes the spatial index metadata.
func WriteSpatialIndexMetadata(out store.IndexOutput, metadata *SpatialIndexMetadata) error {
	if metadata == nil {
		return fmt.Errorf("metadata cannot be nil")
	}

	// Write number of fields
	if err := store.WriteVInt(out, int32(metadata.NumFields)); err != nil {
		return fmt.Errorf("failed to write num fields: %w", err)
	}

	// Write field names
	for _, fieldName := range metadata.FieldNames {
		if err := store.WriteString(out, fieldName); err != nil {
			return fmt.Errorf("failed to write field name: %w", err)
		}
	}

	// Write doc count
	if err := store.WriteVInt(out, int32(metadata.DocCount)); err != nil {
		return fmt.Errorf("failed to write doc count: %w", err)
	}

	return nil
}

// ReadSpatialIndexMetadata reads the spatial index metadata.
func ReadSpatialIndexMetadata(in store.IndexInput) (*SpatialIndexMetadata, error) {
	// Read number of fields
	numFields, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read num fields: %w", err)
	}

	// Read field names
	fieldNames := make([]string, numFields)
	for i := int32(0); i < numFields; i++ {
		fieldName, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read field name: %w", err)
		}
		fieldNames[i] = fieldName
	}

	// Read doc count
	docCount, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read doc count: %w", err)
	}

	return &SpatialIndexMetadata{
		NumFields:  int(numFields),
		FieldNames: fieldNames,
		DocCount:   int(docCount),
	}, nil
}
