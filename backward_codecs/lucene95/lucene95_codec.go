// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// Lucene95StoredFieldsMode controls the compression preset for stored fields.
// Port of org.apache.lucene.backward_codecs.lucene95.Lucene95Codec.Mode.
type Lucene95StoredFieldsMode int

const (
	// Lucene95StoredFieldsBestSpeed trades compression ratio for retrieval speed.
	Lucene95StoredFieldsBestSpeed Lucene95StoredFieldsMode = iota
	// Lucene95StoredFieldsBestCompression trades retrieval speed for compression ratio.
	Lucene95StoredFieldsBestCompression
)

// Lucene95Codec implements the Lucene 9.5 index format.
//
// It extends FilterCodec (delegating to Lucene104Codec for formats unchanged
// between Lucene 9.5 and 10.4) and overrides only the components that differ:
//   - StoredFieldsFormat  → Lucene90StoredFieldsFormat (with user-selected mode)
//   - KnnVectorsFormat    → Lucene95HnswVectorsFormat
//
// If you want to reuse functionality of this codec in another codec, embed it
// (composition) or wrap it with FilterCodec.
//
// Port of org.apache.lucene.backward_codecs.lucene95.Lucene95Codec.
type Lucene95Codec struct {
	*codecs.FilterCodec
	storedFieldsFormat codecs.StoredFieldsFormat
	knnVectorsFormat   *Lucene95HnswVectorsFormat
}

// NewLucene95Codec returns a Lucene95Codec using BEST_SPEED stored-fields compression.
func NewLucene95Codec() *Lucene95Codec {
	return NewLucene95CodecWithMode(Lucene95StoredFieldsBestSpeed)
}

// NewLucene95CodecWithMode returns a Lucene95Codec with the given stored-fields
// compression mode.
func NewLucene95CodecWithMode(mode Lucene95StoredFieldsMode) *Lucene95Codec {
	var sfMode lucene90.Lucene90StoredFieldsMode
	switch mode {
	case Lucene95StoredFieldsBestCompression:
		sfMode = lucene90.Lucene90StoredFieldsBestCompression
	default:
		sfMode = lucene90.Lucene90StoredFieldsBestSpeed
	}
	return &Lucene95Codec{
		FilterCodec:        codecs.NewFilterCodec("Lucene95", codecs.NewLucene104Codec()),
		storedFieldsFormat: lucene90.NewLucene90StoredFieldsFormatWithMode(sfMode),
		knnVectorsFormat:   NewLucene95HnswVectorsFormat(),
	}
}

// StoredFieldsFormat returns the Lucene 9.0 stored fields format.
func (c *Lucene95Codec) StoredFieldsFormat() codecs.StoredFieldsFormat {
	return c.storedFieldsFormat
}

// KnnVectorsFormat returns the Lucene 9.5 HNSW vectors format.
func (c *Lucene95Codec) KnnVectorsFormat() *Lucene95HnswVectorsFormat {
	return c.knnVectorsFormat
}

// GetKnnVectorsFormatForField returns the Lucene95HnswVectorsFormat for all fields.
// Subclasses may override to return different formats per field.
func (c *Lucene95Codec) GetKnnVectorsFormatForField(_ string) *Lucene95HnswVectorsFormat {
	return c.knnVectorsFormat
}
