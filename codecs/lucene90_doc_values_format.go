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
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
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

// FieldsConsumer returns a consumer for writing doc values using the
// format's configured skip-index interval size.
func (f *Lucene90DocValuesFormat) FieldsConsumer(state *SegmentWriteState) (DocValuesConsumer, error) {
	real, err := newLucene90DVConsumer(state, f.skipIndexIntervalSize)
	if err != nil {
		return nil, err
	}
	return newLucene90DocValuesConsumerFromReal(real), nil
}

// FieldsProducer returns a producer for reading doc values. Phase 1
// validates the file framing; per-field reads return nil.
func (f *Lucene90DocValuesFormat) FieldsProducer(state *SegmentReadState) (DocValuesProducer, error) {
	return NewLucene90DocValuesProducer(state)
}

// -----------------------------------------------------------------------------
// Lucene90DocValuesConsumer — real implementation wrapper.
// -----------------------------------------------------------------------------

// Lucene90DocValuesConsumer writes doc values in Lucene 9.0 format. Go port
// of org.apache.lucene.codecs.lucene90.Lucene90DocValuesConsumer, which
// extends DocValuesConsumer and therefore inherits its merge members through
// the embedded BaseDocValuesConsumer.
//
// The byte encoding lives in lucene90DVConsumer, whose writers iterate the
// values through the internal dv*Values contracts; the Add*Field methods
// adapt the DocValuesProducer they receive to those contracts, obtaining a
// fresh iterator from the producer on every Reset, exactly where the Java
// consumer calls valuesProducer.getXxx(field) again.
type Lucene90DocValuesConsumer struct {
	*BaseDocValuesConsumer
	real *lucene90DVConsumer
}

// newLucene90DocValuesConsumerFromReal wraps real and wires the base
// DocValuesConsumer members to the new consumer.
func newLucene90DocValuesConsumerFromReal(real *lucene90DVConsumer) *Lucene90DocValuesConsumer {
	c := &Lucene90DocValuesConsumer{real: real}
	c.BaseDocValuesConsumer = NewBaseDocValuesConsumer(c)
	return c
}

// NewLucene90DocValuesConsumer creates a new Lucene90DocValuesConsumer.
func NewLucene90DocValuesConsumer(state *SegmentWriteState) *Lucene90DocValuesConsumer {
	real, err := newLucene90DVConsumer(state, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		// Fail here: the caller checks the returned consumer, but the
		// DocValuesFormat.FieldsConsumer contract says to propagate errors.
		// Since this constructor can't return an error, panic with the cause
		// so misconfiguration is caught at startup.
		panic(fmt.Sprintf("lucene90 doc values: consumer init failed: %v", err))
	}
	return newLucene90DocValuesConsumerFromReal(real)
}

// Real returns the underlying lucene90DVConsumer for direct use by tests.
func (c *Lucene90DocValuesConsumer) Real() *lucene90DVConsumer { return c.real }

// AddNumericField writes a numeric DV field from valuesProducer.GetNumeric.
// Java wraps the producer as a single-valued sorted-numeric producer
// (DocValues.singleton(valuesProducer.getNumeric(field))); the adapter
// exposes the same one-value-per-document view.
func (c *Lucene90DocValuesConsumer) AddNumericField(field *index.FieldInfo, valuesProducer DocValuesProducer) error {
	a := &numericProducerAsDV{field: field, valuesProducer: valuesProducer}
	if err := a.Reset(); err != nil {
		return err
	}
	return c.real.AddNumericField(field, a)
}

// AddBinaryField writes a binary DV field from valuesProducer.GetBinary.
func (c *Lucene90DocValuesConsumer) AddBinaryField(field *index.FieldInfo, valuesProducer DocValuesProducer) error {
	a := &binaryProducerAsDV{field: field, valuesProducer: valuesProducer}
	if err := a.Reset(); err != nil {
		return err
	}
	return c.real.AddBinaryField(field, a)
}

// AddSortedField writes a sorted DV field from valuesProducer.GetSorted.
func (c *Lucene90DocValuesConsumer) AddSortedField(field *index.FieldInfo, valuesProducer DocValuesProducer) error {
	a := &sortedReaderAsDV{reset: func() (SortedDocValues, error) { return valuesProducer.GetSorted(field) }}
	if err := a.Reset(); err != nil {
		return err
	}
	return c.real.AddSortedField(field, a)
}

// AddSortedSetField writes a sorted-set DV field from
// valuesProducer.GetSortedSet.
func (c *Lucene90DocValuesConsumer) AddSortedSetField(field *index.FieldInfo, valuesProducer DocValuesProducer) error {
	a := &sortedSetReaderAsDV{reset: func() (SortedSetDocValues, error) { return valuesProducer.GetSortedSet(field) }}
	if err := a.Reset(); err != nil {
		return err
	}
	return c.real.AddSortedSetField(field, a)
}

// AddSortedNumericField writes a sorted-numeric DV field from
// valuesProducer.GetSortedNumeric.
func (c *Lucene90DocValuesConsumer) AddSortedNumericField(field *index.FieldInfo, valuesProducer DocValuesProducer) error {
	a := &sortedNumericProducerAsDV{field: field, valuesProducer: valuesProducer}
	if err := a.Reset(); err != nil {
		return err
	}
	return c.real.AddSortedNumericField(field, a)
}

// Close finalises the .dvd and .dvm files.
func (c *Lucene90DocValuesConsumer) Close() error {
	return c.real.Close()
}

// ---------------------------------------------------------------------------
// DocValuesProducer adapters onto the internal dv*Values contracts
// ---------------------------------------------------------------------------

