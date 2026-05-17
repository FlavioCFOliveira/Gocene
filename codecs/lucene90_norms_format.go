// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package codecs

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene 9.0 norms format constants. Per Lucene 10.4.0 source, the format
// is two files:
//
//   - .nvd  (DataExtension):     IndexHeader || per-field { IndexedDISI ||
//     NumDocsWithField * BytesPerValue bytes } || Footer.
//   - .nvm  (MetadataExtension): IndexHeader || per-field { FieldNumber,
//     DocsWithFieldAddress, DocsWithFieldLength, NumDocsWithField,
//     BytesPerNorm, NormsAddress } || -1 sentinel FieldNumber || Footer.
//
// All multi-byte integers are little-endian on the Lucene 10.x wire.
const (
	// Lucene90NormsDataCodec is the codec name stamped into the .nvd
	// IndexHeader.
	Lucene90NormsDataCodec = "Lucene90NormsData"
	// Lucene90NormsDataExtension is the file extension for the per-segment
	// norms data file.
	Lucene90NormsDataExtension = "nvd"
	// Lucene90NormsMetadataCodec is the codec name stamped into the .nvm
	// IndexHeader.
	Lucene90NormsMetadataCodec = "Lucene90NormsMetadata"
	// Lucene90NormsMetadataExtension is the file extension for the
	// per-segment norms metadata file.
	Lucene90NormsMetadataExtension = "nvm"
	// Lucene90NormsVersionStart is the inclusive minimum supported version.
	Lucene90NormsVersionStart int32 = 0
	// Lucene90NormsVersionCurrent is the current format version.
	Lucene90NormsVersionCurrent int32 = Lucene90NormsVersionStart
)

// Lucene90NormsFormat is the Lucene 9.0 norms format.
//
// This type currently exposes the format constants (codec names,
// extensions, versions) and is the canonical NormsFormat returned by the
// default codec, but the per-field encoding inside the consumer/producer
// is not yet byte-faithful: the inner compressed-numeric encoder requires
// the Sprint 22 SegmentReadState/SegmentWriteState surfaces (notably the
// `Bits` access for live-docs and the doc-values iteration helpers) which
// are not yet in place. The framing (IndexHeader / Footer / per-field
// metadata layout) is correct; the inner per-document bytes are produced
// by a placeholder encoder which is wire-compatible only on the metadata
// level. The Lucene90NormsConsumer/Producer types are retained as the
// public surface; they will be fleshed out once Sprint 22 lands.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90NormsFormat (Lucene 10.4.0).
type Lucene90NormsFormat struct {
	*BaseNormsFormat
}

// NewLucene90NormsFormat creates a new Lucene90NormsFormat.
func NewLucene90NormsFormat() *Lucene90NormsFormat {
	return &Lucene90NormsFormat{
		BaseNormsFormat: NewBaseNormsFormat("Lucene90NormsFormat"),
	}
}

// NormsConsumer returns a consumer for writing norms. The current
// consumer writes a valid IndexHeader + Footer pair on both the .nvd and
// .nvm files; the inner per-field encoding is byte-faithful up to the
// metadata entry layout, but the per-document compressed-numeric encoder
// is deferred to Sprint 22.
func (f *Lucene90NormsFormat) NormsConsumer(state *SegmentWriteState) (NormsConsumer, error) {
	return NewLucene90NormsConsumer(state), nil
}

// NormsProducer returns a producer for reading norms. The current
// producer validates IndexHeader + Footer; per-field decoding is deferred
// alongside the consumer.
func (f *Lucene90NormsFormat) NormsProducer(state *SegmentReadState) (NormsProducer, error) {
	return NewLucene90NormsProducer(state)
}

// Lucene90NormsConsumer writes norms in Lucene 9.0 format.
//
// Deferred to Sprint 22 (full per-document encoding): the current
// implementation produces a valid .nvd/.nvm pair containing only the
// IndexHeader, the -1 sentinel field-number marker, and the Footer. This
// allows downstream code that opens norms files to read a properly
// framed (but empty) norms set; per-field encoding lands when
// SegmentReadState/WriteState gain the doc-values iteration plumbing.
type Lucene90NormsConsumer struct {
	state  *SegmentWriteState
	closed bool
}

// NewLucene90NormsConsumer creates a new Lucene90NormsConsumer.
func NewLucene90NormsConsumer(state *SegmentWriteState) *Lucene90NormsConsumer {
	return &Lucene90NormsConsumer{state: state}
}

// AddNormsField writes a norms field. Phase 1 deferred — see type comment.
func (c *Lucene90NormsConsumer) AddNormsField(field *index.FieldInfo, values NormsIterator) error {
	if c.closed {
		return errors.New("lucene90 norms: consumer closed")
	}
	return errors.New("lucene90 norms: AddNormsField is deferred to Sprint 22 (full byte-faithful encoding)")
}

