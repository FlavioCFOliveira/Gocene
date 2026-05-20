// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene92

import (
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// Lucene92StoredFieldsMode controls the compression preset for stored fields.
// Port of org.apache.lucene.backward_codecs.lucene92.Lucene92Codec.Mode.
type Lucene92StoredFieldsMode int

const (
	// Lucene92StoredFieldsBestSpeed trades compression ratio for retrieval speed.
	Lucene92StoredFieldsBestSpeed Lucene92StoredFieldsMode = iota
	// Lucene92StoredFieldsBestCompression trades retrieval speed for compression ratio.
	Lucene92StoredFieldsBestCompression
)

// Lucene92Codec implements the Lucene 9.2 index format.
//
// It extends FilterCodec (delegating to Lucene104Codec for formats unchanged
// between Lucene 9.2 and 10.4) and overrides only the components that differ:
//   - StoredFieldsFormat  → Lucene90StoredFieldsFormat (with user-selected mode)
//   - KnnVectorsFormat    → Lucene92HnswVectorsFormat
//
// If you want to reuse functionality of this codec in another codec, embed it
// (composition) or wrap it with FilterCodec.
//
// Port of org.apache.lucene.backward_codecs.lucene92.Lucene92Codec.
type Lucene92Codec struct {
	*codecs.FilterCodec
	storedFieldsFormat codecs.StoredFieldsFormat
	knnVectorsFormat   *Lucene92HnswVectorsFormat
}

// NewLucene92Codec returns a Lucene92Codec using BEST_SPEED stored-fields compression.
func NewLucene92Codec() *Lucene92Codec {
	return NewLucene92CodecWithMode(Lucene92StoredFieldsBestSpeed)
}

// NewLucene92CodecWithMode returns a Lucene92Codec with the given stored-fields
// compression mode.
func NewLucene92CodecWithMode(mode Lucene92StoredFieldsMode) *Lucene92Codec {
	var sfMode lucene90.Lucene90StoredFieldsMode
	switch mode {
	case Lucene92StoredFieldsBestCompression:
		sfMode = lucene90.Lucene90StoredFieldsBestCompression
	default:
		sfMode = lucene90.Lucene90StoredFieldsBestSpeed
	}
	return &Lucene92Codec{
		FilterCodec:        codecs.NewFilterCodec("Lucene92", codecs.NewLucene104Codec()),
		storedFieldsFormat: lucene90.NewLucene90StoredFieldsFormatWithMode(sfMode),
		knnVectorsFormat:   NewLucene92HnswVectorsFormat(),
	}
}

// StoredFieldsFormat returns the Lucene 9.0 stored fields format.
func (c *Lucene92Codec) StoredFieldsFormat() codecs.StoredFieldsFormat {
	return c.storedFieldsFormat
}

// KnnVectorsFormat returns the Lucene 9.2 HNSW vectors format.
func (c *Lucene92Codec) KnnVectorsFormat() *Lucene92HnswVectorsFormat {
	return c.knnVectorsFormat
}

// GetKnnVectorsFormatForField returns the Lucene92HnswVectorsFormat for all fields.
// Subclasses may override to return different formats per field.
func (c *Lucene92Codec) GetKnnVectorsFormatForField(_ string) *Lucene92HnswVectorsFormat {
	return c.knnVectorsFormat
}
