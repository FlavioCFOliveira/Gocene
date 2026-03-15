// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"sync"
	"testing"
	"time"
)

// TestSetOnce_EmptyCtor tests that a SetOnce created with the empty constructor
// returns nil (zero value) when Get is called before setting.
func TestSetOnce_EmptyCtor(t *testing.T) {
	set := NewSetOnce[int]()

	if set.IsSet() {
		t.Error("Expected IsSet to be false for empty SetOnce")
	}

	val := set.Get()
	if val != 0 {
		t.Errorf("Expected zero value, got %d", val)
	}
}

// TestSetOnce_SettingCtor tests that a SetOnce created with a value
// returns that value and throws AlreadySetException on subsequent Set calls.
func TestSetOnce_SettingCtor(t *testing.T) {
	set := NewSetOnceWithValue(5)

	if !set.IsSet() {
		t.Error("Expected IsSet to be true for SetOnce with initial value")
	}

	val := set.Get()
	if val != 5 {
		t.Errorf("Expected 5, got %d", val)
	}

	// Attempting to set again should return AlreadySetException
	err := set.Set(7)
	if err == nil {
		t.Error("Expected AlreadySetException when setting twice")
	}

	// Verify the value hasn't changed
	val = set.Get()
	if val != 5 {
		t.Errorf("Expected value to remain 5, got %d", val)
	}
}

// TestSetOnce_SetOnce tests basic Set and Get functionality,
// and verifies AlreadySetException is thrown on second Set.
func TestSetOnce_SetOnce(t *testing.T) {
	set := NewSetOnce[int]()

	err := set.Set(5)
	if err != nil {
		t.Fatalf("Unexpected error on first Set: %v", err)
	}

	val := set.Get()
	if val != 5 {
		t.Errorf("Expected 5, got %d", val)
	}

	// Second Set should fail
	err = set.Set(7)
	if err == nil {
		t.Error("Expected AlreadySetException when setting twice")
	}

	// Verify the value hasn't changed
	val = set.Get()
	if val != 5 {
		t.Errorf("Expected value to remain 5, got %d", val)
	}
}

// TestSetOnce_TrySet tests the TrySet method which returns boolean
// instead of throwing exception.
func TestSetOnce_TrySet(t *testing.T) {
	set := NewSetOnce[int]()

	// First TrySet should succeed
	if !set.TrySet(5) {
		t.Error("Expected TrySet to return true on first call")
	}

	val := set.Get()
	if val != 5 {
		t.Errorf("Expected 5, got %d", val)
	}

	// Second TrySet should fail
	if set.TrySet(7) {
		t.Error("Expected TrySet to return false on second call")
	}

	// Verify the value hasn't changed
	val = set.Get()
	if val != 5 {
		t.Errorf("Expected value to remain 5, got %d", val)
	}
}

