// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene104

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// Mode selects the compression mode for stored fields produced
// by Lucene104Codec. Mirrors org.apache.lucene.codecs.lucene104.Lucene104Codec.Mode.
type Mode int

const (
	// BestSpeed trades compression ratio for retrieval speed.
	// Uses LZ4 fast compression.
	BestSpeed Mode = iota
	// BestCompression trades retrieval speed for compression ratio.
	// Uses Deflate (zlib) compression.
	BestCompression
)

// Lucene104Codec implements the Lucene 10.4 index format.
//
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
	*codecs.BaseCodec
	mode               Mode
	postingsFormat     codecs.PostingsFormat
	storedFieldsFormat codecs.StoredFieldsFormat
	fieldInfosFormat   codecs.FieldInfosFormat
	segmentInfosFormat codecs.SegmentInfoFormat
	segmentInfoFormat  codecs.SegmentInfoFormat
	termVectorsFormat  codecs.TermVectorsFormat
	docValuesFormat    codecs.DocValuesFormat
	compoundFormat     codecs.CompoundFormat
	knnVectorsFormat   codecs.KnnVectorsFormat
	pointsFormat       codecs.PointsFormat
	normsFormat        codecs.NormsFormat

	defaultPostingsFormat   codecs.PostingsFormat
	defaultDocValuesFormat  codecs.DocValuesFormat
	defaultKnnVectorsFormat codecs.KnnVectorsFormat
}

// NewLucene104Codec creates a new Lucene104Codec with BestSpeed default
// stored-fields compression.
func NewLucene104Codec() *Lucene104Codec {
	return NewLucene104CodecWithMode(BestSpeed)
}

// NewLucene104CodecWithMode creates a new Lucene104Codec configured with the
// given compression mode for stored fields. BestSpeed uses LZ4 fast compression;
// BestCompression uses Deflate (zlib) compression with larger chunks.
//
// Mirrors org.apache.lucene.codecs.lucene104.Lucene104Codec(Mode).
func NewLucene104CodecWithMode(mode Mode) *Lucene104Codec {
	var sf codecs.StoredFieldsFormat
	switch mode {
	case BestSpeed:
		// Java: new Lucene90StoredFieldsFormat(Objects.requireNonNull(mode).storedMode)
		// (Lucene104Codec(Mode), storedMode == Lucene90StoredFieldsFormat.Mode.BEST_SPEED).
		sf = lucene90.NewLucene90StoredFieldsFormatWithMode(lucene90.Lucene90StoredFieldsBestSpeed)
	case BestCompression:
		// Java: the same call with storedMode ==
		// Lucene90StoredFieldsFormat.Mode.BEST_COMPRESSION.
		sf = lucene90.NewLucene90StoredFieldsFormatWithMode(lucene90.Lucene90StoredFieldsBestCompression)
	default:
		sf = codecs.NewLucene104StoredFieldsFormat()
	}

	defaultPostings := codecs.NewLucene104PostingsFormat()
	defaultDV := codecs.NewLucene90DocValuesFormat()
	defaultKnn, err := codecs.NewLucene99HnswVectorsFormat()
	if err != nil {
		panic("lucene104: NewLucene99HnswVectorsFormat with default params: " + err.Error())
	}

	return &Lucene104Codec{
		BaseCodec:               codecs.NewBaseCodec("Lucene104"),
		mode:                    mode,
		postingsFormat:          codecs.NewPerFieldPostingsFormatWithDefault(defaultPostings),
		storedFieldsFormat:      sf,
		fieldInfosFormat:        codecs.NewLucene104FieldInfosFormat(),
		segmentInfosFormat:      codecs.NewLucene104SegmentInfosFormat(),
		segmentInfoFormat:       codecs.NewLucene99SegmentInfoFormat(),
		termVectorsFormat:       codecs.NewLucene104TermVectorsFormat(),
		docValuesFormat:         codecs.NewPerFieldDocValuesFormatWithDefault(defaultDV),
		compoundFormat:          codecs.NewLucene90CompoundFormat(),
		knnVectorsFormat:        codecs.NewPerFieldKnnVectorsFormatWithDefault(defaultKnn),
		pointsFormat:            codecs.NewLucene90PointsFormat(),
		normsFormat:             codecs.NewLucene90NormsFormat(),
		defaultPostingsFormat:   defaultPostings,
		defaultDocValuesFormat:  defaultDV,
		defaultKnnVectorsFormat: defaultKnn,
	}
}

// StoredFieldsFormat returns the stored fields format.
func (c *Lucene104Codec) StoredFieldsFormat() codecs.StoredFieldsFormat {
	return c.storedFieldsFormat
}

// TermVectorsFormat returns the term vectors format.
func (c *Lucene104Codec) TermVectorsFormat() codecs.TermVectorsFormat {
	return c.termVectorsFormat
}

// PostingsFormat returns the postings format.
func (c *Lucene104Codec) PostingsFormat() codecs.PostingsFormat {
	return c.postingsFormat
}

// FieldInfosFormat returns the field infos format.
func (c *Lucene104Codec) FieldInfosFormat() codecs.FieldInfosFormat {
	return c.fieldInfosFormat
}

// SegmentInfoFormat returns the per-segment .si format.
func (c *Lucene104Codec) SegmentInfoFormat() codecs.SegmentInfoFormat {
	return c.segmentInfoFormat
}

// SegmentInfosFormat returns the segment infos format.
func (c *Lucene104Codec) SegmentInfosFormat() codecs.SegmentInfoFormat {
	return c.segmentInfosFormat
}

// LiveDocsFormat returns the live docs format.
func (c *Lucene104Codec) LiveDocsFormat() codecs.LiveDocsFormat {
	return codecs.NewLucene90LiveDocsFormat()
}

// CompoundFormat returns the compound format.
func (c *Lucene104Codec) CompoundFormat() codecs.CompoundFormat {
	return c.compoundFormat
}

// KnnVectorsFormat returns the KNN vectors format.
func (c *Lucene104Codec) KnnVectorsFormat() codecs.KnnVectorsFormat {
	return c.knnVectorsFormat
}

// PointsFormat returns the points (BKD) format.
func (c *Lucene104Codec) PointsFormat() codecs.PointsFormat {
	return c.pointsFormat
}

// NormsFormat returns the norms format.
func (c *Lucene104Codec) NormsFormat() codecs.NormsFormat {
	return c.normsFormat
}

// DocValuesFormat returns the doc values format.
func (c *Lucene104Codec) DocValuesFormat() codecs.DocValuesFormat {
	return c.docValuesFormat
}

// GetPostingsFormatForField returns the postings format that should be used for
// writing new segments of field.
//
// The default implementation always returns "Lucene104".
func (c *Lucene104Codec) GetPostingsFormatForField(field string) codecs.PostingsFormat {
	return c.defaultPostingsFormat
}

// GetDocValuesFormatForField returns the docvalues format that should be used for
// writing new segments of field.
//
// The default implementation always returns "Lucene90".
func (c *Lucene104Codec) GetDocValuesFormatForField(field string) codecs.DocValuesFormat {
	return c.defaultDocValuesFormat
}

// GetKnnVectorsFormatForField returns the vectors format that should be used for
// writing new segments of field.
//
// The default implementation always returns "Lucene99HnswVectorsFormat".
func (c *Lucene104Codec) GetKnnVectorsFormatForField(field string) codecs.KnnVectorsFormat {
	return c.defaultKnnVectorsFormat
}

// Mode returns the compression mode configured for this codec.
func (c *Lucene104Codec) Mode() Mode {
	return c.mode
}
