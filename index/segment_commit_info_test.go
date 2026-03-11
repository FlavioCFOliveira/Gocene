// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestNewSegmentCommitInfo(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	if sci.SegmentInfo() != si {
		t.Error("SegmentInfo should match")
	}
	if sci.DelCount() != 0 {
		t.Errorf("Expected delCount=0, got %d", sci.DelCount())
	}
	if sci.DelGen() != -1 {
		t.Errorf("Expected delGen=-1, got %d", sci.DelGen())
	}
	if sci.SoftDelCount() != 0 {
		t.Errorf("Expected softDelCount=0, got %d", sci.SoftDelCount())
	}
	if sci.FieldInfosGen() != -1 {
		t.Errorf("Expected fieldInfosGen=-1, got %d", sci.FieldInfosGen())
	}
	if sci.DocValuesGen() != -1 {
		t.Errorf("Expected docValuesGen=-1, got %d", sci.DocValuesGen())
	}
	if sci.HasDeletions() {
		t.Error("Expected HasDeletions=false")
	}
	if sci.HasFieldInfosGen() {
		t.Error("Expected HasFieldInfosGen=false")
	}
	if sci.HasDocValuesGen() {
		t.Error("Expected HasDocValuesGen=false")
	}
}

func TestSegmentCommitInfo_WithDeletions(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 10, 1)

	if sci.DelCount() != 10 {
		t.Errorf("Expected delCount=10, got %d", sci.DelCount())
	}
	if sci.DelGen() != 1 {
		t.Errorf("Expected delGen=1, got %d", sci.DelGen())
	}
	if !sci.HasDeletions() {
		t.Error("Expected HasDeletions=true")
	}

	// Update del count
	sci.SetDelCount(20)
	if sci.DelCount() != 20 {
		t.Errorf("Expected delCount=20 after update, got %d", sci.DelCount())
	}

	// Update del gen
	sci.SetDelGen(2)
	if sci.DelGen() != 2 {
		t.Errorf("Expected delGen=2 after update, got %d", sci.DelGen())
	}
}

func TestSegmentCommitInfo_SoftDeletes(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 10, 1)

	sci.SetSoftDelCount(5)
	if sci.SoftDelCount() != 5 {
		t.Errorf("Expected softDelCount=5, got %d", sci.SoftDelCount())
	}
}

func TestSegmentCommitInfo_FieldInfosGen(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	// Initially no field infos gen
	if sci.HasFieldInfosGen() {
		t.Error("Expected HasFieldInfosGen=false initially")
	}

	// Set field infos gen
	sci.SetFieldInfosGen(1)
	if sci.FieldInfosGen() != 1 {
		t.Errorf("Expected fieldInfosGen=1, got %d", sci.FieldInfosGen())
	}
	if !sci.HasFieldInfosGen() {
		t.Error("Expected HasFieldInfosGen=true after setting")
	}
}

func TestSegmentCommitInfo_DocValuesGen(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	// Initially no doc values gen
	if sci.HasDocValuesGen() {
		t.Error("Expected HasDocValuesGen=false initially")
	}

	// Set doc values gen
	sci.SetDocValuesGen(1)
	if sci.DocValuesGen() != 1 {
		t.Errorf("Expected docValuesGen=1, got %d", sci.DocValuesGen())
	}
	if !sci.HasDocValuesGen() {
		t.Error("Expected HasDocValuesGen=true after setting")
	}
}

func TestSegmentCommitInfo_NumDocs(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 10, 1)

	// NumDocs = DocCount - DelCount = 100 - 10 = 90
	if sci.NumDocs() != 90 {
		t.Errorf("Expected numDocs=90, got %d", sci.NumDocs())
	}

	// MaxDoc = DocCount - 1 = 99
	if sci.MaxDoc() != 99 {
		t.Errorf("Expected maxDoc=99, got %d", sci.MaxDoc())
	}
}

func TestSegmentCommitInfo_Attributes(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	// Set attribute
	sci.SetAttribute("custom", "value")
	if sci.GetAttribute("custom") != "value" {
		t.Errorf("Expected attribute 'custom'='value', got '%s'", sci.GetAttribute("custom"))
	}

	// Get non-existent
	if sci.GetAttribute("nonexistent") != "" {
		t.Error("Expected empty string for non-existent attribute")
	}

	// Get all attributes
	sci.SetAttribute("another", "val")
	attrs := sci.GetAttributes()
	if len(attrs) != 2 {
		t.Errorf("Expected 2 attributes, got %d", len(attrs))
	}
	if attrs["custom"] != "value" {
		t.Error("Attributes should contain 'custom'")
	}
}

