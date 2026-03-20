package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// mockSegmentCommitInfo creates a mock SegmentCommitInfo for testing
func mockSegmentCommitInfoForNRT() *SegmentCommitInfo {
	dir := store.NewByteBuffersDirectory()
	return NewSegmentCommitInfo(
		NewSegmentInfo("test", 100, dir),
		0, 0,
	)
}

func TestNewNRTSegmentReader(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	if nrtReader == nil {
		t.Fatal("expected NRTSegmentReader to not be nil")
	}

	if !nrtReader.IsNRT() {
		t.Error("expected reader to be NRT")
	}

	if nrtReader.GetVersion() != 1 {
		t.Errorf("expected version 1, got %d", nrtReader.GetVersion())
	}

	if nrtReader.NumDocs() != 100 {
		t.Errorf("expected 100 docs, got %d", nrtReader.NumDocs())
	}
}

func TestNewNRTSegmentReader_Nil(t *testing.T) {
	_, err := NewNRTSegmentReader(nil, nil)
	if err == nil {
		t.Error("expected error for nil segment reader")
	}
}

func TestNRTSegmentReader_IsLive(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	// Initially all docs should be live
	if !nrtReader.IsLive(0) {
		t.Error("expected doc 0 to be live")
	}
	if !nrtReader.IsLive(99) {
		t.Error("expected doc 99 to be live")
	}

	// Out of range should not be live
	if nrtReader.IsLive(-1) {
		t.Error("expected doc -1 to not be live")
	}
	if nrtReader.IsLive(100) {
		t.Error("expected doc 100 to not be live (out of range)")
	}
}

func TestNRTSegmentReader_MarkDeleted(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	// Mark doc 5 as deleted
	wasLive, err := nrtReader.MarkDeleted(5)
	if err != nil {
		t.Fatalf("failed to mark deleted: %v", err)
	}
	if !wasLive {
		t.Error("expected doc 5 to be previously live")
	}

	// Check doc 5 is now deleted
	if nrtReader.IsLive(5) {
		t.Error("expected doc 5 to not be live after deletion")
	}

	// NumDocs should decrease
	if nrtReader.NumDocs() != 99 {
		t.Errorf("expected 99 docs after deletion, got %d", nrtReader.NumDocs())
	}

	// Marking again should return false (not previously live)
	wasLive, err = nrtReader.MarkDeleted(5)
	if err != nil {
		t.Fatalf("failed to mark deleted: %v", err)
	}
	if wasLive {
		t.Error("expected doc 5 to not be previously live on second delete")
	}
}

func TestNRTSegmentReader_MarkDeleted_OutOfRange(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	_, err = nrtReader.MarkDeleted(100)
	if err == nil {
		t.Error("expected error for out of range docID")
	}

	_, err = nrtReader.MarkDeleted(-1)
	if err == nil {
		t.Error("expected error for negative docID")
	}
}

func TestNRTSegmentReader_HasDeletions(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	// Initially no deletions
	if nrtReader.HasDeletions() {
		t.Error("expected no deletions initially")
	}

	// Delete a document
	nrtReader.MarkDeleted(0)

	if !nrtReader.HasDeletions() {
		t.Error("expected deletions after marking deleted")
	}
}

func TestNRTSegmentReader_GetPendingDeletes(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	// Initially no pending deletes
	pending := nrtReader.GetPendingDeletes()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending deletes, got %d", len(pending))
	}

	// Delete some documents
	nrtReader.MarkDeleted(5)
	nrtReader.MarkDeleted(10)

	pending = nrtReader.GetPendingDeletes()
	if len(pending) != 2 {
		t.Errorf("expected 2 pending deletes, got %d", len(pending))
	}
}

func TestNRTSegmentReader_ClearPendingDeletes(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	// Delete some documents
	nrtReader.MarkDeleted(5)
	nrtReader.MarkDeleted(10)

	nrtReader.ClearPendingDeletes()

	pending := nrtReader.GetPendingDeletes()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending deletes after clear, got %d", len(pending))
	}
}

