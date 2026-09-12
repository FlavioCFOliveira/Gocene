// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"errors"
	"fmt"
)

// BaseIndexInput provides common functionality for IndexInput implementations.
// Embed this struct in concrete IndexInput implementations.
type BaseIndexInput struct {
	// desc is a description of this IndexInput (for debugging/messages)
	desc string

	// length is the total length of the input
	length int64

	// filePointer is the current position in the input
	filePointer int64
}

// NewBaseIndexInput creates a new BaseIndexInput.
func NewBaseIndexInput(desc string, length int64) *BaseIndexInput {
	return &BaseIndexInput{
		desc:        desc,
		length:      length,
		filePointer: 0,
	}
}

// GetDescription returns the description of this IndexInput.
func (in *BaseIndexInput) GetDescription() string {
	return in.desc
}

// GetFilePointer returns the current position in the input.
func (in *BaseIndexInput) GetFilePointer() int64 {
	return in.filePointer
}

// SetFilePointer sets the current position in the input.
func (in *BaseIndexInput) SetFilePointer(pos int64) {
	in.filePointer = pos
}

// Length returns the total length of the input.
func (in *BaseIndexInput) Length() int64 {
	return in.length
}

// ValidateSeek checks if a seek position is valid.
func (in *BaseIndexInput) ValidateSeek(pos int64) error {
	if pos < 0 {
		return fmt.Errorf("seek position %d is negative", pos)
	}
	if pos > in.length {
		return fmt.Errorf("seek position %d exceeds length %d", pos, in.length)
	}
	return nil
}

// ValidateSlice checks if a slice specification is valid.
func (in *BaseIndexInput) ValidateSlice(offset int64, length int64) error {
	if offset < 0 {
		return fmt.Errorf("slice offset %d is negative", offset)
	}
	if length < 0 {
		return fmt.Errorf("slice length %d is negative", length)
	}
	if offset+length > in.length {
		return fmt.Errorf("slice offset %d + length %d exceeds input length %d", offset, length, in.length)
	}
	return nil
}

// SkipBytes skips n bytes forward in the input.
func (in *BaseIndexInput) SkipBytes(n int64) error {
	if n < 0 {
		return errors.New("cannot skip negative bytes")
	}
	newPos := in.filePointer + n
	if newPos > in.length {
		return fmt.Errorf("cannot skip %d bytes past end of file", n)
	}
	in.filePointer = newPos
	return nil
}
