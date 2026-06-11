// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

// Lucene99PostingsFormat is the Go port of
// org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat from
// Apache Lucene 10.4.0.
//
// In Lucene 10.4.0 this format lives under backward-codecs and is
// READ-ONLY (fieldsConsumer throws UnsupportedOperationException).
// Gocene extends this contract to support BOTH reading and writing so
// that byte-level round-trip tests can be performed, verifying that
// Gocene-produced Lucene 9.9 postings can be read back faithfully.
//
// It pairs Lucene103BlockTreeTermsReader/Writer (the term-dictionary side)
// with Lucene99PostingsReader/Lucene99PostingsWriter (the per-document /
// per-position side, which decodes the 128-wide long-based PFOR-delta
// .doc/.pos/.pay layout described in the package javadoc).
type Lucene99PostingsFormat struct {
	*BasePostingsFormat

	minTermBlockSize int
	maxTermBlockSize int
}

// Lucene99PostingsFormatName is the name under which this format is
// registered in Gocene's PostingsFormat SPI.
const Lucene99PostingsFormatName = "Lucene99PostingsFormat"

// NewLucene99PostingsFormat creates a Lucene99PostingsFormat using the
// default block sizes from Lucene103BlockTreeTermsWriter.
func NewLucene99PostingsFormat() *Lucene99PostingsFormat {
	return NewLucene99PostingsFormatWithBlockSizes(Lucene103DefaultMinBlockSize, Lucene103DefaultMaxBlockSize)
}

// NewLucene99PostingsFormatWithBlockSizes creates a Lucene99PostingsFormat
// pinned to a specific term-block size pair. Block sizes only matter for
// the write side.
func NewLucene99PostingsFormatWithBlockSizes(minBlock, maxBlock int) *Lucene99PostingsFormat {
	return &Lucene99PostingsFormat{
		BasePostingsFormat: NewBasePostingsFormat(Lucene99PostingsFormatName),
		minTermBlockSize:   minBlock,
		maxTermBlockSize:   maxBlock,
	}
}

// MinTermBlockSize returns the configured minimum number of items per term block.
func (f *Lucene99PostingsFormat) MinTermBlockSize() int { return f.minTermBlockSize }

// MaxTermBlockSize returns the configured maximum number of items per term block.
func (f *Lucene99PostingsFormat) MaxTermBlockSize() int { return f.maxTermBlockSize }

// FieldsConsumer produces a FieldsConsumer that writes Lucene 9.9 postings
// format. This extends the Java contract (which throws
// UnsupportedOperationException) to support round-trip testing in Gocene.
func (f *Lucene99PostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	postingsWriter, err := NewLucene99PostingsWriter(state)
	if err != nil {
		return nil, err
	}
	btw, err := NewLucene103BlockTreeTermsWriter(state, postingsWriter, f.minTermBlockSize, f.maxTermBlockSize)
	if err != nil {
		_ = postingsWriter.Close()
		return nil, err
	}
	return btw, nil
}

// FieldsProducer wires a Lucene99PostingsReader into a
// Lucene103BlockTreeTermsReader, mirroring
// Lucene99PostingsFormat.fieldsProducer(SegmentReadState).
func (f *Lucene99PostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	postingsReader, err := NewLucene99PostingsReader(state)
	if err != nil {
		return nil, err
	}
	producer, err := NewLucene103BlockTreeTermsReader(postingsReader, state)
	if err != nil {
		_ = postingsReader.Close()
		return nil, err
	}
	return producer, nil
}

// Compile-time interface check.
var _ PostingsFormat = (*Lucene99PostingsFormat)(nil)
