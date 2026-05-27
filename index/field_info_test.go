// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

func TestNewFieldInfo(t *testing.T) {
	opts := FieldInfoOptions{
		IndexOptions:             IndexOptionsDocsAndFreqs,
		DocValuesType:            DocValuesTypeNone,
		Stored:                   true,
		Tokenized:                true,
		OmitNorms:                false,
		StoreTermVectors:         true,
		StoreTermVectorPositions: true,
		StoreTermVectorOffsets:   false,
		StoreTermVectorPayloads:  false,
	}

	fi := NewFieldInfo("title", 0, opts)

	if fi.Name() != "title" {
		t.Errorf("Expected name 'title', got '%s'", fi.Name())
	}
	if fi.Number() != 0 {
		t.Errorf("Expected number 0, got %d", fi.Number())
	}
	if fi.IndexOptions() != IndexOptionsDocsAndFreqs {
		t.Errorf("Expected IndexOptionsDocsAndFreqs, got %v", fi.IndexOptions())
	}
	if !fi.IsStored() {
		t.Error("Expected IsStored=true")
	}
	if !fi.IsTokenized() {
		t.Error("Expected IsTokenized=true")
	}
	if fi.OmitNorms() {
		t.Error("Expected OmitNorms=false")
	}
	if !fi.StoreTermVectors() {
		t.Error("Expected StoreTermVectors=true")
	}
	if !fi.StoreTermVectorPositions() {
		t.Error("Expected StoreTermVectorPositions=true")
	}
	if fi.StoreTermVectorOffsets() {
		t.Error("Expected StoreTermVectorOffsets=false")
	}
	if fi.StoreTermVectorPayloads() {
		t.Error("Expected StoreTermVectorPayloads=false")
	}
}

func TestFieldInfo_HasNorms(t *testing.T) {
	// Indexed field with freqs and not omitting norms
	fi1 := NewFieldInfo("field1", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
		OmitNorms:    false,
	})
	if !fi1.HasNorms() {
		t.Error("Expected HasNorms=true for indexed field with freqs and not omitting norms")
	}

	// Indexed field but omitting norms
	fi2 := NewFieldInfo("field2", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
		OmitNorms:    true,
	})
	if fi2.HasNorms() {
		t.Error("Expected HasNorms=false when omitting norms")
	}

	// Not indexed
	fi3 := NewFieldInfo("field3", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsNone,
	})
	if fi3.HasNorms() {
		t.Error("Expected HasNorms=false when not indexed")
	}

	// DOCS_ONLY (no freqs)
	fi4 := NewFieldInfo("field4", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocs,
		OmitNorms:    false,
	})
	if fi4.HasNorms() {
		t.Error("Expected HasNorms=false for DOCS_ONLY")
	}
}

func TestFieldInfo_HasPayloads(t *testing.T) {
	// HasPayloads mirrors Java FieldInfo.hasPayloads(): returns the explicit
	// storePayloads flag, not a heuristic derived from IndexOptions.
	// A field with positions but no observed payloads returns false.
	fi1 := NewFieldInfo("field1", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqsAndPositions,
	})
	if fi1.HasPayloads() {
		t.Error("Expected HasPayloads=false before SetStorePayloads is called")
	}

	// After calling SetStorePayloads, HasPayloads must return true.
	fi1.SetStorePayloads()
	if !fi1.HasPayloads() {
		t.Error("Expected HasPayloads=true after SetStorePayloads")
	}

	// No positions — SetStorePayloads is a no-op; HasPayloads stays false.
	fi2 := NewFieldInfo("field2", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
	})
	fi2.SetStorePayloads() // no-op: IndexOptions < POSITIONS
	if fi2.HasPayloads() {
		t.Error("Expected HasPayloads=false when no positions (SetStorePayloads is a no-op)")
	}
}

func TestFieldInfo_HasTermVectors(t *testing.T) {
	fi1 := NewFieldInfo("field1", 0, FieldInfoOptions{
		StoreTermVectors: true,
	})
	if !fi1.HasTermVectors() {
		t.Error("Expected HasTermVectors=true")
	}

	fi2 := NewFieldInfo("field2", 0, FieldInfoOptions{
		StoreTermVectors: false,
	})
	if fi2.HasTermVectors() {
		t.Error("Expected HasTermVectors=false")
	}
}

func TestFieldInfo_Attributes(t *testing.T) {
	fi := NewFieldInfo("field", 0, FieldInfoOptions{})

	// Get non-existent attribute
	if fi.GetAttribute("key") != "" {
		t.Error("Expected empty string for non-existent attribute")
	}

	// Put attribute (should panic since frozen)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when putting attribute on frozen FieldInfo")
		}
	}()
	fi.PutAttribute("key", "value")
}

