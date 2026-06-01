// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package idversion implements
// org.apache.lucene.sandbox.codecs.idversion: a postings format optimised for
// ID-with-version updates.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsFormat
// from Apache Lucene 10.4.0.
package idversion

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// IDVersionPostingsFormat is the PostingsFormat for ID-with-version fields.
//
// Each term must appear in exactly one document. The version is supplied as an
// 8-byte big-endian payload on the single position token. At query time,
// IDVersionSegmentTermsEnum.SeekExactWithVersion allows fast-rejection using
// the per-block max-version stored in the FST index.
//
// Mirrors org.apache.lucene.sandbox.codecs.idversion.IDVersionPostingsFormat.
type IDVersionPostingsFormat struct {
	minTermsInBlock int
	maxTermsInBlock int
}

// NewIDVersionPostingsFormat constructs the format using the same default
// block-size bounds as Lucene103BlockTreeTermsWriter.
//
// Mirrors IDVersionPostingsFormat() (no-arg constructor).
func NewIDVersionPostingsFormat() *IDVersionPostingsFormat {
	return &IDVersionPostingsFormat{
		minTermsInBlock: vbtDefaultMinBlockSize,
		maxTermsInBlock: vbtDefaultMaxBlockSize,
	}
}

// NewIDVersionPostingsFormatWithBlockSize constructs the format with explicit
// min/max block sizes.
//
// Mirrors IDVersionPostingsFormat(int, int).
func NewIDVersionPostingsFormatWithBlockSize(minTermsInBlock, maxTermsInBlock int) (*IDVersionPostingsFormat, error) {
	if err := validateBlockSizeSettings(minTermsInBlock, maxTermsInBlock); err != nil {
		return nil, err
	}
	return &IDVersionPostingsFormat{
		minTermsInBlock: minTermsInBlock,
		maxTermsInBlock: maxTermsInBlock,
	}, nil
}

// Name returns "IDVersion", the codec name embedded in segment metadata.
//
// Mirrors PostingsFormat.getName() → "IDVersion".
func (f *IDVersionPostingsFormat) Name() string { return "IDVersion" }

// FieldsConsumer returns the write-side consumer that serialises term postings.
//
// Creates an IDVersionPostingsWriter (which writes postings data inline with
// the term dictionary) and wraps it in a VersionBlockTreeTermsWriter (which
// writes the block-tree structure to .tiv/.tipv files).
//
// Mirrors IDVersionPostingsFormat.fieldsConsumer(SegmentWriteState).
func (f *IDVersionPostingsFormat) FieldsConsumer(state *codecs.SegmentWriteState) (codecs.FieldsConsumer, error) {
	// LiveDocs is *util.FixedBitSet which implements util.Bits; pass it directly.
	// When LiveDocs is nil all documents are live and postingsWriter ignores it.
	var liveDocs util.Bits
	if state.LiveDocs != nil {
		liveDocs = state.LiveDocs
	}
	postingsWriter := NewIDVersionPostingsWriter(liveDocs)

	writer, err := NewVersionBlockTreeTermsWriter(state, postingsWriter, f.minTermsInBlock, f.maxTermsInBlock)
	if err != nil {
		_ = postingsWriter.Close()
		return nil, fmt.Errorf("IDVersionPostingsFormat.FieldsConsumer: %w", err)
	}
	return writer, nil
}

// FieldsProducer returns the read-side producer for decoding term postings.
//
// Creates an IDVersionPostingsReader (which reads per-term postings data) and
// wraps it in a VersionBlockTreeTermsReader (which opens and reads the .tiv
// and .tipv block-tree files).
//
// Mirrors IDVersionPostingsFormat.fieldsProducer(SegmentReadState).
func (f *IDVersionPostingsFormat) FieldsProducer(state *codecs.SegmentReadState) (codecs.FieldsProducer, error) {
	postingsReader := &IDVersionPostingsReader{}

	reader, err := NewVersionBlockTreeTermsReader(postingsReader, state)
	if err != nil {
		_ = postingsReader.Close()
		return nil, fmt.Errorf("IDVersionPostingsFormat.FieldsProducer: %w", err)
	}
	return reader, nil
}

// validateBlockSizeSettings mirrors
// Lucene103BlockTreeTermsWriter.validateSettings (Apache Lucene 10.4.0).
// The two constraints are: minItemsInBlock >= 2 and
// maxItemsInBlock >= minItemsInBlock.
func validateBlockSizeSettings(minItemsInBlock, maxItemsInBlock int) error {
	if minItemsInBlock <= 1 {
		return fmt.Errorf("minItemsInBlock must be >= 2; got %d", minItemsInBlock)
	}
	if maxItemsInBlock < minItemsInBlock {
		return fmt.Errorf("maxItemsInBlock must be >= minItemsInBlock; got maxItemsInBlock=%d minItemsInBlock=%d",
			maxItemsInBlock, minItemsInBlock)
	}
	return nil
}
