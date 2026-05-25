// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene103PostingsFormat is the canonical postings format for the
// Lucene 10.3 wire layout. It pairs Lucene103BlockTreeTermsWriter /
// Lucene103BlockTreeTermsReader (the term-dictionary side) with the
// Lucene103PostingsWriter / Lucene103PostingsReader pair (the per-document
// / per-position side).
//
// This is the Go port of
// org.apache.lucene.codecs.lucene103.Lucene103PostingsFormat from Apache
// Lucene 10.4.0.
//
// The type surface (NewLucene103PostingsFormat, FieldsConsumer/Producer,
// the PostingsWriter/Reader Base implementations) is in place so that
// dependent packages (e.g. Lucene104Codec, custom codecs that swap the
// per-field PostingsFormat) can reference it. The deep byte-format
// behaviour of Lucene103PostingsWriter/Reader (the PFOR delta-encoded
// .doc/.pos/.pay layout) is still pending in a follow-up sprint; until
// then FieldsConsumer/Producer return a clear error directing callers to
// the typed-stub Writer/Reader for SPI experimentation.
type Lucene103PostingsFormat struct {
	*BasePostingsFormat

	minTermBlockSize int
	maxTermBlockSize int
}

// Lucene103PostingsFormatName mirrors the Java codec name embedded in the
// segment metadata.
const Lucene103PostingsFormatName = "Lucene103PostingsFormat"

// NewLucene103PostingsFormat creates a Lucene103PostingsFormat using the
// default block sizes from Lucene103BlockTreeTermsWriter
// (Lucene103DefaultMinBlockSize / Lucene103DefaultMaxBlockSize).
func NewLucene103PostingsFormat() *Lucene103PostingsFormat {
	return NewLucene103PostingsFormatWithBlockSizes(Lucene103DefaultMinBlockSize, Lucene103DefaultMaxBlockSize)
}

// NewLucene103PostingsFormatWithBlockSizes creates a Lucene103PostingsFormat
// pinned to a specific term-block size pair. The sizes are validated via
// ValidateLucene103BlockTreeBlockSizes by Lucene103BlockTreeTermsWriter at
// FieldsConsumer time.
func NewLucene103PostingsFormatWithBlockSizes(minBlock, maxBlock int) *Lucene103PostingsFormat {
	return &Lucene103PostingsFormat{
		BasePostingsFormat: NewBasePostingsFormat(Lucene103PostingsFormatName),
		minTermBlockSize:   minBlock,
		maxTermBlockSize:   maxBlock,
	}
}

// MinTermBlockSize returns the configured minimum number of items per
// term block.
func (f *Lucene103PostingsFormat) MinTermBlockSize() int { return f.minTermBlockSize }

// MaxTermBlockSize returns the configured maximum number of items per
// term block.
func (f *Lucene103PostingsFormat) MaxTermBlockSize() int { return f.maxTermBlockSize }

// FieldsConsumer wires a Lucene103PostingsWriter into a
// Lucene103BlockTreeTermsWriter. The Lucene103PostingsWriter is a typed
// stub today, so FieldsConsumer returns an explicit error pointing at the
// follow-up deep port instead of producing un-readable segment files.
func (f *Lucene103PostingsFormat) FieldsConsumer(state *SegmentWriteState) (FieldsConsumer, error) {
	return nil, errors.New("Lucene103PostingsFormat.FieldsConsumer: behaviour-level port pending — Lucene103PostingsWriter is a typed stub; use the Lucene104PostingsFormat shim for round-trip data and revisit once the deep byte-format port lands")
}

// FieldsProducer wires a Lucene103PostingsReader into a
// Lucene103BlockTreeTermsReader. As with FieldsConsumer the reader side
// is a typed stub today.
func (f *Lucene103PostingsFormat) FieldsProducer(state *SegmentReadState) (FieldsProducer, error) {
	return nil, errors.New("Lucene103PostingsFormat.FieldsProducer: behaviour-level port pending — Lucene103PostingsReader is a typed stub; use the Lucene104PostingsFormat shim for round-trip data and revisit once the deep byte-format port lands")
}

// Lucene103PostingsWriter is the typed stub for
// org.apache.lucene.codecs.lucene103.Lucene103PostingsWriter — the
// per-document / per-position write-side SPI implementation paired with
// Lucene103BlockTreeTermsWriter.
//
// The struct implements PostingsWriterBase so callers can already wire
// it through codec composition; the methods are intentionally no-op
// stubs that satisfy the contract without emitting bytes. The full PFOR
// delta-encoded layout (.doc, .pos, .pay) is deferred to a dedicated
// behavioural-port sprint.
type Lucene103PostingsWriter struct {
	state *SegmentWriteState
}

// NewLucene103PostingsWriter creates a Lucene103PostingsWriter bound to
// the given segment write state.
func NewLucene103PostingsWriter(state *SegmentWriteState) *Lucene103PostingsWriter {
	return &Lucene103PostingsWriter{state: state}
}

