// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import "github.com/FlavioCFOliveira/Gocene/schema"

// NormsFormat encodes and decodes per-field, per-document normalization
// factors (the .nvd / .nvm pair in the on-disk codec).
//
// Mirrors org.apache.lucene.codecs.NormsFormat in Apache Lucene 10.4.0.
// Lifted onto the SPI by rmp #120 (Sprint 3) so that index/ and codecs/
// can both reach the canonical interface through a single declaration
// site, exactly as the doc-values family was lifted by rmp #4708.
//
// Norms are stored as a NumericDocValues stream internally (one value per
// value-bearing document), so the read side mirrors the doc-values
// NumericDocValues iterator, while the write side accepts the single-pass
// NormsIterator the indexing flush replays.
type NormsFormat interface {
	// Name returns the format name persisted in segment metadata.
	Name() string

	// NormsConsumer returns the per-field write side of the norms pipeline
	// for the segment described by state. The caller is responsible for
	// closing the returned consumer.
	NormsConsumer(state *SegmentWriteState) (NormsConsumer, error)

	// NormsProducer returns the per-field read side of the norms pipeline
	// for the segment described by state. The caller is responsible for
	// closing the returned producer.
	NormsProducer(state *SegmentReadState) (NormsProducer, error)
}

// NormsConsumer is the per-segment write side of the norms pipeline.
// Mirrors org.apache.lucene.codecs.NormsConsumer in Apache Lucene 10.4.0.
//
// The flush path feeds AddNormsField a single-pass writer-side iterator
// over the in-memory accumulator's contents; the consumer serializes the
// values to the segment's .nvd / .nvm files.
type NormsConsumer interface {
	// AddNormsField persists the norms for a single field. The values are
	// drawn from the iterator in strictly increasing docID order.
	AddNormsField(field *schema.FieldInfo, values NormsIterator) error

	// Close flushes any pending bytes and releases the consumer's
	// resources.
	Close() error
}

// NormsProducer is the per-segment read side of the norms pipeline.
// Mirrors org.apache.lucene.codecs.NormsProducer in Apache Lucene 10.4.0.
type NormsProducer interface {
	// GetNorms returns a NumericDocValues iterator over the norms of the
	// given field, or nil when the field has no norms.
	GetNorms(field *schema.FieldInfo) (NumericDocValues, error)

	// CheckIntegrity walks the per-field data and validates the checksum
	// framing.
	CheckIntegrity() error

	// Close releases the producer's resources.
	Close() error
}

// NormsIterator is the single-pass writer-side cursor the norms flush path
// replays into NormsConsumer.AddNormsField.
//
// Mirrors the iteration contract NormsConsumer.addNormsField consumes from
// a NumericDocValues in Apache Lucene 10.4.0: the consumer walks the
// value-bearing documents once (Gocene materialises the values internally
// when it needs more than one pass). Documents are visited in strictly
// increasing docID order.
type NormsIterator interface {
	// Next advances to the next value-bearing document, returning false
	// once the stream is exhausted.
	Next() bool

	// DocID returns the current document ID.
	DocID() int

	// LongValue returns the current document's norm value. Norms are a
	// single byte (0-255) in Apache Lucene's default similarity but are
	// carried as int64 to match the NumericDocValues surface.
	LongValue() int64
}
