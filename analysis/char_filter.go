// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// CharFilter is the base class for character filters.
// CharFilters are used to preprocess text before tokenization.
// They can add, remove, or modify characters in the input stream.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.CharFilter.
type CharFilter struct {
	// input is the underlying reader
	input io.Reader

	// cumulativeDelta tracks the offset difference between input and output
	cumulativeDelta int
}

// NewCharFilter creates a new CharFilter wrapping the given reader.
func NewCharFilter(input io.Reader) *CharFilter {
	return &CharFilter{
		input:           input,
		cumulativeDelta: 0,
	}
}

// Read reads characters into the provided buffer.
// This implements the io.Reader interface.
func (cf *CharFilter) Read(p []byte) (n int, err error) {
	return cf.input.Read(p)
}

// CorrectOffset adjusts the offset to account for character filtering.
// This is called to map positions in the filtered text back to positions
// in the original text.
func (cf *CharFilter) CorrectOffset(currentOff int) int {
	return currentOff + cf.cumulativeDelta
}

// AddOffsetDelta adds to the cumulative offset delta.
// This should be called when characters are added or removed.
func (cf *CharFilter) AddOffsetDelta(delta int) {
	cf.cumulativeDelta += delta
}

// GetCumulativeDelta returns the current cumulative offset delta.
func (cf *CharFilter) GetCumulativeDelta() int {
	return cf.cumulativeDelta
}

// SetCumulativeDelta sets the cumulative offset delta.
func (cf *CharFilter) SetCumulativeDelta(delta int) {
	cf.cumulativeDelta = delta
}

// Close closes the underlying reader.
func (cf *CharFilter) Close() error {
	if closer, ok := cf.input.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// CharFilterFactory creates CharFilter instances.
type CharFilterFactory interface {
	// Create creates a new CharFilter wrapping the given reader.
	Create(input io.Reader) *CharFilter
}

// BaseCharFilterFactory is a base implementation of CharFilterFactory.
type BaseCharFilterFactory struct {
	name string
}

// NewBaseCharFilterFactory creates a new BaseCharFilterFactory.
func NewBaseCharFilterFactory(name string) *BaseCharFilterFactory {
	return &BaseCharFilterFactory{name: name}
}

// Create creates a new CharFilter.
func (f *BaseCharFilterFactory) Create(input io.Reader) *CharFilter {
	return NewCharFilter(input)
}

// GetName returns the name of this factory.
func (f *BaseCharFilterFactory) GetName() string {
	return f.name
}
