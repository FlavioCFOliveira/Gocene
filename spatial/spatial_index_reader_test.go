// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNewSpatialIndexReader(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()

	tests := []struct {
		name        string
		directory   store.Directory
		segmentInfo *index.SegmentInfo
		fieldInfos  *index.FieldInfos
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "valid parameters",
			directory:   directory,
			segmentInfo: segmentInfo,
			fieldInfos:  fieldInfos,
			wantErr:     false,
		},
		{
			name:        "nil directory",
			directory:   nil,
			segmentInfo: segmentInfo,
			fieldInfos:  fieldInfos,
			wantErr:     true,
			errMsg:      "directory cannot be nil",
		},
		{
			name:        "nil segmentInfo",
			directory:   directory,
			segmentInfo: nil,
			fieldInfos:  fieldInfos,
			wantErr:     true,
			errMsg:      "segmentInfo cannot be nil",
		},
		{
			name:        "nil fieldInfos",
			directory:   directory,
			segmentInfo: segmentInfo,
			fieldInfos:  nil,
			wantErr:     true,
			errMsg:      "fieldInfos cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := NewSpatialIndexReader(tt.directory, tt.segmentInfo, tt.fieldInfos)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewSpatialIndexReader() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("NewSpatialIndexReader() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("NewSpatialIndexReader() unexpected error = %v", err)
					return
				}
				if reader == nil {
					t.Error("NewSpatialIndexReader() returned nil reader")
					return
				}
				if reader.directory != tt.directory {
					t.Error("reader.directory mismatch")
				}
				if reader.segmentInfo != tt.segmentInfo {
					t.Error("reader.segmentInfo mismatch")
				}
				if reader.fieldInfos != tt.fieldInfos {
					t.Error("reader.fieldInfos mismatch")
				}
				if reader.closed {
					t.Error("reader should not be closed")
				}
				if reader.docCount != 10 {
					t.Errorf("reader.docCount = %d, want 10", reader.docCount)
				}
			}
		})
	}
}