// TestSetOnce_SetMultiThreaded tests that SetOnce is thread-safe
// and only one thread succeeds in setting the value.
func TestSetOnce_SetMultiThreaded(t *testing.T) {
	set := NewSetOnce[int]()
	numThreads := 10

	var wg sync.WaitGroup
	wg.Add(numThreads)

	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numThreads; i++ {
		go func(id int) {
			defer wg.Done()

			// Random sleep to increase chance of race conditions
			time.Sleep(time.Duration(RandomIntN(10)) * time.Millisecond)

			err := set.Set(id)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Only one thread should have succeeded
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful Set, got %d", successCount)
	}

	// The value should be set
	if !set.IsSet() {
		t.Error("Expected IsSet to be true after concurrent Sets")
	}

	// Get the value and verify it's one of the thread IDs
	val := set.Get()
	if val < 0 || val >= numThreads {
		t.Errorf("Expected value to be between 0 and %d, got %d", numThreads-1, val)
	}
}

// TestSetOnce_TrySetMultiThreaded tests that TrySet is thread-safe
// and only one thread succeeds.
func TestSetOnce_TrySetMultiThreaded(t *testing.T) {
	set := NewSetOnce[int]()
	numThreads := 10

	var wg sync.WaitGroup
	wg.Add(numThreads)

	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numThreads; i++ {
		go func(id int) {
			defer wg.Done()

			// Random sleep to increase chance of race conditions
			time.Sleep(time.Duration(RandomIntN(10)) * time.Millisecond)

			if set.TrySet(id) {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Only one thread should have succeeded
	if successCount != 1 {
		t.Errorf("Expected exactly 1 successful TrySet, got %d", successCount)
	}

	// The value should be set
	if !set.IsSet() {
		t.Error("Expected IsSet to be true after concurrent TrySets")
	}
}

// TestSetOnce_WithString tests SetOnce with string type.
func TestSetOnce_WithString(t *testing.T) {
	set := NewSetOnce[string]()

	err := set.Set("hello")
	if err != nil {
		t.Fatalf("Unexpected error on first Set: %v", err)
	}

	val := set.Get()
	if val != "hello" {
		t.Errorf("Expected 'hello', got '%s'", val)
	}

	// Second Set should fail
	err = set.Set("world")
	if err == nil {
		t.Error("Expected AlreadySetException when setting twice")
	}

	// Value should remain unchanged
	val = set.Get()
	if val != "hello" {
		t.Errorf("Expected value to remain 'hello', got '%s'", val)
	}
}

// TestSetOnce_WithPointer tests SetOnce with pointer type.
func TestSetOnce_WithPointer(t *testing.T) {
	type testStruct struct {
		Value int
	}

	set := NewSetOnce[*testStruct]()

	obj := &testStruct{Value: 42}
	err := set.Set(obj)
	if err != nil {
		t.Fatalf("Unexpected error on first Set: %v", err)
	}

	val := set.Get()
	if val == nil {
		t.Fatal("Expected non-nil value")
	}
	if val.Value != 42 {
		t.Errorf("Expected Value 42, got %d", val.Value)
	}

	// Second Set should fail
	err = set.Set(&testStruct{Value: 100})
	if err == nil {
		t.Error("Expected AlreadySetException when setting twice")
	}

	// Value should remain unchanged
	val = set.Get()
	if val.Value != 42 {
		t.Errorf("Expected Value to remain 42, got %d", val.Value)
	}
}

// TestSetOnce_AlreadySetExceptionError tests the AlreadySetException error message.
func TestSetOnce_AlreadySetExceptionError(t *testing.T) {
	set := NewSetOnce[int]()
	set.Set(1)

	err := set.Set(2)
	if err == nil {
		t.Fatal("Expected error on second Set")
	}

	// Check error message
	if err.Error() != "The object cannot be set twice!" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

// TestSetOnce_IsSetConsistency tests that IsSet is consistent with Get behavior.
func TestSetOnce_IsSetConsistency(t *testing.T) {
	set := NewSetOnce[int]()

	// Before setting
	if set.IsSet() {
		t.Error("IsSet should be false before any Set")
	}
	if set.Get() != 0 {
		t.Error("Get should return zero value before any Set")
	}

	// After setting
	set.Set(42)
	if !set.IsSet() {
		t.Error("IsSet should be true after Set")
	}
	if set.Get() != 42 {
		t.Errorf("Get should return 42 after Set, got %d", set.Get())
	}
}

// TestSetOnce_ZeroValueCanBeSet tests that a zero value can be explicitly set.
func TestSetOnce_ZeroValueCanBeSet(t *testing.T) {
	set := NewSetOnce[int]()

	// Set zero value
	err := set.Set(0)
	if err != nil {
		t.Fatalf("Unexpected error when setting zero value: %v", err)
	}

	if !set.IsSet() {
		t.Error("IsSet should be true after setting zero value")
	}

	if set.Get() != 0 {
		t.Errorf("Expected 0, got %d", set.Get())
	}

	// Second Set should still fail
	err = set.Set(1)
	if err == nil {
		t.Error("Expected AlreadySetException when setting twice, even with zero value first")
	}
}
