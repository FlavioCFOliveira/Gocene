// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// Lucene94StoredFieldsMode controls the compression preset for stored fields.
// Port of org.apache.lucene.backward_codecs.lucene94.Lucene94Codec.Mode.
type Lucene94StoredFieldsMode int

const (
	// Lucene94StoredFieldsBestSpeed trades compression ratio for retrieval speed.
	Lucene94StoredFieldsBestSpeed Lucene94StoredFieldsMode = iota
	// Lucene94StoredFieldsBestCompression trades retrieval speed for compression ratio.
	Lucene94StoredFieldsBestCompression
)

// Lucene94Codec implements the Lucene 9.4 index format.
//
// It extends FilterCodec (delegating to Lucene104Codec for formats unchanged
// between Lucene 9.4 and 10.4) and overrides only the components that differ:
//   - StoredFieldsFormat  → Lucene90StoredFieldsFormat (with user-selected mode)
//   - KnnVectorsFormat    → Lucene94HnswVectorsFormat
//
// If you want to reuse functionality of this codec in another codec, embed it
// (composition) or wrap it with FilterCodec.
//
// Port of org.apache.lucene.backward_codecs.lucene94.Lucene94Codec.
type Lucene94Codec struct {
	*codecs.FilterCodec
	storedFieldsFormat codecs.StoredFieldsFormat
	knnVectorsFormat   *Lucene94HnswVectorsFormat
}

// NewLucene94Codec returns a Lucene94Codec using BEST_SPEED stored-fields compression.
func NewLucene94Codec() *Lucene94Codec {
	return NewLucene94CodecWithMode(Lucene94StoredFieldsBestSpeed)
}

// NewLucene94CodecWithMode returns a Lucene94Codec with the given stored-fields
// compression mode.
func NewLucene94CodecWithMode(mode Lucene94StoredFieldsMode) *Lucene94Codec {
	var sfMode lucene90.Lucene90StoredFieldsMode
	switch mode {
	case Lucene94StoredFieldsBestCompression:
		sfMode = lucene90.Lucene90StoredFieldsBestCompression
	default:
		sfMode = lucene90.Lucene90StoredFieldsBestSpeed
	}
	return &Lucene94Codec{
		FilterCodec:        codecs.NewFilterCodec("Lucene94", codecs.NewLucene104Codec()),
		storedFieldsFormat: lucene90.NewLucene90StoredFieldsFormatWithMode(sfMode),
		knnVectorsFormat:   NewLucene94HnswVectorsFormat(),
	}
}

// StoredFieldsFormat returns the Lucene 9.0 stored fields format.
func (c *Lucene94Codec) StoredFieldsFormat() codecs.StoredFieldsFormat {
	return c.storedFieldsFormat
}

// KnnVectorsFormat returns the Lucene 9.4 HNSW vectors format.
func (c *Lucene94Codec) KnnVectorsFormat() *Lucene94HnswVectorsFormat {
	return c.knnVectorsFormat
}

// GetKnnVectorsFormatForField returns the Lucene94HnswVectorsFormat for all fields.
// Subclasses may override to return different formats per field.
func (c *Lucene94Codec) GetKnnVectorsFormatForField(_ string) *Lucene94HnswVectorsFormat {
	return c.knnVectorsFormat
}
