// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"testing"
)

// TestBlockState_Basic tests basic BlockState creation
func TestBlockState_Basic(t *testing.T) {
	state := NewBlockState(1, 0)

	if state.ChunkID != 1 {
		t.Errorf("expected ChunkID 1, got %d", state.ChunkID)
	}

	if state.StartDocID != 0 {
		t.Errorf("expected StartDocID 0, got %d", state.StartDocID)
	}

	if state.NumDocs != 0 {
		t.Errorf("expected NumDocs 0, got %d", state.NumDocs)
	}

	if !state.IsEmpty() {
		t.Error("expected IsEmpty to be true")
	}
}

// TestBlockState_AddDocument tests AddDocument
func TestBlockState_AddDocument(t *testing.T) {
	state := NewBlockState(1, 0)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test Document"},
		{fieldType: fieldTypeString, name: "content", value: "This is a test"},
	}

	err := state.AddDocument(doc)
	if err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}

	if state.NumDocs != 1 {
		t.Errorf("expected NumDocs 1, got %d", state.NumDocs)
	}

	if state.IsEmpty() {
		t.Error("expected IsEmpty to be false")
	}

	// Check fields tracking
	if !state.HasField("title") {
		t.Error("expected HasField('title') to be true")
	}

	if !state.HasField("content") {
		t.Error("expected HasField('content') to be true")
	}

	if state.HasField("nonexistent") {
		t.Error("expected HasField('nonexistent') to be false")
	}
}

// TestBlockState_GetDocument tests GetDocument
func TestBlockState_GetDocument(t *testing.T) {
	state := NewBlockState(1, 0)

	doc1 := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Doc 1"},
	}

	doc2 := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Doc 2"},
	}

	state.AddDocument(doc1)
	state.AddDocument(doc2)

	// Get first document
	retrieved, err := state.GetDocument(0)
	if err != nil {
		t.Errorf("GetDocument(0) failed: %v", err)
	}
	if len(retrieved) != 1 || retrieved[0].value != "Doc 1" {
		t.Errorf("expected Doc 1, got %v", retrieved)
	}

	// Get second document
	retrieved, err = state.GetDocument(1)
	if err != nil {
		t.Errorf("GetDocument(1) failed: %v", err)
	}
	if len(retrieved) != 1 || retrieved[0].value != "Doc 2" {
		t.Errorf("expected Doc 2, got %v", retrieved)
	}
}

// TestBlockState_GetDocument_OutOfRange tests out of range access
func TestBlockState_GetDocument_OutOfRange(t *testing.T) {
	state := NewBlockState(1, 0)

	_, err := state.GetDocument(0)
	if err == nil {
		t.Error("expected error for out of range doc")
	}

	_, err = state.GetDocument(-1)
	if err == nil {
		t.Error("expected error for negative doc offset")
	}
}

// TestBlockState_GetDocID tests GetDocID
func TestBlockState_GetDocID(t *testing.T) {
	state := NewBlockState(1, 100)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}

	state.AddDocument(doc)
	state.AddDocument(doc)

	docID, err := state.GetDocID(0)
	if err != nil {
		t.Errorf("GetDocID(0) failed: %v", err)
	}
	if docID != 100 {
		t.Errorf("expected docID 100, got %d", docID)
	}

	docID, err = state.GetDocID(1)
	if err != nil {
		t.Errorf("GetDocID(1) failed: %v", err)
	}
	if docID != 101 {
		t.Errorf("expected docID 101, got %d", docID)
	}
}

// TestBlockState_GetDocumentOffset tests GetDocumentOffset
func TestBlockState_GetDocumentOffset(t *testing.T) {
	state := NewBlockState(1, 100)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}

	state.AddDocument(doc)
	state.AddDocument(doc)

	offset := state.GetDocumentOffset(100)
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}

	offset = state.GetDocumentOffset(101)
	if offset != 1 {
		t.Errorf("expected offset 1, got %d", offset)
	}

	offset = state.GetDocumentOffset(200)
	if offset != -1 {
		t.Errorf("expected offset -1, got %d", offset)
	}
}

// TestBlockState_GetEndDocID tests GetEndDocID
func TestBlockState_GetEndDocID(t *testing.T) {
	state := NewBlockState(1, 100)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}

	state.AddDocument(doc)
	state.AddDocument(doc)
	state.AddDocument(doc)

	endDocID := state.GetEndDocID()
	if endDocID != 103 {
		t.Errorf("expected endDocID 103, got %d", endDocID)
	}
}

// TestBlockState_IsFull tests IsFull
func TestBlockState_IsFull(t *testing.T) {
	state := NewBlockState(1, 0)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}

	// Add 5 documents
	for i := 0; i < 5; i++ {
		state.AddDocument(doc)
	}

	// Should not be full with max 10 docs
	if state.IsFull(10, 1024) {
		t.Error("expected IsFull to be false with max 10 docs")
	}

	// Should be full with max 5 docs
	if !state.IsFull(5, 1024) {
		t.Error("expected IsFull to be true with max 5 docs")
	}

	// Should be full with max 0 docs
	if !state.IsFull(0, 1024) {
		t.Error("expected IsFull to be true with max 0 docs")
	}
}

// TestBlockState_GetFields tests GetFields
func TestBlockState_GetFields(t *testing.T) {
	state := NewBlockState(1, 0)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
		{fieldType: fieldTypeString, name: "content", value: "Content"},
	}

	state.AddDocument(doc)

	fields := state.GetFields()
	if len(fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(fields))
	}
}

