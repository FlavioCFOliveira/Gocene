// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spatial

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestNewFieldCacheProvider(t *testing.T) {
	provider := NewFieldCacheProvider()
	if provider == nil {
		t.Fatal("expected provider to be non-nil")
	}
	if provider.cache == nil {
		t.Error("expected cache to be initialized")
	}
}

func TestNewSpatialPrefixTreeFieldCacheProvider(t *testing.T) {
	// Create a geohash prefix tree
	prefixTree, err := NewGeohashPrefixTree(12)
	if err != nil {
		t.Fatalf("failed to create geohash prefix tree: %v", err)
	}

	tests := []struct {
		name        string
		fieldName   string
		prefixTree  SpatialPrefixTree
		expectError bool
	}{
		{
			name:        "valid provider",
			fieldName:   "location",
			prefixTree:  prefixTree,
			expectError: false,
		},
		{
			name:        "empty field name",
			fieldName:   "",
			prefixTree:  prefixTree,
			expectError: true,
		},
		{
			name:        "nil prefix tree",
			fieldName:   "location",
			prefixTree:  nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewSpatialPrefixTreeFieldCacheProvider(tt.fieldName, tt.prefixTree)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if provider == nil {
				t.Fatal("expected provider to be non-nil")
			}
			if provider.GetFieldName() != tt.fieldName {
				t.Errorf("expected field name %s, got %s", tt.fieldName, provider.GetFieldName())
			}
			if provider.GetPrefixTree() != tt.prefixTree {
				t.Error("expected prefix tree to match")
			}
		})
	}
}

func TestSpatialPrefixTreeFieldCacheProvider_GetFieldName(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, err := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if provider.GetFieldName() != "test_field" {
		t.Errorf("expected field name 'test_field', got %s", provider.GetFieldName())
	}
}

func TestSpatialPrefixTreeFieldCacheProvider_GetPrefixTree(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, err := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if provider.GetPrefixTree() == nil {
		t.Error("expected prefix tree to be non-nil")
	}
}

func TestSpatialPrefixTreeFieldCacheProvider_InvalidateAll(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, err := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Initially cache should be empty
	if provider.CacheSize() != 0 {
		t.Errorf("expected empty cache, got size %d", provider.CacheSize())
	}

	// Invalidate all should work on empty cache
	provider.InvalidateAll()

	if provider.CacheSize() != 0 {
		t.Errorf("expected empty cache after invalidate, got size %d", provider.CacheSize())
	}
}

func TestCacheStats(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, err := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	stats := provider.GetStats()
	if stats == nil {
		t.Fatal("expected stats to be non-nil")
	}

	// Initially should have 0 entries
	if stats.EntryCount != 0 {
		t.Errorf("expected 0 entries, got %d", stats.EntryCount)
	}
	if stats.TotalDocs != 0 {
		t.Errorf("expected 0 total docs, got %d", stats.TotalDocs)
	}
	if stats.TotalTokens != 0 {
		t.Errorf("expected 0 total tokens, got %d", stats.TotalTokens)
	}
}

func TestNewSpatialFieldCacheValueSource(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, _ := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)
	center := NewPoint(0, 0)

	tests := []struct {
		name        string
		provider    *SpatialPrefixTreeFieldCacheProvider
		center      Point
		multiplier  float64
		expectError bool
	}{
		{
			name:        "valid value source",
			provider:    provider,
			center:      center,
			multiplier:  1.0,
			expectError: false,
		},
		{
			name:        "nil provider",
			provider:    nil,
			center:      center,
			multiplier:  1.0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vs, err := NewSpatialFieldCacheValueSource(tt.provider, tt.center, tt.multiplier)
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if vs == nil {
				t.Fatal("expected value source to be non-nil")
			}
		})
	}
}

func TestSpatialFieldCacheValueSource_Description(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, _ := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)
	center := NewPoint(10, 20)

	vs, err := NewSpatialFieldCacheValueSource(provider, center, 1.0)
	if err != nil {
		t.Fatalf("failed to create value source: %v", err)
	}

	desc := vs.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}

	// Description should contain field name and center
	expectedField := "spatial_field_cache(test_field"
	if len(desc) < len(expectedField) || desc[:len(expectedField)] != expectedField {
		t.Errorf("expected description to start with '%s', got '%s'", expectedField, desc)
	}
}

func TestSpatialPrefixTreeFieldCacheProvider_GetCellTokens_InvalidDocID(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, _ := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)

	// Should return error for negative docID
	var nilReader *index.IndexReader = nil
	_, err := provider.GetCellTokens(-1, nilReader)
	if err == nil {
		t.Error("expected error for negative docID")
	}
}

func TestSpatialPrefixTreeFieldCacheProvider_GetCacheEntry_NilReader(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, _ := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)

	// Should return error for nil reader
	var nilReader *index.IndexReader = nil
	_, err := provider.GetCacheEntry(nilReader)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestSpatialPrefixTreeFieldCacheProvider_HasValues(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, _ := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)

	// With nil reader, should return false
	var nilReader *index.IndexReader = nil
	hasValues := provider.HasValues(0, nilReader)
	if hasValues {
		t.Error("expected HasValues to return false for nil reader")
	}
}

func TestSpatialPrefixTreeFieldCacheProvider_Invalidate(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)
	provider, _ := NewSpatialPrefixTreeFieldCacheProvider("test_field", prefixTree)

	// Invalidate with nil reader should not panic
	var nilReader *index.IndexReader = nil
	provider.Invalidate(nilReader)
}

func TestGenerateReaderKey(t *testing.T) {
	// Test with nil reader - should not panic
	var nilReader *index.IndexReader = nil
	key := generateReaderKey(nilReader)
	if key != "r0_0" {
		t.Errorf("expected key 'r0_0' for nil reader, got '%s'", key)
	}
}

func TestFieldCacheEntry(t *testing.T) {
	prefixTree, _ := NewGeohashPrefixTree(12)

	entry := &FieldCacheEntry{
		fieldName:  "test_field",
		readerKey:  "test_key",
		cellTokens: make([][]string, 10),
		docCount:   10,
		prefixTree: prefixTree,
		hasValues:  make([]bool, 10),
	}

	if entry.fieldName != "test_field" {
		t.Errorf("expected field name 'test_field', got '%s'", entry.fieldName)
	}
	if entry.readerKey != "test_key" {
		t.Errorf("expected reader key 'test_key', got '%s'", entry.readerKey)
	}
	if entry.docCount != 10 {
		t.Errorf("expected doc count 10, got %d", entry.docCount)
	}
}
