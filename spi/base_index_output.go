// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// BaseIndexOutput provides common functionality for IndexOutput implementations.
// Embed this struct in concrete IndexOutput implementations.
type BaseIndexOutput struct {
	// name is the name of the file being written
	name string

	// filePointer is the current position in the output
	filePointer int64
}

// NewBaseIndexOutput creates a new BaseIndexOutput.
func NewBaseIndexOutput(name string) *BaseIndexOutput {
	return &BaseIndexOutput{
		name:        name,
		filePointer: 0,
	}
}

// GetName returns the name of the file being written.
func (out *BaseIndexOutput) GetName() string {
	return out.name
}

// GetFilePointer returns the current position in the output.
func (out *BaseIndexOutput) GetFilePointer() int64 {
	return out.filePointer
}

// SetFilePointer sets the current position in the output.
// This should only be called by implementations.
func (out *BaseIndexOutput) SetFilePointer(pos int64) {
	out.filePointer = pos
}

// IncrementFilePointer increments the file pointer by n bytes.
func (out *BaseIndexOutput) IncrementFilePointer(n int64) {
	out.filePointer += n
}
