// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"strings"
	"testing"
)

func TestNewCharFilter(t *testing.T) {
	input := strings.NewReader("hello")
	cf := NewCharFilter(input)

	if cf == nil {
		t.Fatal("Expected non-nil CharFilter")
	}
	if cf.GetCumulativeDelta() != 0 {
		t.Error("Expected initial delta to be 0")
	}
}

func TestCharFilterRead(t *testing.T) {
	input := strings.NewReader("hello world")
	cf := NewCharFilter(input)

	buf := make([]byte, 5)
	n, err := cf.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("Expected to read 5 bytes, got %d", n)
	}
	if string(buf) != "hello" {
		t.Errorf("Expected 'hello', got '%s'", string(buf))
	}
}

func TestCharFilterCorrectOffset(t *testing.T) {
	input := strings.NewReader("hello")
	cf := NewCharFilter(input)

	// Initially, offset should be unchanged
	if cf.CorrectOffset(10) != 10 {
		t.Error("Expected offset to be unchanged when delta is 0")
	}

	// Add delta
	cf.AddOffsetDelta(5)
	if cf.CorrectOffset(10) != 15 {
		t.Errorf("Expected offset 15, got %d", cf.CorrectOffset(10))
	}
}

func TestCharFilterAddOffsetDelta(t *testing.T) {
	cf := NewCharFilter(nil)

	cf.AddOffsetDelta(5)
	if cf.GetCumulativeDelta() != 5 {
		t.Errorf("Expected delta 5, got %d", cf.GetCumulativeDelta())
	}

	cf.AddOffsetDelta(-2)
	if cf.GetCumulativeDelta() != 3 {
		t.Errorf("Expected delta 3, got %d", cf.GetCumulativeDelta())
	}
}

func TestCharFilterSetCumulativeDelta(t *testing.T) {
	cf := NewCharFilter(nil)

	cf.SetCumulativeDelta(10)
	if cf.GetCumulativeDelta() != 10 {
		t.Errorf("Expected delta 10, got %d", cf.GetCumulativeDelta())
	}
}

func TestCharFilterClose(t *testing.T) {
	// Test with a reader that supports Close
	input := &testReadCloser{Reader: strings.NewReader("test")}
	cf := NewCharFilter(input)

	err := cf.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !input.closed {
		t.Error("Expected reader to be closed")
	}
}

func TestCharFilterCloseNoCloser(t *testing.T) {
	// Test with a reader that doesn't support Close
	input := strings.NewReader("test")
	cf := NewCharFilter(input)

	err := cf.Close()
	if err != nil {
		t.Errorf("Expected no error for non-closer reader, got %v", err)
	}
}

// testReadCloser is a test helper that implements io.ReadCloser
type testReadCloser struct {
	io.Reader
	closed bool
}

func (trc *testReadCloser) Close() error {
	trc.closed = true
	return nil
}

func TestBaseCharFilterFactory(t *testing.T) {
	factory := NewBaseCharFilterFactory("testFactory")

	if factory.GetName() != "testFactory" {
		t.Errorf("Expected name 'testFactory', got '%s'", factory.GetName())
	}

	input := strings.NewReader("hello")
	cf := factory.Create(input)

	if cf == nil {
		t.Error("Expected non-nil CharFilter from factory")
	}
}

func TestCharFilterFactoryInterface(t *testing.T) {
	var factory CharFilterFactory = NewBaseCharFilterFactory("interfaceTest")

	input := strings.NewReader("test")
	cf := factory.Create(input)

	if cf == nil {
		t.Error("Factory should create CharFilter")
	}
}
