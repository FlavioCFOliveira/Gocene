// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"testing"
)

func TestNewBytesRefArray(t *testing.T) {
	bra := NewBytesRefArray(0)
	if bra.blockSize != 1024 {
		t.Errorf("Expected default block size 1024, got %d", bra.blockSize)
	}

	bra = NewBytesRefArray(512)
	if bra.blockSize != 512 {
		t.Errorf("Expected block size 512, got %d", bra.blockSize)
	}
}

func TestBytesRefArray_Append(t *testing.T) {
	bra := NewBytesRefArray(1024)

	// Append some data
	data1 := NewBytesRef([]byte("hello"))
	data2 := NewBytesRef([]byte("world"))
	data3 := NewBytesRef([]byte("test"))

	idx1 := bra.Append(data1)
	idx2 := bra.Append(data2)
	idx3 := bra.Append(data3)

	if idx1 != 0 || idx2 != 1 || idx3 != 2 {
		t.Errorf("Expected indices 0, 1, 2, got %d, %d, %d", idx1, idx2, idx3)
	}

	if bra.Size() != 3 {
		t.Errorf("Expected size 3, got %d", bra.Size())
	}
}

func TestBytesRefArray_AppendBytes(t *testing.T) {
	bra := NewBytesRefArray(1024)

	idx := bra.AppendBytes([]byte("direct bytes"))
	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}

	if bra.Size() != 1 {
		t.Errorf("Expected size 1, got %d", bra.Size())
	}
}

func TestBytesRefArray_Get(t *testing.T) {
	bra := NewBytesRefArray(1024)

	// Append some data
	bra.Append(NewBytesRef([]byte("first")))
	bra.Append(NewBytesRef([]byte("second")))
	bra.Append(NewBytesRef([]byte("third")))

	var spare BytesRef

	// Get first element
	if !bra.Get(0, &spare) {
		t.Error("Failed to get element 0")
	}
	if string(spare.ValidBytes()) != "first" {
		t.Errorf("Expected 'first', got '%s'", string(spare.ValidBytes()))
	}

	// Get second element
	if !bra.Get(1, &spare) {
		t.Error("Failed to get element 1")
	}
	if string(spare.ValidBytes()) != "second" {
		t.Errorf("Expected 'second', got '%s'", string(spare.ValidBytes()))
	}

	// Get out of bounds
	if bra.Get(10, &spare) {
		t.Error("Should return false for out of bounds index")
	}

	// Get negative index
	if bra.Get(-1, &spare) {
		t.Error("Should return false for negative index")
	}
}

func TestBytesRefArray_GetBytes(t *testing.T) {
	bra := NewBytesRefArray(1024)

	bra.AppendBytes([]byte("test data"))

	result := bra.GetBytes(0)
	if !bytes.Equal(result, []byte("test data")) {
		t.Errorf("Expected 'test data', got '%s'", string(result))
	}

	// Out of bounds
	result = bra.GetBytes(10)
	if result != nil {
		t.Error("Expected nil for out of bounds")
	}
}

func TestBytesRefArray_EmptyEntry(t *testing.T) {
	bra := NewBytesRefArray(1024)

	// Append empty data
	idx := bra.Append(nil)
	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}

	var spare BytesRef
	if !bra.Get(0, &spare) {
		t.Error("Failed to get empty entry")
	}
	if spare.Length != 0 {
		t.Errorf("Expected empty entry, got length %d", spare.Length)
	}
}

func TestBytesRefArray_Clear(t *testing.T) {
	bra := NewBytesRefArray(1024)

	bra.Append(NewBytesRef([]byte("data")))
	if bra.Size() != 1 {
		t.Fatal("Expected size 1 before clear")
	}

	bra.Clear()

	if bra.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", bra.Size())
	}
	if len(bra.blocks) != 0 {
		t.Error("Expected blocks to be cleared")
	}
	if len(bra.offsets) != 0 {
		t.Error("Expected offsets to be cleared")
	}
}

func TestBytesRefArray_BytesUsed(t *testing.T) {
	bra := NewBytesRefArray(1024)

	initialUsed := bra.BytesUsed()
	if initialUsed < 0 {
		t.Error("BytesUsed should be non-negative")
	}

	bra.Append(NewBytesRef([]byte("some data")))

	afterUsed := bra.BytesUsed()
	if afterUsed <= initialUsed {
		t.Error("BytesUsed should increase after append")
	}
}

func TestBytesRefArray_Iterator(t *testing.T) {
	bra := NewBytesRefArray(1024)

	data := [][]byte{
		[]byte("one"),
		[]byte("two"),
		[]byte("three"),
	}

	for _, d := range data {
		bra.Append(NewBytesRef(d))
	}

	iter := bra.Iterator()

	count := 0
	for iter.HasNext() {
		val, ok := iter.Next()
		if !ok {
			t.Fatal("Next returned false unexpectedly")
		}
		if !bytes.Equal(val.ValidBytes(), data[count]) {
			t.Errorf("Expected '%s', got '%s'", string(data[count]), string(val.ValidBytes()))
		}
		count++
	}

	if count != 3 {
		t.Errorf("Expected 3 iterations, got %d", count)
	}
}

