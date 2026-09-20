// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
	"strings"
	"testing"
)

// mockCharFilter is a simple implementation of CharFilter for testing.
type mockCharFilter struct {
	BaseCharFilter
	delta int
}

func (m *mockCharFilter) Correct(currentOff int) int {
	return currentOff + m.delta
}

func (m *mockCharFilter) CorrectOffset(currentOff int) int {
	return CorrectOffsetLogic(m, currentOff)
}

func TestNewCharFilter(t *testing.T) {
	input := strings.NewReader("hello")
	cf := NewCharFilter(input)

	if cf == nil {
		t.Fatal("Expected non-nil CharFilter")
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
	// Use mockCharFilter to provide a specific delta
	cf := &mockCharFilter{
		BaseCharFilter: BaseCharFilter{Input: input},
		delta:          5,
	}

	// 10 + 5 = 15
	if got := cf.CorrectOffset(10); got != 15 {
		t.Errorf("Expected offset 15, got %d", got)
	}
}

func TestCharFilterCorrectOffsetChaining(t *testing.T) {
	input := strings.NewReader("hello")

	// Chain: Inner (delta 10) -> Outer (delta 5)
	inner := &mockCharFilter{
		BaseCharFilter: BaseCharFilter{Input: input},
		delta:          10,
	}
	outer := &mockCharFilter{
		BaseCharFilter: BaseCharFilter{Input: inner},
		delta:          5,
	}

	// Outer.Correct(10) = 15
	// Inner.Correct(15) = 25
	// Result: 25
	if got := outer.CorrectOffset(10); got != 25 {
		t.Errorf("Expected chained offset 25, got %d", got)
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
