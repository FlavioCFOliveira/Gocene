// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Codec abstracts index format encoding/decoding.
type Codec interface {
	// Name returns the name of this codec.
	Name() string

	// PostingsFormat returns the postings format.
	PostingsFormat() PostingsFormat

	// StoredFieldsFormat returns the stored fields format.
	StoredFieldsFormat() StoredFieldsFormat

	// FieldInfosFormat returns the field infos format.
	FieldInfosFormat() FieldInfosFormat

	// SegmentInfosFormat returns the segment infos format.
	SegmentInfosFormat() SegmentInfosFormat
}

// BaseCodec provides common functionality.
type BaseCodec struct {
	name string
}

// NewBaseCodec creates a new BaseCodec.
func NewBaseCodec(name string) *BaseCodec {
	return &BaseCodec{name: name}
}

// Name returns the codec name.
func (c *BaseCodec) Name() string {
	return c.name
}

// PostingsFormat returns the postings format.
func (c *BaseCodec) PostingsFormat() PostingsFormat {
	return nil
}

// StoredFieldsFormat returns the stored fields format.
func (c *BaseCodec) StoredFieldsFormat() StoredFieldsFormat {
	return nil
}

// FieldInfosFormat returns the field infos format.
func (c *BaseCodec) FieldInfosFormat() FieldInfosFormat {
	return nil
}

// SegmentInfosFormat returns the segment infos format.
func (c *BaseCodec) SegmentInfosFormat() SegmentInfosFormat {
	return nil
}
