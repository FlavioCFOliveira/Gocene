// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"testing"
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
	if sis.Version() != "10.0.0" {
		t.Errorf("Expected version='10.0.0', got '%s'", sis.Version())
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

func TestSegmentInfos_List(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	list := sis.List()
	if len(list) != 1 {
		t.Errorf("Expected list length=1, got %d", len(list))
	}

	// Modifying the returned list should not affect original
	list = append(list, sci1)
	if sis.Size() != 1 {
		t.Error("Modifying returned list should not affect original")
	}
}

func TestSegmentInfos_Contains(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	if !sis.Contains(sci1) {
		t.Error("Should contain sci1")
	}

	sci2 := NewSegmentCommitInfo(si1, 0, -1)
	if sis.Contains(sci2) {
		t.Error("Should not contain sci2 (different pointer)")
	}
}

func TestSegmentInfos_IndexOf(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 0, -1)
	sis.Add(sci2)

	if sis.IndexOf(sci1) != 0 {
		t.Errorf("Expected IndexOf(sci1)=0, got %d", sis.IndexOf(sci1))
	}
	if sis.IndexOf(sci2) != 1 {
		t.Errorf("Expected IndexOf(sci2)=1, got %d", sis.IndexOf(sci2))
	}

	sci3 := NewSegmentCommitInfo(si1, 0, -1)
	if sis.IndexOf(sci3) != -1 {
		t.Error("IndexOf should return -1 for segment not in list")
	}
}

func TestSegmentInfos_RemoveByName(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_1", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 0, -1)
	sis.Add(sci2)

	si3 := NewSegmentInfo("_0", 75, nil) // Same name as sci1
	sci3 := NewSegmentCommitInfo(si3, 0, -1)
	sis.Add(sci3)

	count := sis.RemoveByName("_0")
	if count != 2 {
		t.Errorf("Expected remove count=2, got %d", count)
	}
	if sis.Size() != 1 {
		t.Errorf("Expected size=1, got %d", sis.Size())
	}
	if sis.Get(0).Name() != "_1" {
		t.Error("Remaining segment should be _1")
	}
}

func TestSegmentInfos_SortByName(t *testing.T) {
	sis := NewSegmentInfos()

	// Add segments in non-sorted order
	si2 := NewSegmentInfo("_2", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 0, -1)
	sis.Add(sci2)

	si0 := NewSegmentInfo("_0", 100, nil)
	sci0 := NewSegmentCommitInfo(si0, 0, -1)
	sis.Add(sci0)

	si1 := NewSegmentInfo("_1", 75, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	sis.SortByName()

	if sis.Get(0).Name() != "_0" {
		t.Errorf("Expected _0 at index 0, got %s", sis.Get(0).Name())
	}
	if sis.Get(1).Name() != "_1" {
		t.Errorf("Expected _1 at index 1, got %s", sis.Get(1).Name())
	}
	if sis.Get(2).Name() != "_2" {
		t.Errorf("Expected _2 at index 2, got %s", sis.Get(2).Name())
	}
}

func TestSegmentInfos_String(t *testing.T) {
	sis := NewSegmentInfos()

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 10, 1)
	sis.Add(sci1)

	str := sis.String()
	if str == "" {
		t.Error("String should not be empty")
	}

	expected := "SegmentInfos(segments=1, generation=1, version=10.0.0, docs=90)"
	if str != expected {
		t.Errorf("Expected '%s', got '%s'", expected, str)
	}
}

func TestSegmentInfos_GetMaxSegmentName(t *testing.T) {
	sis := NewSegmentInfos()

	// Empty
	if sis.GetMaxSegmentName() != "" {
		t.Errorf("Expected empty string for empty segments, got '%s'", sis.GetMaxSegmentName())
	}

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	si2 := NewSegmentInfo("_9", 50, nil)
	sci2 := NewSegmentCommitInfo(si2, 0, -1)
	sis.Add(sci2)

	si3 := NewSegmentInfo("_2", 25, nil)
	sci3 := NewSegmentCommitInfo(si3, 0, -1)
	sis.Add(sci3)

	maxName := sis.GetMaxSegmentName()
	if maxName != "_9" {
		t.Errorf("Expected max name '_9', got '%s'", maxName)
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

func TestSegmentInfos_GetOutOfBounds(t *testing.T) {
	sis := NewSegmentInfos()

	// Get from empty
	if sis.Get(0) != nil {
		t.Error("Get from empty should return nil")
	}

	si1 := NewSegmentInfo("_0", 100, nil)
	sci1 := NewSegmentCommitInfo(si1, 0, -1)
	sis.Add(sci1)

	// Get negative index
	if sis.Get(-1) != nil {
		t.Error("Get(-1) should return nil")
	}

	// Get past end
	if sis.Get(10) != nil {
		t.Error("Get(10) should return nil")
	}
}

func TestSegmentInfos_ConcurrentAccess(t *testing.T) {
	sis := NewSegmentInfos()

	// Pre-populate with some segments
	for i := 0; i < 5; i++ {
		si := NewSegmentInfo("_"+fmt.Sprintf("%d", i), 100, nil)
		sci := NewSegmentCommitInfo(si, 0, -1)
		sis.Add(sci)
	}

	done := make(chan bool, 4)

	// Reader 1: iterate
	go func() {
		for i := 0; i < 100; i++ {
			sis.Size()
			sis.TotalDocCount()
		}
		done <- true
	}()

	// Reader 2: get segments
	go func() {
		for i := 0; i < 100; i++ {
			sis.Get(i % 5)
			sis.Contains(nil)
		}
		done <- true
	}()

	// Writer 1: add segments
	go func() {
		for i := 0; i < 50; i++ {
			si := NewSegmentInfo("_"+fmt.Sprintf("%d", i+100), 10, nil)
			sci := NewSegmentCommitInfo(si, 0, -1)
			sis.Add(sci)
		}
		done <- true
	}()

	// Writer 2: modify generation
	go func() {
		for i := 0; i < 50; i++ {
			sis.NextGeneration()
			sis.SetUserDataValue("key", fmt.Sprintf("value%d", i))
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}
}
