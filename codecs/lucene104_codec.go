// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Lucene104Codec is the default codec for Lucene 10.4.
type Lucene104Codec struct {
	*BaseCodec
}

// NewLucene104Codec creates a new Lucene104Codec.
func NewLucene104Codec() *Lucene104Codec {
	return &Lucene104Codec{
		BaseCodec: NewBaseCodec("Lucene104"),
	}
}

// PostingsFormat returns the postings format.
func (c *Lucene104Codec) PostingsFormat() PostingsFormat {
	return NewBasePostingsFormat("Lucene104PostingsFormat")
}

// StoredFieldsFormat returns the stored fields format.
func (c *Lucene104Codec) StoredFieldsFormat() StoredFieldsFormat {
	return NewBaseStoredFieldsFormat("Lucene104StoredFieldsFormat")
}

// FieldInfosFormat returns the field infos format.
func (c *Lucene104Codec) FieldInfosFormat() FieldInfosFormat {
	return NewBaseFieldInfosFormat("Lucene104FieldInfosFormat")
}

// SegmentInfosFormat returns the segment infos format.
func (c *Lucene104Codec) SegmentInfosFormat() SegmentInfosFormat {
	return NewBaseSegmentInfosFormat("Lucene104SegmentInfosFormat")
}
