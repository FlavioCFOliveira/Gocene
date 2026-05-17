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

// Lucene 9.0 doc values format constants. Per Lucene 10.4.0 source, the
// format stores two files per segment:
//
//   - .dvd (DataExtension): per-field payloads using one of several
//     numeric / binary encodings (delta-compressed, table-compressed,
//     GCD-compressed, monotonic-compressed, const-compressed for
//     numerics; fixed-width / variable-width / prefix-compressed for
//     binary).
//   - .dvm (MetadataExtension): per-field metadata, including the type
//     byte and offsets into the data file.
const (
	// Lucene90DocValuesDataCodec is the codec name stamped into the .dvd
	// IndexHeader.
	Lucene90DocValuesDataCodec = "Lucene90DocValuesData"
	// Lucene90DocValuesDataExtension is the file extension for the .dvd
	// data file.
	Lucene90DocValuesDataExtension = "dvd"
	// Lucene90DocValuesMetaCodec is the codec name stamped into the .dvm
	// IndexHeader.
	Lucene90DocValuesMetaCodec = "Lucene90DocValuesMetadata"
	// Lucene90DocValuesMetaExtension is the file extension for the .dvm
	// metadata file.
	Lucene90DocValuesMetaExtension = "dvm"

	// Lucene90DocValuesVersionStart is the inclusive minimum supported
	// format version.
	Lucene90DocValuesVersionStart int32 = 0
	// Lucene90DocValuesVersionCurrent is the current format version.
	Lucene90DocValuesVersionCurrent int32 = Lucene90DocValuesVersionStart

	// Per-field type-byte sentinels for the .dvm entries (Lucene's
	// DocValuesType enum encoding).
	Lucene90DocValuesTypeNumeric       byte = 0
	Lucene90DocValuesTypeBinary        byte = 1
	Lucene90DocValuesTypeSorted        byte = 2
	Lucene90DocValuesTypeSortedSet     byte = 3
	Lucene90DocValuesTypeSortedNumeric byte = 4

	// Lucene90DocValuesDirectMonotonicBlockShift is the block-shift used
	// by DirectMonotonicWriter for the per-field offsets table.
	Lucene90DocValuesDirectMonotonicBlockShift = 16

	// Lucene90DocValuesNumericBlockShift is the block-shift used to split
	// large numeric fields into bitpacked blocks of 16384 values.
	Lucene90DocValuesNumericBlockShift = 14
	// Lucene90DocValuesNumericBlockSize is 1 << Lucene90DocValuesNumericBlockShift = 16384.
	Lucene90DocValuesNumericBlockSize = 1 << Lucene90DocValuesNumericBlockShift

	// Lucene90DocValuesTermsDictBlockLZ4Shift is the block-shift used by
	// the LZ4-prefix-compressed terms dictionary (binary doc-values).
	Lucene90DocValuesTermsDictBlockLZ4Shift = 6
	// Lucene90DocValuesTermsDictBlockLZ4Size is 1<<6 = 64.
	Lucene90DocValuesTermsDictBlockLZ4Size = 1 << Lucene90DocValuesTermsDictBlockLZ4Shift
	// Lucene90DocValuesTermsDictBlockLZ4Mask is Size - 1 = 63.
	Lucene90DocValuesTermsDictBlockLZ4Mask = Lucene90DocValuesTermsDictBlockLZ4Size - 1

	// Lucene90DocValuesTermsDictReverseIndexShift is the block-shift used
	// by the reverse-lookup index for the terms dictionary.
	Lucene90DocValuesTermsDictReverseIndexShift = 10
	// Lucene90DocValuesTermsDictReverseIndexSize = 1024.
	Lucene90DocValuesTermsDictReverseIndexSize = 1 << Lucene90DocValuesTermsDictReverseIndexShift
	// Lucene90DocValuesTermsDictReverseIndexMask = 1023.
	Lucene90DocValuesTermsDictReverseIndexMask = Lucene90DocValuesTermsDictReverseIndexSize - 1

	// Lucene90DocValuesDefaultSkipIndexIntervalSize is the default
	// number of documents per skip-index interval.
	Lucene90DocValuesDefaultSkipIndexIntervalSize = 4096

	// Lucene90DocValuesSkipIndexIntervalBytes is the byte size of one
	// skip-index interval entry:
	//   * 1 byte : number of levels
	//   * 16 bytes: min / max value
	//   * 8 bytes : min / max docID
	//   * 4 bytes : number of documents
	Lucene90DocValuesSkipIndexIntervalBytes int64 = 29

	// Lucene90DocValuesSkipIndexLevelShift = log2(8) = 3 (8 intervals per
	// level).
	Lucene90DocValuesSkipIndexLevelShift = 3
	// Lucene90DocValuesSkipIndexMaxLevel is the maximum number of skip
	// levels (4). Larger values trade write-time heap for read-time
	// O(1) seek time.
	Lucene90DocValuesSkipIndexMaxLevel = 4
)

