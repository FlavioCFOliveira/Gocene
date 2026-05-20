// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/analysis/FakeTokenFilterFactory.java
//
// FakeTokenFilterFactory is a test helper with no @Test methods. This file
// defines the helper struct and verifies its contract with simple tests.

package analysis

import (
	"testing"
)

// FakeTokenFilterFactoryName is the SPI name for fakeTokenFilterFactory.
//
// Mirrors FakeTokenFilterFactory.NAME (Lucene 10.4.0).
const FakeTokenFilterFactoryName = "fake"

// passthroughTokenFilter is a TokenFilter that delegates every call to
// its input without modification. Used by fakeTokenFilterFactory.
type passthroughTokenFilter struct {
	*BaseTokenFilter
}

// newPassthroughTokenFilter wraps input in a no-op TokenFilter.
func newPassthroughTokenFilter(input TokenStream) *passthroughTokenFilter {
	return &passthroughTokenFilter{BaseTokenFilter: NewBaseTokenFilter(input)}
}

// IncrementToken delegates to the wrapped input.
func (f *passthroughTokenFilter) IncrementToken() (bool, error) {
	return f.GetInput().IncrementToken()
}

// fakeTokenFilterFactory is a pass-through TokenFilterFactory used as a test
// double. Its Create method wraps input in a no-op TokenFilter.
//
// Mirrors org.apache.lucene.analysis.FakeTokenFilterFactory (Lucene 10.4.0).
type fakeTokenFilterFactory struct {
	AbstractAnalysisFactory
}

// newFakeTokenFilterFactory constructs a fakeTokenFilterFactory from an args
// map. Reserved keys ("luceneMatchVersion", "class", "name") are consumed by
// AbstractAnalysisFactory; remaining entries are tolerated.
func newFakeTokenFilterFactory(args map[string]string) (*fakeTokenFilterFactory, error) {
	base, err := NewAbstractAnalysisFactory(args)
	if err != nil {
		return nil, err
	}
	return &fakeTokenFilterFactory{AbstractAnalysisFactory: *base}, nil
}

// Create returns a pass-through TokenFilter wrapping input unchanged.
//
// Mirrors FakeTokenFilterFactory.create(TokenStream) (Lucene 10.4.0).
func (f *fakeTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return newPassthroughTokenFilter(input)
}

// ─── Tests ───────────────────────────────────────────────────────────────────

// TestFakeTokenFilterFactory_Name verifies the SPI name constant.
func TestFakeTokenFilterFactory_Name(t *testing.T) {
	if FakeTokenFilterFactoryName != "fake" {
		t.Fatalf("expected name 'fake', got %q", FakeTokenFilterFactoryName)
	}
}

// TestFakeTokenFilterFactory_Create verifies that Create returns a non-nil
// TokenFilter.
func TestFakeTokenFilterFactory_Create(t *testing.T) {
	f, err := newFakeTokenFilterFactory(map[string]string{})
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}
	base := &BaseTokenStream{attributes: nil}
	_ = base
	// Use a no-op token stream as the input.
	type noopStream struct{ BaseTokenStream }
	src := &noopStream{}
	src.attributes = src.GetAttributeSource()

	tf := f.Create(src)
	if tf == nil {
		t.Fatal("Create returned nil")
	}
}

// TestFakeTokenFilterFactory_WithArgs verifies that the args-based constructor
// accepts valid reserved keys without error.
func TestFakeTokenFilterFactory_WithArgs(t *testing.T) {
	_, err := newFakeTokenFilterFactory(map[string]string{"luceneMatchVersion": "10.4.0"})
	if err != nil {
		t.Fatalf("unexpected error with luceneMatchVersion arg: %v", err)
	}
}

// TestFakeTokenFilterFactory_ImplementsInterface verifies that the factory
// satisfies the TokenFilterFactory interface at compile time.
func TestFakeTokenFilterFactory_ImplementsInterface(t *testing.T) {
	f, err := newFakeTokenFilterFactory(map[string]string{})
	if err != nil {
		t.Fatalf("constructor error: %v", err)
	}
	var _ TokenFilterFactory = f
}
