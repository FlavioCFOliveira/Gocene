// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNewSegmentInfos(t *testing.T) {
	sis := NewSegmentInfos()

	if sis.Size() != 0 {
		t.Errorf("Expected size=0, got %d", sis.Size())
	}
	if sis.Generation() != 1 {
		t.Errorf("Expected generation=1, got %d", sis.Generation())
	}
	if sis.LastGeneration() != 0 {
		t.Errorf("Expected lastGeneration=0, got %d", sis.LastGeneration())
	}
	if sis.Version() != 0 {
		t.Errorf("Expected version=0, got %d", sis.Version())
	}
	if sis.LuceneVersion() != "10.0.0" {
		t.Errorf("Expected luceneVersion='10.0.0', got '%s'", sis.LuceneVersion())
	}
	if sis.Counter() != 0 {
		t.Errorf("Expected counter=0, got %d", sis.Counter())
	}
}

func TestSegmentInfos_Add(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)

	sis.Add(sci1)

	if sis.Size() != 1 {
		t.Errorf("Expected size=1, got %d", sis.Size())
	}
	if sis.Get(0) != sci1 {
		t.Error("Get(0) should return the added segment")
	}

	// Add another
	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 5, 1)
	sis.Add(sci2)

	if sis.Size() != 2 {
		t.Errorf("Expected size=2, got %d", sis.Size())
	}
}

func TestSegmentInfos_Insert(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 0, -1)
	sis.Insert(0, sci2)

	if sis.Size() != 2 {
		t.Errorf("Expected size=2, got %d", sis.Size())
	}
	if sis.Get(0) != sci2 {
		t.Error("Get(0) should be the inserted segment")
	}
	if sis.Get(1) != sci1 {
		t.Error("Get(1) should be the original segment")
	}

	// Insert out of bounds - should not panic or add
	si3 := NewSegmentInfo("_2", 25, nil)
	sci3 := NewSegmentCommitInfo(si3, 0, -1)
	sis.Insert(10, sci3)
	if sis.Size() != 2 {
		t.Errorf("Insert out of bounds should not change size, got %d", sis.Size())
	}
}

func TestSegmentInfos_Remove(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 0, -1)
	sis.Add(sci2)

	removed := sis.Remove(0)
	if removed == nil {
		t.Error("Remove should return the removed segment")
	}
	if sis.Size() != 1 {
		t.Errorf("Expected size=1, got %d", sis.Size())
	}
	if sis.Get(0) != sci2 {
		t.Error("Get(0) should be the second segment after remove")
	}

	// Remove out of bounds
	removed = sis.Remove(10)
	if removed != nil {
		t.Error("Remove out of bounds should return nil")
	}
}

func TestSegmentInfos_Clear(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 0, -1)
	sis.Add(sci2)

	sis.Clear()

	if sis.Size() != 0 {
		t.Errorf("Expected size=0 after Clear, got %d", sis.Size())
	}
}

func TestSegmentInfos_Generation(t *testing.T) {
	sis := NewSegmentInfos()

	if sis.Generation() != 1 {
		t.Errorf("Expected generation=1, got %d", sis.Generation())
	}

	sis.SetGeneration(5)
	if sis.Generation() != 5 {
		t.Errorf("Expected generation=5, got %d", sis.Generation())
	}

	// Test NextGeneration
	gen := sis.NextGeneration()
	if gen != 6 {
		t.Errorf("Expected generation=6 after NextGeneration, got %d", gen)
	}
	if sis.Generation() != 6 {
		t.Errorf("Expected generation=6, got %d", sis.Generation())
	}
}

func TestSegmentInfos_LastGeneration(t *testing.T) {
	sis := NewSegmentInfos()

	if sis.LastGeneration() != 0 {
		t.Errorf("Expected lastGeneration=0, got %d", sis.LastGeneration())
	}

	sis.SetLastGeneration(10)
	if sis.LastGeneration() != 10 {
		t.Errorf("Expected lastGeneration=10, got %d", sis.LastGeneration())
	}
}

