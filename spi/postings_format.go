// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

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
// serialising postings.
//
// Mirrors org.apache.lucene.codecs.FieldsConsumer of Apache Lucene 10.5.0.
type FieldsConsumer interface {
	// Write serialises all fields, terms and postings. This is the "pull"
	// API: the consumer iterates fields itself and may traverse the postings
	// more than once.
	//
	// Notes (carried from the Java contract):
	//
	//   - The implementation must compute the index statistics, including
	//     each term's docFreq and totalTermFreq, as well as the summary
	//     sumTotalTermFreq, sumDocFreq and docCount.
	//   - It must skip terms that have no docs and fields that have no terms,
	//     even though the provided Fields exposes them; this typically
	//     requires lazily writing the field or term until the first term or
	//     document has actually been seen.
	//   - The provided Fields is limited: methods that return
	//     statistics/counts must not be called, and a non-nil live-docs set
	//     must not be passed when pulling docs/positions enums.
	//
	// Mirrors FieldsConsumer.write(Fields, NormsProducer)
	// (FieldsConsumer.java:64).
	Write(fields Fields, norms NormsProducer) error

	// Close releases any resources held by the consumer.
	Close() error
}

// FieldsProducer is the read-side surface a PostingsFormat exposes for
// iterating over per-field postings.
//
// Mirrors org.apache.lucene.codecs.FieldsProducer of Apache Lucene 10.5.0,
// which extends org.apache.lucene.index.Fields (iterator(), terms(String),
// size()) and implements Closeable.
type FieldsProducer interface {
	// Fields supplies Iterator, Terms and Size.
	Fields

	// Close releases any resources held by the producer.
	Close() error

	// CheckIntegrity walks the per-field postings data and validates the
	// checksum framing.
	CheckIntegrity() error

	// GetMergeInstance returns an instance optimized for merging. This
	// instance may only be consumed in the thread that called
	// GetMergeInstance.
	//
	// The default implementation returns the receiver itself.
	GetMergeInstance() FieldsProducer
}
