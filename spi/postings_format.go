// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// PostingsFormat encodes and decodes the term -> document postings of a
// segment. Lucene stores postings in the per-segment .pst / .doc / .pos
// / .pay files; the format is responsible for both the on-disk byte
// layout and for surfacing the postings via the FieldsConsumer (write)
// and FieldsProducer (read) interfaces.
//
// Mirrors org.apache.lucene.codecs.PostingsFormat in Apache Lucene
// 10.4.0.
type PostingsFormat interface {
	// Name returns the codec name embedded in segment metadata.
	Name() string

	// FieldsConsumer returns a consumer that the writer drives to
	// serialise postings for one segment. The caller closes the
	// returned consumer when done.
	FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error)

	// FieldsProducer returns a producer that the reader uses to scan
	// postings for one segment. The caller closes the returned
	// producer when done.
	FieldsProducer(state *SegmentReadState) (FieldsProducer, error)
}

// FieldsConsumer is the write-side surface a PostingsFormat exposes for
// serialising postings field by field.
//
// Mirrors org.apache.lucene.codecs.FieldsConsumer.
type FieldsConsumer interface {
	// Write serialises the postings for one field.
	Write(field string, terms schema.Terms) error

	// Close releases any resources held by the consumer.
	Close() error
}

// FieldsProducer is the read-side surface a PostingsFormat exposes for
// iterating over per-field postings.
//
// Mirrors org.apache.lucene.codecs.FieldsProducer.
type FieldsProducer interface {
	// Terms returns the Terms enumeration for the given field, or nil
	// when the field has no postings in this segment.
	Terms(field string) (schema.Terms, error)

	// Close releases any resources held by the producer.
	Close() error
}
