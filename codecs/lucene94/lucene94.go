// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Mode selects the compression mode for stored fields produced
// by Lucene94Codec. Mirrors org.apache.lucene.codecs.lucene94.Lucene94Codec.Mode.
type Mode int

const (
	// BestSpeed trades compression ratio for retrieval speed.
	BestSpeed Mode = iota
	// BestCompression trades retrieval speed for compression ratio.
	BestCompression
)

// Lucene94Codec implements the Lucene 9.4 index format.
//
// This is the Go port of Lucene's org.apache.lucene.backward_codecs.lucene94.Lucene94Codec.
type Lucene94Codec struct {
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

// NewLucene94Codec creates a new Lucene94Codec with BestSpeed default
// stored-fields compression.
func NewLucene94Codec() *Lucene94Codec {
	return NewLucene94CodecWithMode(BestSpeed)
}

// NewLucene94CodecWithMode creates a new Lucene94Codec configured with the
// given compression mode for stored fields.
func NewLucene94CodecWithMode(mode Mode) *Lucene94Codec {
	var sf codecs.StoredFieldsFormat
	switch mode {
	case BestSpeed:
		// Java: new Lucene90StoredFieldsFormat(Objects.requireNonNull(mode).storedMode)
		// (Lucene94Codec(Mode), storedMode == Lucene90StoredFieldsFormat.Mode.BEST_SPEED).
		sf = lucene90.NewLucene90StoredFieldsFormatWithMode(lucene90.Lucene90StoredFieldsBestSpeed)
	case BestCompression:
		// Java: the same call with storedMode ==
		// Lucene90StoredFieldsFormat.Mode.BEST_COMPRESSION.
		sf = lucene90.NewLucene90StoredFieldsFormatWithMode(lucene90.Lucene90StoredFieldsBestCompression)
	default:
		sf = lucene90.NewLucene90StoredFieldsFormat()
	}

	defaultPostings := lucene90.NewLucene90PostingsFormat()
	defaultDV := lucene90.NewLucene90DocValuesFormat()
	defaultKnn := NewLucene94HnswVectorsFormat()

	return &Lucene94Codec{
		BaseCodec:               codecs.NewBaseCodec("Lucene94"),
		mode:                    mode,
		postingsFormat:          codecs.NewPerFieldPostingsFormatWithDefault(defaultPostings),
		storedFieldsFormat:      sf,
		fieldInfosFormat:        NewLucene94FieldInfosFormat(),
		segmentInfosFormat:      lucene90.NewLucene90SegmentInfoFormat(),
		segmentInfoFormat:       lucene90.NewLucene90SegmentInfoFormat(),
		termVectorsFormat:       lucene90.NewLucene90TermVectorsFormat(),
		docValuesFormat:         codecs.NewPerFieldDocValuesFormatWithDefault(defaultDV),
		compoundFormat:          lucene90.NewLucene90CompoundFormat(),
		knnVectorsFormat:        codecs.NewPerFieldKnnVectorsFormatWithDefault(defaultKnn),
		pointsFormat:            lucene90.NewLucene90PointsFormat(),
		normsFormat:             lucene90.NewLucene90NormsFormat(),
		defaultPostingsFormat:   defaultPostings,
		defaultDocValuesFormat:  defaultDV,
		defaultKnnVectorsFormat: defaultKnn,
	}
}

// StoredFieldsFormat returns the stored fields format.
func (c *Lucene94Codec) StoredFieldsFormat() codecs.StoredFieldsFormat {
	return c.storedFieldsFormat
}

// TermVectorsFormat returns the term vectors format.
func (c *Lucene94Codec) TermVectorsFormat() codecs.TermVectorsFormat {
	return c.termVectorsFormat
}

// PostingsFormat returns the postings format.
func (c *Lucene94Codec) PostingsFormat() codecs.PostingsFormat {
	return c.postingsFormat
}

// FieldInfosFormat returns the field infos format.
func (c *Lucene94Codec) FieldInfosFormat() codecs.FieldInfosFormat {
	return c.fieldInfosFormat
}

// SegmentInfoFormat returns the per-segment .si format.
func (c *Lucene94Codec) SegmentInfoFormat() codecs.SegmentInfoFormat {
	return c.segmentInfoFormat
}

// SegmentInfosFormat returns the segment infos format.
func (c *Lucene94Codec) SegmentInfosFormat() codecs.SegmentInfoFormat {
	return c.segmentInfosFormat
}

// LiveDocsFormat returns the live docs format.
func (c *Lucene94Codec) LiveDocsFormat() codecs.LiveDocsFormat {
	return lucene90.NewLucene90LiveDocsFormat()
}

// CompoundFormat returns the compound format.
func (c *Lucene94Codec) CompoundFormat() codecs.CompoundFormat {
	return c.compoundFormat
}

// KnnVectorsFormat returns the KNN vectors format.
func (c *Lucene94Codec) KnnVectorsFormat() codecs.KnnVectorsFormat {
	return c.knnVectorsFormat
}

// PointsFormat returns the points (BKD) format.
func (c *Lucene94Codec) PointsFormat() codecs.PointsFormat {
	return c.pointsFormat
}

// NormsFormat returns the norms format.
func (c *Lucene94Codec) NormsFormat() codecs.NormsFormat {
	return c.normsFormat
}

// DocValuesFormat returns the doc values format.
func (c *Lucene94Codec) DocValuesFormat() codecs.DocValuesFormat {
	return c.docValuesFormat
}

// GetPostingsFormatForField returns the postings format that should be used for
// writing new segments of field.
func (c *Lucene94Codec) GetPostingsFormatForField(field string) codecs.PostingsFormat {
	return c.defaultPostingsFormat
}

// GetDocValuesFormatForField returns the docvalues format that should be used for
// writing new segments of field.
func (c *Lucene94Codec) GetDocValuesFormatForField(field string) codecs.DocValuesFormat {
	return c.defaultDocValuesFormat
}

// GetKnnVectorsFormatForField returns the vectors format that should be used for
// writing new segments of field.
func (c *Lucene94Codec) GetKnnVectorsFormatForField(field string) codecs.KnnVectorsFormat {
	return c.defaultKnnVectorsFormat
}

// Mode returns the compression mode configured for this codec.
func (c *Lucene94Codec) Mode() Mode {
	return c.mode
}