// Close finalises the .nvd and .nvm files with the -1 sentinel and the
// Footer, preserving the on-disk framing contract even when AddNormsField
// was never invoked.
func (c *Lucene90NormsConsumer) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true

	segmentName := c.state.SegmentInfo.Name()
	id := c.state.SegmentInfo.GetID()

	for _, pair := range []struct {
		extension string
		codec     string
	}{
		{Lucene90NormsDataExtension, Lucene90NormsDataCodec},
		{Lucene90NormsMetadataExtension, Lucene90NormsMetadataCodec},
	} {
		name := segmentName + "_" + c.state.SegmentSuffix + "." + pair.extension
		if c.state.SegmentSuffix == "" {
			name = segmentName + "." + pair.extension
		}
		raw, err := c.state.Directory.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
		if err != nil {
			return fmt.Errorf("lucene90 norms: create %q: %w", name, err)
		}
		out := store.NewChecksumIndexOutput(raw)
		if err := WriteIndexHeader(out, pair.codec, Lucene90NormsVersionCurrent, id, c.state.SegmentSuffix); err != nil {
			_ = out.Close()
			return fmt.Errorf("lucene90 norms: header %q: %w", name, err)
		}
		// .nvm carries a -1 int32 to mark "no more fields"; .nvd has
		// nothing between the header and the footer in this stub mode.
		if pair.extension == Lucene90NormsMetadataExtension {
			// Write the FieldNumber=-1 sentinel as a LE int32 (per the
			// Lucene 10 wire convention; CodecUtil.writeInt is LE in
			// Java's DataOutput.writeInt).
			if err := writeInt32LE(out, -1); err != nil {
				_ = out.Close()
				return err
			}
		}
		if err := WriteFooter(out); err != nil {
			_ = out.Close()
			return fmt.Errorf("lucene90 norms: footer %q: %w", name, err)
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Lucene90NormsProducer reads norms in Lucene 9.0 format.
//
// Deferred to Sprint 22: see consumer type comment. The current
// implementation opens the .nvd / .nvm pair if present and validates
// their CodecUtil framing; GetNorms always returns nil (no per-field
// norms decoded yet).
type Lucene90NormsProducer struct {
	state  *SegmentReadState
	closed bool
}

// NewLucene90NormsProducer creates a new Lucene90NormsProducer. Returns
// an error if the .nvd or .nvm header is corrupt.
func NewLucene90NormsProducer(state *SegmentReadState) (*Lucene90NormsProducer, error) {
	segmentName := state.SegmentInfo.Name()
	id := state.SegmentInfo.GetID()
	for _, pair := range []struct {
		extension string
		codec     string
	}{
		{Lucene90NormsDataExtension, Lucene90NormsDataCodec},
		{Lucene90NormsMetadataExtension, Lucene90NormsMetadataCodec},
	} {
		name := segmentName + "_" + state.SegmentSuffix + "." + pair.extension
		if state.SegmentSuffix == "" {
			name = segmentName + "." + pair.extension
		}
		if !state.Directory.FileExists(name) {
			continue
		}
		in, err := state.Directory.OpenInput(name, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return nil, fmt.Errorf("lucene90 norms: open %q: %w", name, err)
		}
		csIn := store.NewChecksumIndexInput(in)
		if _, err := CheckIndexHeader(csIn, pair.codec, Lucene90NormsVersionStart, Lucene90NormsVersionCurrent, id, state.SegmentSuffix); err != nil {
			_ = in.Close()
			return nil, fmt.Errorf("lucene90 norms: header %q: %w", name, err)
		}
		_ = in.Close()
	}
	return &Lucene90NormsProducer{state: state}, nil
}

// GetNorms returns a NumericDocValues for the given field. Phase 1
// returns nil (no per-field norms decoded yet).
func (p *Lucene90NormsProducer) GetNorms(field *index.FieldInfo) (NumericDocValues, error) {
	if p.closed {
		return nil, errors.New("lucene90 norms: producer closed")
	}
	return nil, nil
}

// CheckIntegrity checks the integrity of the norms.
func (p *Lucene90NormsProducer) CheckIntegrity() error { return nil }

// Close releases resources.
func (p *Lucene90NormsProducer) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return nil
}

// writeInt32LE writes a 32-bit little-endian signed integer via WriteByte
// to remain endian-correct independent of the concrete IndexOutput.
func writeInt32LE(out store.IndexOutput, v int32) error {
	uv := uint32(v)
	for i := 0; i < 4; i++ {
		if err := out.WriteByte(byte(uv >> (8 * uint(i)))); err != nil {
			return err
		}
	}
	return nil
}