// Init satisfies PostingsWriterBase. The stub keeps the contract but
// performs no I/O; the Java reference emits a CodecUtil.writeIndexHeader
// here.
func (w *Lucene103PostingsWriter) Init(termsOut store.IndexOutput, state *SegmentWriteState) error {
	return nil
}

// NewTermState satisfies PostingsWriterBase. It returns a fresh
// BlockTermState since the Lucene103-specific subclass adds no exported
// fields used by callers today.
func (w *Lucene103PostingsWriter) NewTermState() *BlockTermState {
	return NewBlockTermState()
}

// SetField satisfies PostingsWriterBase. The Java reference inspects the
// field's IndexOptions to choose between docs-only / docs-and-freqs /
// docs-and-freqs-and-positions encoders; the stub returns 0 (no extra
// metadata bytes) and lets the deep port refine this later.
func (w *Lucene103PostingsWriter) SetField(fieldInfo *index.FieldInfo) (int, error) {
	return 0, nil
}

// StartTerm satisfies PostingsWriterBase. The stub records nothing.
func (w *Lucene103PostingsWriter) StartTerm(norms index.NumericDocValues) error {
	return nil
}

// FinishTerm satisfies PostingsWriterBase. The stub leaves the
// BlockTermState untouched.
func (w *Lucene103PostingsWriter) FinishTerm(state *BlockTermState) error {
	return nil
}

// EncodeTerm satisfies PostingsWriterBase. The stub emits no bytes so
// the resulting term-dictionary metadata is empty; this is sufficient
// for the API surface to compile but cannot round-trip until the deep
// port lands.
func (w *Lucene103PostingsWriter) EncodeTerm(out store.IndexOutput, fieldInfo *index.FieldInfo, state *BlockTermState, absolute bool) error {
	return nil
}

// Close satisfies PostingsWriterBase. The stub releases nothing because
// it owns no file handles.
func (w *Lucene103PostingsWriter) Close() error {
	return nil
}

// Lucene103PostingsReader is the typed stub for
// org.apache.lucene.codecs.lucene103.Lucene103PostingsReader — the
// per-document / per-position read-side SPI implementation paired with
// Lucene103BlockTreeTermsReader.
//
// The struct implements PostingsReaderBase so callers can already wire
// it through codec composition; the methods are typed-stub
// implementations that satisfy the contract without decoding bytes.
type Lucene103PostingsReader struct {
	state *SegmentReadState
}

// NewLucene103PostingsReader creates a Lucene103PostingsReader bound to
// the given segment read state.
func NewLucene103PostingsReader(state *SegmentReadState) *Lucene103PostingsReader {
	return &Lucene103PostingsReader{state: state}
}

// Init satisfies PostingsReaderBase. The Java reference reads a
// CodecUtil.checkIndexHeader; the stub performs no I/O.
func (r *Lucene103PostingsReader) Init(termsIn store.IndexInput, state *SegmentReadState) error {
	return nil
}

// NewTermState satisfies PostingsReaderBase. It returns a fresh
// BlockTermState (see Lucene103PostingsWriter.NewTermState).
func (r *Lucene103PostingsReader) NewTermState() *BlockTermState {
	return NewBlockTermState()
}

// DecodeTerm satisfies PostingsReaderBase. The stub leaves termState
// untouched, matching the no-op EncodeTerm on the writer side.
func (r *Lucene103PostingsReader) DecodeTerm(in store.DataInput, fieldInfo *index.FieldInfo, termState *BlockTermState, absolute bool) error {
	return nil
}

// Postings satisfies PostingsReaderBase. The stub returns a clear error
// because there is no per-document data to enumerate until the deep
// port lands.
func (r *Lucene103PostingsReader) Postings(fieldInfo *index.FieldInfo, termState *BlockTermState, reuse index.PostingsEnum, flags int) (index.PostingsEnum, error) {
	return nil, errors.New("Lucene103PostingsReader.Postings: behaviour-level port pending")
}

// Impacts satisfies PostingsReaderBase. The stub returns a clear error
// because there is no impact data to enumerate until the deep port
// lands.
func (r *Lucene103PostingsReader) Impacts(fieldInfo *index.FieldInfo, termState *BlockTermState, flags int) (index.ImpactsEnum, error) {
	return nil, errors.New("Lucene103PostingsReader.Impacts: behaviour-level port pending")
}

// CheckIntegrity satisfies PostingsReaderBase. The stub owns no files
// and therefore has nothing to validate.
func (r *Lucene103PostingsReader) CheckIntegrity() error { return nil }

// Close satisfies PostingsReaderBase. The stub owns no file handles.
func (r *Lucene103PostingsReader) Close() error { return nil }

// Compile-time interface checks.
var (
	_ PostingsFormat     = (*Lucene103PostingsFormat)(nil)
	_ PostingsWriterBase = (*Lucene103PostingsWriter)(nil)
	_ PostingsReaderBase = (*Lucene103PostingsReader)(nil)
)
