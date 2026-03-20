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

func TestNewSpatialIndexWriter(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
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
			writer, err := NewSpatialIndexWriter(tt.directory, tt.segmentInfo, tt.fieldInfos)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewSpatialIndexWriter() expected error, got nil")
					return
				}
				if err.Error() != tt.errMsg {
					t.Errorf("NewSpatialIndexWriter() error = %v, want %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("NewSpatialIndexWriter() unexpected error = %v", err)
					return
				}
				if writer == nil {
					t.Error("NewSpatialIndexWriter() returned nil writer")
					return
				}
				if writer.directory != tt.directory {
					t.Error("writer.directory mismatch")
				}
				if writer.segmentInfo != tt.segmentInfo {
					t.Error("writer.segmentInfo mismatch")
				}
				if writer.fieldInfos != tt.fieldInfos {
					t.Error("writer.fieldInfos mismatch")
				}
				if writer.closed {
					t.Error("writer should not be closed")
				}
				if writer.currentDoc != -1 {
					t.Errorf("writer.currentDoc = %d, want -1", writer.currentDoc)
				}
				if writer.docCount != 0 {
					t.Errorf("writer.docCount = %d, want 0", writer.docCount)
				}
			}
		})
	}
}

func TestSpatialIndexWriter_RegisterStrategy(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	tests := []struct {
		name      string
		fieldName string
		strategy  SpatialStrategy
		wantErr   bool
		errMsg    string
		preSetup  func()
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
			if tt.preSetup != nil {
				tt.preSetup()
			}

			err := writer.RegisterStrategy(tt.fieldName, tt.strategy)
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

func TestSpatialIndexWriter_GetStrategy(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Register a strategy
	writer.RegisterStrategy("location", strategy)

	// Test getting existing strategy
	got := writer.GetStrategy("location")
	if got == nil {
		t.Error("GetStrategy() returned nil for existing field")
	}
	if got != strategy {
		t.Error("GetStrategy() returned wrong strategy")
	}

	// Test getting non-existent strategy
	got = writer.GetStrategy("nonexistent")
	if got != nil {
		t.Error("GetStrategy() should return nil for non-existent field")
	}
}

func TestSpatialIndexWriter_HasStrategy(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)

	// Initially should not have strategy
	if writer.HasStrategy("location") {
		t.Error("HasStrategy() should return false for unregistered field")
	}

	// Register strategy
	writer.RegisterStrategy("location", strategy)

	// Now should have strategy
	if !writer.HasStrategy("location") {
		t.Error("HasStrategy() should return true for registered field")
	}
}

func TestSpatialIndexWriter_DocumentLifecycle(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	// Test StartDocument
	err := writer.StartDocument()
	if err != nil {
		t.Errorf("StartDocument() error = %v", err)
	}
	if writer.GetCurrentDoc() != 0 {
		t.Errorf("GetCurrentDoc() = %d, want 0", writer.GetCurrentDoc())
	}

	// Test FinishDocument
	err = writer.FinishDocument()
	if err != nil {
		t.Errorf("FinishDocument() error = %v", err)
	}
	if writer.GetDocCount() != 1 {
		t.Errorf("GetDocCount() = %d, want 1", writer.GetDocCount())
	}

	// Test multiple documents
	for i := 0; i < 5; i++ {
		writer.StartDocument()
		writer.FinishDocument()
	}
	if writer.GetDocCount() != 6 {
		t.Errorf("GetDocCount() = %d, want 6", writer.GetDocCount())
	}
	if writer.GetCurrentDoc() != 5 {
		t.Errorf("GetCurrentDoc() = %d, want 5", writer.GetCurrentDoc())
	}
}

func TestSpatialIndexWriter_WriteSpatialField(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	writer.RegisterStrategy("location", strategy)

	point := NewPoint(-122.0, 37.0)

	// Test writing spatial field
	fields, err := writer.WriteSpatialField("location", point)
	if err != nil {
		t.Errorf("WriteSpatialField() error = %v", err)
	}
	if len(fields) == 0 {
		t.Error("WriteSpatialField() returned no fields")
	}

	// Test writing to unregistered field
	_, err = writer.WriteSpatialField("unregistered", point)
	if err == nil {
		t.Error("WriteSpatialField() should error for unregistered field")
	}

	// Test writing nil shape
	_, err = writer.WriteSpatialField("location", nil)
	if err == nil {
		t.Error("WriteSpatialField() should error for nil shape")
	}
}

func TestSpatialIndexWriter_WriteShape(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	writer.RegisterStrategy("location", strategy)

	point := NewPoint(-122.0, 37.0)

	// Test WriteShape
	fields, err := writer.WriteShape("location", point)
	if err != nil {
		t.Errorf("WriteShape() error = %v", err)
	}
	if len(fields) == 0 {
		t.Error("WriteShape() returned no fields")
	}
	if writer.GetDocCount() != 1 {
		t.Errorf("GetDocCount() = %d, want 1", writer.GetDocCount())
	}

	// Test WriteShape with unregistered field
	_, err = writer.WriteShape("unregistered", point)
	if err == nil {
		t.Error("WriteShape() should error for unregistered field")
	}
}

func TestSpatialIndexWriter_Close(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	// Test initial state
	if writer.IsClosed() {
		t.Error("IsClosed() should return false initially")
	}

	// Test close
	err := writer.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
	if !writer.IsClosed() {
		t.Error("IsClosed() should return true after Close()")
	}

	// Test double close
	err = writer.Close()
	if err == nil {
		t.Error("Close() should error when already closed")
	}

	// Test operations after close
	err = writer.StartDocument()
	if err == nil {
		t.Error("StartDocument() should error after close")
	}

	err = writer.FinishDocument()
	if err == nil {
		t.Error("FinishDocument() should error after close")
	}

	err = writer.RegisterStrategy("test", nil)
	if err == nil {
		t.Error("RegisterStrategy() should error after close")
	}
}

func TestSpatialIndexWriter_Getters(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	// Test GetDirectory
	if writer.GetDirectory() != directory {
		t.Error("GetDirectory() returned wrong directory")
	}

	// Test GetSegmentInfo
	if writer.GetSegmentInfo() != segmentInfo {
		t.Error("GetSegmentInfo() returned wrong segment info")
	}

	// Test GetFieldInfos
	if writer.GetFieldInfos() != fieldInfos {
		t.Error("GetFieldInfos() returned wrong field infos")
	}

	// Test GetCurrentDoc
	if writer.GetCurrentDoc() != -1 {
		t.Errorf("GetCurrentDoc() = %d, want -1", writer.GetCurrentDoc())
	}

	// Test GetDocCount
	if writer.GetDocCount() != 0 {
		t.Errorf("GetDocCount() = %d, want 0", writer.GetDocCount())
	}
}

func TestSpatialIndexWriter_GetRegisteredFields(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()

	// Initially should have no fields
	fields := writer.GetRegisteredFields()
	if len(fields) != 0 {
		t.Errorf("GetRegisteredFields() = %v, want empty", fields)
	}

	// Register some strategies
	strategy1, _ := NewPointVectorStrategy("location1", ctx)
	strategy2, _ := NewPointVectorStrategy("location2", ctx)

	writer.RegisterStrategy("location1", strategy1)
	writer.RegisterStrategy("location2", strategy2)

	// Now should have 2 fields
	fields = writer.GetRegisteredFields()
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

func TestSpatialIndexWriter_ClearStrategies(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	writer.RegisterStrategy("location", strategy)

	// Verify strategy is registered
	if !writer.HasStrategy("location") {
		t.Error("Strategy should be registered")
	}

	// Clear strategies
	err := writer.ClearStrategies()
	if err != nil {
		t.Errorf("ClearStrategies() error = %v", err)
	}

	// Verify strategies are cleared
	if writer.HasStrategy("location") {
		t.Error("Strategies should be cleared")
	}

	// Test ClearStrategies on closed writer
	writer.Close()
	err = writer.ClearStrategies()
	if err == nil {
		t.Error("ClearStrategies() should error on closed writer")
	}
}

func TestSpatialIndexWriter_GetState(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	writer.RegisterStrategy("location", strategy)

	// Write some documents
	for i := 0; i < 3; i++ {
		writer.StartDocument()
		writer.FinishDocument()
	}

	state := writer.GetState()

	if state.CurrentDoc != 2 {
		t.Errorf("State.CurrentDoc = %d, want 2", state.CurrentDoc)
	}
	if state.DocCount != 3 {
		t.Errorf("State.DocCount = %d, want 3", state.DocCount)
	}
	if state.Closed {
		t.Error("State.Closed should be false")
	}
	if len(state.RegisteredFields) != 1 {
		t.Errorf("len(State.RegisteredFields) = %d, want 1", len(state.RegisteredFields))
	}

	// Close and check state again
	writer.Close()
	state = writer.GetState()
	if !state.Closed {
		t.Error("State.Closed should be true after Close()")
	}
}

func TestSpatialIndexWriter_ConcurrentAccess(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()
	strategy, _ := NewPointVectorStrategy("location", ctx)
	writer.RegisterStrategy("location", strategy)

	// Test concurrent document writing
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			point := NewPoint(-122.0+float64(i), 37.0+float64(i))
			writer.WriteShape("location", point)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	if writer.GetDocCount() != 10 {
		t.Errorf("GetDocCount() = %d, want 10", writer.GetDocCount())
	}
}

func TestSpatialIndexWriter_WithDifferentStrategies(t *testing.T) {
	directory := store.NewByteBuffersDirectory()
	segmentInfo := index.NewSegmentInfo("test_segment", 1, directory)
	fieldInfos := index.NewFieldInfos()
	writer, _ := NewSpatialIndexWriter(directory, segmentInfo, fieldInfos)

	ctx := NewSpatialContext()

	// Register different strategies
	pointStrategy, _ := NewPointVectorStrategy("points", ctx)
	bboxStrategy, _ := NewBBoxStrategy("bbox", ctx)

	writer.RegisterStrategy("points", pointStrategy)
	writer.RegisterStrategy("bbox", bboxStrategy)

	// Test writing with point strategy
	point := NewPoint(-122.0, 37.0)
	pointFields, err := writer.WriteShape("points", point)
	if err != nil {
		t.Errorf("WriteShape(points) error = %v", err)
	}
	if len(pointFields) == 0 {
		t.Error("WriteShape(points) returned no fields")
	}

	// Test writing with bbox strategy
	rect := NewRectangle(-123.0, 36.0, -121.0, 38.0)
	bboxFields, err := writer.WriteShape("bbox", rect)
	if err != nil {
		t.Errorf("WriteShape(bbox) error = %v", err)
	}
	if len(bboxFields) == 0 {
		t.Error("WriteShape(bbox) returned no fields")
	}

	// Verify document count
	if writer.GetDocCount() != 2 {
		t.Errorf("GetDocCount() = %d, want 2", writer.GetDocCount())
	}
}