// TestBlockState_GetNumFields tests GetNumFields
func TestBlockState_GetNumFields(t *testing.T) {
	state := NewBlockState(1, 0)

	if state.GetNumFields() != 0 {
		t.Errorf("expected 0 fields initially, got %d", state.GetNumFields())
	}

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
		{fieldType: fieldTypeString, name: "content", value: "Content"},
	}

	state.AddDocument(doc)

	if state.GetNumFields() != 2 {
		t.Errorf("expected 2 fields, got %d", state.GetNumFields())
	}
}

// TestBlockState_GetTotalFieldValues tests GetTotalFieldValues
func TestBlockState_GetTotalFieldValues(t *testing.T) {
	state := NewBlockState(1, 0)

	doc1 := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test 1"},
	}

	doc2 := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test 2"},
		{fieldType: fieldTypeString, name: "content", value: "Content 2"},
	}

	state.AddDocument(doc1)
	state.AddDocument(doc2)

	total := state.GetTotalFieldValues()
	if total != 3 {
		t.Errorf("expected 3 total field values, got %d", total)
	}
}

// TestBlockState_CompressionRatio tests GetCompressionRatio
func TestBlockState_CompressionRatio(t *testing.T) {
	state := NewBlockState(1, 0)

	// Initially should be 0
	ratio := state.GetCompressionRatio()
	if ratio != 0.0 {
		t.Errorf("expected ratio 0.0, got %f", ratio)
	}

	state.SetUncompressedLength(1000)
	state.SetCompressedLength(500)

	ratio = state.GetCompressionRatio()
	if ratio != 0.5 {
		t.Errorf("expected ratio 0.5, got %f", ratio)
	}
}

// TestBlockState_Close tests Close
func TestBlockState_Close(t *testing.T) {
	state := NewBlockState(1, 0)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}

	state.AddDocument(doc)

	err := state.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// After close, operations should fail
	err = state.AddDocument(doc)
	if err == nil {
		t.Error("expected error after close")
	}

	_, err = state.GetDocument(0)
	if err == nil {
		t.Error("expected error after close")
	}
}

// TestBlockState_Clone tests Clone
func TestBlockState_Clone(t *testing.T) {
	state := NewBlockState(1, 100)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}

	state.AddDocument(doc)
	state.SetUncompressedLength(100)
	state.SetCompressedLength(50)

	clone := state.Clone()

	if clone.ChunkID != state.ChunkID {
		t.Error("ChunkID mismatch")
	}

	if clone.StartDocID != state.StartDocID {
		t.Error("StartDocID mismatch")
	}

	if clone.NumDocs != state.NumDocs {
		t.Error("NumDocs mismatch")
	}

	if clone.UncompressedLength != state.UncompressedLength {
		t.Error("UncompressedLength mismatch")
	}

	if clone.CompressedLength != state.CompressedLength {
		t.Error("CompressedLength mismatch")
	}
}

// TestBlockStatePool tests BlockStatePool
func TestBlockStatePool(t *testing.T) {
	pool := NewBlockStatePool()

	// Get a state from the pool
	state := pool.Get()
	if state == nil {
		t.Fatal("expected non-nil state from pool")
	}

	// Use the state
	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}
	state.AddDocument(doc)

	// Return to pool
	pool.Put(state)

	// Get another state - should be reset
	state2 := pool.Get()
	if state2 == nil {
		t.Fatal("expected non-nil state from pool")
	}

	if !state2.IsEmpty() {
		t.Error("expected state from pool to be empty")
	}
}

// TestBlockStatePool_Global tests global pool functions
func TestBlockStatePool_Global(t *testing.T) {
	state := GetBlockStateFromPool()
	if state == nil {
		t.Fatal("expected non-nil state from global pool")
	}

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}
	state.AddDocument(doc)

	PutBlockStateToPool(state)
}

// TestBlockState_Serialize tests Serialize and Deserialize
func TestBlockState_Serialize(t *testing.T) {
	state := NewBlockState(1, 100)

	doc1 := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test Document"},
	}

	doc2 := []storedField{
		{fieldType: fieldTypeString, name: "content", value: "This is content"},
	}

	state.AddDocument(doc1)
	state.AddDocument(doc2)

	data, err := state.Serialize()
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty serialized data")
	}

	// Deserialize
	state2, err := DeserializeBlockState(data, 1)
	if err != nil {
		t.Fatalf("DeserializeBlockState failed: %v", err)
	}

	if state2.StartDocID != state.StartDocID {
		t.Errorf("expected StartDocID %d, got %d", state.StartDocID, state2.StartDocID)
	}

	if state2.NumDocs != state.NumDocs {
		t.Errorf("expected NumDocs %d, got %d", state.NumDocs, state2.NumDocs)
	}
}

// TestBlockState_GetFieldInfo tests GetFieldInfo
func TestBlockState_GetFieldInfo(t *testing.T) {
	state := NewBlockState(1, 0)

	doc := []storedField{
		{fieldType: fieldTypeString, name: "title", value: "Test"},
	}

	state.AddDocument(doc)

	info, err := state.GetFieldInfo("title")
	if err != nil {
		t.Errorf("GetFieldInfo failed: %v", err)
	}

	if info.Name != "title" {
		t.Errorf("expected field name 'title', got '%s'", info.Name)
	}

	if info.NumValues != 1 {
		t.Errorf("expected NumValues 1, got %d", info.NumValues)
	}

	// Non-existent field
	_, err = state.GetFieldInfo("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent field")
	}
}
