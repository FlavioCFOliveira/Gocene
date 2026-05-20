// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/FakeCharFilterFactory.java
//
// FakeCharFilterFactory is a test helper with no @Test methods. This file
// defines the helper struct and verifies its contract with simple tests.

package analysis

import (
	"io"
	"strings"
	"testing"
)

// FakeCharFilterFactoryName is the SPI name for FakeCharFilterFactory.
//
// Mirrors FakeCharFilterFactory.NAME (Lucene 10.4.0).
const FakeCharFilterFactoryName = "fake"

// fakeCharFilterFactory is a pass-through CharFilterFactory used as a test
// double. Its Create method wraps the input reader in a CharFilter without
// altering the character stream.
//
// Mirrors org.apache.lucene.analysis.FakeCharFilterFactory (Lucene 10.4.0).
type fakeCharFilterFactory struct {
	AbstractAnalysisFactory
}

// newFakeCharFilterFactory constructs a fakeCharFilterFactory from an args map.
// Unknown parameters are tolerated (mirroring the Java super(args) call which
// consumes reserved keys but does not validate remaining ones).
func newFakeCharFilterFactory(args map[string]string) (*fakeCharFilterFactory, error) {
	base, err := NewAbstractAnalysisFactory(args)
	if err != nil {
		return nil, err
	}
	return &fakeCharFilterFactory{AbstractAnalysisFactory: *base}, nil
}

// Create returns a pass-through CharFilter wrapping input unchanged.
//
// Mirrors FakeCharFilterFactory.create(Reader) (Lucene 10.4.0).
func (f *fakeCharFilterFactory) Create(input io.Reader) *CharFilter {
	return NewCharFilter(input)
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestFakeCharFilterFactory_Name verifies the SPI name constant.
func TestFakeCharFilterFactory_Name(t *testing.T) {
	if FakeCharFilterFactoryName != "fake" {
		t.Fatalf("expected name 'fake', got %q", FakeCharFilterFactoryName)
	}
}

// TestFakeCharFilterFactory_Create verifies that Create returns a non-nil
// CharFilter that passes bytes through unchanged.
func TestFakeCharFilterFactory_Create(t *testing.T) {
	f, err := newFakeCharFilterFactory(map[string]string{})
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}

	src := strings.NewReader("hello")
	cf := f.Create(src)
	if cf == nil {
		t.Fatal("Create returned nil")
	}

	buf := make([]byte, 5)
	n, err := cf.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Read: %v", err)
	}
	if string(buf[:n]) != "hello" {
		t.Errorf("expected pass-through, got %q", string(buf[:n]))
	}
}

// TestFakeCharFilterFactory_CorrectOffset verifies that the pass-through
// filter introduces no offset delta (CorrectOffset is identity).
func TestFakeCharFilterFactory_CorrectOffset(t *testing.T) {
	f, _ := newFakeCharFilterFactory(map[string]string{})
	cf := f.Create(strings.NewReader("abc"))
	for _, off := range []int{0, 1, 5, 100} {
		if got := cf.CorrectOffset(off); got != off {
			t.Errorf("CorrectOffset(%d) = %d, want %d", off, got, off)
		}
	}
}

// TestFakeCharFilterFactory_WithArgs verifies that arbitrary args do not
// cause a constructor error (the factory delegates to AbstractAnalysisFactory
// which only removes reserved keys).
func TestFakeCharFilterFactory_WithArgs(t *testing.T) {
	_, err := newFakeCharFilterFactory(map[string]string{"luceneMatchVersion": "10.4.0"})
	if err != nil {
		t.Fatalf("unexpected error with luceneMatchVersion arg: %v", err)
	}
}
