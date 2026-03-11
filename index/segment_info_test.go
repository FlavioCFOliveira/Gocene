// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestNewSegmentInfo(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	si := NewSegmentInfo("_0", 100, dir)

	if si.Name() != "_0" {
		t.Errorf("Expected name '_0', got '%s'", si.Name())
	}
	if si.DocCount() != 100 {
		t.Errorf("Expected docCount=100, got %d", si.DocCount())
	}
	if si.Directory() != dir {
		t.Error("Directory should match")
	}
	if si.Version() != "10.0.0" {
		t.Errorf("Expected version '10.0.0', got '%s'", si.Version())
	}
	if si.Codec() != "Lucene104" {
		t.Errorf("Expected codec 'Lucene104', got '%s'", si.Codec())
	}
	if si.IsCompoundFile() {
		t.Error("Expected isCompoundFile=false by default")
	}
}

func TestSegmentInfo_Files(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	// Initially empty
	if len(si.Files()) != 0 {
		t.Errorf("Expected 0 files initially, got %d", len(si.Files()))
	}

	// Add files
	si.AddFile("_0.cfs")
	si.AddFile("_0.cfe")
	si.AddFile("_0.si")

	files := si.Files()
	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}

	// Files should be sorted
	expected := []string{"_0.cfe", "_0.cfs", "_0.si"}
	for i, exp := range expected {
		if files[i] != exp {
			t.Errorf("Expected file '%s' at index %d, got '%s'", exp, i, files[i])
		}
	}

	// Set files (replaces all)
	si.SetFiles([]string{"_1.cfs", "_1.si"})
	files = si.Files()
	if len(files) != 2 {
		t.Errorf("Expected 2 files after SetFiles, got %d", len(files))
	}

	// HasFile
	if !si.HasFile("_1.cfs") {
		t.Error("Should have file '_1.cfs'")
	}
	if si.HasFile("_0.cfs") {
		t.Error("Should not have file '_0.cfs' after SetFiles")
	}
}

func TestSegmentInfo_Documents(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	if si.DocCount() != 100 {
		t.Errorf("Expected docCount=100, got %d", si.DocCount())
	}

	// Update doc count (e.g., after deletions)
	si.SetDocCount(90)
	if si.DocCount() != 90 {
		t.Errorf("Expected docCount=90 after update, got %d", si.DocCount())
	}
}

func TestSegmentInfo_CompoundFile(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	if si.IsCompoundFile() {
		t.Error("Expected isCompoundFile=false initially")
	}

	si.SetCompoundFile(true)
	if !si.IsCompoundFile() {
		t.Error("Expected isCompoundFile=true after setting")
	}
}

func TestSegmentInfo_Codec(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	if si.Codec() != "Lucene104" {
		t.Errorf("Expected default codec 'Lucene104', got '%s'", si.Codec())
	}

	si.SetCodec("Lucene95")
	if si.Codec() != "Lucene95" {
		t.Errorf("Expected codec 'Lucene95', got '%s'", si.Codec())
	}
}

func TestSegmentInfo_Version(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	if si.Version() != "10.0.0" {
		t.Errorf("Expected default version '10.0.0', got '%s'", si.Version())
	}

	si.SetVersion("9.8.0")
	if si.Version() != "9.8.0" {
		t.Errorf("Expected version '9.8.0', got '%s'", si.Version())
	}
}

func TestSegmentInfo_Diagnostics(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	// Initially empty
	if len(si.GetDiagnostics()) != 0 {
		t.Error("Expected empty diagnostics initially")
	}

	// Set diagnostic
	si.SetDiagnostic("source", "flush")
	if si.GetDiagnostic("source") != "flush" {
		t.Errorf("Expected diagnostic 'source'='flush', got '%s'", si.GetDiagnostic("source"))
	}

	// Get non-existent
	if si.GetDiagnostic("nonexistent") != "" {
		t.Error("Expected empty string for non-existent diagnostic")
	}

	// Set multiple diagnostics
	diagnostics := map[string]string{
		"source": "merge",
		"time":   "2026-03-11",
	}
	si.SetDiagnostics(diagnostics)

	got := si.GetDiagnostics()
	if len(got) != 2 {
		t.Errorf("Expected 2 diagnostics, got %d", len(got))
	}
	if got["source"] != "merge" {
		t.Error("Expected source='merge'")
	}
}

func TestSegmentInfo_Attributes(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	// Set attribute
	si.SetAttribute("custom", "value")
	if si.GetAttribute("custom") != "value" {
		t.Errorf("Expected attribute 'custom'='value', got '%s'", si.GetAttribute("custom"))
	}

	// Get non-existent
	if si.GetAttribute("nonexistent") != "" {
		t.Error("Expected empty string for non-existent attribute")
	}
}

func TestSegmentInfo_IndexSort(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	// Initially nil
	if si.IndexSort() != nil {
		t.Error("Expected nil index sort initially")
	}
	if si.GetIndexSortDescription() != "<not sorted>" {
		t.Errorf("Expected '<not sorted>', got '%s'", si.GetIndexSortDescription())
	}

	// Set sort
	sort := &Sort{
		fields: []SortField{
			{field: "date", descending: true, sortType: SortTypeLong},
			{field: "title", descending: false, sortType: SortTypeString},
		},
	}
	si.SetIndexSort(sort)

	if si.IndexSort() == nil {
		t.Fatal("IndexSort should not be nil")
	}
	if len(si.IndexSort().fields) != 2 {
		t.Errorf("Expected 2 sort fields, got %d", len(si.IndexSort().fields))
	}

	desc := si.GetIndexSortDescription()
	if desc == "" {
		t.Error("GetIndexSortDescription should not be empty")
	}
	// Description should contain field names
	t.Logf("Sort description: %s", desc)
}