func TestBytesRefArray_IteratorReset(t *testing.T) {
	bra := NewBytesRefArray(1024)
	bra.Append(NewBytesRef([]byte("data")))

	iter := bra.Iterator()

	// First iteration
	iter.Next()

	// Reset and iterate again
	iter.Reset()

	if !iter.HasNext() {
		t.Error("Should have next after reset")
	}
}

func TestBytesRefArray_Sort(t *testing.T) {
	bra := NewBytesRefArray(1024)

	// Append in unsorted order
	bra.Append(NewBytesRef([]byte("charlie")))
	bra.Append(NewBytesRef([]byte("alpha")))
	bra.Append(NewBytesRef([]byte("bravo")))

	// Sort lexicographically
	sortState := bra.SortByBytes()

	var spare BytesRef
	expected := []string{"alpha", "bravo", "charlie"}
	idx := 0

	for sortState.Next(&spare) {
		if idx >= len(expected) {
			t.Fatal("More elements than expected")
		}
		if string(spare.ValidBytes()) != expected[idx] {
			t.Errorf("Expected '%s', got '%s'", expected[idx], string(spare.ValidBytes()))
		}
		idx++
	}

	if idx != len(expected) {
		t.Errorf("Expected %d elements, got %d", len(expected), idx)
	}
}

func TestBytesRefArray_SortCustom(t *testing.T) {
	bra := NewBytesRefArray(1024)

	// Append data
	bra.Append(NewBytesRef([]byte("short")))
	bra.Append(NewBytesRef([]byte("a very long string here")))
	bra.Append(NewBytesRef([]byte("medium")))

	// Sort by length
	sortState := bra.Sort(func(a, b *BytesRef) bool {
		return a.Length < b.Length
	})

	var spare BytesRef
	expected := []string{"short", "medium", "a very long string here"}
	idx := 0

	for sortState.Next(&spare) {
		if idx >= len(expected) {
			t.Fatal("More elements than expected")
		}
		if string(spare.ValidBytes()) != expected[idx] {
			t.Errorf("Expected '%s', got '%s'", expected[idx], string(spare.ValidBytes()))
		}
		idx++
	}
}

func TestBytesRefArray_SortEmpty(t *testing.T) {
	bra := NewBytesRefArray(1024)

	sortState := bra.SortByBytes()

	var spare BytesRef
	if sortState.Next(&spare) {
		t.Error("Should not have any elements in empty sort")
	}

	if sortState.Size() != 0 {
		t.Errorf("Expected size 0, got %d", sortState.Size())
	}
}

func TestBytesRefArray_SortStateReset(t *testing.T) {
	bra := NewBytesRefArray(1024)
	bra.Append(NewBytesRef([]byte("data")))

	sortState := bra.SortByBytes()

	// First iteration
	var spare BytesRef
	sortState.Next(&spare)

	// Reset
	sortState.Reset()

	// Should be able to iterate again
	if !sortState.Next(&spare) {
		t.Error("Should have element after reset")
	}
}

func TestBytesRefArray_LargeData(t *testing.T) {
	bra := NewBytesRefArray(64) // Small block size to test block allocation

	// Append data larger than block size
	largeData := make([]byte, 100)
	for i := range largeData {
		largeData[i] = byte('a' + (i % 26))
	}

	idx := bra.Append(NewBytesRef(largeData))
	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}

	// Verify we can retrieve it
	result := bra.GetBytes(0)
	if !bytes.Equal(result, largeData) {
		t.Error("Large data retrieval failed")
	}
}

func TestBytesRefArray_MultipleBlocks(t *testing.T) {
	bra := NewBytesRefArray(32) // Small block size

	// Append multiple items to trigger block allocation
	for i := 0; i < 10; i++ {
		data := []byte("item " + string(rune('0'+i)))
		bra.Append(NewBytesRef(data))
	}

	if len(bra.blocks) < 2 {
		t.Error("Expected multiple blocks to be allocated")
	}

	// Verify all items can be retrieved
	for i := 0; i < 10; i++ {
		result := bra.GetBytes(i)
		expected := "item " + string(rune('0'+i))
		if string(result) != expected {
			t.Errorf("Item %d: expected '%s', got '%s'", i, expected, string(result))
		}
	}
}

func TestBytesRefArray_Independence(t *testing.T) {
	bra := NewBytesRefArray(1024)

	original := NewBytesRef([]byte("original"))
	bra.Append(original)

	// Modify original
	original.Bytes[0] = 'X'

	// Retrieved data should be independent
	result := bra.GetBytes(0)
	if string(result) != "original" {
		t.Error("BytesRefArray should store independent copies")
	}
}