func TestSegmentCommitInfo_Delegation(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	// Name delegates to SegmentInfo
	if sci.Name() != "_0" {
		t.Errorf("Expected name '_0', got '%s'", sci.Name())
	}

	// DocCount delegates to SegmentInfo
	if sci.DocCount() != 100 {
		t.Errorf("Expected docCount=100, got %d", sci.DocCount())
	}
}

func TestSegmentCommitInfo_String(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 10, 1)

	str := sci.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	expected := "SegmentCommitInfo(name=_0, delCount=10, delGen=1, fieldInfosGen=-1)"
	if str != expected {
		t.Errorf("Expected '%s', got '%s'", expected, str)
	}
}

func TestSegmentCommitInfo_Clone(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	original := NewSegmentCommitInfo(si, 10, 1)
	original.SetSoftDelCount(5)
	original.SetFieldInfosGen(2)
	original.SetDocValuesGen(3)
	original.SetAttribute("custom", "value")

	clone := original.Clone()

	// Clone should have same values
	if clone.DelCount() != original.DelCount() {
		t.Error("Clone should have same delCount")
	}
	if clone.SoftDelCount() != original.SoftDelCount() {
		t.Error("Clone should have same softDelCount")
	}
	if clone.DelGen() != original.DelGen() {
		t.Error("Clone should have same delGen")
	}
	if clone.FieldInfosGen() != original.FieldInfosGen() {
		t.Error("Clone should have same fieldInfosGen")
	}
	if clone.DocValuesGen() != original.DocValuesGen() {
		t.Error("Clone should have same docValuesGen")
	}
	if clone.GetAttribute("custom") != "value" {
		t.Error("Clone should have copied attributes")
	}

	// Modify clone should not affect original
	clone.SetDelCount(20)
	if original.DelCount() == 20 {
		t.Error("Modifying clone should not affect original")
	}
}

func TestSegmentCommitInfo_AdvanceDelGen(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	// Start with no deletions
	sci := NewSegmentCommitInfo(si, 0, -1)
	gen := sci.AdvanceDelGen()
	if gen != 1 {
		t.Errorf("Expected gen=1 after first advance from -1, got %d", gen)
	}
	if sci.DelGen() != 1 {
		t.Errorf("Expected delGen=1, got %d", sci.DelGen())
	}

	// Advance again
	gen = sci.AdvanceDelGen()
	if gen != 2 {
		t.Errorf("Expected gen=2 after second advance, got %d", gen)
	}

	// Start with existing generation
	sci2 := NewSegmentCommitInfo(si, 0, 5)
	gen = sci2.AdvanceDelGen()
	if gen != 6 {
		t.Errorf("Expected gen=6 after advance from 5, got %d", gen)
	}
}

func TestSegmentCommitInfo_AdvanceFieldInfosGen(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	gen := sci.AdvanceFieldInfosGen()
	if gen != 1 {
		t.Errorf("Expected fieldInfosGen=1, got %d", gen)
	}

	gen = sci.AdvanceFieldInfosGen()
	if gen != 2 {
		t.Errorf("Expected fieldInfosGen=2, got %d", gen)
	}
}

func TestSegmentCommitInfo_AdvanceDocValuesGen(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	gen := sci.AdvanceDocValuesGen()
	if gen != 1 {
		t.Errorf("Expected docValuesGen=1, got %d", gen)
	}

	gen = sci.AdvanceDocValuesGen()
	if gen != 2 {
		t.Errorf("Expected docValuesGen=2, got %d", gen)
	}
}

func TestSegmentCommitInfo_GetDelFileName(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	// No deletions
	sci := NewSegmentCommitInfo(si, 0, -1)
	if sci.GetDelFileName() != "" {
		t.Error("Expected empty del file name when no deletions")
	}

	// With deletions
	sci.SetDelGen(5)
	name := sci.GetDelFileName()
	if name != "_0_5.del" {
		t.Errorf("Expected '_0_5.del', got '%s'", name)
	}
}

