package index

import (
	"testing"
	"time"
)

func TestNewIndexRevision(t *testing.T) {
	files := []string{"file1.txt", "file2.txt"}
	revision := NewIndexRevision(1, 2, files)

	if revision == nil {
		t.Fatal("expected revision to not be nil")
	}

	if revision.Generation != 1 {
		t.Errorf("expected generation 1, got %d", revision.Generation)
	}

	if revision.Version != 2 {
		t.Errorf("expected version 2, got %d", revision.Version)
	}

	if len(revision.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(revision.Files))
	}

	if revision.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}

	if revision.Metadata == nil {
		t.Error("expected metadata to be initialized")
	}
}

func TestIndexRevision_Clone(t *testing.T) {
	files := []string{"file1.txt", "file2.txt"}
	original := NewIndexRevision(1, 2, files)
	original.SetMetadata("key1", "value1")

	cloned := original.Clone()

	if cloned == nil {
		t.Fatal("expected cloned to not be nil")
	}

	if cloned.Generation != original.Generation {
		t.Error("expected cloned generation to match")
	}

	if cloned.Version != original.Version {
		t.Error("expected cloned version to match")
	}

	// Verify files are deep copied
	if len(cloned.Files) != len(original.Files) {
		t.Error("expected cloned files to have same length")
	}

	// Modify original files and verify clone is unaffected
	original.Files[0] = "modified.txt"
	if cloned.Files[0] == "modified.txt" {
		t.Error("expected cloned files to be independent")
	}

	// Verify metadata is deep copied
	if cloned.GetMetadata("key1") != "value1" {
		t.Error("expected cloned metadata to be copied")
	}

	// Modify original metadata and verify clone is unaffected
	original.SetMetadata("key1", "modified")
	if cloned.GetMetadata("key1") == "modified" {
		t.Error("expected cloned metadata to be independent")
	}
}

func TestIndexRevision_Clone_Nil(t *testing.T) {
	var revision *IndexRevision
	cloned := revision.Clone()
	if cloned != nil {
		t.Error("expected nil clone for nil revision")
	}
}

func TestIndexRevision_Equals(t *testing.T) {
	files1 := []string{"file1.txt", "file2.txt"}
	revision1 := NewIndexRevision(1, 2, files1)

	files2 := []string{"file1.txt", "file2.txt"}
	revision2 := NewIndexRevision(1, 2, files2)

	if !revision1.Equals(revision2) {
		t.Error("expected revisions to be equal")
	}

	// Different generation
	revision3 := NewIndexRevision(2, 2, files2)
	if revision1.Equals(revision3) {
		t.Error("expected revisions with different generations to not be equal")
	}

	// Different version
	revision4 := NewIndexRevision(1, 3, files2)
	if revision1.Equals(revision4) {
		t.Error("expected revisions with different versions to not be equal")
	}

	// Different files
	files3 := []string{"file1.txt", "file3.txt"}
	revision5 := NewIndexRevision(1, 2, files3)
	if revision1.Equals(revision5) {
		t.Error("expected revisions with different files to not be equal")
	}
}

func TestIndexRevision_Equals_Nil(t *testing.T) {
	revision := NewIndexRevision(1, 2, nil)

	if revision.Equals(nil) {
		t.Error("expected revision to not equal nil")
	}

	var nilRevision *IndexRevision
	if nilRevision.Equals(revision) {
		t.Error("expected nil revision to not equal non-nil")
	}

	if !nilRevision.Equals(nil) {
		t.Error("expected nil revisions to be equal")
	}
}

func TestIndexRevision_GetFileCount(t *testing.T) {
	files := []string{"file1.txt", "file2.txt"}
	revision := NewIndexRevision(1, 1, files)

	if revision.GetFileCount() != 2 {
		t.Errorf("expected 2 files, got %d", revision.GetFileCount())
	}

	var nilRevision *IndexRevision
	if nilRevision.GetFileCount() != 0 {
		t.Error("expected nil revision to have 0 files")
	}
}

func TestIndexRevision_HasFile(t *testing.T) {
	files := []string{"file1.txt", "file2.txt"}
	revision := NewIndexRevision(1, 1, files)

	if !revision.HasFile("file1.txt") {
		t.Error("expected to have file1.txt")
	}

	if !revision.HasFile("file2.txt") {
		t.Error("expected to have file2.txt")
	}

	if revision.HasFile("file3.txt") {
		t.Error("expected to not have file3.txt")
	}

	var nilRevision *IndexRevision
	if nilRevision.HasFile("file1.txt") {
		t.Error("expected nil revision to not have any files")
	}
}

