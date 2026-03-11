// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// FieldInfosFormat handles encoding/decoding of field metadata.
type FieldInfosFormat interface {
	// Name returns the name of this format.
	Name() string
}

// BaseFieldInfosFormat provides common functionality.
type BaseFieldInfosFormat struct {
	name string
}

// NewBaseFieldInfosFormat creates a new BaseFieldInfosFormat.
func NewBaseFieldInfosFormat(name string) *BaseFieldInfosFormat {
	return &BaseFieldInfosFormat{name: name}
}

// Name returns the format name.
func (f *BaseFieldInfosFormat) Name() string {
	return f.name
}