func TestSegmentCommitInfo_GetFieldInfosFileName(t *testing.T) {
	si := NewSegmentInfo("_1", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	// No field infos
	if sci.GetFieldInfosFileName() != "" {
		t.Error("Expected empty field infos file name when no gen")
	}

	// With field infos gen
	sci.SetFieldInfosGen(3)
	name := sci.GetFieldInfosFileName()
	if name != "_1_3.fnm" {
		t.Errorf("Expected '_1_3.fnm', got '%s'", name)
	}
}

func TestSegmentCommitInfo_GetDocValuesFileName(t *testing.T) {
	si := NewSegmentInfo("_2", 100, nil)
	sci := NewSegmentCommitInfo(si, 0, -1)

	// No doc values
	if sci.GetDocValuesFileName() != "" {
		t.Error("Expected empty doc values file name when no gen")
	}

	// With doc values gen
	sci.SetDocValuesGen(7)
	name := sci.GetDocValuesFileName()
	if name != "_2_7.dvd" {
		t.Errorf("Expected '_2_7.dvd', got '%s'", name)
	}
}

func TestSegmentCommitInfo_GetFiles(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	si.SetFiles([]string{"_0.cfs", "_0.cfe"})

	sci := NewSegmentCommitInfo(si, 0, -1)

	// Just segment files
	files := sci.GetFiles()
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	// Add deletion file
	sci.SetDelGen(1)
	files = sci.GetFiles()
	foundDel := false
	for _, f := range files {
		if f == "_0_1.del" {
			foundDel = true
			break
		}
	}
	if !foundDel {
		t.Error("Expected deletion file in list")
	}

	// Add field infos file
	sci.SetFieldInfosGen(2)
	files = sci.GetFiles()
	foundFnm := false
	for _, f := range files {
		if f == "_0_2.fnm" {
			foundFnm = true
			break
		}
	}
	if !foundFnm {
		t.Error("Expected field infos file in list")
	}

	// Add doc values file
	sci.SetDocValuesGen(3)
	files = sci.GetFiles()
	foundDvd := false
	for _, f := range files {
		if f == "_0_3.dvd" {
			foundDvd = true
			break
		}
	}
	if !foundDvd {
		t.Error("Expected doc values file in list")
	}
}

func TestSegmentCommitInfoList(t *testing.T) {
	si1 := NewSegmentInfo("_0", 100, nil)
	si2 := NewSegmentInfo("_1", 50, nil)

	sci1 := NewSegmentCommitInfo(si1, 10, 1)
	sci2 := NewSegmentCommitInfo(si2, 5, 2)

	list := SegmentCommitInfoList{sci1, sci2}

	if list.Size() != 2 {
		t.Errorf("Expected size=2, got %d", list.Size())
	}

	// Total doc count
	if list.TotalDocCount() != 150 {
		t.Errorf("Expected totalDocCount=150, got %d", list.TotalDocCount())
	}

	// Total num docs (live)
	if list.TotalNumDocs() != 135 { // 150 - 15 deletions
		t.Errorf("Expected totalNumDocs=135, got %d", list.TotalNumDocs())
	}

	// Total del count
	if list.TotalDelCount() != 15 {
		t.Errorf("Expected totalDelCount=15, got %d", list.TotalDelCount())
	}

	// String
	str := list.String()
	if str != "SegmentCommitInfoList(count=2)" {
		t.Errorf("Expected 'SegmentCommitInfoList(count=2)', got '%s'", str)
	}
}

func TestSegmentCommitInfoList_Empty(t *testing.T) {
	list := SegmentCommitInfoList{}

	if list.Size() != 0 {
		t.Error("Empty list should have size 0")
	}
	if list.TotalDocCount() != 0 {
		t.Error("Empty list should have totalDocCount 0")
	}
	if list.TotalNumDocs() != 0 {
		t.Error("Empty list should have totalNumDocs 0")
	}
	if list.TotalDelCount() != 0 {
		t.Error("Empty list should have totalDelCount 0")
	}
}

func TestSegmentCommitInfo_ConcurrentAccess(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)
	sci := NewSegmentCommitInfo(si, 10, 1)

	// Concurrent reads
	done := make(chan bool, 3)

	// Reader 1
	go func() {
		for i := 0; i < 100; i++ {
			sci.DelCount()
		}
		done <- true
	}()

	// Reader 2
	go func() {
		for i := 0; i < 100; i++ {
			sci.NumDocs()
		}
		done <- true
	}()

	// Reader 3
	go func() {
		for i := 0; i < 100; i++ {
			sci.HasDeletions()
		}
		done <- true
	}()

	// Wait for all readers
	for i := 0; i < 3; i++ {
		<-done
	}
}
