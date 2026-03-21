// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

// TestNewFilterLeafReader tests creating a FilterLeafReader
func TestNewFilterLeafReader(t *testing.T) {
	// Create a minimal LeafReader for testing
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)

	// Create FilterLeafReader
	filterReader := NewFilterLeafReader(leafReader)
	if filterReader == nil {
		t.Fatal("Expected FilterLeafReader to be created")
	}

	// Test GetDelegate
	if filterReader.GetDelegate() != leafReader {
		t.Error("GetDelegate should return the wrapped reader")
	}
}

// TestFilterLeafReaderDocCount tests DocCount delegation
func TestFilterLeafReaderDocCount(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	if filterReader.DocCount() != 10 {
		t.Errorf("Expected DocCount 10, got %d", filterReader.DocCount())
	}
}

// TestFilterLeafReaderNumDocs tests NumDocs delegation
func TestFilterLeafReaderNumDocs(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// Default NumDocs is 0 since we haven't set it
	if filterReader.NumDocs() != 0 {
		t.Errorf("Expected NumDocs 0, got %d", filterReader.NumDocs())
	}
}

// TestFilterLeafReaderMaxDoc tests MaxDoc delegation
func TestFilterLeafReaderMaxDoc(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// MaxDoc returns DocCount for base LeafReader
	if filterReader.MaxDoc() != 10 {
		t.Errorf("Expected MaxDoc 10, got %d", filterReader.MaxDoc())
	}
}

// TestFilterLeafReaderHasDeletions tests HasDeletions delegation
func TestFilterLeafReaderHasDeletions(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// Default HasDeletions is false
	if filterReader.HasDeletions() {
		t.Error("Expected HasDeletions to be false")
	}
}

// TestFilterLeafReaderNumDeletedDocs tests NumDeletedDocs delegation
func TestFilterLeafReaderNumDeletedDocs(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// Default NumDeletedDocs is 0
	if filterReader.NumDeletedDocs() != 0 {
		t.Errorf("Expected NumDeletedDocs 0, got %d", filterReader.NumDeletedDocs())
	}
}

