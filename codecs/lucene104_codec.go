// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Lucene104CodecMode selects the compression mode for stored fields produced
// by Lucene104Codec. Mirrors org.apache.lucene.codecs.lucene104.Lucene104Codec.Mode.
type Lucene104CodecMode int

const (
	// Lucene104CodecBestSpeed trades compression ratio for retrieval speed.
	// Uses LZ4 fast compression.
	Lucene104CodecBestSpeed Lucene104CodecMode = iota
	// Lucene104CodecBestCompression trades retrieval speed for compression ratio.
	// Uses Deflate (zlib) compression.
	Lucene104CodecBestCompression
)

// Lucene104Codec is the default codec for Lucene 10.4.
// This codec uses:
//   - Lucene104PostingsFormat for postings (term -> document mappings)
//   - Lucene104StoredFieldsFormat for stored fields (or CompressingStoredFieldsFormat when Mode is specified)
//   - Lucene104FieldInfosFormat for field metadata
//   - Lucene104SegmentInfosFormat for segment metadata
//   - Lucene104TermVectorsFormat for term vectors
//   - Lucene90DocValuesFormat for doc values (Lucene 10.x uses the same format as 9.x)
//   - Lucene99HnswVectorsFormat (via PerFieldKnnVectorsFormat) for KNN vectors
//
// This is the Go port of Lucene's org.apache.lucene.codecs.lucene104.Lucene104Codec.
type Lucene104Codec struct {
	*BaseCodec
	mode               Lucene104CodecMode
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
	normsFormat        NormsFormat      // Lucene90NormsFormat (.nvd / .nvm)
}

// newLucene104CodecDefaults constructs a *Lucene104Codec with all format fields
// populated, ready for the caller to override before returning.
func newLucene104CodecDefaults(mode Lucene104CodecMode, sf StoredFieldsFormat) *Lucene104Codec {
	defaultKnn, err := NewLucene99HnswVectorsFormat()
	if err != nil {
		panic("lucene104: NewLucene99HnswVectorsFormat with default params: " + err.Error())
	}
	return &Lucene104Codec{
		BaseCodec:          NewBaseCodec("Lucene104"),
		mode:               mode,
		postingsFormat:     NewPerFieldPostingsFormatWithDefault(NewLucene104PostingsFormat()),
		storedFieldsFormat: sf,
		fieldInfosFormat:   NewLucene104FieldInfosFormat(),
		segmentInfosFormat: NewLucene104SegmentInfosFormat(),
		segmentInfoFormat:  NewLucene99SegmentInfoFormat(),
		termVectorsFormat:  NewLucene104TermVectorsFormat(),
		docValuesFormat:    NewPerFieldDocValuesFormatWithDefault(NewLucene90DocValuesFormat()),
		compoundFormat:     NewLucene90CompoundFormat(),
		knnVectorsFormat:   NewPerFieldKnnVectorsFormatWithDefault(defaultKnn),
		pointsFormat:       NewLucene90PointsFormat(),
		normsFormat:        NewLucene90NormsFormat(),
	}
}

// NewLucene104Codec creates a new Lucene104Codec with BEST_SPEED default
// stored-fields compression, using the simplified Lucene104StoredFieldsFormat.
func NewLucene104Codec() *Lucene104Codec {
	return newLucene104CodecDefaults(Lucene104CodecBestSpeed, NewLucene104StoredFieldsFormat())
}

// NewLucene104CodecWithMode creates a new Lucene104Codec configured with the
// given compression mode for stored fields. BEST_SPEED uses LZ4 fast compression;
// BEST_COMPRESSION uses Deflate (zlib) compression with larger chunks.
//
// Mirrors org.apache.lucene.codecs.lucene104.Lucene104Codec(Mode).
func NewLucene104CodecWithMode(mode Lucene104CodecMode) *Lucene104Codec {
	var sf StoredFieldsFormat
	switch mode {
	case Lucene104CodecBestSpeed:
		sf = NewCompressingStoredFieldsFormat(CompressionModeLZ4Fast, 16*1024, 128)
	case Lucene104CodecBestCompression:
		sf = NewCompressingStoredFieldsFormat(CompressionModeDeflate, 64*1024, 256)
	default:
		sf = NewLucene104StoredFieldsFormat()
	}
	return newLucene104CodecDefaults(mode, sf)
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

// Mode returns the compression mode configured for this codec.
func (c *Lucene104Codec) Mode() Lucene104CodecMode {
	return c.mode
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

// NormsFormat returns the Lucene90NormsFormat used for per-field,
// per-document normalization factors, mirroring
// org.apache.lucene.codecs.lucene104.Lucene104Codec.normsFormat() which
// returns Lucene90NormsFormat in Lucene 10.4.0.
func (c *Lucene104Codec) NormsFormat() NormsFormat {
	return c.normsFormat
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