// Lucene90DocValuesSkipIndexJumpLengthPerLevel returns the per-level
// jump length used by the skip index. Computed lazily in a Go
// equivalent of Lucene's static initialiser block.
//
//	level 0: SKIP_INDEX_INTERVAL_BYTES - 5
//	level n (n>=1): previous + (1 << (n*LEVEL_SHIFT)) * INTERVAL_BYTES
//	              - (1 << ((n-1) * LEVEL_SHIFT))
func Lucene90DocValuesSkipIndexJumpLengthPerLevel() [Lucene90DocValuesSkipIndexMaxLevel]int64 {
	var out [Lucene90DocValuesSkipIndexMaxLevel]int64
	out[0] = Lucene90DocValuesSkipIndexIntervalBytes - 5
	for level := 1; level < Lucene90DocValuesSkipIndexMaxLevel; level++ {
		out[level] = out[level-1]
		out[level] += int64(1<<uint(level*Lucene90DocValuesSkipIndexLevelShift)) * Lucene90DocValuesSkipIndexIntervalBytes
		out[level] -= int64(1 << uint((level-1)*Lucene90DocValuesSkipIndexLevelShift))
	}
	return out
}

// Lucene90DocValuesFormat is the Lucene 9.0 doc values format.
//
// This type currently exposes the format constants verbatim from
// Lucene 10.4.0 and produces well-framed (IndexHeader + Footer) .dvd /
// .dvm pairs. The per-field encoding/decoding bodies (the heavy 500+
// LOC of compressed-numeric and prefix-compressed-binary writers) are
// not yet wired through; the consumer/producer returned here yield
// deferred-errors when asked to add or read fields.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90DocValuesFormat (Lucene 10.4.0).
type Lucene90DocValuesFormat struct {
	*BaseDocValuesFormat
	skipIndexIntervalSize int
}

// NewLucene90DocValuesFormat creates a new Lucene90DocValuesFormat with
// the default skip-index interval size (4096 docs).
func NewLucene90DocValuesFormat() *Lucene90DocValuesFormat {
	return NewLucene90DocValuesFormatWithSkipInterval(Lucene90DocValuesDefaultSkipIndexIntervalSize)
}

// NewLucene90DocValuesFormatWithSkipInterval creates a new format with
// an explicit skip-index interval size. Must be > 1.
func NewLucene90DocValuesFormatWithSkipInterval(skipIndexIntervalSize int) *Lucene90DocValuesFormat {
	if skipIndexIntervalSize < 2 {
		// Match the Java IAE; Go callers surface this as a panic at
		// construction time so a bad config is caught at startup.
		panic(fmt.Sprintf("lucene90 doc values: skipIndexIntervalSize must be > 1, got %d", skipIndexIntervalSize))
	}
	return &Lucene90DocValuesFormat{
		BaseDocValuesFormat:   NewBaseDocValuesFormat("Lucene90"),
		skipIndexIntervalSize: skipIndexIntervalSize,
	}
}

// SkipIndexIntervalSize returns the configured skip-index interval size.
func (f *Lucene90DocValuesFormat) SkipIndexIntervalSize() int { return f.skipIndexIntervalSize }

// FieldsConsumer returns a consumer for writing doc values. Phase 1
// returns a consumer that produces valid framing only; per-field
// encoding is deferred to Sprint 22.
func (f *Lucene90DocValuesFormat) FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error) {
	return NewLucene90DocValuesConsumer(state), nil
}

// FieldsProducer returns a producer for reading doc values. Phase 1
// validates the file framing; per-field reads return nil.
func (f *Lucene90DocValuesFormat) FieldsProducer(state *SegmentReadState) (DocValuesProducer, error) {
	return NewLucene90DocValuesProducer(state)
}