func TestSegmentInfo_ID(t *testing.T) {
	si := NewSegmentInfo("_5", 100, nil)

	if si.GetID() != "_5" {
		t.Errorf("Expected ID '_5', got '%s'", si.GetID())
	}
}

func TestSegmentInfo_Generation(t *testing.T) {
	// Test parsing generation from name
	tests := []struct {
		name string
		gen  int64
	}{
		{"_0", 0},
		{"_1", 1},
		{"_10", 10},
		{"_123", 123},
		{"segment", 0}, // invalid format returns 0
		{"", 0},
	}

	for _, tc := range tests {
		si := NewSegmentInfo(tc.name, 100, nil)
		gen := si.GetGeneration()
		if gen != tc.gen {
			t.Errorf("For name '%s', expected generation %d, got %d", tc.name, tc.gen, gen)
		}
	}
}

func TestSegmentInfo_String(t *testing.T) {
	si := NewSegmentInfo("_0", 100, nil)

	str := si.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	expected := "SegmentInfo(name=_0, docCount=100, version=10.0.0, codec=Lucene104)"
	if str != expected {
		t.Errorf("Expected '%s', got '%s'", expected, str)
	}
}

func TestSegmentInfo_Clone(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	original := NewSegmentInfo("_0", 100, dir)
	original.SetFiles([]string{"_0.cfs", "_0.cfe"})
	original.SetDiagnostic("source", "flush")
	original.SetAttribute("custom", "value")
	original.SetCompoundFile(true)
	original.SetCodec("Lucene95")
	original.SetVersion("9.8.0")

	clone := original.Clone()

	// Clone should have same values
	if clone.Name() != original.Name() {
		t.Error("Clone should have same name")
	}
	if clone.DocCount() != original.DocCount() {
		t.Error("Clone should have same docCount")
	}
	if clone.Version() != original.Version() {
		t.Error("Clone should have same version")
	}
	if clone.Codec() != original.Codec() {
		t.Error("Clone should have same codec")
	}
	if clone.IsCompoundFile() != original.IsCompoundFile() {
		t.Error("Clone should have same isCompoundFile")
	}
	if clone.GetDiagnostic("source") != "flush" {
		t.Error("Clone should have copied diagnostics")
	}
	if clone.GetAttribute("custom") != "value" {
		t.Error("Clone should have copied attributes")
	}

	// Modify clone should not affect original
	clone.SetDocCount(50)
	if original.DocCount() == 50 {
		t.Error("Modifying clone should not affect original")
	}
}

func TestSegmentInfo_AddDiagnosticsFromMerge(t *testing.T) {
	si := NewSegmentInfo("_2", 200, nil)

	merged := []*SegmentInfo{
		NewSegmentInfo("_0", 100, nil),
		NewSegmentInfo("_1", 100, nil),
	}

	si.AddDiagnosticsFromMerge(merged)

	if si.GetDiagnostic("merge") != "true" {
		t.Error("Expected merge=true")
	}
	if si.GetDiagnostic("mergedSegmentCount") != "2" {
		t.Error("Expected mergedSegmentCount=2")
	}
	if si.GetDiagnostic("sourceSegment0") != "_0" {
		t.Error("Expected sourceSegment0=_0")
	}
	if si.GetDiagnostic("sourceSegment1") != "_1" {
		t.Error("Expected sourceSegment1=_1")
	}
}

func TestSegmentInfo_SizeInBytes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	si := NewSegmentInfo("_0", 100, dir)

	// Create some files
	out, _ := dir.CreateOutput("_0.cfs", store.IOContext{})
	out.WriteBytes([]byte("test data"))
	out.Close()

	si.SetFiles([]string{"_0.cfs"})

	size := si.SizeInBytes()
	if size != 9 { // "test data" is 9 bytes
		t.Errorf("Expected size=9, got %d", size)
	}
}

func TestSegmentInfoList(t *testing.T) {
	list := SegmentInfoList{
		NewSegmentInfo("_0", 100, nil),
		NewSegmentInfo("_1", 50, nil),
		NewSegmentInfo("_2", 25, nil),
	}

	total := list.TotalDocCount()
	if total != 175 {
		t.Errorf("Expected total doc count 175, got %d", total)
	}

	maxDoc := list.GetMaxDoc()
	if maxDoc != 174 {
		t.Errorf("Expected max doc 174, got %d", maxDoc)
	}

	str := list.String()
	if str != "SegmentInfoList(count=3)" {
		t.Errorf("Expected 'SegmentInfoList(count=3)', got '%s'", str)
	}
}

func TestSegmentInfoList_Empty(t *testing.T) {
	list := SegmentInfoList{}

	if list.TotalDocCount() != 0 {
		t.Error("Empty list should have total=0")
	}
	if list.GetMaxDoc() != -1 {
		t.Errorf("Empty list should have maxDoc=-1, got %d", list.GetMaxDoc())
	}
}