// TestFieldInfo_PutCodecAttribute verifies that PutCodecAttribute writes
// attributes on a frozen FieldInfo without panicking and that the value
// is observable through GetAttribute and GetAttributes. This is the
// escape hatch used by per-field codec writers to record format metadata.
func TestFieldInfo_PutCodecAttribute(t *testing.T) {
	fi := NewFieldInfo("field", 0, FieldInfoOptions{})

	// Sanity: FieldInfo is frozen after construction.
	if !fi.IsFrozen() {
		t.Fatalf("FieldInfo should be frozen after construction")
	}

	// Codec writers must be able to record attributes on frozen FieldInfos.
	fi.PutCodecAttribute("PerFieldPostingsFormat.format", "Lucene104PostingsFormat")
	fi.PutCodecAttribute("PerFieldPostingsFormat.suffix", "0")

	if got := fi.GetAttribute("PerFieldPostingsFormat.format"); got != "Lucene104PostingsFormat" {
		t.Errorf("GetAttribute(format) = %q, want %q", got, "Lucene104PostingsFormat")
	}
	if got := fi.GetAttribute("PerFieldPostingsFormat.suffix"); got != "0" {
		t.Errorf("GetAttribute(suffix) = %q, want %q", got, "0")
	}

	// Last write wins; codec writers may overwrite their own keys.
	fi.PutCodecAttribute("PerFieldPostingsFormat.suffix", "1")
	if got := fi.GetAttribute("PerFieldPostingsFormat.suffix"); got != "1" {
		t.Errorf("GetAttribute(suffix) after overwrite = %q, want %q", got, "1")
	}

	// Snapshot returned by GetAttributes must reflect the codec writes.
	attrs := fi.GetAttributes()
	if attrs["PerFieldPostingsFormat.format"] != "Lucene104PostingsFormat" {
		t.Errorf("GetAttributes()[format] = %q, want %q",
			attrs["PerFieldPostingsFormat.format"], "Lucene104PostingsFormat")
	}
	if attrs["PerFieldPostingsFormat.suffix"] != "1" {
		t.Errorf("GetAttributes()[suffix] = %q, want %q",
			attrs["PerFieldPostingsFormat.suffix"], "1")
	}
}

func TestFieldInfo_Clone(t *testing.T) {
	// Use builder to set attributes before freezing
	fi := NewFieldInfoBuilder("title", 0).
		SetIndexOptions(IndexOptionsDocsAndFreqs).
		SetStored(true).
		SetAttribute("custom", "value").
		Build()

	clone := fi.Clone(1)

	// Check clone has new number
	if clone.Number() != 1 {
		t.Errorf("Expected clone number=1, got %d", clone.Number())
	}

	// Check clone has same properties
	if clone.Name() != fi.Name() {
		t.Error("Clone should have same name")
	}
	if clone.IndexOptions() != fi.IndexOptions() {
		t.Error("Clone should have same IndexOptions")
	}
	if clone.IsStored() != fi.IsStored() {
		t.Error("Clone should have same stored setting")
	}

	// Check clone has attributes
	if clone.GetAttribute("custom") != "value" {
		t.Error("Clone should have copied attributes")
	}

	// Clone should also be frozen - verify by checking PutAttribute panics
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when putting attribute on frozen clone")
			}
		}()
		clone.PutAttribute("new", "newvalue") // Should panic
	}()
}

func TestFieldInfo_CheckConsistency(t *testing.T) {
	// Valid FieldInfo
	fi1 := NewFieldInfo("valid", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
		Stored:       true,
	})
	if err := fi1.CheckConsistency(); err != nil {
		t.Errorf("Valid FieldInfo should pass: %v", err)
	}

	// Empty name
	fi2 := NewFieldInfo("", 0, FieldInfoOptions{})
	if err := fi2.CheckConsistency(); err == nil {
		t.Error("Expected error for empty name")
	}

	// Negative number
	fi3 := NewFieldInfo("field", -1, FieldInfoOptions{})
	if err := fi3.CheckConsistency(); err == nil {
		t.Error("Expected error for negative number")
	}

	// Indexed with NONE
	fi4 := NewFieldInfo("field", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsDocsAndFreqs,
	})
	// This is auto-corrected during construction, so it should pass
	if err := fi4.CheckConsistency(); err != nil {
		t.Errorf("Auto-corrected FieldInfo should pass: %v", err)
	}
}