// -----------------------------------------------------------------------------
// Lucene90DocValuesConsumer — Phase 1 shell.
// -----------------------------------------------------------------------------

// Lucene90DocValuesConsumer writes doc values in Lucene 9.0 format.
//
// DEFERRED to Sprint 22: per-field compressed-numeric / prefix-binary
// encoding. Phase 1 stamps a valid CodecUtil-framed .dvd / .dvm pair on
// Close (so downstream code that opens the files sees well-formed but
// empty doc values).
type Lucene90DocValuesConsumer struct {
	state  *SegmentWriteState
	closed bool
}

// NewLucene90DocValuesConsumer creates a new Lucene90DocValuesConsumer.
func NewLucene90DocValuesConsumer(state *SegmentWriteState) *Lucene90DocValuesConsumer {
	return &Lucene90DocValuesConsumer{state: state}
}

// AddNumericField writes a numeric doc values field. Deferred — see type comment.
func (c *Lucene90DocValuesConsumer) AddNumericField(field *index.FieldInfo, values NumericDocValuesIterator) error {
	if c.closed {
		return errors.New("lucene90 doc values: consumer closed")
	}
	return errors.New("lucene90 doc values: AddNumericField is deferred to Sprint 22")
}

// AddBinaryField writes a binary doc values field. Deferred.
func (c *Lucene90DocValuesConsumer) AddBinaryField(field *index.FieldInfo, values BinaryDocValuesIterator) error {
	if c.closed {
		return errors.New("lucene90 doc values: consumer closed")
	}
	return errors.New("lucene90 doc values: AddBinaryField is deferred to Sprint 22")
}

// AddSortedField writes a sorted doc values field. Deferred.
func (c *Lucene90DocValuesConsumer) AddSortedField(field *index.FieldInfo, values SortedDocValuesIterator) error {
	if c.closed {
		return errors.New("lucene90 doc values: consumer closed")
	}
	return errors.New("lucene90 doc values: AddSortedField is deferred to Sprint 22")
}

// AddSortedSetField writes a sorted-set doc values field. Deferred.
func (c *Lucene90DocValuesConsumer) AddSortedSetField(field *index.FieldInfo, values SortedSetDocValuesIterator) error {
	if c.closed {
		return errors.New("lucene90 doc values: consumer closed")
	}
	return errors.New("lucene90 doc values: AddSortedSetField is deferred to Sprint 22")
}

// AddSortedNumericField writes a sorted-numeric doc values field. Deferred.
func (c *Lucene90DocValuesConsumer) AddSortedNumericField(field *index.FieldInfo, values SortedNumericDocValuesIterator) error {
	if c.closed {
		return errors.New("lucene90 doc values: consumer closed")
	}
	return errors.New("lucene90 doc values: AddSortedNumericField is deferred to Sprint 22")
}

// Close finalises the .dvd and .dvm files with their CodecUtil
// IndexHeader and Footer (the .dvm file additionally carries a -1
// int32 sentinel to mark "no more fields").
func (c *Lucene90DocValuesConsumer) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return finaliseLucene90DocValuesFiles(c.state)
}

// -----------------------------------------------------------------------------
// Lucene90DocValuesProducer — Phase 1 shell.
// -----------------------------------------------------------------------------

// Lucene90DocValuesProducer reads doc values in Lucene 9.0 format.
//
// DEFERRED to Sprint 22: per-field decoding. Phase 1 validates the
// IndexHeader on .dvd / .dvm when present.
type Lucene90DocValuesProducer struct {
	state  *SegmentReadState
	closed bool
}

// NewLucene90DocValuesProducer creates a new Lucene90DocValuesProducer.
func NewLucene90DocValuesProducer(state *SegmentReadState) (*Lucene90DocValuesProducer, error) {
	if err := validateLucene90DocValuesFiles(state); err != nil {
		return nil, err
	}
	return &Lucene90DocValuesProducer{state: state}, nil
}

// GetNumeric returns the numeric DV for the field. Phase 1 returns nil.
func (p *Lucene90DocValuesProducer) GetNumeric(field *index.FieldInfo) (NumericDocValues, error) {
	if p.closed {
		return nil, errors.New("lucene90 doc values: producer closed")
	}
	return nil, nil
}

// GetBinary returns the binary DV for the field. Phase 1 returns nil.
func (p *Lucene90DocValuesProducer) GetBinary(field *index.FieldInfo) (BinaryDocValues, error) {
	if p.closed {
		return nil, errors.New("lucene90 doc values: producer closed")
	}
	return nil, nil
}

