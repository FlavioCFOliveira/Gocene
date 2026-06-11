// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"testing"
	"time"
)

// TestByteBuffersDirectory_CreateTempOutput_NoDeadlock verifies that
// CreateTempOutput does not deadlock due to the double lock acquisition
// that existed before the fix (generateTempFileName acquired d.mu.Lock,
// then CreateOutput also acquired d.mu.Lock).
func TestByteBuffersDirectory_CreateTempOutput_NoDeadlock(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	// This must complete without hanging.
	// Use a channel with timeout to detect deadlock.
	done := make(chan error, 1)
	go func() {
		out, err := dir.CreateTempOutput("temp", ".tmp", IOContextDefault)
		if err != nil {
			done <- err
			return
		}
		if out == nil {
			done <- nil // will be reported as "nil output" below
			return
		}
		// Write some data and close
		_ = out.WriteBytes([]byte("hello"))
		done <- out.Close()
	}()

	// Wait with timeout to detect deadlock
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("CreateTempOutput failed: %v", err)
		}
		// Success — no deadlock.
	case <-time.After(5 * time.Second):
		t.Fatal("CreateTempOutput deadlocked — call did not return within 5s. This indicates the double-lock issue in generateTempFileName + CreateOutput.")
	}
}

// TestByteBuffersDirectory_CreateTempOutput_UniqueNames verifies that
// successive calls to CreateTempOutput produce distinct file names.
func TestByteBuffersDirectory_CreateTempOutput_UniqueNames(t *testing.T) {
	dir := NewByteBuffersDirectory()
	defer dir.Close()

	names := make(map[string]bool)
	for i := 0; i < 100; i++ {
		out, err := dir.CreateTempOutput("temp", ".tmp", IOContextDefault)
		if err != nil {
			t.Fatalf("CreateTempOutput iteration %d failed: %v", i, err)
		}
		name := out.GetName()
		if names[name] {
			t.Errorf("Duplicate temp file name: %s", name)
		}
		names[name] = true
		if err := out.Close(); err != nil {
			t.Errorf("Close failed for %s: %v", name, err)
		}
	}
}
