// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// StoredFieldsFormat handles encoding/decoding of stored fields.
type StoredFieldsFormat interface {
	// Name returns the name of this format.
	Name() string
}

// BaseStoredFieldsFormat provides common functionality.
type BaseStoredFieldsFormat struct {
	name string
}

// NewBaseStoredFieldsFormat creates a new BaseStoredFieldsFormat.
func NewBaseStoredFieldsFormat(name string) *BaseStoredFieldsFormat {
	return &BaseStoredFieldsFormat{name: name}
}

// Name returns the format name.
func (f *BaseStoredFieldsFormat) Name() string {
	return f.name
}
