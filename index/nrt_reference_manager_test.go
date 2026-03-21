// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"
)

// TestNewNRTReferenceManager tests creating an NRTReferenceManager
func TestNewNRTReferenceManager(t *testing.T) {
	// Test with nil writer
	_, err := NewNRTReferenceManager(nil, nil)
	if err == nil {
		t.Error("Expected error for nil writer")
	}

	// Test with nil reader
	writer := &IndexWriter{}
	_, err = NewNRTReferenceManager(writer, nil)
	if err == nil {
		t.Error("Expected error for nil reader")
	}
}

// TestNRTReferenceManagerIsReopening tests the IsReopening method
func TestNRTReferenceManagerIsReopening(t *testing.T) {
	// Create a minimal manager for testing
	rm := &NRTReferenceManager{}

	// Default should be false
	if rm.IsReopening() {
		t.Error("Expected IsReopening to be false initially")
	}
}

// TestNRTReferenceManagerIsOpen tests the IsOpen method
func TestNRTReferenceManagerIsOpen(t *testing.T) {
	// Create a minimal manager
	rm := &NRTReferenceManager{}

	// Should return false when ReferenceManager is nil
	if rm.IsOpen() {
		t.Error("Expected IsOpen to be false when ReferenceManager is nil")
	}
}

// TestNRTReferenceManagerGetGeneration tests the GetGeneration method
func TestNRTReferenceManagerGetGeneration(t *testing.T) {
	// Create a minimal manager
	rm := &NRTReferenceManager{}

	// Should return 0 when ReferenceManager is nil
	if rm.GetGeneration() != 0 {
		t.Errorf("Expected generation 0, got %d", rm.GetGeneration())
	}
}

// TestNRTReferenceManagerGetWriter tests the GetWriter method
func TestNRTReferenceManagerGetWriter(t *testing.T) {
	writer := &IndexWriter{}
	rm := &NRTReferenceManager{
		writer: writer,
	}

	if rm.GetWriter() != writer {
		t.Error("GetWriter should return the writer")
	}
}

// TestNRTReferenceManagerWaitForGeneration tests waiting for generation
func TestNRTReferenceManagerWaitForGeneration(t *testing.T) {
	// Create a minimal manager
	rm := &NRTReferenceManager{}

	// Test with no ReferenceManager - should return current state
	reached, err := rm.WaitForGeneration(1, 1000)
	if err != nil {
		t.Errorf("WaitForGeneration returned error: %v", err)
	}

	// With no ReferenceManager, generation is 0, so 1 won't be reached
	if reached {
		t.Error("Expected generation not to be reached")
	}
}