// GetSorted returns the sorted DV for the field. Phase 1 returns nil.
func (p *Lucene90DocValuesProducer) GetSorted(field *index.FieldInfo) (SortedDocValues, error) {
	if p.closed {
		return nil, errors.New("lucene90 doc values: producer closed")
	}
	return nil, nil
}

// GetSortedSet returns the sorted-set DV for the field. Phase 1 returns nil.
func (p *Lucene90DocValuesProducer) GetSortedSet(field *index.FieldInfo) (SortedSetDocValues, error) {
	if p.closed {
		return nil, errors.New("lucene90 doc values: producer closed")
	}
	return nil, nil
}

// GetSortedNumeric returns the sorted-numeric DV. Phase 1 returns nil.
func (p *Lucene90DocValuesProducer) GetSortedNumeric(field *index.FieldInfo) (SortedNumericDocValues, error) {
	if p.closed {
		return nil, errors.New("lucene90 doc values: producer closed")
	}
	return nil, nil
}

// CheckIntegrity checks the integrity of the doc values. Phase 1 no-op.
func (p *Lucene90DocValuesProducer) CheckIntegrity() error { return nil }

// Close releases resources.
func (p *Lucene90DocValuesProducer) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return nil
}

// -----------------------------------------------------------------------------
// Shared file-framing helpers.
// -----------------------------------------------------------------------------

// finaliseLucene90DocValuesFiles stamps the CodecUtil IndexHeader and
// Footer onto each of the two .dvd / .dvm files; the .dvm additionally
// records the -1 int32 sentinel marking "no more fields".
func finaliseLucene90DocValuesFiles(state *SegmentWriteState) error {
	seg := state.SegmentInfo.Name()
	suffix := state.SegmentSuffix
	id := state.SegmentInfo.GetID()

	pairs := []struct {
		ext   string
		codec string
		isMeta bool
	}{
		{Lucene90DocValuesDataExtension, Lucene90DocValuesDataCodec, false},
		{Lucene90DocValuesMetaExtension, Lucene90DocValuesMetaCodec, true},
	}
	for _, p := range pairs {
		name := seg + "." + p.ext
		if suffix != "" {
			name = seg + "_" + suffix + "." + p.ext
		}
		raw, err := state.Directory.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
		if err != nil {
			return fmt.Errorf("lucene90 doc values: create %q: %w", name, err)
		}
		out := store.NewChecksumIndexOutput(raw)
		if err := WriteIndexHeader(out, p.codec, Lucene90DocValuesVersionCurrent, id, suffix); err != nil {
			_ = out.Close()
			return fmt.Errorf("lucene90 doc values: header %q: %w", name, err)
		}
		if p.isMeta {
			if err := writeInt32LE(out, -1); err != nil {
				_ = out.Close()
				return err
			}
		}
		if err := WriteFooter(out); err != nil {
			_ = out.Close()
			return fmt.Errorf("lucene90 doc values: footer %q: %w", name, err)
		}
		if err := out.Close(); err != nil {
			return err
		}
	}
	return nil
}

// validateLucene90DocValuesFiles iterates the two known doc-values
// files and validates each one's IndexHeader when present.
func validateLucene90DocValuesFiles(state *SegmentReadState) error {
	seg := state.SegmentInfo.Name()
	suffix := state.SegmentSuffix
	id := state.SegmentInfo.GetID()

	pairs := []struct{ ext, codec string }{
		{Lucene90DocValuesDataExtension, Lucene90DocValuesDataCodec},
		{Lucene90DocValuesMetaExtension, Lucene90DocValuesMetaCodec},
	}
	for _, p := range pairs {
		name := seg + "." + p.ext
		if suffix != "" {
			name = seg + "_" + suffix + "." + p.ext
		}
		if !state.Directory.FileExists(name) {
			continue
		}
		in, err := state.Directory.OpenInput(name, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return err
		}
		csIn := store.NewChecksumIndexInput(in)
		if _, err := CheckIndexHeader(csIn, p.codec, Lucene90DocValuesVersionStart, Lucene90DocValuesVersionCurrent, id, suffix); err != nil {
			_ = in.Close()
			return fmt.Errorf("lucene90 doc values: header %q: %w", name, err)
		}
		_ = in.Close()
	}
	return nil
}
