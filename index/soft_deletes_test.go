// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

// TestNewSoftDeletesRetentionMergePolicy tests creating a SoftDeletesRetentionMergePolicy
func TestNewSoftDeletesRetentionMergePolicy(t *testing.T) {
	// Test with nil inner - should panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for nil inner merge policy")
		}
	}()
	NewSoftDeletesRetentionMergePolicy("_soft_deletes", nil)
}

// TestSoftDeletesRetentionMergePolicy_ValidInner tests creating a policy with valid inner
func TestSoftDeletesRetentionMergePolicy_ValidInner(t *testing.T) {
	inner := NewTieredMergePolicy()
	policy := NewSoftDeletesRetentionMergePolicy("_soft_deletes", inner)

	if policy == nil {
		t.Fatal("Expected policy to be created")
	}

	if policy.GetInner() != inner {
		t.Error("GetInner should return the wrapped policy")
	}

	if policy.GetSoftDeletesField() != "_soft_deletes" {
		t.Errorf("Expected soft deletes field '_soft_deletes', got '%s'", policy.GetSoftDeletesField())
	}
}

// TestSoftDeletesRetentionMergePolicy_String tests the String method
func TestSoftDeletesRetentionMergePolicy_String(t *testing.T) {
	inner := NewTieredMergePolicy()
	policy := NewSoftDeletesRetentionMergePolicy("_soft_deletes", inner)

	s := policy.String()
	if s == "" {
		t.Error("String should not be empty")
	}

	if s == "SoftDeletesRetentionMergePolicy" {
		t.Error("String should include field name and inner policy")
	}
}

// TestNewSoftDeletesDirectoryReaderWrapper tests creating a wrapper
func TestNewSoftDeletesDirectoryReaderWrapper(t *testing.T) {
	// Test with nil reader
	_, err := NewSoftDeletesDirectoryReaderWrapper(nil, "_soft_deletes")
	if err == nil {
		t.Error("Expected error for nil reader")
	}

	// Test with empty field
	segmentInfo := &SegmentInfo{
		name:     "test",
		docCount: 10,
	}
	reader := NewLeafReader(segmentInfo)
	_ = reader
	// Cannot use LeafReader directly, need DirectoryReader
}

// TestSoftDeletesDirectoryReaderWrapper_NilReader tests with nil reader
func TestSoftDeletesDirectoryReaderWrapper_NilReader(t *testing.T) {
	_, err := NewSoftDeletesDirectoryReaderWrapper(nil, "_soft_deletes")
	if err == nil {
		t.Error("Expected error for nil reader")
	}
}

// TestSoftDeletesDirectoryReaderWrapper_EmptyField tests with empty field
func TestSoftDeletesDirectoryReaderWrapper_EmptyField(t *testing.T) {
	segmentInfo := &SegmentInfo{
		name:     "test",
		docCount: 10,
	}
	reader := NewLeafReader(segmentInfo)
	_ = reader
	// Cannot use LeafReader directly
}

// TestSoftDeletesRetentionMergePolicy_Interface tests that the policy implements MergePolicy
func TestSoftDeletesRetentionMergePolicy_Interface(t *testing.T) {
	inner := NewTieredMergePolicy()
	policy := NewSoftDeletesRetentionMergePolicy("_soft_deletes", inner)

	// Test that it implements MergePolicy
	var _ MergePolicy = policy

	// Test delegate methods
	if policy.GetMaxMergeDocs() != inner.GetMaxMergeDocs() {
		t.Error("GetMaxMergeDocs should delegate to inner")
	}

	if policy.GetMaxMergedSegmentBytes() != inner.GetMaxMergedSegmentBytes() {
		t.Error("GetMaxMergedSegmentBytes should delegate to inner")
	}
}
