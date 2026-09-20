// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// NormsFormat is an alias of [spi.NormsFormat] — the canonical norms
// (.nvd / .nvm) format accessor lifted onto the SPI by rmp #120, exactly
// as the doc-values family was lifted by rmp #4708. The codecs-package
// name is now identical to spi.NormsFormat at the type-system level, so
// every implementation here (Lucene90NormsFormat, CompressingNormsFormat,
// BaseNormsFormat) continues to satisfy it without any adapter, and the
// index-side flush/read/merge paths can name it via index.NormsFormat
// (itself an alias of spi.NormsFormat).
//
// Field norms are per-document normalization factors used during scoring.
// They are stored as NumericDocValues internally (one value per
// value-bearing document), so the format is shaped like doc values but
// specialised for the single-byte values the default similarity emits.
type NormsFormat = spi.NormsFormat

// BaseNormsFormat provides common functionality for NormsFormat implementations.
type BaseNormsFormat struct {
	name string
}

// NewBaseNormsFormat creates a new BaseNormsFormat.
func NewBaseNormsFormat(name string) *BaseNormsFormat {
	return &BaseNormsFormat{name: name}
}

// Name returns the format name.
func (f *BaseNormsFormat) Name() string {
	return f.name
}

// NormsConsumer returns a norms consumer (must be implemented by subclasses).
func (f *BaseNormsFormat) NormsConsumer(state *SegmentWriteState) (NormsConsumer, error) {
	return nil, fmt.Errorf("NormsConsumer not implemented")
}

// NormsProducer returns a norms producer (must be implemented by subclasses).
func (f *BaseNormsFormat) NormsProducer(state *SegmentReadState) (NormsProducer, error) {
	return nil, fmt.Errorf("NormsProducer not implemented")
}

// NormsConsumer is an alias of [spi.NormsConsumer] — the per-segment
// write side of the norms pipeline. AddNormsField takes *index.FieldInfo,
// which is itself an alias of *schema.FieldInfo (the type spi.NormsConsumer
// names), so existing implementations compile unchanged under the alias.
type NormsConsumer = spi.NormsConsumer

// NormsIterator is an alias of [spi.NormsIterator] — the single-pass
// writer-side cursor the norms flush replays into
// NormsConsumer.AddNormsField.
type NormsIterator = spi.NormsIterator

// NormsWriter is a helper for writing norms.
type NormsWriter struct {
	out    store.IndexOutput
	closed bool
}

// NewNormsWriter creates a new NormsWriter.
func NewNormsWriter(out store.IndexOutput) *NormsWriter {
	return &NormsWriter{out: out}
}

// WriteHeader writes the norms file header.
func (w *NormsWriter) WriteHeader() error {
	// Write magic number (NRM = Norms)
	if err := store.WriteBEInt(w.out, 0x4E524D00); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteBEInt(w.out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Close closes the writer.
func (w *NormsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.out.Close()
}

// NormsReader is a helper for reading norms.
type NormsReader struct {
	in     store.IndexInput
	closed bool
}

// NewNormsReader creates a new NormsReader.
func NewNormsReader(in store.IndexInput) *NormsReader {
	return &NormsReader{in: in}
}

// ReadHeader reads and validates the norms file header.
func (r *NormsReader) ReadHeader() error {
	// Read magic number
	magic, err := store.ReadBEInt(r.in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x4E524D00 {
		return fmt.Errorf("invalid magic number: expected 0x4E524D00, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadBEInt(r.in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// Close closes the reader.
func (r *NormsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.in.Close()
}