func TestFieldInfo_String(t *testing.T) {
	fi := NewFieldInfo("title", 5, FieldInfoOptions{
		IndexOptions: IndexOptionsDocs,
		Stored:       true,
		Tokenized:    true,
	})

	str := fi.String()
	if str == "" {
		t.Error("String should not be empty")
	}
	if str != "FieldInfo(name=title, number=5, indexed=true, stored=true, tokenized=true)" {
		t.Logf("String representation: %s", str)
	}
}

func TestFieldInfoBuilder(t *testing.T) {
	fi := NewFieldInfoBuilder("body", 1).
		SetIndexOptions(IndexOptionsDocsAndFreqsAndPositions).
		SetDocValuesType(DocValuesTypeNumeric).
		SetStored(true).
		SetTokenized(true).
		SetOmitNorms(true).
		SetStoreTermVectors(true).
		SetStoreTermVectorPositions(true).
		SetStoreTermVectorOffsets(true).
		SetStoreTermVectorPayloads(true).
		SetAttribute("custom", "value").
		Build()

	if fi.Name() != "body" {
		t.Errorf("Expected name 'body', got '%s'", fi.Name())
	}
	if fi.Number() != 1 {
		t.Errorf("Expected number 1, got %d", fi.Number())
	}
	if fi.IndexOptions() != IndexOptionsDocsAndFreqsAndPositions {
		t.Error("Expected IndexOptionsDocsAndFreqsAndPositions")
	}
	if fi.DocValuesType() != DocValuesTypeNumeric {
		t.Error("Expected DocValuesTypeNumeric")
	}
	if !fi.IsStored() {
		t.Error("Expected IsStored=true")
	}
	if !fi.IsTokenized() {
		t.Error("Expected IsTokenized=true")
	}
	if !fi.OmitNorms() {
		t.Error("Expected OmitNorms=true")
	}
	if !fi.StoreTermVectors() {
		t.Error("Expected StoreTermVectors=true")
	}
	if !fi.StoreTermVectorPositions() {
		t.Error("Expected StoreTermVectorPositions=true")
	}
	if !fi.StoreTermVectorOffsets() {
		t.Error("Expected StoreTermVectorOffsets=true")
	}
	if !fi.StoreTermVectorPayloads() {
		t.Error("Expected StoreTermVectorPayloads=true")
	}
	if fi.GetAttribute("custom") != "value" {
		t.Error("Expected custom attribute to be set")
	}
}

func TestFieldInfoBuilder_Defaults(t *testing.T) {
	fi := NewFieldInfoBuilder("field", 0).Build()

	if fi.IndexOptions() != IndexOptionsNone {
		t.Error("Expected default IndexOptionsNone")
	}
	if fi.DocValuesType() != DocValuesTypeNone {
		t.Error("Expected default DocValuesTypeNone")
	}
	if fi.IsStored() {
		t.Error("Expected default Stored=false")
	}
	if fi.IsTokenized() {
		t.Error("Expected default Tokenized=false")
	}
}

func TestFieldInfo_TermVectorAutoEnable(t *testing.T) {
	// Positions enabled should auto-enable term vectors
	fi := NewFieldInfo("field", 0, FieldInfoOptions{
		StoreTermVectorPositions: true,
		StoreTermVectors:         false,
	})
	if !fi.StoreTermVectors() {
		t.Error("StoreTermVectors should be auto-enabled when positions are enabled")
	}

	// Offsets enabled should auto-enable term vectors
	fi2 := NewFieldInfo("field2", 0, FieldInfoOptions{
		StoreTermVectorOffsets: true,
		StoreTermVectors:       false,
	})
	if !fi2.StoreTermVectors() {
		t.Error("StoreTermVectors should be auto-enabled when offsets are enabled")
	}

	// Payloads enabled should auto-enable term vectors
	fi3 := NewFieldInfo("field3", 0, FieldInfoOptions{
		StoreTermVectorPayloads: true,
		StoreTermVectors:        false,
	})
	if !fi3.StoreTermVectors() {
		t.Error("StoreTermVectors should be auto-enabled when payloads are enabled")
	}
}

func TestFieldInfo_TokenizedRequiresIndexing(t *testing.T) {
	// Tokenized without indexing should be auto-corrected
	fi := NewFieldInfo("field", 0, FieldInfoOptions{
		IndexOptions: IndexOptionsNone,
		Tokenized:    true,
	})
	if fi.IsTokenized() {
		t.Error("Tokenized should be auto-disabled when not indexed")
	}
}
