// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

func TestNewPriorityQueue(t *testing.T) {
	// Valid creation
	pq, err := NewPriorityQueue(10, func(a, b int) bool { return a < b })
	if err != nil {
		t.Fatalf("Failed to create PriorityQueue: %v", err)
	}
	if pq == nil {
		t.Fatal("Expected non-nil PriorityQueue")
	}
	if pq.MaxSize() != 10 {
		t.Errorf("Expected maxSize 10, got %d", pq.MaxSize())
	}

	// Negative size
	_, err = NewPriorityQueue[int](-1, func(a, b int) bool { return a < b })
	if err == nil {
		t.Error("Expected error for negative size")
	}

	// Nil less function
	_, err = NewPriorityQueue[int](10, nil)
	if err == nil {
		t.Error("Expected error for nil less function")
	}
}

func TestPriorityQueue_Add(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	// Add elements
	if !pq.Add(3) {
		t.Error("Expected Add to succeed")
	}
	if !pq.Add(1) {
		t.Error("Expected Add to succeed")
	}
	if !pq.Add(4) {
		t.Error("Expected Add to succeed")
	}
	if !pq.Add(2) {
		t.Error("Expected Add to succeed")
	}

	if pq.Size() != 4 {
		t.Errorf("Expected size 4, got %d", pq.Size())
	}
}

func TestPriorityQueue_Add_Full(t *testing.T) {
	pq, _ := NewPriorityQueue(3, func(a, b int) bool { return a < b })

	// Fill the queue
	pq.Add(3)
	pq.Add(1)
	pq.Add(2)

	if pq.Size() != 3 {
		t.Errorf("Expected size 3, got %d", pq.Size())
	}

	// Try to add element with lower priority
	if pq.Add(5) {
		t.Error("Expected Add to fail for lower priority element")
	}

	// Add element with higher priority
	if !pq.Add(0) {
		t.Error("Expected Add to succeed for higher priority element")
	}

	// Top should now be 0 (0 replaced 3, and 0 is now the highest priority)
	if pq.Top() != 0 {
		t.Errorf("Expected top to be 0, got %d", pq.Top())
	}
}

func TestPriorityQueue_Pop(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	// Add elements
	pq.Add(3)
	pq.Add(1)
	pq.Add(4)
	pq.Add(2)
	pq.Add(5)

	// Pop should return in order
	expected := []int{1, 2, 3, 4, 5}
	for _, exp := range expected {
		val := pq.Pop()
		if val != exp {
			t.Errorf("Expected %d, got %d", exp, val)
		}
	}

	if !pq.IsEmpty() {
		t.Error("Expected queue to be empty")
	}

	// Pop from empty queue
	val := pq.Pop()
	if val != 0 {
		t.Errorf("Expected zero value from empty queue, got %d", val)
	}
}

func TestPriorityQueue_Top(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	// Top from empty
	if pq.Top() != 0 {
		t.Errorf("Expected zero from empty queue, got %d", pq.Top())
	}

	pq.Add(3)
	if pq.Top() != 3 {
		t.Errorf("Expected top 3, got %d", pq.Top())
	}

	pq.Add(1)
	if pq.Top() != 1 {
		t.Errorf("Expected top 1, got %d", pq.Top())
	}
}

func TestPriorityQueue_UpdateTop(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	pq.Add(3)
	pq.Add(1)
	pq.Add(2)

	// Modify top through Set
	pq.Set(0, 5)
	pq.UpdateTop()

	// New top should be 2
	if pq.Top() != 2 {
		t.Errorf("Expected top 2 after update, got %d", pq.Top())
	}
}

func TestPriorityQueue_IsEmpty(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	if !pq.IsEmpty() {
		t.Error("Expected empty initially")
	}

	pq.Add(1)
	if pq.IsEmpty() {
		t.Error("Expected not empty after add")
	}
}

func TestPriorityQueue_Clear(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	pq.Add(1)
	pq.Add(2)
	pq.Clear()

	if !pq.IsEmpty() {
		t.Error("Expected empty after clear")
	}
	if pq.Size() != 0 {
		t.Errorf("Expected size 0, got %d", pq.Size())
	}
}

func TestPriorityQueue_Get(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	pq.Add(1)
	pq.Add(2)

	val, err := pq.Get(0)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != 1 {
		t.Errorf("Expected 1 at index 0, got %d", val)
	}

	// Invalid index
	_, err = pq.Get(-1)
	if err == nil {
		t.Error("Expected error for negative index")
	}

	_, err = pq.Get(10)
	if err == nil {
		t.Error("Expected error for out of bounds index")
	}
}

func TestPriorityQueue_Set(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	pq.Add(1)
	pq.Add(2)

	err := pq.Set(0, 5)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, _ := pq.Get(0)
	if val != 5 {
		t.Errorf("Expected 5 at index 0, got %d", val)
	}

	// Invalid index
	err = pq.Set(-1, 0)
	if err == nil {
		t.Error("Expected error for negative index")
	}
}

func TestNewIntMinPriorityQueue(t *testing.T) {
	pq, err := NewIntMinPriorityQueue(5)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	pq.Add(3)
	pq.Add(1)
	pq.Add(2)

	if pq.Pop() != 1 {
		t.Error("Expected min-heap to pop smallest first")
	}
}