func TestIndexRevision_AddFile(t *testing.T) {
	revision := NewIndexRevision(1, 1, []string{"file1.txt"})

	revision.AddFile("file2.txt")

	if revision.GetFileCount() != 2 {
		t.Errorf("expected 2 files, got %d", revision.GetFileCount())
	}

	if !revision.HasFile("file2.txt") {
		t.Error("expected to have file2.txt")
	}

	// Test nil revision
	var nilRevision *IndexRevision
	nilRevision.AddFile("file.txt") // Should not panic
}

func TestIndexRevision_RemoveFile(t *testing.T) {
	revision := NewIndexRevision(1, 1, []string{"file1.txt", "file2.txt", "file3.txt"})

	if !revision.RemoveFile("file2.txt") {
		t.Error("expected removal to succeed")
	}

	if revision.GetFileCount() != 2 {
		t.Errorf("expected 2 files, got %d", revision.GetFileCount())
	}

	if revision.HasFile("file2.txt") {
		t.Error("expected file2.txt to be removed")
	}

	if revision.RemoveFile("file2.txt") {
		t.Error("expected removal of non-existent file to fail")
	}

	// Test nil revision
	var nilRevision *IndexRevision
	if nilRevision.RemoveFile("file.txt") {
		t.Error("expected nil revision removal to fail")
	}
}

func TestIndexRevision_SetGetMetadata(t *testing.T) {
	revision := NewIndexRevision(1, 1, nil)

	revision.SetMetadata("key1", "value1")
	revision.SetMetadata("key2", "value2")

	if revision.GetMetadata("key1") != "value1" {
		t.Errorf("expected value1, got %s", revision.GetMetadata("key1"))
	}

	if revision.GetMetadata("key2") != "value2" {
		t.Errorf("expected value2, got %s", revision.GetMetadata("key2"))
	}

	if revision.GetMetadata("key3") != "" {
		t.Error("expected empty value for non-existent key")
	}

	// Update existing key
	revision.SetMetadata("key1", "updated")
	if revision.GetMetadata("key1") != "updated" {
		t.Error("expected value to be updated")
	}

	// Test nil revision
	var nilRevision *IndexRevision
	nilRevision.SetMetadata("key", "value") // Should not panic
	if nilRevision.GetMetadata("key") != "" {
		t.Error("expected nil revision to return empty metadata")
	}
}

func TestIndexRevision_IsNewerThan(t *testing.T) {
	revision1 := NewIndexRevision(1, 1, nil)
	revision2 := NewIndexRevision(2, 1, nil)
	revision3 := NewIndexRevision(1, 1, nil)

	if !revision2.IsNewerThan(revision1) {
		t.Error("expected revision2 to be newer than revision1")
	}

	if revision1.IsNewerThan(revision2) {
		t.Error("expected revision1 to not be newer than revision2")
	}

	if revision1.IsNewerThan(revision3) {
		t.Error("expected equal generations to not be newer")
	}

	// Test nil
	if revision1.IsNewerThan(nil) {
		t.Error("expected any revision to not be newer than nil")
	}

	var nilRevision *IndexRevision
	if !nilRevision.IsNewerThan(revision1) {
		t.Error("expected nil to not be newer than any revision")
	}
}

func TestIndexRevision_GetAge(t *testing.T) {
	revision := NewIndexRevision(1, 1, nil)

	// Should have some age
	age := revision.GetAge()
	if age < 0 {
		t.Error("expected age to be non-negative")
	}

	// Wait a bit and check age increased
	time.Sleep(10 * time.Millisecond)
	newAge := revision.GetAge()
	if newAge <= age {
		t.Error("expected age to increase")
	}

	// Test nil revision
	var nilRevision *IndexRevision
	if nilRevision.GetAge() != 0 {
		t.Error("expected nil revision to have 0 age")
	}
}

func TestIndexRevision_String(t *testing.T) {
	revision := NewIndexRevision(1, 2, []string{"file1.txt"})

	str := revision.String()
	if str == "" {
		t.Error("expected non-empty string")
	}

	if str == "IndexRevision(nil)" {
		t.Error("expected non-nil revision string")
	}

	// Test nil revision
	var nilRevision *IndexRevision
	nilStr := nilRevision.String()
	if nilStr != "IndexRevision(nil)" {
		t.Errorf("expected nil revision string, got %s", nilStr)
	}
}
