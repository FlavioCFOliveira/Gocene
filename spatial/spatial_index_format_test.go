// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNewSpatialIndexFormat(t *testing.T) {
	format := NewSpatialIndexFormat()
	if format == nil {
		t.Error("NewSpatialIndexFormat() returned nil")
		return
	}
	if format.Name() != "SpatialIndexFormat" {
		t.Errorf("Name() = %s, want SpatialIndexFormat", format.Name())
	}
}

func TestNewSpatialIndexFormatWithName(t *testing.T) {
	format := NewSpatialIndexFormatWithName("CustomSpatialFormat")
	if format == nil {
		t.Error("NewSpatialIndexFormatWithName() returned nil")
		return
	}
	if format.Name() != "CustomSpatialFormat" {
		t.Errorf("Name() = %s, want CustomSpatialFormat", format.Name())
	}
}

func TestSpatialIndexFormatImpl_FieldsWriter(t *testing.T) {
	format := NewSpatialIndexFormat()

	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()

	state := &index.SegmentWriteState{
		Directory:   directory,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	tests := []struct {
		name    string
		state   *index.SegmentWriteState
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid state",
			state:   state,
			wantErr: false,
		},
		{
			name:    "nil state",
			state:   nil,
			wantErr: true,
			errMsg:  "state cannot be nil",
		},
		{
			name: "nil directory",
			state: &index.SegmentWriteState{
				Directory:   nil,
				SegmentInfo: segmentInfo,
				FieldInfos:  fieldInfos,
			},
			wantErr: true,
			errMsg:  "state.Directory cannot be nil",
		},
		{
			name: "nil segmentInfo",
			state: &index.SegmentWriteState{
				Directory:   directory,
				SegmentInfo: nil,
				FieldInfos:  fieldInfos,
			},
			wantErr: true,
			errMsg:  "state.SegmentInfo cannot be nil",
		},
		{
			name: "nil fieldInfos",
			state: &index.SegmentWriteState{
				Directory:   directory,
				SegmentInfo: segmentInfo,
				FieldInfos:  nil,
			},
			wantErr: true,
			errMsg:  "state.FieldInfos cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := format.FieldsWriter(tt.state)
			if tt.wantErr {
				if err == nil {
					t.Errorf("FieldsWriter() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("FieldsWriter() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("FieldsWriter() unexpected error = %v", err)
					return
				}
				if writer == nil {
					t.Error("FieldsWriter() returned nil writer")
					return
				}
				// Clean up
				writer.Close()
			}
		})
	}
}

func TestSpatialIndexFormatImpl_FieldsReader(t *testing.T) {
	format := NewSpatialIndexFormat()

	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()

	state := &index.SegmentReadState{
		Directory:   directory,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	tests := []struct {
		name    string
		state   *index.SegmentReadState
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid state",
			state:   state,
			wantErr: false,
		},
		{
			name:    "nil state",
			state:   nil,
			wantErr: true,
			errMsg:  "state cannot be nil",
		},
		{
			name: "nil directory",
			state: &index.SegmentReadState{
				Directory:   nil,
				SegmentInfo: segmentInfo,
				FieldInfos:  fieldInfos,
			},
			wantErr: true,
			errMsg:  "state.Directory cannot be nil",
		},
		{
			name: "nil segmentInfo",
			state: &index.SegmentReadState{
				Directory:   directory,
				SegmentInfo: nil,
				FieldInfos:  fieldInfos,
			},
			wantErr: true,
			errMsg:  "state.SegmentInfo cannot be nil",
		},
		{
			name: "nil fieldInfos",
			state: &index.SegmentReadState{
				Directory:   directory,
				SegmentInfo: segmentInfo,
				FieldInfos:  nil,
			},
			wantErr: true,
			errMsg:  "state.FieldInfos cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := format.FieldsReader(tt.state)
			if tt.wantErr {
				if err == nil {
					t.Errorf("FieldsReader() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("FieldsReader() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("FieldsReader() unexpected error = %v", err)
					return
				}
				if reader == nil {
					t.Error("FieldsReader() returned nil reader")
					return
				}
				// Clean up
				reader.Close()
			}
		})
	}
}

