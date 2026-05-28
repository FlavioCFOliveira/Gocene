// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// This file aliases the doc-values family onto the canonical SPI
// declarations that rmp #4708 lifted into package spi/, and keeps the
// thin BaseDocValuesFormat / DocValuesWriter / DocValuesReader helpers
// the codecs package historically exposed.
//
// All eleven interfaces (DocValuesFormat, DocValuesConsumer,
// DocValuesProducer plus the six value-type interfaces and the five
// writer-side iterators) and their previous concrete bodies in this
// file were collapsed to Go type aliases of spi.* by rmp #4708
// (Sprint 118 phase 2d). Existing callers compile unchanged: an
// implementation that satisfied codecs.DocValuesProducer continues to
// satisfy it under the alias because the alias makes the codecs name
// identical to spi.DocValuesProducer at the type-system level.

// DocValuesFormat is an alias of [spi.DocValuesFormat].
type DocValuesFormat = spi.DocValuesFormat

// DocValuesConsumer is an alias of [spi.DocValuesConsumer].
type DocValuesConsumer = spi.DocValuesConsumer

// DocValuesProducer is an alias of [spi.DocValuesProducer]. The SPI
// surface carries GetSkipper, matching Apache Lucene 10.4.0; every
// production implementation in this package and in backward_codecs/
// satisfies the new method (returning (nil, nil) when the format does
// not write a sparse skipper companion).
type DocValuesProducer = spi.DocValuesProducer

// NumericDocValues is an alias of [spi.NumericDocValues] — the
// iterator-shaped surface exposed by codec doc-values producers.
type NumericDocValues = spi.NumericDocValues

// BinaryDocValues is an alias of [spi.BinaryDocValues].
type BinaryDocValues = spi.BinaryDocValues

// SortedDocValues is an alias of [spi.SortedDocValues].
type SortedDocValues = spi.SortedDocValues

// SortedSetDocValues is an alias of [spi.SortedSetDocValues].
type SortedSetDocValues = spi.SortedSetDocValues

// SortedNumericDocValues is an alias of [spi.SortedNumericDocValues].
type SortedNumericDocValues = spi.SortedNumericDocValues

// DocValuesSkipper is an alias of [spi.DocValuesSkipper].
type DocValuesSkipper = spi.DocValuesSkipper

// NumericDocValuesIterator is an alias of
// [spi.NumericDocValuesIterator] — the writer-side iterator that the
// flush path feeds into DocValuesConsumer.AddNumericField.
type NumericDocValuesIterator = spi.NumericDocValuesIterator

// BinaryDocValuesIterator is an alias of [spi.BinaryDocValuesIterator].
type BinaryDocValuesIterator = spi.BinaryDocValuesIterator

// SortedDocValuesIterator is an alias of [spi.SortedDocValuesIterator].
type SortedDocValuesIterator = spi.SortedDocValuesIterator

// SortedSetDocValuesIterator is an alias of
// [spi.SortedSetDocValuesIterator].
type SortedSetDocValuesIterator = spi.SortedSetDocValuesIterator

// SortedNumericDocValuesIterator is an alias of
// [spi.SortedNumericDocValuesIterator].
type SortedNumericDocValuesIterator = spi.SortedNumericDocValuesIterator

// BaseDocValuesFormat provides the partial DocValuesFormat
// implementation that codec ports embed to inherit the Name accessor
// and the "not implemented" placeholders for FieldsConsumer /
// FieldsProducer. Concrete formats override the latter two.
type BaseDocValuesFormat struct {
	name string
}

// NewBaseDocValuesFormat returns a BaseDocValuesFormat that reports the
// given format name.
func NewBaseDocValuesFormat(name string) *BaseDocValuesFormat {
	return &BaseDocValuesFormat{name: name}
}

// Name returns the format name persisted in segment metadata.
func (f *BaseDocValuesFormat) Name() string {
	return f.name
}

// FieldsConsumer is the default unimplemented FieldsConsumer accessor;
// concrete formats override it.
func (f *BaseDocValuesFormat) FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error) {
	return nil, fmt.Errorf("FieldsConsumer not implemented")
}

// FieldsProducer is the default unimplemented FieldsProducer accessor;
// concrete formats override it.
func (f *BaseDocValuesFormat) FieldsProducer(state *SegmentReadState) (DocValuesProducer, error) {
	return nil, fmt.Errorf("FieldsProducer not implemented")
}

// DocValuesWriter is a thin helper that prefixes a doc-values output
// stream with the historical Gocene magic-number / version envelope.
// It pre-dates the codec envelope helpers (CodecUtil) and is retained
// for the few in-tree call sites that still use it.
type DocValuesWriter struct {
	out    store.IndexOutput
	closed bool
}

// NewDocValuesWriter wraps out in a DocValuesWriter.
func NewDocValuesWriter(out store.IndexOutput) *DocValuesWriter {
	return &DocValuesWriter{out: out}
}

// WriteHeader writes the legacy magic-number / version pair.
func (w *DocValuesWriter) WriteHeader() error {
	if err := store.WriteUint32(w.out, 0x44564C00); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	if err := store.WriteUint32(w.out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Close closes the underlying output, ignoring repeated calls.
func (w *DocValuesWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.out.Close()
}

// DocValuesReader is the companion to DocValuesWriter on the read side.
type DocValuesReader struct {
	in     store.IndexInput
	closed bool
}

// NewDocValuesReader wraps in in a DocValuesReader.
func NewDocValuesReader(in store.IndexInput) *DocValuesReader {
	return &DocValuesReader{in: in}
}

// ReadHeader reads and validates the legacy magic-number / version
// pair written by [DocValuesWriter.WriteHeader].
func (r *DocValuesReader) ReadHeader() error {
	magic, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x44564C00 {
		return fmt.Errorf("invalid magic number: expected 0x44564C00, got 0x%08x", magic)
	}

	version, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// Close closes the underlying input, ignoring repeated calls.
func (r *DocValuesReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.in.Close()
}