func TestSegmentInfos_GetFileName(t *testing.T) {
	tests := []struct {
		gen      int64
		expected string
	}{
		{1, "segments_1"},
		{5, "segments_5"},
		{100, "segments_100"},
		{0, "segments_0"},
		{-1, ""},
	}

	for _, tc := range tests {
		name := GetSegmentFileName(tc.gen)
		if name != tc.expected {
			t.Errorf("GetSegmentFileName(%d) expected '%s', got '%s'", tc.gen, tc.expected, name)
		}
	}

	// Test via SegmentInfos
	sis := NewSegmentInfos()
	sis.SetGeneration(5)
	if sis.GetFileName() != "segments_5" {
		t.Errorf("Expected 'segments_5', got '%s'", sis.GetFileName())
	}
}

func TestSegmentInfos_GetNextSegmentName(t *testing.T) {
	sis := NewSegmentInfos()

	names := make(map[string]bool)
	for i := 0; i < 10; i++ {
		name := sis.GetNextSegmentName()
		if name == "" {
			t.Error("GetNextSegmentName should not return empty string")
		}
		if names[name] {
			t.Errorf("Duplicate segment name: %s", name)
		}
		names[name] = true
	}

	// Names should be _0, _1, _2, ...
	if !names["_0"] {
		t.Error("Should have generated name '_0'")
	}
	if !names["_1"] {
		t.Error("Should have generated name '_1'")
	}
	if !names["_9"] {
		t.Error("Should have generated name '_9'")
	}
}

func TestSegmentInfos_TotalCounts(t *testing.T) {
	sis := NewSegmentInfos()

	// Empty
	if sis.TotalDocCount() != 0 {
		t.Errorf("Expected totalDocCount=0, got %d", sis.TotalDocCount())
	}
	if sis.TotalNumDocs() != 0 {
		t.Errorf("Expected totalNumDocs=0, got %d", sis.TotalNumDocs())
	}
	if sis.TotalDelCount() != 0 {
		t.Errorf("Expected totalDelCount=0, got %d", sis.TotalDelCount())
	}

	// Add segments
	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 10, 1) // 90 live docs
	sis.Add(sci1)

	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 5, 1) // 45 live docs
	sis.Add(sci2)

	// Total doc count = 100 + 50 = 150
	if sis.TotalDocCount() != 150 {
		t.Errorf("Expected totalDocCount=150, got %d", sis.TotalDocCount())
	}

	// Total num docs = 90 + 45 = 135
	if sis.TotalNumDocs() != 135 {
		t.Errorf("Expected totalNumDocs=135, got %d", sis.TotalNumDocs())
	}

	// Total del count = 10 + 5 = 15
	if sis.TotalDelCount() != 15 {
		t.Errorf("Expected totalDelCount=15, got %d", sis.TotalDelCount())
	}
}

func TestSegmentInfos_UserData(t *testing.T) {
	sis := NewSegmentInfos()

	// Empty
	if len(sis.GetUserData()) != 0 {
		t.Error("Expected empty user data initially")
	}

	// Set single value
	sis.SetUserDataValue("commit_time", "2026-03-11T10:00:00Z")
	if sis.GetUserDataValue("commit_time") != "2026-03-11T10:00:00Z" {
		t.Error("User data value should match")
	}

	// Set multiple values
	data := map[string]string{
		"source":     "merge",
		"user":       "admin",
		"batch_size": "1000",
	}
	sis.SetUserData(data)

	userData := sis.GetUserData()
	if len(userData) != 3 {
		t.Errorf("Expected 3 user data entries, got %d", len(userData))
	}
	if userData["source"] != "merge" {
		t.Error("User data should contain 'source'")
	}

	// Get non-existent
	if sis.GetUserDataValue("nonexistent") != "" {
		t.Error("Non-existent key should return empty string")
	}
}