// TestFilterLeafReaderTerms tests Terms delegation
func TestFilterLeafReaderTerms(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// Terms returns nil for base LeafReader
	terms, err := filterReader.Terms("field")
	if err != nil {
		t.Fatalf("Terms returned error: %v", err)
	}
	if terms != nil {
		t.Error("Expected Terms to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderPostings tests Postings delegation
func TestFilterLeafReaderPostings(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	term := Term{Field: "test", Text: "value"}
	postings, err := filterReader.Postings(term)
	if err != nil {
		t.Fatalf("Postings returned error: %v", err)
	}
	if postings != nil {
		t.Error("Expected Postings to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetNumericDocValues tests GetNumericDocValues delegation
func TestFilterLeafReaderGetNumericDocValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetNumericDocValues returns nil for base LeafReader
	dv, err := filterReader.GetNumericDocValues("field")
	if err != nil {
		t.Fatalf("GetNumericDocValues returned error: %v", err)
	}
	if dv != nil {
		t.Error("Expected NumericDocValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetBinaryDocValues tests GetBinaryDocValues delegation
func TestFilterLeafReaderGetBinaryDocValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetBinaryDocValues returns nil for base LeafReader
	dv, err := filterReader.GetBinaryDocValues("field")
	if err != nil {
		t.Fatalf("GetBinaryDocValues returned error: %v", err)
	}
	if dv != nil {
		t.Error("Expected BinaryDocValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetSortedDocValues tests GetSortedDocValues delegation
func TestFilterLeafReaderGetSortedDocValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetSortedDocValues returns nil for base LeafReader
	dv, err := filterReader.GetSortedDocValues("field")
	if err != nil {
		t.Fatalf("GetSortedDocValues returned error: %v", err)
	}
	if dv != nil {
		t.Error("Expected SortedDocValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetSortedNumericDocValues tests GetSortedNumericDocValues delegation
func TestFilterLeafReaderGetSortedNumericDocValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetSortedNumericDocValues returns nil for base LeafReader
	dv, err := filterReader.GetSortedNumericDocValues("field")
	if err != nil {
		t.Fatalf("GetSortedNumericDocValues returned error: %v", err)
	}
	if dv != nil {
		t.Error("Expected SortedNumericDocValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetSortedSetDocValues tests GetSortedSetDocValues delegation
func TestFilterLeafReaderGetSortedSetDocValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetSortedSetDocValues returns nil for base LeafReader
	dv, err := filterReader.GetSortedSetDocValues("field")
	if err != nil {
		t.Fatalf("GetSortedSetDocValues returned error: %v", err)
	}
	if dv != nil {
		t.Error("Expected SortedSetDocValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetNormValues tests GetNormValues delegation
func TestFilterLeafReaderGetNormValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetNormValues returns nil for base LeafReader
	dv, err := filterReader.GetNormValues("field")
	if err != nil {
		t.Fatalf("GetNormValues returned error: %v", err)
	}
	if dv != nil {
		t.Error("Expected NormValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetPointValues tests GetPointValues delegation
func TestFilterLeafReaderGetPointValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetPointValues returns nil for base LeafReader
	pv, err := filterReader.GetPointValues("field")
	if err != nil {
		t.Fatalf("GetPointValues returned error: %v", err)
	}
	if pv != nil {
		t.Error("Expected PointValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetFloatVectorValues tests GetFloatVectorValues delegation
func TestFilterLeafReaderGetFloatVectorValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetFloatVectorValues returns nil for base LeafReader
	vv, err := filterReader.GetFloatVectorValues("field")
	if err != nil {
		t.Fatalf("GetFloatVectorValues returned error: %v", err)
	}
	if vv != nil {
		t.Error("Expected FloatVectorValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetByteVectorValues tests GetByteVectorValues delegation
func TestFilterLeafReaderGetByteVectorValues(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetByteVectorValues returns nil for base LeafReader
	vv, err := filterReader.GetByteVectorValues("field")
	if err != nil {
		t.Fatalf("GetByteVectorValues returned error: %v", err)
	}
	if vv != nil {
		t.Error("Expected ByteVectorValues to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderGetDocValuesSkipper tests GetDocValuesSkipper delegation
func TestFilterLeafReaderGetDocValuesSkipper(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// GetDocValuesSkipper returns nil for base LeafReader
	dvs, err := filterReader.GetDocValuesSkipper("field")
	if err != nil {
		t.Fatalf("GetDocValuesSkipper returned error: %v", err)
	}
	if dvs != nil {
		t.Error("Expected DocValuesSkipper to be nil for base LeafReader")
	}
}

// TestFilterLeafReaderCheckIntegrity tests CheckIntegrity delegation
func TestFilterLeafReaderCheckIntegrity(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// CheckIntegrity returns nil for base LeafReader
	err := filterReader.CheckIntegrity()
	if err != nil {
		t.Errorf("CheckIntegrity returned error: %v", err)
	}
}

// TestFilterLeafReaderIncRef tests IncRef delegation
func TestFilterLeafReaderIncRef(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	err := filterReader.IncRef()
	if err != nil {
		t.Errorf("IncRef returned error: %v", err)
	}

	// Check that ref count was incremented
	if filterReader.GetRefCount() != 2 {
		t.Errorf("Expected RefCount 2, got %d", filterReader.GetRefCount())
	}
}

// TestFilterLeafReaderDecRef tests DecRef delegation
func TestFilterLeafReaderDecRef(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	// First increment
	filterReader.IncRef()

	// Then decrement
	err := filterReader.DecRef()
	if err != nil {
		t.Errorf("DecRef returned error: %v", err)
	}

	// Check that ref count was decremented
	if filterReader.GetRefCount() != 1 {
		t.Errorf("Expected RefCount 1, got %d", filterReader.GetRefCount())
	}
}

// TestFilterLeafReaderTryIncRef tests TryIncRef delegation
func TestFilterLeafReaderTryIncRef(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	if !filterReader.TryIncRef() {
		t.Error("TryIncRef should return true for open reader")
	}

	// Check that ref count was incremented
	if filterReader.GetRefCount() != 2 {
		t.Errorf("Expected RefCount 2, got %d", filterReader.GetRefCount())
	}
}

// TestFilterLeafReaderEnsureOpen tests EnsureOpen delegation
func TestFilterLeafReaderEnsureOpen(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	err := filterReader.EnsureOpen()
	if err != nil {
		t.Errorf("EnsureOpen returned error: %v", err)
	}
}

// TestFilterLeafReaderGetMetaData tests GetMetaData delegation
func TestFilterLeafReaderGetMetaData(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	meta := filterReader.GetMetaData()
	if meta == nil {
		t.Fatal("GetMetaData returned nil")
	}

	if meta.MaxDoc != 10 {
		t.Errorf("Expected MaxDoc 10, got %d", meta.MaxDoc)
	}
}

// TestFilterLeafReaderGetSegmentInfo tests GetSegmentInfo delegation
func TestFilterLeafReaderGetSegmentInfo(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	si := filterReader.GetSegmentInfo()
	if si == nil {
		t.Fatal("GetSegmentInfo returned nil")
	}

	if si.name != "test_segment" {
		t.Errorf("Expected segment name 'test_segment', got '%s'", si.name)
	}
}

// TestFilterLeafReaderLeaves tests Leaves delegation
func TestFilterLeafReaderLeaves(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	leaves, err := filterReader.Leaves()
	if err != nil {
		t.Fatalf("Leaves returned error: %v", err)
	}

	if len(leaves) != 1 {
		t.Errorf("Expected 1 leaf, got %d", len(leaves))
	}
}

// TestFilterLeafReaderGetContext tests GetContext delegation
func TestFilterLeafReaderGetContext(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test_segment",
		docCount: 10,
	}
	leafReader := NewLeafReader(segmentInfo)
	filterReader := NewFilterLeafReader(leafReader)

	ctx, err := filterReader.GetContext()
	if err != nil {
		t.Fatalf("GetContext returned error: %v", err)
	}

	if ctx == nil {
		t.Error("GetContext returned nil")
	}
}

// TestNewFilterDirectoryReader tests creating a FilterDirectoryReader
func TestNewFilterDirectoryReader(t *testing.T) {
	// We can't easily create a DirectoryReader without an index,
	// so we'll just test the FilterDirectoryReader struct exists
	// and can be instantiated with nil (which would panic on use,
	// but shows the API is correct)

	// Test with nil - this is just a compile-time check
	_ = FilterDirectoryReader{}
}

// TestNewFilterCodecReader tests creating a FilterCodecReader
func TestNewFilterCodecReader(t *testing.T) {
	// Test with nil - this is just a compile-time check
	_ = FilterCodecReader{}
}