// numericProducerAsDV presents valuesProducer.GetNumeric(field) as a
// single-valued dvSortedNumericValues. Reset obtains a fresh iterator.
type numericProducerAsDV struct {
	field          *index.FieldInfo
	valuesProducer DocValuesProducer
	cur            NumericDocValues
}

func (a *numericProducerAsDV) Reset() error {
	values, err := a.valuesProducer.GetNumeric(a.field)
	if err != nil {
		return err
	}
	a.cur = values
	return nil
}

func (a *numericProducerAsDV) NextDoc() (int, error) {
	doc, err := a.cur.NextDoc()
	if err != nil {
		return 0, err
	}
	return normalizeDVDoc(doc), nil
}

func (a *numericProducerAsDV) DocValueCount() (int, error) { return 1, nil }

// NextValue returns the current document's value; NextDoc owns cursor
// advancement.
func (a *numericProducerAsDV) NextValue() (int64, error) { return a.cur.LongValue() }

// binaryProducerAsDV presents valuesProducer.GetBinary(field) as a
// dvBinaryValues. Reset obtains a fresh iterator.
type binaryProducerAsDV struct {
	field          *index.FieldInfo
	valuesProducer DocValuesProducer
	cur            BinaryDocValues
}

func (a *binaryProducerAsDV) Reset() error {
	values, err := a.valuesProducer.GetBinary(a.field)
	if err != nil {
		return err
	}
	a.cur = values
	return nil
}

func (a *binaryProducerAsDV) NextDoc() (int, error) {
	doc, err := a.cur.NextDoc()
	if err != nil {
		return 0, err
	}
	return normalizeDVDoc(doc), nil
}

func (a *binaryProducerAsDV) BinaryValue() ([]byte, error) { return a.cur.BinaryValue() }

// sortedNumericProducerAsDV presents valuesProducer.GetSortedNumeric(field)
// as a dvSortedNumericValues. Reset obtains a fresh iterator.
type sortedNumericProducerAsDV struct {
	field          *index.FieldInfo
	valuesProducer DocValuesProducer
	cur            SortedNumericDocValues
}

func (a *sortedNumericProducerAsDV) Reset() error {
	values, err := a.valuesProducer.GetSortedNumeric(a.field)
	if err != nil {
		return err
	}
	a.cur = values
	return nil
}

func (a *sortedNumericProducerAsDV) NextDoc() (int, error) {
	doc, err := a.cur.NextDoc()
	if err != nil {
		return 0, err
	}
	return normalizeDVDoc(doc), nil
}

func (a *sortedNumericProducerAsDV) DocValueCount() (int, error) { return a.cur.DocValueCount() }

func (a *sortedNumericProducerAsDV) NextValue() (int64, error) { return a.cur.NextValue() }

// -----------------------------------------------------------------------------
// Lucene90DocValuesProducer — real implementation.
// -----------------------------------------------------------------------------

// Lucene90DocValuesProducer reads doc values in Lucene 9.0 format.
//
// This wraps the real lucene90DVProducer and satisfies the DocValuesProducer
// interface.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90DocValuesProducer (Lucene 10.4.0).
type Lucene90DocValuesProducer struct {
	real *lucene90DVProducer
}

// NewLucene90DocValuesProducer creates a new Lucene90DocValuesProducer.
func NewLucene90DocValuesProducer(state *SegmentReadState) (*Lucene90DocValuesProducer, error) {
	real, err := newLucene90DVProducer(state)
	if err != nil {
		return nil, err
	}
	return &Lucene90DocValuesProducer{real: real}, nil
}

// GetNumeric returns the numeric DV for the field.
func (p *Lucene90DocValuesProducer) GetNumeric(field *index.FieldInfo) (NumericDocValues, error) {
	return p.real.GetNumeric(field)
}

// GetBinary returns the binary DV for the field.
func (p *Lucene90DocValuesProducer) GetBinary(field *index.FieldInfo) (BinaryDocValues, error) {
	return p.real.GetBinary(field)
}

// GetSorted returns the sorted DV for the field.
func (p *Lucene90DocValuesProducer) GetSorted(field *index.FieldInfo) (SortedDocValues, error) {
	return p.real.GetSorted(field)
}

// GetSortedSet returns the sorted-set DV for the field.
func (p *Lucene90DocValuesProducer) GetSortedSet(field *index.FieldInfo) (SortedSetDocValues, error) {
	return p.real.GetSortedSet(field)
}

// GetSortedNumeric returns the sorted-numeric DV.
func (p *Lucene90DocValuesProducer) GetSortedNumeric(field *index.FieldInfo) (SortedNumericDocValues, error) {
	return p.real.GetSortedNumeric(field)
}

// GetSkipper returns the DocValuesSkipper for the field. The Gocene
// Lucene90 doc-values producer does not yet decode the per-block
// sparse-skipper sidecar, so it always returns (nil, nil); reads that
// would benefit from skipping fall back to the dense iterator path.
//
// Required by spi.DocValuesProducer since rmp #4708 lifted the
// doc-values family onto the SPI with the Lucene-faithful method set.
func (p *Lucene90DocValuesProducer) GetSkipper(field *index.FieldInfo) (DocValuesSkipper, error) {
	return p.real.GetSkipper(field)
}

// CheckIntegrity checks the integrity of the doc values files.
// GetMergeInstance returns an instance optimized for merging.
//
// Mirrors Lucene90DocValuesProducer.getMergeInstance() of Apache Lucene 10.5.0.
func (p *Lucene90DocValuesProducer) GetMergeInstance() DocValuesProducer {
	return &Lucene90DocValuesProducer{real: p.real.GetMergeInstance()}
}

func (p *Lucene90DocValuesProducer) CheckIntegrity() error { return p.real.CheckIntegrity() }

// Close releases resources.
func (p *Lucene90DocValuesProducer) Close() error { return p.real.Close() }
