// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// PostingsFormat handles encoding/decoding of postings.
type PostingsFormat interface {
	// Name returns the name of this format.
	Name() string
}

// BasePostingsFormat provides common functionality.
type BasePostingsFormat struct {
	name string
}

// NewBasePostingsFormat creates a new BasePostingsFormat.
func NewBasePostingsFormat(name string) *BasePostingsFormat {
	return &BasePostingsFormat{name: name}
}

// Name returns the format name.
func (f *BasePostingsFormat) Name() string {
	return f.name
}
