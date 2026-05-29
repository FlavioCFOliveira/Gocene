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
//   - Lucene104TermVectorsFormat for term vectors
//   - Lucene90DocValuesFormat for doc values (Lucene 10.x uses the same format as 9.x)
//   - Lucene99HnswVectorsFormat (via PerFieldKnnVectorsFormat) for KNN vectors
//
// This is the Go port of Lucene's org.apache.lucene.codecs.lucene104.Lucene104Codec.
type Lucene104Codec struct {
	*BaseCodec
	postingsFormat     PostingsFormat
	storedFieldsFormat StoredFieldsFormat
	fieldInfosFormat   FieldInfosFormat
	segmentInfosFormat SegmentInfosFormat
	segmentInfoFormat  SegmentInfoFormat
	termVectorsFormat  TermVectorsFormat
	docValuesFormat    DocValuesFormat
	compoundFormat     CompoundFormat
	knnVectorsFormat   KnnVectorsFormat // PerFieldKnnVectorsFormat wrapping Lucene99HnswVectorsFormat
	pointsFormat       PointsFormat     // Lucene90PointsFormat (BKD)
}

// NewLucene104Codec creates a new Lucene104Codec.
func NewLucene104Codec() *Lucene104Codec {
	defaultKnn, err := NewLucene99HnswVectorsFormat()
	if err != nil {
		// Default parameters are always valid; this path is unreachable in
		// production. If it fires, the binary is misconfigured at compile time.
		panic("lucene104: NewLucene99HnswVectorsFormat with default params: " + err.Error())
	}
	return &Lucene104Codec{
		BaseCodec:          NewBaseCodec("Lucene104"),
		postingsFormat:     NewLucene104PostingsFormat(),
		storedFieldsFormat: NewLucene104StoredFieldsFormat(),
		fieldInfosFormat:   NewLucene104FieldInfosFormat(),
		segmentInfosFormat: NewLucene104SegmentInfosFormat(),
		segmentInfoFormat:  NewLucene99SegmentInfoFormat(),
		termVectorsFormat:  NewLucene104TermVectorsFormat(),
		docValuesFormat:    NewLucene90DocValuesFormat(),
		compoundFormat:     NewLucene90CompoundFormat(),
		knnVectorsFormat:   NewPerFieldKnnVectorsFormatWithDefault(defaultKnn),
		pointsFormat:       NewLucene90PointsFormat(),
	}
}

// SegmentInfoFormat returns the per-segment .si format. Added as part
// of the SPI unification (rmp #4693) so Lucene104Codec satisfies the
// spi.Codec interface that requires the singular SegmentInfoFormat
// accessor. Returns NewLucene99SegmentInfoFormat(), matching the
// Lucene 9.9/10.4 .si wire format.
func (c *Lucene104Codec) SegmentInfoFormat() SegmentInfoFormat {
	return c.segmentInfoFormat
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

// TermVectorsFormat returns the term vectors format.
func (c *Lucene104Codec) TermVectorsFormat() TermVectorsFormat {
	return c.termVectorsFormat
}

// DocValuesFormat returns the doc values format.
func (c *Lucene104Codec) DocValuesFormat() DocValuesFormat {
	return c.docValuesFormat
}

// CompoundFormat returns the compound format.
func (c *Lucene104Codec) CompoundFormat() CompoundFormat {
	return c.compoundFormat
}

// KnnVectorsFormat returns the PerFieldKnnVectorsFormat used for KNN vector
// indexing. The default sub-format is Lucene99HnswVectorsFormat, mirroring
// org.apache.lucene.codecs.lucene104.Lucene104Codec.knnVectorsFormat().
func (c *Lucene104Codec) KnnVectorsFormat() KnnVectorsFormat {
	return c.knnVectorsFormat
}

// PointsFormat returns the Lucene90PointsFormat used for multi-dimensional
// point (BKD) indexing, mirroring
// org.apache.lucene.codecs.lucene104.Lucene104Codec.pointsFormat() which
// returns Lucene90PointsFormat in Lucene 10.4.0.
func (c *Lucene104Codec) PointsFormat() PointsFormat {
	return c.pointsFormat
}

// NewLucene99Codec creates a codec that is functionally identical to Lucene104Codec.
// It is provided for tests that reference the Lucene 9.9 codec name.
//
// Gocene maps all legacy codec names to the latest implementation.
func NewLucene99Codec() *Lucene104Codec {
	return NewLucene104Codec()
}

// NewLucene90Codec creates a codec that is functionally identical to Lucene104Codec.
// It is provided for tests that reference the Lucene 9.0 codec name.
//
// Gocene maps all legacy codec names to the latest implementation.
func NewLucene90Codec() *Lucene104Codec {
	return NewLucene104Codec()
}
