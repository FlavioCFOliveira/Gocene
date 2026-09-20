// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"io"
)

// CharFilter is the interface for character filters.
// Subclasses of CharFilter can be chained to filter a Reader.
// They can be used as io.Reader with additional offset correction.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.CharFilter.
type CharFilter interface {
	io.Reader
	io.Closer
	// Correct adjusts the current offset.
	Correct(currentOff int) int
	// CorrectOffset chains the corrected offset through the input CharFilter(s).
	CorrectOffset(currentOff int) int
	// GetInput returns the underlying input stream.
	GetInput() io.Reader
}

// BaseCharFilter provides a basic implementation of the non-abstract
// parts of the Lucene CharFilter class, specifically the Reader and Closer.
type BaseCharFilter struct {
	Input io.Reader
}

// Read implements the io.Reader interface.
func (b *BaseCharFilter) Read(p []byte) (n int, err error) {
	return b.Input.Read(p)
}

// Close closes the underlying input stream.
func (b *BaseCharFilter) Close() error {
	if closer, ok := b.Input.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// GetInput returns the underlying input stream.
func (b *BaseCharFilter) GetInput() io.Reader {
	return b.Input
}

// CorrectOffsetLogic implements the recursive offset correction logic from Lucene.
// In Java, this is a final method in the abstract class. In Go, concrete
// CharFilter implementations should call this helper within their CorrectOffset method.
func CorrectOffsetLogic(cf CharFilter, currentOff int) int {
	corrected := cf.Correct(currentOff)
	if inputCF, ok := cf.GetInput().(CharFilter); ok {
		return inputCF.CorrectOffset(corrected)
	}
	return corrected
}

type defaultCharFilter struct {
	BaseCharFilter
}

func (d *defaultCharFilter) Correct(currentOff int) int {
	return currentOff
}

func (d *defaultCharFilter) CorrectOffset(currentOff int) int {
	return CorrectOffsetLogic(d, currentOff)
}

// NewCharFilter creates a new CharFilter wrapping the given reader.
// This returns a default implementation that performs no correction.
func NewCharFilter(input io.Reader) CharFilter {
	return &defaultCharFilter{
		BaseCharFilter: BaseCharFilter{Input: input},
	}
}
