// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// SegmentInfosFormat handles encoding/decoding of segment metadata.
type SegmentInfosFormat interface {
	// Name returns the name of this format.
	Name() string
}

// BaseSegmentInfosFormat provides common functionality.
type BaseSegmentInfosFormat struct {
	name string
}

// NewBaseSegmentInfosFormat creates a new BaseSegmentInfosFormat.
func NewBaseSegmentInfosFormat(name string) *BaseSegmentInfosFormat {
	return &BaseSegmentInfosFormat{name: name}
}

// Name returns the format name.
func (f *BaseSegmentInfosFormat) Name() string {
	return f.name
}