func TestSpatialIndexFileNames(t *testing.T) {
	names := NewSpatialIndexFileNames("_0")

	// Test GetIndexFileName
	indexFileName := names.GetIndexFileName()
	if indexFileName != "_0.spatial" {
		t.Errorf("GetIndexFileName() = %s, want _0.spatial", indexFileName)
	}

	// Test GetMetadataFileName
	metadataFileName := names.GetMetadataFileName()
	if metadataFileName != "_0.spatial_meta" {
		t.Errorf("GetMetadataFileName() = %s, want _0.spatial_meta", metadataFileName)
	}

	// Test GetFileNames
	fileNames := names.GetFileNames()
	if len(fileNames) != 2 {
		t.Errorf("GetFileNames() returned %d files, want 2", len(fileNames))
	}
	if fileNames[0] != "_0.spatial" {
		t.Errorf("GetFileNames()[0] = %s, want _0.spatial", fileNames[0])
	}
	if fileNames[1] != "_0.spatial_meta" {
		t.Errorf("GetFileNames()[1] = %s, want _0.spatial_meta", fileNames[1])
	}
}

func TestWriteAndReadSpatialIndexFileHeader(t *testing.T) {
	directory := store.NewByteBuffersDirectory()

	// Create output
	out, err := directory.CreateOutput("test.spatial", store.IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Write header
	err = WriteSpatialIndexFileHeader(out)
	if err != nil {
		t.Errorf("WriteSpatialIndexFileHeader() error = %v", err)
	}

	// Close output
	out.Close()

	// Create input
	in, err := directory.OpenInput("test.spatial", store.IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	// Read header
	header, err := ReadSpatialIndexFileHeader(in)
	if err != nil {
		t.Errorf("ReadSpatialIndexFileHeader() error = %v", err)
	}
	if header == nil {
		t.Error("ReadSpatialIndexFileHeader() returned nil header")
		return
	}
	if header.Magic != SpatialIndexFileMagic {
		t.Errorf("Header.Magic = 0x%08x, want 0x%08x", header.Magic, SpatialIndexFileMagic)
	}
	if header.Version != SpatialIndexFileHeaderCurrentVersion {
		t.Errorf("Header.Version = %d, want %d", header.Version, SpatialIndexFileHeaderCurrentVersion)
	}
}

func TestReadSpatialIndexFileHeader_InvalidMagic(t *testing.T) {
	directory := store.NewByteBuffersDirectory()

	// Create output with invalid data
	out, err := directory.CreateOutput("test.spatial", store.IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Write invalid magic
	store.WriteUint32(out, 0xDEADBEEF)
	out.Close()

	// Create input
	in, err := directory.OpenInput("test.spatial", store.IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	// Try to read header
	_, err = ReadSpatialIndexFileHeader(in)
	if err == nil {
		t.Error("ReadSpatialIndexFileHeader() should error for invalid magic")
	}
}

func TestWriteAndReadSpatialIndexMetadata(t *testing.T) {
	directory := store.NewByteBuffersDirectory()

	// Create output
	out, err := directory.CreateOutput("test_meta", store.IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Write metadata
	metadata := &SpatialIndexMetadata{
		NumFields:  2,
		FieldNames: []string{"location", "area"},
		DocCount:   100,
	}
	err = WriteSpatialIndexMetadata(out, metadata)
	if err != nil {
		t.Errorf("WriteSpatialIndexMetadata() error = %v", err)
	}
	out.Close()

	// Create input
	in, err := directory.OpenInput("test_meta", store.IOContext{})
	if err != nil {
		t.Fatalf("Failed to open input: %v", err)
	}
	defer in.Close()

	// Read metadata
	readMetadata, err := ReadSpatialIndexMetadata(in)
	if err != nil {
		t.Errorf("ReadSpatialIndexMetadata() error = %v", err)
	}
	if readMetadata == nil {
		t.Error("ReadSpatialIndexMetadata() returned nil metadata")
		return
	}
	if readMetadata.NumFields != 2 {
		t.Errorf("NumFields = %d, want 2", readMetadata.NumFields)
	}
	if len(readMetadata.FieldNames) != 2 {
		t.Errorf("len(FieldNames) = %d, want 2", len(readMetadata.FieldNames))
	}
	if readMetadata.FieldNames[0] != "location" {
		t.Errorf("FieldNames[0] = %s, want location", readMetadata.FieldNames[0])
	}
	if readMetadata.FieldNames[1] != "area" {
		t.Errorf("FieldNames[1] = %s, want area", readMetadata.FieldNames[1])
	}
	if readMetadata.DocCount != 100 {
		t.Errorf("DocCount = %d, want 100", readMetadata.DocCount)
	}
}

func TestWriteSpatialIndexMetadata_NilMetadata(t *testing.T) {
	directory := store.NewByteBuffersDirectory()

	// Create output
	out, err := directory.CreateOutput("test_meta", store.IOContext{})
	if err != nil {
		t.Fatalf("Failed to create output: %v", err)
	}

	// Try to write nil metadata
	err = WriteSpatialIndexMetadata(out, nil)
	if err == nil {
		t.Error("WriteSpatialIndexMetadata() should error for nil metadata")
	}
	out.Close()
}

func TestBaseSpatialIndexFormat(t *testing.T) {
	base := NewBaseSpatialIndexFormat("TestFormat")

	// Test Name
	if base.Name() != "TestFormat" {
		t.Errorf("Name() = %s, want TestFormat", base.Name())
	}

	// Test FieldsWriter (should return error)
	_, err := base.FieldsWriter(nil)
	if err == nil {
		t.Error("FieldsWriter() should return error")
	}

	// Test FieldsReader (should return error)
	_, err = base.FieldsReader(nil)
	if err == nil {
		t.Error("FieldsReader() should return error")
	}
}

func TestSpatialIndexFormatImpl_Integration(t *testing.T) {
	format := NewSpatialIndexFormat()

	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("_0", 10, directory)
	fieldInfos := index.NewFieldInfos()

	// Test writer creation
	writeState := &index.SegmentWriteState{
		Directory:   directory,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	writer, err := format.FieldsWriter(writeState)
	if err != nil {
		t.Fatalf("FieldsWriter() error = %v", err)
	}
	if writer == nil {
		t.Fatal("FieldsWriter() returned nil writer")
	}

	// Register a strategy
	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	err = writer.RegisterStrategy("location", strategy)
	if err != nil {
		t.Errorf("RegisterStrategy() error = %v", err)
	}

	// Write a document
	writer.StartDocument()
	point := NewPoint(-122.0, 37.0)
	_, err = writer.WriteSpatialField("location", point)
	if err != nil {
		t.Errorf("WriteSpatialField() error = %v", err)
	}
	writer.FinishDocument()

	// Close writer
	writer.Close()

	// Test reader creation
	readState := &index.SegmentReadState{
		Directory:   directory,
		SegmentInfo: segmentInfo,
		FieldInfos:  fieldInfos,
	}

	reader, err := format.FieldsReader(readState)
	if err != nil {
		t.Fatalf("FieldsReader() error = %v", err)
	}
	if reader == nil {
		t.Fatal("FieldsReader() returned nil reader")
	}

	// Register strategy on reader
	err = reader.RegisterStrategy("location", strategy)
	if err != nil {
		t.Errorf("RegisterStrategy() error = %v", err)
	}

	// Make a query
	query, err := reader.MakeQuery("location", SpatialOperationIntersects, point)
	if err != nil {
		t.Errorf("MakeQuery() error = %v", err)
	}
	if query == nil {
		t.Error("MakeQuery() returned nil query")
	}

	// Close reader
	reader.Close()
}
