// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"
)

// Lucene103PostingsFormat is the read-only port of
// org.apache.lucene.backward_codecs.lucene103.Lucene103PostingsFormat from
// Apache Lucene 10.4.0.
//
// In Lucene 10.4.0 the Lucene 10.3 postings format lives under
// backward-codecs and is BACKWARD-COMPATIBLE / READ-ONLY: its fieldsConsumer
// throws UnsupportedOperationException ("This postings format may not be used
// for writing, use the current postings format"). Gocene mirrors that contract
// exactly — FieldsConsumer returns errLucene103ReadOnly, while FieldsProducer
// wires a fully functional Lucene103PostingsReader into a
// Lucene103BlockTreeTermsReader so that segments written by Apache Lucene 10.3
// can be read byte-faithfully.
//
// It pairs Lucene103BlockTreeTermsReader (the term-dictionary side) with
// Lucene103PostingsReader (the per-document / per-position side, which decodes
// the 128-wide PFOR-delta .doc/.pos/.pay layout described in the package
// javadoc).
type Lucene103PostingsFormat struct {
	*BasePostingsFormat

	minTermBlockSize int
	maxTermBlockSize int
}

// Lucene103PostingsFormatName is the name under which this format is registered
// in Gocene's PostingsFormat SPI.
//
// NOTE on the Lucene-true name: Apache Lucene's Lucene103PostingsFormat
// registers itself under the SPI name "Lucene103" (its constructor calls
// super("Lucene103")). Gocene's pre-existing SPI registration and the
// baseline TestPostingsFormatByName_CoreFormats suite expect this format under
// "Lucene103PostingsFormat", so that name is retained here to preserve the
// established public SPI surface. Renaming the registered SPI entry to the
// Lucene-true "Lucene103" is a separate, out-of-scope concern (it would also
// require updating spi_test.go), and is intentionally not changed by this
// read-only postings-port task.
const Lucene103PostingsFormatName = "Lucene103PostingsFormat"

// errLucene103ReadOnly mirrors the UnsupportedOperationException thrown by
// Apache Lucene's Lucene103PostingsFormat.fieldsConsumer. The Lucene 10.3
// postings format is a backward-compatibility format and may only be read.
var errLucene103ReadOnly = errors.New(
	"Lucene103PostingsFormat: this postings format may not be used for writing, " +
		"use the current postings format (Lucene 10.3 is a read-only backward-compatibility format)")

// NewLucene103PostingsFormat creates a Lucene103PostingsFormat using the
// default block sizes from Lucene103BlockTreeTermsWriter.
func NewLucene103PostingsFormat() *Lucene103PostingsFormat {
	return NewLucene103PostingsFormatWithBlockSizes(Lucene103DefaultMinBlockSize, Lucene103DefaultMaxBlockSize)
}

// NewLucene103PostingsFormatWithBlockSizes creates a Lucene103PostingsFormat
// pinned to a specific term-block size pair. (Block sizes only matter for the
// write side, which is unsupported; they are retained for API parity.)
func NewLucene103PostingsFormatWithBlockSizes(minBlock, maxBlock int) *Lucene103PostingsFormat {
	return &Lucene103PostingsFormat{
		BasePostingsFormat: NewBasePostingsFormat(Lucene103PostingsFormatName),
		minTermBlockSize:   minBlock,
		maxTermBlockSize:   maxBlock,
	}
}

// MinTermBlockSize returns the configured minimum number of items per term block.
func (f *Lucene103PostingsFormat) MinTermBlockSize() int { return f.minTermBlockSize }

// MaxTermBlockSize returns the configured maximum number of items per term block.
func (f *Lucene103PostingsFormat) MaxTermBlockSize() int { return f.maxTermBlockSize }

// FieldsConsumer always returns errLucene103ReadOnly. The Lucene 10.3 postings
// format is backward-compatibility / read-only in Lucene 10.4.0; only the
// current postings format may be used for writing.
//
// Mirrors Lucene103PostingsFormat.fieldsConsumer (throws
// UnsupportedOperationException).
func (f *Lucene103PostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	return nil, errLucene103ReadOnly
}

// FieldsProducer wires a Lucene103PostingsReader into a
// Lucene103BlockTreeTermsReader, mirroring
// Lucene103PostingsFormat.fieldsProducer(SegmentReadState).
func (f *Lucene103PostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	postingsReader, err := NewLucene103PostingsReader(state)
	if err != nil {
		return nil, err
	}
	producer, err := NewLucene103BlockTreeTermsReader(postingsReader, state)
	if err != nil {
		// Close the postings reader on failure (IOUtils.closeWhileHandlingException).
		_ = postingsReader.Close()
		return nil, err
	}
	return producer, nil
}

// Compile-time interface check.
var _ PostingsFormat = (*Lucene103PostingsFormat)(nil)