func TestNRTSegmentReader_IncrementVersion(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	if nrtReader.GetVersion() != 1 {
		t.Errorf("expected version 1, got %d", nrtReader.GetVersion())
	}

	nrtReader.IncrementVersion()

	if nrtReader.GetVersion() != 2 {
		t.Errorf("expected version 2, got %d", nrtReader.GetVersion())
	}
}

func TestNRTSegmentReader_Clone(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	// Delete a document in original
	nrtReader.MarkDeleted(5)

	// Clone
	cloned, err := nrtReader.Clone()
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}
	defer cloned.Close()

	// Cloned should share the same live docs
	if cloned.IsLive(5) {
		t.Error("expected cloned to also see doc 5 as deleted")
	}

	// Cloned should have its own pending deletes
	cloned.MarkDeleted(10)
	if len(nrtReader.GetPendingDeletes()) != 1 {
		t.Error("expected original to have 1 pending delete")
	}
}

func TestNRTSegmentReader_RefreshLiveDocs(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	// Delete some documents
	nrtReader.MarkDeleted(5)
	nrtReader.MarkDeleted(10)

	oldVersion := nrtReader.GetVersion()

	// Refresh live docs
	err = nrtReader.RefreshLiveDocs()
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}

	// All docs should be live again
	if !nrtReader.IsLive(5) {
		t.Error("expected doc 5 to be live after refresh")
	}
	if !nrtReader.IsLive(10) {
		t.Error("expected doc 10 to be live after refresh")
	}

	// Version should be incremented
	if nrtReader.GetVersion() <= oldVersion {
		t.Error("expected version to be incremented")
	}
}

func TestNRTSegmentReader_GetTotalDocs(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	if nrtReader.GetTotalDocs() != 100 {
		t.Errorf("expected total docs 100, got %d", nrtReader.GetTotalDocs())
	}

	// Delete should not affect total
	nrtReader.MarkDeleted(5)

	if nrtReader.GetTotalDocs() != 100 {
		t.Errorf("expected total docs still 100, got %d", nrtReader.GetTotalDocs())
	}
}

func TestNRTSegmentReader_GetNumDeleted(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	if nrtReader.GetNumDeleted() != 0 {
		t.Errorf("expected 0 deleted initially, got %d", nrtReader.GetNumDeleted())
	}

	nrtReader.MarkDeleted(5)
	nrtReader.MarkDeleted(10)

	if nrtReader.GetNumDeleted() != 2 {
		t.Errorf("expected 2 deleted, got %d", nrtReader.GetNumDeleted())
	}
}

func TestNRTSegmentReader_ForEachLiveDoc(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	// Delete some docs
	nrtReader.MarkDeleted(5)
	nrtReader.MarkDeleted(10)

	// Count live docs
	liveCount := 0
	err = nrtReader.ForEachLiveDoc(func(docID int) error {
		liveCount++
		return nil
	})
	if err != nil {
		t.Fatalf("foreach error: %v", err)
	}

	if liveCount != 98 { // 100 - 2 deleted
		t.Errorf("expected 98 live docs, got %d", liveCount)
	}
}

func TestNRTSegmentReader_GetLiveDocs(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	liveDocs := nrtReader.GetLiveDocs()
	if liveDocs == nil {
		t.Fatal("expected live docs to not be nil")
	}

	// Should have at least 2 uint64s for 100 docs
	if len(liveDocs) < 2 {
		t.Errorf("expected at least 2 words, got %d", len(liveDocs))
	}
}

func TestNRTSegmentReader_GetWriter(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	// Without writer
	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}
	defer nrtReader.Close()

	if nrtReader.GetWriter() != nil {
		t.Error("expected nil writer")
	}

	// Note: Testing with actual writer would require IndexWriter setup
}

func TestNRTSegmentReader_Close(t *testing.T) {
	segmentInfo := mockSegmentCommitInfoForNRT()
	segmentReader := NewSegmentReader(segmentInfo)

	nrtReader, err := NewNRTSegmentReader(segmentReader, nil)
	if err != nil {
		t.Fatalf("failed to create NRTSegmentReader: %v", err)
	}

	err = nrtReader.Close()
	if err != nil {
		t.Fatalf("failed to close: %v", err)
	}

	if nrtReader.IsNRT() {
		t.Error("expected reader to not be NRT after close")
	}
}
