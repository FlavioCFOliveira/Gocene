// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"testing"
)

func TestNewBytesRef(t *testing.T) {
	testData := []byte("Hello, World!")
	br := NewBytesRef(testData)

	if br == nil {
		t.Fatal("Expected non-nil BytesRef")
	}
	if br.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", br.Offset)
	}
	if br.Length != len(testData) {
		t.Errorf("Expected length %d, got %d", len(testData), br.Length)
	}
	if !bytes.Equal(br.Bytes, testData) {
		t.Error("Bytes should be a copy of input")
	}

	testData[0] = 'X'
	if br.Bytes[0] != 'H' {
		t.Error("BytesRef should be immutable")
	}
}

func TestNewBytesRefEmpty(t *testing.T) {
	br := NewBytesRefEmpty()
	if br == nil {
		t.Fatal("Expected non-nil BytesRef")
	}
	if br.Bytes != nil {
		t.Error("Expected nil bytes for empty BytesRef")
	}
	if br.Offset != 0 {
		t.Errorf("Expected offset 0, got %d", br.Offset)
	}
	if br.Length != 0 {
		t.Errorf("Expected length 0, got %d", br.Length)
	}
}

func TestNewBytesRefWithCapacity(t *testing.T) {
	br := NewBytesRefWithCapacity(100)
	if br == nil {
		t.Fatal("Expected non-nil BytesRef")
	}
	if cap(br.Bytes) < 100 {
		t.Errorf("Expected capacity at least 100, got %d", cap(br.Bytes))
	}
	if br.Length != 0 {
		t.Errorf("Expected length 0, got %d", br.Length)
	}
}

func TestBytesRef_String(t *testing.T) {
	testData := []byte("test string")
	br := NewBytesRef(testData)
	if br.String() != "test string" {
		t.Errorf("Expected 'test string', got %s", br.String())
	}

	br2 := NewBytesRefEmpty()
	if br2.String() != "" {
		t.Errorf("Expected empty string for nil, got %s", br2.String())
	}
}

func TestBytesRef_IsValid(t *testing.T) {
	br := NewBytesRef([]byte("test"))
	if !br.IsValid() {
		t.Error("Expected valid BytesRef")
	}

	br2 := NewBytesRefEmpty()
	if !br2.IsValid() {
		t.Error("Expected empty BytesRef to be valid")
	}
}

func TestBytesRef_ValidBytes(t *testing.T) {
	testData := []byte("Hello, World!")
	br := NewBytesRef(testData)

	valid := br.ValidBytes()
	if !bytes.Equal(valid, testData) {
		t.Errorf("Expected %v, got %v", testData, valid)
	}

	br.Offset = 7
	br.Length = 5
	valid = br.ValidBytes()
	expected := []byte("World")
	if !bytes.Equal(valid, expected) {
		t.Errorf("Expected %v, got %v", expected, valid)
	}

	br2 := NewBytesRefEmpty()
	if br2.ValidBytes() != nil {
		t.Error("Expected nil for empty BytesRef")
	}
}

func TestBytesRef_Append(t *testing.T) {
	br := NewBytesRef([]byte("Hello"))
	br.Append([]byte(", World!"))

	if br.String() != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got %s", br.String())
	}

	br2 := NewBytesRefEmpty()
	br2.Append([]byte("data"))
	if br2.String() != "data" {
		t.Errorf("Expected 'data', got %s", br2.String())
	}

	br3 := NewBytesRef([]byte("test"))
	br3.Append([]byte{})
	if br3.String() != "test" {
		t.Errorf("Expected 'test', got %s", br3.String())
	}
}

func TestBytesRef_AppendBytesRef(t *testing.T) {
	br1 := NewBytesRef([]byte("Hello"))
	br2 := NewBytesRef([]byte(" World"))

	br1.AppendBytesRef(br2)
	if br1.String() != "Hello World" {
		t.Errorf("Expected 'Hello World', got %s", br1.String())
	}

	br1.AppendBytesRef(nil)
	if br1.String() != "Hello World" {
		t.Errorf("Expected 'Hello World' after nil append, got %s", br1.String())
	}
}

func TestBytesRef_Copy(t *testing.T) {
	br1 := NewBytesRef([]byte("source"))
	br2 := NewBytesRefEmpty()

	br2.Copy(br1)
	if br2.String() != "source" {
		t.Errorf("Expected 'source', got %s", br2.String())
	}

	br1.Bytes[0] = 'X'
	if br2.Bytes[0] != 's' {
		t.Error("Copy should be independent")
	}

	br3 := NewBytesRef([]byte("data"))
	br3.Copy(nil)
	if br3.String() != "" {
		t.Errorf("Expected empty after nil copy, got %s", br3.String())
	}
}

func TestBytesRef_Grow(t *testing.T) {
	br := NewBytesRef([]byte("test"))
	br.Grow(100)

	if cap(br.Bytes) < 100 {
		t.Errorf("Expected capacity at least 100, got %d", cap(br.Bytes))
	}

	if br.String() != "test" {
		t.Errorf("Expected 'test' after grow, got %s", br.String())
	}
}

func TestBytesEquals(t *testing.T) {
	a := []byte("hello")
	b := []byte("hello")
	c := []byte("world")

	if !BytesEquals(a, b) {
		t.Error("Expected equal bytes")
	}
	if BytesEquals(a, c) {
		t.Error("Expected unequal bytes")
	}
	if BytesEquals(a, nil) {
		t.Error("Expected unequal with nil")
	}
}