func TestSpatialIndexReader_RegisterStrategy(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	tests := []struct {
		name      string
		fieldName string
		strategy  SpatialStrategy
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid registration",
			fieldName: "location",
			strategy:  strategy,
			wantErr:   false,
		},
		{
			name:      "nil strategy",
			fieldName: "test",
			strategy:  nil,
			wantErr:   true,
			errMsg:    "strategy cannot be nil",
		},
		{
			name:      "duplicate registration",
			fieldName: "location",
			strategy:  strategy,
			wantErr:   true,
			errMsg:    "strategy already registered for field \"location\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reader.RegisterStrategy(tt.fieldName, tt.strategy)
			if tt.wantErr {
				if err == nil {
					t.Errorf("RegisterStrategy() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("RegisterStrategy() error = %v, want containing %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("RegisterStrategy() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestSpatialIndexReader_GetStrategy(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Register a strategy
	reader.RegisterStrategy("location", strategy)

	// Test getting existing strategy
	got := reader.GetStrategy("location")
	if got == nil {
		t.Error("GetStrategy() returned nil for existing field")
	}
	if got != strategy {
		t.Error("GetStrategy() returned wrong strategy")
	}

	// Test getting non-existent strategy
	got = reader.GetStrategy("nonexistent")
	if got != nil {
		t.Error("GetStrategy() should return nil for non-existent field")
	}
}

func TestSpatialIndexReader_HasStrategy(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Initially should not have strategy
	if reader.HasStrategy("location") {
		t.Error("HasStrategy() should return false for unregistered field")
	}

	// Register strategy
	reader.RegisterStrategy("location", strategy)

	// Now should have strategy
	if !reader.HasStrategy("location") {
		t.Error("HasStrategy() should return true for registered field")
	}
}

func TestSpatialIndexReader_MakeQuery(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	reader.RegisterStrategy("location", strategy)

	point := NewPoint(-122.0, 37.0)

	// Test making query
	query, err := reader.MakeQuery("location", SpatialOperationIntersects, point)
	if err != nil {
		t.Errorf("MakeQuery() error = %v", err)
	}
	if query == nil {
		t.Error("MakeQuery() returned nil query")
	}

	// Test making query for unregistered field
	_, err = reader.MakeQuery("unregistered", SpatialOperationIntersects, point)
	if err == nil {
		t.Error("MakeQuery() should error for unregistered field")
	}

	// Test making query with nil shape
	_, err = reader.MakeQuery("location", SpatialOperationIntersects, nil)
	if err == nil {
		t.Error("MakeQuery() should error for nil shape")
	}
}

func TestSpatialIndexReader_MakeDistanceValueSource(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	reader.RegisterStrategy("location", strategy)

	point := NewPoint(-122.0, 37.0)

	// Test making distance value source
	// Note: This currently returns an error because the full ValueSource integration is not complete
	_, err := reader.MakeDistanceValueSource("location", point, 1.0)
	// We expect an error because the ValueSource integration is not complete
	if err == nil {
		t.Error("MakeDistanceValueSource() should return error (not fully implemented)")
	}

	// Test with unregistered field
	_, err = reader.MakeDistanceValueSource("unregistered", point, 1.0)
	if err == nil {
		t.Error("MakeDistanceValueSource() should error for unregistered field")
	}
}

func TestSpatialIndexReader_Close(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	// Test initial state
	if reader.IsClosed() {
		t.Error("IsClosed() should return false initially")
	}

	// Test close
	err := reader.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if !reader.IsClosed() {
		t.Error("IsClosed() should return true after Close()")
	}

	// Test double close
	err = reader.Close()
	if err == nil {
		t.Error("Close() should error when already closed")
	}

	// Test operations after close
	_, err = reader.MakeQuery("test", SpatialOperationIntersects, NewPoint(0, 0))
	if err == nil {
		t.Error("MakeQuery() should error after close")
	}

	err = reader.RegisterStrategy("test", nil)
	if err == nil {
		t.Error("RegisterStrategy() should error after close")
	}
}

func TestSpatialIndexReader_Getters(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 25, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	// Test GetDirectory
	if reader.GetDirectory() != directory {
		t.Error("GetDirectory() returned wrong directory")
	}

	// Test GetSegmentInfo
	if reader.GetSegmentInfo() != segmentInfo {
		t.Error("GetSegmentInfo() returned wrong segment info")
	}

	// Test GetFieldInfos
	if reader.GetFieldInfos() != fieldInfos {
		t.Error("GetFieldInfos() returned wrong field infos")
	}

	// Test GetDocCount
	if reader.GetDocCount() != 25 {
		t.Errorf("GetDocCount() = %d, want 25", reader.GetDocCount())
	}
}

func TestSpatialIndexReader_GetRegisteredFields(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()

	// Initially should have no fields
	fields := reader.GetRegisteredFields()
	if len(fields) != 0 {
		t.Errorf("GetRegisteredFields() = %v, want empty", fields)
	}

	// Register some strategies
	strategy1, _ := NewPointVectorStrategy("location1", ctx)
	strategy2, _ := NewPointVectorStrategy("location2", ctx)

	reader.RegisterStrategy("location1", strategy1)
	reader.RegisterStrategy("location2", strategy2)

	// Now should have 2 fields
	fields = reader.GetRegisteredFields()
	if len(fields) != 2 {
		t.Errorf("GetRegisteredFields() returned %d fields, want 2", len(fields))
	}

	// Check that both fields are present
	fieldMap := make(map[string]bool)
	for _, f := range fields {
		fieldMap[f] = true
	}
	if !fieldMap["location1"] || !fieldMap["location2"] {
		t.Error("GetRegisteredFields() missing expected fields")
	}
}

func TestSpatialIndexReader_ClearStrategies(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	reader.RegisterStrategy("location", strategy)

	// Verify strategy is registered
	if !reader.HasStrategy("location") {
		t.Error("Strategy should be registered")
	}

	// Clear strategies
	err := reader.ClearStrategies()
	if err != nil {
		t.Errorf("ClearStrategies() error = %v", err)
	}

	// Verify strategies are cleared
	if reader.HasStrategy("location") {
		t.Error("Strategies should be cleared")
	}

	// Test ClearStrategies on closed reader
	reader.Close()
	err = reader.ClearStrategies()
	if err == nil {
		t.Error("ClearStrategies() should error on closed reader")
	}
}

func TestSpatialIndexReader_GetState(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 50, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	reader.RegisterStrategy("location", strategy)

	state := reader.GetState()

	if state.DocCount != 50 {
		t.Errorf("State.DocCount = %d, want 50", state.DocCount)
	}
	if state.Closed {
		t.Error("State.Closed should be false")
	}
	if len(state.RegisteredFields) != 1 {
		t.Errorf("len(State.RegisteredFields) = %d, want 1", len(state.RegisteredFields))
	}

	// Close and check state again
	reader.Close()
	state = reader.GetState()
	if !state.Closed {
		t.Error("State.Closed should be true after Close()")
	}
}

func TestSpatialIndexReader_CheckIntegrity(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	reader.RegisterStrategy("location", strategy)

	// Test integrity check
	err := reader.CheckIntegrity()
	if err != nil {
		t.Errorf("CheckIntegrity() error = %v", err)
	}

	// Test integrity check on closed reader
	reader.Close()
	err = reader.CheckIntegrity()
	if err == nil {
		t.Error("CheckIntegrity() should error on closed reader")
	}
}

func TestSpatialIndexReader_ConcurrentAccess(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 100, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	reader.RegisterStrategy("location", strategy)

	// Test concurrent query creation
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			point := NewPoint(-122.0+float64(idx), 37.0+float64(idx))
			_, err := reader.MakeQuery("location", SpatialOperationIntersects, point)
			if err != nil {
				t.Errorf("MakeQuery() error = %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestSpatialIndexReader_WithDifferentStrategies(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 10, directory)
	fieldInfos := index.NewFieldInfos()
	reader, _ := NewSpatialIndexReader(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()

	// Register different strategies
	pointStrategy, _ := NewPointVectorStrategy("points", ctx)
	bboxStrategy, _ := NewBBoxStrategy("bbox", ctx)

	reader.RegisterStrategy("points", pointStrategy)
	reader.RegisterStrategy("bbox", bboxStrategy)

	// Test making query with point strategy
	point := NewPoint(-122.0, 37.0)
	pointQuery, err := reader.MakeQuery("points", SpatialOperationIntersects, point)
	if err != nil {
		t.Errorf("MakeQuery(points) error = %v", err)
	}
	if pointQuery == nil {
		t.Error("MakeQuery(points) returned nil query")
	}

	// Test making query with bbox strategy
	rect := NewRectangle(-123.0, 36.0, -121.0, 38.0)
	bboxQuery, err := reader.MakeQuery("bbox", SpatialOperationIntersects, rect)
	if err != nil {
		t.Errorf("MakeQuery(bbox) error = %v", err)
	}
	if bboxQuery == nil {
		t.Error("MakeQuery(bbox) returned nil query")
	}
}