func TestNewIntMaxPriorityQueue(t *testing.T) {
	pq, err := NewIntMaxPriorityQueue(5)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	pq.Add(1)
	pq.Add(3)
	pq.Add(2)

	if pq.Pop() != 3 {
		t.Error("Expected max-heap to pop largest first")
	}
}

func TestNewFloat64MinPriorityQueue(t *testing.T) {
	pq, err := NewFloat64MinPriorityQueue(5)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	pq.Add(3.5)
	pq.Add(1.2)
	pq.Add(2.8)

	if pq.Pop() != 1.2 {
		t.Error("Expected min-heap to pop smallest first")
	}
}

func TestNewStringPriorityQueue(t *testing.T) {
	pq, err := NewStringPriorityQueue(5)
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	pq.Add("banana")
	pq.Add("apple")
	pq.Add("cherry")

	if pq.Pop() != "apple" {
		t.Error("Expected lexicographic order")
	}
}

func TestPriorityQueue_ToSlice(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })

	pq.Add(3)
	pq.Add(1)
	pq.Add(2)

	slice := pq.ToSlice()
	if len(slice) != 3 {
		t.Errorf("Expected slice length 3, got %d", len(slice))
	}
}

func TestPriorityQueue_CustomType(t *testing.T) {
	type Item struct {
		Priority int
		Name     string
	}

	pq, _ := NewPriorityQueue(5, func(a, b Item) bool {
		return a.Priority < b.Priority
	})

	pq.Add(Item{Priority: 2, Name: "medium"})
	pq.Add(Item{Priority: 1, Name: "high"})
	pq.Add(Item{Priority: 3, Name: "low"})

	top := pq.Pop()
	if top.Name != "high" {
		t.Errorf("Expected 'high', got %s", top.Name)
	}
}

// TestPriorityQueue_AddAll covers the Floyd build-heap bulk-add path.
func TestPriorityQueue_AddAll(t *testing.T) {
	pq, _ := NewPriorityQueue(10, func(a, b int) bool { return a < b })
	if err := pq.AddAll([]int{5, 1, 4, 2, 3}); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	want := []int{1, 2, 3, 4, 5}
	for i, w := range want {
		got := pq.Pop()
		if got != w {
			t.Fatalf("pos %d: got=%d want=%d", i, got, w)
		}
	}
}

// TestPriorityQueue_AddAllOverflow verifies the capacity guard.
func TestPriorityQueue_AddAllOverflow(t *testing.T) {
	pq, _ := NewPriorityQueue(3, func(a, b int) bool { return a < b })
	if err := pq.AddAll([]int{1, 2, 3, 4}); err == nil {
		t.Fatalf("expected error for AddAll exceeding maxSize")
	}
}

// TestPriorityQueue_InsertWithOverflow covers all three branches of
// the Java contract.
func TestPriorityQueue_InsertWithOverflow(t *testing.T) {
	pq, _ := NewPriorityQueue(3, func(a, b int) bool { return a < b })

	// Branch 1: room available, returns zero, ok=false.
	if v, ok := pq.InsertWithOverflow(10); v != 0 || ok {
		t.Fatalf("expected (0,false) when room, got (%d,%t)", v, ok)
	}
	pq.Add(5)
	pq.Add(7) // pq = [5,7,10] (min-heap top=5)

	// Branch 2: queue full, smaller-than-current-top element rejected.
	// With min-heap and lessFunc(a<b), top is smallest. lessFunc(top,e)
	// is true iff top<e — meaning e is *bigger*. A bigger e replaces the
	// top. A smaller e (<=top) is rejected.
	if v, ok := pq.InsertWithOverflow(1); v != 1 || !ok {
		t.Fatalf("expected (1,true) for rejected smaller insert, got (%d,%t)", v, ok)
	}

	// Branch 3: full, element replaces displaced top.
	if v, ok := pq.InsertWithOverflow(20); v != 5 || !ok {
		t.Fatalf("expected (5,true) for replacement, got (%d,%t)", v, ok)
	}
}

// TestPriorityQueue_UpdateTopWith verifies the Java updateTop(newTop).
func TestPriorityQueue_UpdateTopWith(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })
	pq.Add(1)
	pq.Add(3)
	pq.Add(5)
	pq.UpdateTopWith(100)
	// 100 should bubble down; min should now be 3.
	if top := pq.Top(); top != 3 {
		t.Fatalf("after UpdateTopWith(100), top=%d want 3", top)
	}
}

// TestPriorityQueue_Remove exercises identity-based removal.
func TestPriorityQueue_Remove(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })
	for _, v := range []int{5, 1, 3, 2, 4} {
		pq.Add(v)
	}
	if !pq.Remove(func(v int) bool { return v == 3 }) {
		t.Fatalf("Remove returned false for present element")
	}
	out := []int{}
	for pq.Size() > 0 {
		out = append(out, pq.Pop())
	}
	want := []int{1, 2, 4, 5}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("after Remove(3) pop order = %v want %v", out, want)
		}
	}
}

// TestPriorityQueue_All exercises the range-over-func iterator.
func TestPriorityQueue_All(t *testing.T) {
	pq, _ := NewPriorityQueue(5, func(a, b int) bool { return a < b })
	for _, v := range []int{3, 1, 4, 1, 5} {
		pq.Add(v)
	}
	count := 0
	for range pq.All {
		count++
	}
	if count != pq.Size() {
		t.Fatalf("All iterated %d, expected %d", count, pq.Size())
	}
}