func TestSegmentInfos_Clone(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 10, 1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 5, 1)
	sis.Add(sci2)

	sis.SetGeneration(5)
	sis.SetLastGeneration(4)
	sis.SetUserDataValue("key", "value")

	clone := sis.Clone()

	// Clone should have same values
	if clone.Size() != sis.Size() {
		t.Error("Clone should have same size")
	}
	if clone.Generation() != sis.Generation() {
		t.Error("Clone should have same generation")
	}
	if clone.LastGeneration() != sis.LastGeneration() {
		t.Error("Clone should have same lastGeneration")
	}
	if clone.GetUserDataValue("key") != "value" {
		t.Error("Clone should have copied user data")
	}

	// Modifying clone should not affect original
	clone.Add(sci2)
	if sis.Size() == clone.Size() {
		t.Error("Adding to clone should not affect original")
	}
}

func TestSegmentInfos_ReadWrite(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sis := NewSegmentInfos()
	sis.SetGeneration(5)
	sis.SetCounter(10)

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 5, 1)
	sis.Add(sci2)

	// Write
	err := WriteSegmentInfos(sis, dir)
	if err != nil {
		t.Fatalf("WriteSegmentInfos error: %v", err)
	}

	// Read
	readSis, err := ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos error: %v", err)
	}

	// Verify
	if readSis.Generation() != sis.Generation() {
		t.Errorf("Expected generation %d, got %d", sis.Generation(), readSis.Generation())
	}
	if readSis.Counter() != sis.Counter() {
		t.Errorf("Expected counter %d, got %d", sis.Counter(), readSis.Counter())
	}
	if readSis.Size() != sis.Size() {
		t.Errorf("Expected size %d, got %d", sis.Size(), readSis.Size())
	}

	for i := 0; i < sis.Size(); i++ {
		orig := sis.Get(i)
		read := readSis.Get(i)
		if read.Name() != orig.Name() {
			t.Errorf("At index %d: expected name %s, got %s", i, orig.Name(), read.Name())
		}
		if read.segmentInfo.docCount != orig.segmentInfo.docCount {
			t.Errorf("At index %d: expected docCount %d, got %d", i, orig.segmentInfo.docCount, read.segmentInfo.docCount)
		}
	}
}

func TestSegmentInfos_ReadNoSegmentsFile(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	_, err := ReadSegmentInfos(dir)
	if err == nil {
		t.Error("Expected error when no segments file exists")
	}
}

func TestSegmentInfos_ReadInvalidMagic(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create a fake segments file with invalid magic
	out, _ := dir.CreateOutput("segments_1", store.IOContextWrite)
	store.WriteInt32(out, 0x12345678) // Invalid magic
	out.Close()

	_, err := ReadSegmentInfos(dir)
	if err == nil {
		t.Error("Expected error for invalid magic")
	}
}

func TestSegmentInfos_Versioning(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sis := NewSegmentInfos()
	sis.SetGeneration(1)
	WriteSegmentInfos(sis, dir)

	// Advance to generation 2
	sis.NextGeneration()
	WriteSegmentInfos(sis, dir)

	// Read should get the latest (generation 2)
	readSis, err := ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos error: %v", err)
	}
	if readSis.Generation() != 2 {
		t.Errorf("Expected generation 2, got %d", readSis.Generation())
	}
}

func TestSegmentInfos_UpdateCounterFromSegments(t *testing.T) {
	sis := NewSegmentInfos()

	// Add segments
	si1 := NewSegmentInfo("_5", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_2", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 0, -1)
	sis.Add(sci2)

	si3 := NewSegmentInfo("_10", 25, nil)
	sci3 := NewSegmentCommitInfo(si3, 0, -1)
	sis.Add(sci3)

	sis.UpdateCounterFromSegments()

	// Counter should be set to max generation + 1 = 10 + 1 = 11
	if sis.Counter() != 11 {
		t.Errorf("Expected counter=11, got %d", sis.Counter())
	}

	// Next segment name should be _11
	name := sis.GetNextSegmentName()
	if name != "_11" {
		t.Errorf("Expected next name '_11', got '%s'", name)
	}
}
