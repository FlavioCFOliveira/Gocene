// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Lucene104Codec is the default codec for Lucene 10.4.
// This codec uses:
//   - Lucene104PostingsFormat for postings (term -> document mappings)
//   - Lucene104StoredFieldsFormat for stored fields
//   - Lucene104FieldInfosFormat for field metadata
//   - Lucene104SegmentInfosFormat for segment metadata
//
// This is the Go port of Lucene's org.apache.lucene.codecs.lucene104.Lucene104Codec.
type Lucene104Codec struct {
	*BaseCodec
	postingsFormat     PostingsFormat
	storedFieldsFormat StoredFieldsFormat
	fieldInfosFormat   FieldInfosFormat
	segmentInfosFormat SegmentInfosFormat
}

// NewLucene104Codec creates a new Lucene104Codec.
func NewLucene104Codec() *Lucene104Codec {
	return &Lucene104Codec{
		BaseCodec:          NewBaseCodec("Lucene104"),
		postingsFormat:     NewLucene104PostingsFormat(),
		storedFieldsFormat: NewLucene104StoredFieldsFormat(),
		fieldInfosFormat:   NewLucene104FieldInfosFormat(),
		segmentInfosFormat: NewLucene104SegmentInfosFormat(),
	}
}

// PostingsFormat returns the postings format.
func (c *Lucene104Codec) PostingsFormat() PostingsFormat {
	return c.postingsFormat
}

// StoredFieldsFormat returns the stored fields format.
func (c *Lucene104Codec) StoredFieldsFormat() StoredFieldsFormat {
	return c.storedFieldsFormat
}

// FieldInfosFormat returns the field infos format.
func (c *Lucene104Codec) FieldInfosFormat() FieldInfosFormat {
	return c.fieldInfosFormat
}

// SegmentInfosFormat returns the segment infos format.
func (c *Lucene104Codec) SegmentInfosFormat() SegmentInfosFormat {
	return c.segmentInfosFormat
}