func TestBytesRefEquals(t *testing.T) {
	br1 := NewBytesRef([]byte("test"))
	br2 := NewBytesRef([]byte("test"))
	br3 := NewBytesRef([]byte("other"))

	if !BytesRefEquals(br1, br2) {
		t.Error("Expected equal BytesRef")
	}
	if BytesRefEquals(br1, br3) {
		t.Error("Expected unequal BytesRef")
	}
	if !BytesRefEquals(br1, br1) {
		t.Error("Expected equal with itself")
	}
	if BytesRefEquals(br1, nil) {
		t.Error("Expected unequal with nil")
	}
	if !BytesRefEquals(nil, nil) {
		t.Error("Expected nil == nil")
	}
}

func TestBytesRefCompare(t *testing.T) {
	br1 := NewBytesRef([]byte("aaa"))
	br2 := NewBytesRef([]byte("bbb"))
	br3 := NewBytesRef([]byte("aaa"))

	if BytesRefCompare(br1, br2) >= 0 {
		t.Error("Expected aaa < bbb")
	}
	if BytesRefCompare(br2, br1) <= 0 {
		t.Error("Expected bbb > aaa")
	}
	if BytesRefCompare(br1, br3) != 0 {
		t.Error("Expected equal comparison")
	}
	if BytesRefCompare(nil, br1) >= 0 {
		t.Error("Expected nil < anything")
	}
}

func TestBytesRef_BytesRefCompareTo(t *testing.T) {
	br1 := NewBytesRef([]byte("abc"))
	br2 := NewBytesRef([]byte("def"))

	if br1.BytesRefCompareTo(br2) >= 0 {
		t.Error("Expected abc < def")
	}
}

func TestBytesRef_Clone(t *testing.T) {
	br1 := NewBytesRef([]byte("original"))
	br2 := br1.Clone()

	if br2.String() != "original" {
		t.Errorf("Expected 'original', got %s", br2.String())
	}

	br1.Bytes[0] = 'X'
	if br2.Bytes[0] != 'o' {
		t.Error("Clone should be independent")
	}
}

func TestBytesRef_DeepCopyEquals(t *testing.T) {
	br1 := NewBytesRef([]byte("test"))
	br2 := NewBytesRef([]byte("test"))

	if !br1.DeepCopyEquals(br2) {
		t.Error("Expected deep copy equals")
	}
}

func TestBytesRef_HashCode(t *testing.T) {
	br1 := NewBytesRef([]byte("test"))
	h1 := br1.HashCode()

	br2 := NewBytesRef([]byte("test"))
	h2 := br2.HashCode()

	if h1 != h2 {
		t.Error("Expected same hash code for equal content")
	}

	br3 := NewBytesRef([]byte("different"))
	h3 := br3.HashCode()
	if h1 == h3 {
		t.Error("Expected different hash codes for different content")
	}

	br4 := NewBytesRefEmpty()
	if br4.HashCode() != 0 {
		t.Error("Expected 0 hash for empty")
	}
}

func TestNewIntsRef(t *testing.T) {
	testData := []int{1, 2, 3, 4, 5}
	ir := NewIntsRef(testData)

	if ir == nil {
		t.Fatal("Expected non-nil IntsRef")
	}
	if ir.Length != len(testData) {
		t.Errorf("Expected length %d, got %d", len(testData), ir.Length)
	}

	testData[0] = 999
	if ir.Ints[0] != 1 {
		t.Error("IntsRef should be immutable")
	}
}

func TestNewIntsRefEmpty(t *testing.T) {
	ir := NewIntsRefEmpty()
	if ir == nil {
		t.Fatal("Expected non-nil IntsRef")
	}
	if ir.Ints != nil {
		t.Error("Expected nil ints for empty IntsRef")
	}
}

func TestIntsRef_ValidInts(t *testing.T) {
	testData := []int{1, 2, 3, 4, 5}
	ir := NewIntsRef(testData)

	valid := ir.ValidInts()
	if len(valid) != 5 {
		t.Errorf("Expected length 5, got %d", len(valid))
	}

	ir.Offset = 2
	ir.Length = 2
	valid = ir.ValidInts()
	if len(valid) != 2 {
		t.Errorf("Expected length 2, got %d", len(valid))
	}
	if valid[0] != 3 || valid[1] != 4 {
		t.Errorf("Expected [3, 4], got %v", valid)
	}
}

func TestIntsRefEquals(t *testing.T) {
	ir1 := NewIntsRef([]int{1, 2, 3})
	ir2 := NewIntsRef([]int{1, 2, 3})
	ir3 := NewIntsRef([]int{4, 5, 6})

	if !IntsRefEquals(ir1, ir2) {
		t.Error("Expected equal IntsRef")
	}
	if IntsRefEquals(ir1, ir3) {
		t.Error("Expected unequal IntsRef")
	}
}

func TestIntsRefCompare(t *testing.T) {
	ir1 := NewIntsRef([]int{1, 2, 3})
	ir2 := NewIntsRef([]int{4, 5, 6})
	ir3 := NewIntsRef([]int{1, 2, 3})

	if IntsRefCompare(ir1, ir2) >= 0 {
		t.Error("Expected ir1 < ir2")
	}
	if IntsRefCompare(ir2, ir1) <= 0 {
		t.Error("Expected ir2 > ir1")
	}
	if IntsRefCompare(ir1, ir3) != 0 {
		t.Error("Expected equal comparison")
	}
}
